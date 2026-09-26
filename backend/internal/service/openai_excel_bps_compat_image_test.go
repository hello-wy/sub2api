package service

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service/basispoints"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func TestExcelBPSCompatImagesUseSharedRelay(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	var imageBytes bytes.Buffer
	require.NoError(t, png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 2, 3))))
	encoded := base64.StdEncoding.EncodeToString(imageBytes.Bytes())
	for _, kind := range []string{"chat/completions", "messages"} {
		for _, enabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/enabled=%t", kind, enabled), func(t *testing.T) {
				upstream := excelBPSCompatUpstream(excelBPSCompatWire("image received"))
				svc := openAIClientToolsTestService(upstream)
				t.Cleanup(func() { require.NoError(t, svc.CloseExcelBPSImages()) })
				router := gin.New()
				router.GET(basispoints.ImageRelayPath+":token", svc.ServeExcelBPSImage)
				server := httptest.NewTLSServer(router)
				defer server.Close()
				if enabled {
					svc.settingService = NewSettingService(&excelBPSImageSettingsRepo{values: map[string]string{SettingKeyExcelBPSImageRelayEnabled: "true", SettingKeyExcelBPSImageBaseURL: server.URL}}, svc.cfg)
				}
				imagePart := map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64," + encoded, "detail": "high"}}
				if kind == "messages" {
					imagePart = map[string]any{"type": "image", "source": map[string]any{"type": "base64", "media_type": "image/png", "data": encoded}}
				}
				body, err := sjson.SetBytes(excelBPSCompatBody(t, kind, false, "high"), "messages.0.content", []any{map[string]any{"type": "text", "text": "describe"}, imagePart})
				require.NoError(t, err)
				_, rec, err := forwardExcelBPSCompatTest(t, svc, excelAccount(), kind, body, "")
				if !enabled {
					require.Error(t, err)
					require.Empty(t, upstream.requests)
					require.Equal(t, 400, rec.Code)
					require.NotContains(t, rec.Body.String(), encoded)
					return
				}
				require.NoError(t, err)
				require.Contains(t, rec.Body.String(), "image received")
				require.Equal(t, "basispoints", rec.Header().Get("X-Codex2API-Upstream"))
				require.NotContains(t, string(upstream.lastBody), "data:image")
				var imageURL string
				for _, item := range gjson.GetBytes(upstream.lastBody, "input").Array() {
					for _, part := range item.Get("content").Array() {
						if part.Get("type").String() == "input_image" {
							imageURL = part.Get("image_url").String()
						}
					}
				}
				require.True(t, strings.HasPrefix(imageURL, server.URL+basispoints.ImageRelayPath), imageURL)
				response, err := server.Client().Get(imageURL)
				require.NoError(t, err)
				defer func() { _ = response.Body.Close() }()
				data, err := io.ReadAll(response.Body)
				require.NoError(t, err)
				require.Equal(t, imageBytes.Bytes(), data)
			})
		}
	}
}
