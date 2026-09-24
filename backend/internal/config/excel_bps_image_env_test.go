//go:build unit

package config

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestLoadExcelBPSImageRelayFromEnv(t *testing.T) {
	resetViperWithJWTSecret(t)
	t.Setenv("GATEWAY_EXCEL_BPS_IMAGE_BASE_URL", "https://images.example")
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "https://images.example", cfg.Gateway.ExcelBPSImageBaseURL)
}
