//go:build unit

package config

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadPluginTrustedPublishersFromEnvironment(t *testing.T) {
	resetViperWithJWTSecret(t)
	configuredKey := encodedTestPublisherKey(1)
	environmentKey := encodedTestPublisherKey(2)
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	configYAML := fmt.Sprintf("plugins:\n  trusted_publishers:\n    configured: %q\n", configuredKey)
	require.NoError(t, os.WriteFile(configFile, []byte(configYAML), 0o600))
	t.Setenv("CONFIG_FILE", configFile)
	t.Setenv(pluginTrustedPublishersJSONEnv, `{"configured":"`+environmentKey+`"}`)

	cfg, err := Load()

	require.NoError(t, err)
	require.NotEqual(t, configuredKey, environmentKey)
	require.Equal(t, environmentKey, cfg.Plugins.TrustedPublishers["configured"])
}

func TestLoadPluginTrustedPublishersRejectsInvalidEnvironment(t *testing.T) {
	tests := map[string]string{
		"invalid JSON":       `{`,
		"non-object JSON":    `null`,
		"blank key ID":       `{" ":"MGeztZt8xeQAikuzJG9+fdADZmioulH5Ls2Baom1ATA="}`,
		"invalid public key": `{"publisher":"not-base64"}`,
	}

	for name, value := range tests {
		t.Run(name, func(t *testing.T) {
			resetViperWithJWTSecret(t)
			t.Setenv(pluginTrustedPublishersJSONEnv, value)

			_, err := Load()

			require.ErrorContains(t, err, pluginTrustedPublishersJSONEnv)
		})
	}
}

func encodedTestPublisherKey(value byte) string {
	return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{value}, 32))
}
