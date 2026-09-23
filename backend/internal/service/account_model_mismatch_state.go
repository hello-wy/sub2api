package service

import (
	"context"
	"strings"
)

// AccountModelMismatchExtraKey is server-owned detection state. It is removed
// only by an explicit administrator resume, never by successful requests or
// routine account/credential updates.
const AccountModelMismatchExtraKey = "model_mismatch"

type modelMismatchResumeContextKey struct{}

// WithModelMismatchResumeConfirmation records an explicit administrator
// confirmation; ordinary schedulable toggles must not clear a new quarantine.
func WithModelMismatchResumeConfirmation(ctx context.Context) context.Context {
	return context.WithValue(ctx, modelMismatchResumeContextKey{}, true)
}

func ModelMismatchResumeConfirmed(ctx context.Context) bool {
	confirmed, _ := ctx.Value(modelMismatchResumeContextKey{}).(bool)
	return confirmed
}

func (a *Account) HasModelMismatch() bool {
	if a == nil {
		return false
	}
	marker, ok := a.Extra[AccountModelMismatchExtraKey].(map[string]any)
	if !ok {
		return false
	}
	expected, _ := marker["expected_model"].(string)
	actual, _ := marker["actual_model"].(string)
	return IsAccountModelDegradation(expected, actual)
}

// IsAccountModelDegradation is deliberately directional and exact. Callers
// supply the model actually sent after mapping, never the public request alias.
// General model differences remain audit evidence, not account degradation.
func IsAccountModelDegradation(expected, actual string) bool {
	return strings.EqualFold(strings.TrimSpace(expected), "gpt-6-astra") &&
		strings.EqualFold(strings.TrimSpace(actual), "gpt-5.6-luna")
}

// Historical broad mismatches remain available for review and explicit
// recovery. Merely upgrading the interpretation never enables a paused route.
func (a *Account) HasLegacyModelMismatch() bool {
	if a == nil || a.HasModelMismatch() {
		return false
	}
	marker, ok := a.Extra[AccountModelMismatchExtraKey].(map[string]any)
	return ok && len(marker) > 0
}

func (a *Account) IsLegacyModelMismatchQuarantined() bool {
	if !a.HasLegacyModelMismatch() {
		return false
	}
	marker := a.Extra[AccountModelMismatchExtraKey].(map[string]any)
	quarantined, explicit := marker["quarantined"].(bool)
	return !explicit || quarantined
}

// Old records have no quarantined field and retain their original fail-closed
// meaning. Only an explicit false denotes an observation without quarantine.
func (a *Account) IsModelMismatchQuarantined() bool {
	if !a.HasModelMismatch() {
		return false
	}
	marker := a.Extra[AccountModelMismatchExtraKey].(map[string]any)
	quarantined, explicit := marker["quarantined"].(bool)
	return !explicit || quarantined
}
