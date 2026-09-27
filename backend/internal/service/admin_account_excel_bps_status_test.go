//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdateAccountExcelBPSStatusLifecycle(t *testing.T) {
	const disabledAt = "2026-09-27T01:02:03Z"
	repo := &updateAccountOveragesRepoStub{account: &Account{
		ID: 18, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive,
		Extra: map[string]any{
			"openai_excel_bps":             false,
			ExcelBPSDisabledReasonExtraKey: ExcelBPSDisabledReasonHTTP403,
			ExcelBPSDisabledAtExtraKey:     disabledAt,
		},
	}}
	svc := &adminServiceImpl{accountRepo: repo}
	for _, tc := range []struct {
		name  string
		extra map[string]any
		want  string
	}{
		{"ordinary edit preserves server record", map[string]any{"openai_passthrough": true}, "disabled_403"},
		{"client cannot overwrite server timestamp", map[string]any{ExcelBPSDisabledAtExtraKey: "forged", ExcelBPSDisabledReasonExtraKey: "forged"}, "disabled_403"},
		{"explicit manual closure clears record", map[string]any{"openai_excel_bps": false}, "disabled"},
		{"client cannot forge 403", map[string]any{ExcelBPSDisabledReasonExtraKey: ExcelBPSDisabledReasonHTTP403, ExcelBPSDisabledAtExtraKey: disabledAt}, "disabled"},
		{"reopen BPS", map[string]any{"openai_excel_bps": true}, "enabled"},
		{"editor deletes switch on closure", map[string]any{}, "disabled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			updated, err := svc.UpdateAccount(context.Background(), 18, &UpdateAccountInput{Extra: tc.extra})
			require.NoError(t, err)
			require.Equal(t, tc.want, updated.ExcelBPSStatus())
			if tc.want == "disabled_403" {
				require.Equal(t, disabledAt, updated.Extra[ExcelBPSDisabledAtExtraKey])
			} else {
				require.NotContains(t, updated.Extra, ExcelBPSDisabledReasonExtraKey)
				require.NotContains(t, updated.Extra, ExcelBPSDisabledAtExtraKey)
			}
		})
	}
}

func TestCreateAccountExcelBPSStatusDoesNotCopyDisabledRecord(t *testing.T) {
	account, err := buildAccountForCreate(&CreateAccountInput{
		Name: "copied account", Platform: PlatformOpenAI, Type: AccountTypeOAuth,
	}, map[string]any{
		ExcelBPSDisabledReasonExtraKey: ExcelBPSDisabledReasonHTTP403,
		ExcelBPSDisabledAtExtraKey:     "2026-09-27T01:02:03Z",
	})
	require.NoError(t, err)
	require.Equal(t, "disabled", account.ExcelBPSStatus())
	require.NotContains(t, account.Extra, ExcelBPSDisabledReasonExtraKey)
	require.NotContains(t, account.Extra, ExcelBPSDisabledAtExtraKey)
}

func TestUpdateAccountExcelBPSReopenClearsAutomaticRecord(t *testing.T) {
	repo := &updateAccountOveragesRepoStub{account: &Account{
		ID: 18, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Extra: map[string]any{
			"openai_excel_bps":             false,
			ExcelBPSDisabledReasonExtraKey: ExcelBPSDisabledReasonHTTP403,
			ExcelBPSDisabledAtExtraKey:     "2026-09-27T01:02:03Z",
		},
	}}
	svc := &adminServiceImpl{accountRepo: repo}
	updated, err := svc.UpdateAccount(context.Background(), 18, &UpdateAccountInput{
		Extra: map[string]any{"openai_excel_bps": true},
	})
	require.NoError(t, err)
	require.Equal(t, "enabled", updated.ExcelBPSStatus())
	require.NotContains(t, updated.Extra, ExcelBPSDisabledReasonExtraKey)
	require.NotContains(t, updated.Extra, ExcelBPSDisabledAtExtraKey)
}

type excelBPSExtraPatchRepoStub struct {
	AccountRepository
	patch map[string]any
}

func (r *excelBPSExtraPatchRepoStub) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	r.patch = updates
	return nil
}

func TestUpdateAccountExtraExcelBPSRecordProtection(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		repo := &excelBPSExtraPatchRepoStub{}
		svc := &adminServiceImpl{accountRepo: repo}
		err := svc.UpdateAccountExtra(context.Background(), 18, map[string]any{
			"openai_excel_bps":             enabled,
			ExcelBPSDisabledReasonExtraKey: ExcelBPSDisabledReasonHTTP403,
			ExcelBPSDisabledAtExtraKey:     "forged",
		})
		require.NoError(t, err)
		require.Equal(t, enabled, repo.patch["openai_excel_bps"])
		require.Contains(t, repo.patch, ExcelBPSDisabledReasonExtraKey)
		require.Nil(t, repo.patch[ExcelBPSDisabledReasonExtraKey])
		require.Contains(t, repo.patch, ExcelBPSDisabledAtExtraKey)
		require.Nil(t, repo.patch[ExcelBPSDisabledAtExtraKey])
	}

	repo := &excelBPSExtraPatchRepoStub{}
	svc := &adminServiceImpl{accountRepo: repo}
	err := svc.UpdateAccountExtra(context.Background(), 18, map[string]any{
		"openai_passthrough": true, ExcelBPSDisabledReasonExtraKey: ExcelBPSDisabledReasonHTTP403,
	})
	require.NoError(t, err)
	require.Equal(t, map[string]any{"openai_passthrough": true}, repo.patch)
}
