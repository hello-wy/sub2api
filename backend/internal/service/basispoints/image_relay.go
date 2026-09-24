package basispoints

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "golang.org/x/image/webp"
)

var ErrImageRelayFull = errors.New("basispoints temporary image storage is full; retry after images expire")

const (
	ImageRelayPath             = "/api/bps-images/"
	imageRelayTTL              = 30 * time.Minute
	imageRelayMaxBytes         = 128 << 20
	imageRelayMaxEntries       = 512
	imageRelayMaxImageBytes    = 20 << 20
	imageRelayMaxRequestBytes  = 32 << 20
	imageRelayMaxRequestImages = 20
	imageRelayMaxPixels        = 64 * 1024 * 1024
)

// ImageRelay temporarily hosts inline images for the HTTPS-only BPS endpoint.
// Uploads only occur inside authenticated gateway requests. The unguessable URL
// is a bearer capability; callers must never log it or cache the response.
// Storage is process-local, bounded, and automatically expires after inactivity.
type ImageRelay struct {
	baseURL string
	key     [32]byte
	mu      sync.Mutex
	entries map[string]*relayImage
	bytes   int
}

type relayImage struct {
	data        []byte
	contentType string
	expires     time.Time
}

func ValidateImageRelayOrigin(baseURL string) error {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Opaque != "" || parsed.RawQuery != "" || parsed.ForceQuery || strings.Contains(baseURL, "#") || (parsed.Path != "" && parsed.Path != "/") {
		return fmt.Errorf("BPS image relay requires an HTTPS public origin without credentials, path, query or fragment")
	}
	return nil
}

func NewImageRelay(baseURL string) (*ImageRelay, error) {
	if err := ValidateImageRelayOrigin(baseURL); err != nil {
		return nil, err
	}
	relay := &ImageRelay{baseURL: strings.TrimRight(baseURL, "/"), entries: make(map[string]*relayImage)}
	if _, err := rand.Read(relay.key[:]); err != nil {
		return nil, fmt.Errorf("initialize BPS image relay")
	}
	return relay, nil
}

// SetPublicOrigin changes new links without discarding in-flight images.
func (r *ImageRelay) SetPublicOrigin(baseURL string) error {
	if err := ValidateImageRelayOrigin(baseURL); err != nil {
		return err
	}
	r.mu.Lock()
	r.baseURL = strings.TrimRight(baseURL, "/")
	r.mu.Unlock()
	return nil
}

// Rewrite covers user/assistant content and function/custom tool image results.
// Decode with UseNumber so unrelated tool arguments retain integer precision.
// Validate the entire image batch before publishing any bytes.
func (r *ImageRelay) Rewrite(raw []byte, scope string) ([]byte, error) {
	if r == nil {
		return raw, nil
	}
	r.mu.Lock()
	baseURL := r.baseURL
	r.mu.Unlock()
	var source object
	if err := decode(raw, &source); err != nil || source == nil {
		return nil, fmt.Errorf("invalid Basispoints request JSON")
	}
	input, _ := source["input"].([]any)
	pending := 0
	images := make(map[string]*relayImage)
	totalBytes := 0
	for _, rawItem := range input {
		item, _ := rawItem.(object)
		for _, field := range []string{"content", "output"} {
			if field == "output" && text(item["type"]) != "function_call_output" && text(item["type"]) != "custom_tool_call_output" {
				continue
			}
			parts, _ := item[field].([]any)
			for _, rawPart := range parts {
				part, _ := rawPart.(object)
				if text(part["type"]) != "input_image" {
					continue
				}
				rawURL := text(part["image_url"])
				if len(rawURL) < len("data:") || !strings.EqualFold(rawURL[:len("data:")], "data:") {
					continue
				}
				if pending >= imageRelayMaxRequestImages {
					return nil, fmt.Errorf("basispoints accepts at most 20 inline images per request")
				}
				data, contentType, err := decodeRelayImage(rawURL)
				if err != nil {
					return nil, err
				}
				totalBytes += len(data)
				if totalBytes > imageRelayMaxRequestBytes {
					return nil, fmt.Errorf("basispoints inline images exceed the 32 MiB request limit")
				}
				mac := hmac.New(sha256.New, r.key[:])
				_, _ = mac.Write([]byte(scope))
				_, _ = mac.Write([]byte{0})
				_, _ = mac.Write(data)
				token := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
				part["image_url"] = baseURL + ImageRelayPath + token
				if err := validateImage(part); err != nil {
					return nil, err
				}
				images[token] = &relayImage{data: data, contentType: contentType}
				pending++
			}
		}
	}
	if pending == 0 {
		return raw, nil
	}
	out, err := json.Marshal(source)
	if err != nil {
		return nil, fmt.Errorf("encode basispoints image request")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	r.pruneLocked(now)
	newBytes, newEntries := 0, 0
	for token, img := range images {
		if r.entries[token] == nil {
			newBytes += len(img.data)
			newEntries++
		}
	}
	if r.bytes+newBytes > imageRelayMaxBytes || len(r.entries)+newEntries > imageRelayMaxEntries {
		return nil, ErrImageRelayFull
	}
	for token, img := range images {
		if existing := r.entries[token]; existing != nil {
			existing.expires = now.Add(imageRelayTTL)
			continue
		}
		img.expires = now.Add(imageRelayTTL)
		r.entries[token] = img
		r.bytes += len(img.data)
		time.AfterFunc(imageRelayTTL, func() { r.expire(token, img) })
	}
	return out, nil
}

func decodeRelayImage(raw string) ([]byte, string, error) {
	header, payload, ok := strings.Cut(raw[len("data:"):], ",")
	if !ok || !strings.HasSuffix(strings.ToLower(header), ";base64") {
		return nil, "", fmt.Errorf("basispoints inline image requires a base64 image data URL")
	}
	declared, params, err := mime.ParseMediaType(header[:len(header)-len(";base64")])
	if err != nil || len(params) != 0 {
		return nil, "", fmt.Errorf("basispoints inline image has an invalid media type")
	}
	switch declared {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
	default:
		return nil, "", fmt.Errorf("basispoints inline images must be PNG, JPEG, GIF or WebP")
	}
	if len(payload) > base64.StdEncoding.EncodedLen(imageRelayMaxImageBytes) {
		return nil, "", fmt.Errorf("basispoints inline image exceeds the 20 MiB limit")
	}
	data, err := io.ReadAll(io.LimitReader(base64.NewDecoder(base64.StdEncoding, strings.NewReader(payload)), imageRelayMaxImageBytes+1))
	if err != nil || len(data) == 0 {
		return nil, "", fmt.Errorf("basispoints inline image contains invalid base64 data")
	}
	if len(data) > imageRelayMaxImageBytes {
		return nil, "", fmt.Errorf("basispoints inline image exceeds the 20 MiB limit")
	}
	dimensions, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || dimensions.Width <= 0 || dimensions.Height <= 0 || int64(dimensions.Width)*int64(dimensions.Height) > imageRelayMaxPixels {
		return nil, "", fmt.Errorf("basispoints inline image is invalid or exceeds 64 megapixels")
	}
	contentType := "image/" + format
	if contentType != declared {
		return nil, "", fmt.Errorf("basispoints inline image media type does not match its contents")
	}
	return data, contentType, nil
}

func (r *ImageRelay) pruneLocked(now time.Time) {
	for token, img := range r.entries {
		if !now.Before(img.expires) {
			r.bytes -= len(img.data)
			delete(r.entries, token)
		}
	}
}

func (r *ImageRelay) expire(token string, expected *relayImage) {
	r.mu.Lock()
	defer r.mu.Unlock()
	img := r.entries[token]
	if img == nil || img != expected {
		return
	}
	if remaining := time.Until(img.expires); remaining > 0 {
		time.AfterFunc(remaining, func() { r.expire(token, img) })
		return
	}
	r.bytes -= len(img.data)
	delete(r.entries, token)
}

// ServeHTTP intentionally has no API-key authentication: BPS fetches the URL
// itself. Only exact random tokens resolve; directory listing is impossible.
func (r *ImageRelay) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	token, ok := strings.CutPrefix(req.URL.Path, ImageRelayPath)
	if r == nil || !ok || len(token) != 43 {
		http.NotFound(w, req)
		return
	}
	r.mu.Lock()
	img := r.entries[token]
	if img != nil && !time.Now().Before(img.expires) {
		r.bytes -= len(img.data)
		delete(r.entries, token)
		img = nil
	}
	r.mu.Unlock()
	if img == nil {
		http.NotFound(w, req)
		return
	}
	w.Header().Set("Content-Type", img.contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(img.data)))
	w.WriteHeader(http.StatusOK)
	if req.Method == http.MethodGet {
		_, _ = w.Write(img.data)
	}
}
