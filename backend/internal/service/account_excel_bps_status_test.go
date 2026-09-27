package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccountExcelBPSStatus(t *testing.T) {
	parentID := int64(12)
	for _, tc := range []struct {
		name    string
		account *Account
		want    string
	}{
		{"nil account", nil, ""},
		{"unsupported platform", &Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth}, ""},
		{"API key", &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, ""},
		{"shadow account", &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, ParentAccountID: &parentID}, ""},
		{"agent identity", &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"auth_mode": OpenAIAuthModeAgentIdentity}}, ""},
		{"personal access token", &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"auth_mode": OpenAIAuthModePersonalAccessToken}}, ""},
		{"switch missing", &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}, "disabled"},
		{"opt-in is not evidence of 403", &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{"openai_excel_bps": false, "openai_excel_bps_auto_disable_on_403": true}}, "disabled"},
		{"recorded 403", &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{"openai_excel_bps": false, ExcelBPSDisabledReasonExtraKey: ExcelBPSDisabledReasonHTTP403}}, "disabled_403"},
		{"enabled supersedes old record", &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{"openai_excel_bps": true, ExcelBPSDisabledReasonExtraKey: ExcelBPSDisabledReasonHTTP403}}, "enabled"},
		{"scope and account health do not change switch", &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusError, Extra: map[string]any{"openai_excel_bps": true, "openai_excel_bps_models": []string{}}}, "enabled"},
		{"unrecognized reason", &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{ExcelBPSDisabledReasonExtraKey: "timeout"}}, "disabled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.account.ExcelBPSStatus())
		})
	}
}
