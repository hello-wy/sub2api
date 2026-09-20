package config

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const pluginTrustedPublishersJSONEnv = "PLUGINS_TRUSTED_PUBLISHERS_JSON"

func applyPluginTrustedPublishersEnv(cfg *PluginConfig) error {
	raw, configured := os.LookupEnv(pluginTrustedPublishersJSONEnv)
	if !configured || strings.TrimSpace(raw) == "" {
		return nil
	}
	publishers, err := parsePluginTrustedPublishersJSON(raw)
	if err != nil {
		return err
	}
	if cfg.TrustedPublishers == nil {
		cfg.TrustedPublishers = make(map[string]string, len(publishers))
	}
	for keyID, publicKey := range publishers {
		cfg.TrustedPublishers[keyID] = publicKey
	}
	return nil
}

func parsePluginTrustedPublishersJSON(raw string) (map[string]string, error) {
	var publishers map[string]string
	if err := json.Unmarshal([]byte(raw), &publishers); err != nil {
		return nil, fmt.Errorf("%s must be a JSON object: %w", pluginTrustedPublishersJSONEnv, err)
	}
	if publishers == nil {
		return nil, fmt.Errorf("%s must be a JSON object", pluginTrustedPublishersJSONEnv)
	}
	for keyID, encodedKey := range publishers {
		if err := validatePluginPublisherEnvEntry(keyID, encodedKey); err != nil {
			return nil, err
		}
	}
	return publishers, nil
}

func validatePluginPublisherEnvEntry(keyID, encodedKey string) error {
	if keyID == "" || strings.TrimSpace(keyID) != keyID {
		return fmt.Errorf("%s contains an invalid publisher key ID", pluginTrustedPublishersJSONEnv)
	}
	if encodedKey == "" || strings.TrimSpace(encodedKey) != encodedKey {
		return fmt.Errorf("%s contains an invalid public key for %q", pluginTrustedPublishersJSONEnv, keyID)
	}
	publicKey, err := base64.StdEncoding.Strict().DecodeString(encodedKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("%s contains an invalid Ed25519 public key for %q", pluginTrustedPublishersJSONEnv, keyID)
	}
	return nil
}
