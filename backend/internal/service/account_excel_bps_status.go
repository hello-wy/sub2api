package service

import "maps"

const (
	ExcelBPSDisabledReasonExtraKey = "openai_excel_bps_disabled_reason"
	ExcelBPSDisabledAtExtraKey     = "openai_excel_bps_disabled_at"
	ExcelBPSDisabledReasonHTTP403  = "http_403"
)

func (a *Account) supportsExcelBPS() bool {
	return a != nil && a.Platform == PlatformOpenAI && a.Type == AccountTypeOAuth &&
		!a.IsShadow() && !a.IsOpenAIAgentIdentity() && !a.IsOpenAIPersonalAccessToken()
}

// ExcelBPSStatus describes the account switch, independent of model scope and
// account health. An opt-in to automatic disabling is not evidence of a 403.
func (a *Account) ExcelBPSStatus() string {
	if !a.supportsExcelBPS() {
		return ""
	}
	if a.IsExcelBPSEnabled() {
		return "enabled"
	}
	if a.Extra[ExcelBPSDisabledReasonExtraKey] == ExcelBPSDisabledReasonHTTP403 {
		return "disabled_403"
	}
	return "disabled"
}

func stripExcelBPSManagedExtra(extra map[string]any) map[string]any {
	clean := maps.Clone(extra)
	delete(clean, ExcelBPSDisabledReasonExtraKey)
	delete(clean, ExcelBPSDisabledAtExtraKey)
	return clean
}

// An ordinary edit of an already disabled account retains the server record.
// Explicit switch changes clear it, so reopening and later closing BPS cannot
// resurrect an old 403. Incoming clients cannot supply or overwrite the record.
func prepareExcelBPSStatusExtraForUpdate(account *Account, extra map[string]any) map[string]any {
	clean := stripExcelBPSManagedExtra(extra)
	if !account.supportsExcelBPS() || account.IsExcelBPSEnabled() {
		return clean
	}
	if _, explicit := clean["openai_excel_bps"].(bool); explicit {
		return clean
	}
	if account.ExcelBPSStatus() == "disabled_403" {
		if clean == nil {
			clean = make(map[string]any)
		}
		for _, key := range []string{ExcelBPSDisabledReasonExtraKey, ExcelBPSDisabledAtExtraKey} {
			if value, exists := account.Extra[key]; exists {
				clean[key] = value
			}
		}
	}
	return clean
}
