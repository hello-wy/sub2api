package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const SettingKeyOpenAINewAccountCodexTicketDefaults = "openai_new_account_codex_ticket_defaults"

// This controls future account creation only. It never enables the harvester,
// reconfigures an existing account, or claims that a ticket is already valid.
type NewAccountCodexTicketDefaults struct {
	Enabled    bool   `json:"enabled"`
	TicketPlan string `json:"ticket_plan"`
	Model      string `json:"model"`
}

func normalizeNewAccountCodexTicketDefaults(value NewAccountCodexTicketDefaults) (NewAccountCodexTicketDefaults, error) {
	value.TicketPlan = strings.ToLower(strings.TrimSpace(value.TicketPlan))
	if value.TicketPlan != codexTicketPlanPro && value.TicketPlan != codexTicketPlanTeam {
		return value, infraerrors.BadRequest("CODEX_TICKET_DEFAULT_PLAN", "新账号 STATE 套餐必须为 Pro（292）或 Team（332）")
	}
	value.Model = openAICodexTicketDefaultModel
	return value, nil
}

func (s *SettingService) GetNewAccountCodexTicketDefaults(ctx context.Context) (*NewAccountCodexTicketDefaults, error) {
	if s == nil || s.settingRepo == nil {
		return nil, errors.New("new-account STATE defaults service unavailable")
	}
	values, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeyOpenAINewAccountCodexTicketDefaults})
	if err != nil {
		return nil, errors.New("unable to read new-account STATE defaults")
	}
	value := NewAccountCodexTicketDefaults{TicketPlan: codexTicketPlanPro, Model: openAICodexTicketDefaultModel}
	if raw, exists := values[SettingKeyOpenAINewAccountCodexTicketDefaults]; exists {
		// A malformed saved policy must not silently let new accounts bypass it.
		var stored struct {
			Enabled    *bool   `json:"enabled"`
			TicketPlan *string `json:"ticket_plan"`
		}
		if strings.TrimSpace(raw) == "" || json.Unmarshal([]byte(raw), &stored) != nil || stored.Enabled == nil || stored.TicketPlan == nil {
			return nil, errors.New("invalid saved new-account STATE defaults")
		}
		value.Enabled, value.TicketPlan = *stored.Enabled, *stored.TicketPlan
	}
	value, err = normalizeNewAccountCodexTicketDefaults(value)
	if err != nil {
		return nil, errors.New("invalid saved new-account STATE defaults")
	}
	return &value, nil
}

func (s *SettingService) UpdateNewAccountCodexTicketDefaults(ctx context.Context, input NewAccountCodexTicketDefaults) (*NewAccountCodexTicketDefaults, error) {
	if s == nil || s.settingRepo == nil {
		return nil, errors.New("new-account STATE defaults service unavailable")
	}
	value, err := normalizeNewAccountCodexTicketDefaults(input)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	// One JSON value is one database write, so another app instance cannot read
	// a newly enabled switch with the previous plan halfway through a save.
	if err = s.settingRepo.SetMultiple(ctx, map[string]string{SettingKeyOpenAINewAccountCodexTicketDefaults: string(raw)}); err != nil {
		return nil, errors.New("unable to save new-account STATE defaults")
	}
	return &value, nil
}
