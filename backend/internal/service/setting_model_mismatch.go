package service

import (
	"context"
	"errors"
	"strconv"
)

const SettingKeyOpenAIModelMismatchAutoQuarantineEnabled = "openai_model_mismatch_auto_quarantine_enabled"

type ModelMismatchSettings struct {
	AutoQuarantineEnabled bool `json:"auto_quarantine_enabled"`
}

// Only an explicit false disables the existing protection. Missing or malformed
// stored values keep the historic default. Runtime reads happen only on detection
// inside the account transaction, so no stale cache can ignore a saved switch.
func ModelMismatchAutoQuarantineEnabled(value string) bool { return value != "false" }

func (s *SettingService) GetModelMismatchSettings(ctx context.Context) (*ModelMismatchSettings, error) {
	if s == nil || s.settingRepo == nil {
		return nil, errors.New("model mismatch settings service unavailable")
	}
	values, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeyOpenAIModelMismatchAutoQuarantineEnabled})
	if err != nil {
		return nil, errors.New("unable to read model mismatch settings")
	}
	return &ModelMismatchSettings{AutoQuarantineEnabled: ModelMismatchAutoQuarantineEnabled(values[SettingKeyOpenAIModelMismatchAutoQuarantineEnabled])}, nil
}

func (s *SettingService) UpdateModelMismatchSettings(ctx context.Context, input ModelMismatchSettings) (*ModelMismatchSettings, error) {
	if s == nil || s.settingRepo == nil {
		return nil, errors.New("model mismatch settings service unavailable")
	}
	if err := s.settingRepo.SetMultiple(ctx, map[string]string{SettingKeyOpenAIModelMismatchAutoQuarantineEnabled: strconv.FormatBool(input.AutoQuarantineEnabled)}); err != nil {
		return nil, errors.New("unable to save model mismatch settings")
	}
	// Never touch accounts here: changing future handling must not resume any
	// existing quarantine, manually disabled account, or paused IP channel.
	return &input, nil
}
