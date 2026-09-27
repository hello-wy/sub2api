package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAccountListItemCarriesExcelBPSStatus(t *testing.T) {
	for _, tc := range []struct {
		name  string
		extra map[string]any
		want  string
	}{
		{"enabled", map[string]any{"openai_excel_bps": true}, "enabled"},
		{"disabled", nil, "disabled"},
		{"automatic 403", map[string]any{
			"openai_excel_bps":                     false,
			service.ExcelBPSDisabledReasonExtraKey: service.ExcelBPSDisabledReasonHTTP403,
			service.ExcelBPSDisabledAtExtraKey:     "2026-09-27T01:02:03Z",
		}, "disabled_403"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := &service.Account{
				ID: 18, Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Extra: tc.extra,
			}
			full := AccountFromServiceShallow(account)
			require.Equal(t, tc.want, full.ExcelBPSStatus)
			lite := AccountListItemFromAccount(full)
			require.Equal(t, full.ExcelBPSStatus, lite.ExcelBPSStatus)
			require.Equal(t, tc.extra, lite.Extra)
			raw, err := json.Marshal(lite)
			require.NoError(t, err)
			require.Contains(t, string(raw), `"excel_bps_status":"`+tc.want+`"`)
		})
	}
}

func TestAccountListItemOmitsUnsupportedExcelBPSStatus(t *testing.T) {
	item := AccountListItemFromAccount(AccountFromServiceShallow(&service.Account{
		ID: 19, Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth,
	}))
	raw, err := json.Marshal(item)
	require.NoError(t, err)
	require.NotContains(t, string(raw), `"excel_bps_status"`)
}
