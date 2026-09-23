package service

import (
	"context"
	"net/http"
	"strings"
	"time"

	apperrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	CodexTicketBatchMaxAccounts = 100
	CodexTicketBatchMaxChannels = 500
)

type CodexAccountTicketBatchUpdate struct {
	AccountIDs []int64 `json:"account_ids"`
	Enabled    bool    `json:"enabled"`
	TicketPlan string  `json:"ticket_plan"`
	Model      string  `json:"model"`
	Harvest    bool    `json:"harvest"`
}

type CodexAccountTicketBatchItem struct {
	AccountID        int64                     `json:"account_id"`
	LogicalAccountID int64                     `json:"logical_account_id"`
	Saved            bool                      `json:"saved"`
	Outcome          string                    `json:"outcome"`
	Message          string                    `json:"message"`
	Status           *CodexAccountTicketStatus `json:"status,omitempty"`
}

type CodexAccountTicketBatchResult struct {
	SelectedAccounts int                           `json:"selected_accounts"`
	TotalChannels    int                           `json:"total_channels"`
	Saved            int                           `json:"saved"`
	Harvesting       int                           `json:"harvesting"`
	Waiting          int                           `json:"waiting"`
	Cooldown         int                           `json:"cooldown"`
	Skipped          int                           `json:"skipped"`
	Failed           int                           `json:"failed"`
	Results          []CodexAccountTicketBatchItem `json:"results"`
}

// BatchCodexAccountTickets updates each physical route once. Configuration is
// persisted through the same row-locked mutation as the single-account editor;
// selecting a logical account never makes its siblings share a STATE ticket.
func (s *OpenAIGatewayService) BatchCodexAccountTickets(ctx context.Context, input CodexAccountTicketBatchUpdate) (*CodexAccountTicketBatchResult, error) {
	if s == nil || s.accountRepo == nil {
		return nil, apperrors.New(http.StatusServiceUnavailable, "CODEX_TICKET_UNAVAILABLE", "Ticket service is unavailable")
	}
	if len(input.AccountIDs) == 0 || len(input.AccountIDs) > CodexTicketBatchMaxAccounts {
		return nil, apperrors.BadRequest("CODEX_TICKET_BATCH_LIMIT", "Select between 1 and 100 accounts")
	}
	ids := make([]int64, 0, len(input.AccountIDs))
	selected := make(map[int64]bool, len(input.AccountIDs))
	for _, id := range input.AccountIDs {
		if id <= 0 {
			return nil, apperrors.BadRequest("CODEX_TICKET_BATCH_IDS", "Account IDs must be positive")
		}
		if !selected[id] {
			ids = append(ids, id)
			selected[id] = true
		}
	}
	input.TicketPlan = strings.ToLower(strings.TrimSpace(input.TicketPlan))
	input.Model = strings.TrimSpace(input.Model)
	if input.Enabled {
		if input.TicketPlan != codexTicketPlanPro && input.TicketPlan != codexTicketPlanTeam {
			return nil, apperrors.BadRequest("CODEX_TICKET_PLAN", "Select pro (292) or team (332) explicitly")
		}
		if input.Model != openAICodexTicketDefaultModel && input.Model != openAICodexTicketDefaultSolModel {
			return nil, apperrors.BadRequest("CODEX_TICKET_MODEL", "Select gpt-6-astra or gpt-5.6-sol explicitly")
		}
		// An explicit administrative action reads fresh settings. A repository
		// outage is not a disabled switch or missing pool the user should edit.
		if s.settingService != nil {
			if _, err := s.settingService.GetCodexTicketSettings(ctx); err != nil {
				return nil, apperrors.New(http.StatusServiceUnavailable, "CODEX_TICKET_SETTINGS_UNAVAILABLE", "Unable to read STATE settings; no accounts were changed")
			}
		}
		pool := s.openAICodexTicketHarvestProxyURLContext(ctx)
		if pool == "" || ValidateOpenAICodexTicketHarvestProxyURL(pool) != nil {
			return nil, apperrors.BadRequest("CODEX_TICKET_GLOBAL_PROXY", "Configure the global dynamic proxy pool in gateway settings")
		}
		if !s.openAICodexTicketEnabledContext(ctx) {
			return nil, apperrors.BadRequest("CODEX_TICKET_GLOBAL_DISABLED", "Enable the gateway STATE master switch first")
		}
	} else {
		// A bulk disable must not silently replace heterogeneous plans/models.
		input.TicketPlan, input.Model, input.Harvest = "", "", false
	}

	groups := map[int64][]AccountIPChannel{}
	if repo, ok := s.accountRepo.(interface {
		GetAccountIPChannels(context.Context, []int64) (map[int64][]AccountIPChannel, error)
	}); ok {
		var err error
		groups, err = repo.GetAccountIPChannels(ctx, ids)
		if err != nil {
			return nil, apperrors.New(http.StatusServiceUnavailable, "CODEX_TICKET_CHANNELS_UNAVAILABLE", "Unable to load account IP channels; no settings were changed")
		}
	}

	// Expand and bound the whole selection before the first write. The channel
	// repository excludes retired routes, including retired logical root routes.
	targets := make([]CodexAccountTicketBatchItem, 0, len(ids))
	seen := make(map[int64]bool)
	for _, id := range ids {
		channels := groups[id]
		if len(channels) == 0 {
			channels = []AccountIPChannel{{Account: &Account{ID: id}, LogicalAccountID: id}}
		}
		for _, channel := range channels {
			if channel.Account == nil || channel.Account.ID <= 0 || seen[channel.Account.ID] {
				continue
			}
			seen[channel.Account.ID] = true
			targets = append(targets, CodexAccountTicketBatchItem{AccountID: channel.Account.ID, LogicalAccountID: channel.LogicalAccountID})
			if len(targets) > CodexTicketBatchMaxChannels {
				return nil, apperrors.BadRequest("CODEX_TICKET_CHANNEL_LIMIT", "Selection expands to more than 500 IP channels; select fewer accounts")
			}
		}
	}

	result := &CodexAccountTicketBatchResult{SelectedAccounts: len(ids), TotalChannels: len(targets), Results: make([]CodexAccountTicketBatchItem, 0, len(targets))}
	for _, target := range targets {
		item := s.configureCodexTicketBatchItem(ctx, target, input)
		if item.Saved {
			result.Saved++
		}
		switch item.Outcome {
		case "harvesting":
			result.Harvesting++
		case "waiting":
			result.Waiting++
		case "cooldown":
			result.Cooldown++
		case "skipped":
			result.Skipped++
		case "error":
			result.Failed++
		}
		result.Results = append(result.Results, item)
	}
	return result, nil
}

func (s *OpenAIGatewayService) configureCodexTicketBatchItem(ctx context.Context, item CodexAccountTicketBatchItem, input CodexAccountTicketBatchUpdate) CodexAccountTicketBatchItem {
	account, err := s.codexTicketAccountByID(ctx, item.AccountID)
	if err != nil {
		item.Outcome, item.Message = "skipped", codexTicketBatchPublicError(err)
		return item
	}
	if input.Enabled && !codexAccountTicketEligible(account) {
		item.Outcome, item.Message = "skipped", "Account must be active and have an active fixed business proxy"
		return item
	}
	status, err := s.configureCodexAccountTicket(ctx, item.AccountID, CodexAccountTicketUpdate{Enabled: input.Enabled, TicketPlan: input.TicketPlan, Model: input.Model}, false)
	if err != nil {
		item.Outcome, item.Message = "error", codexTicketBatchPublicError(err)
		return item
	}
	item.Saved, item.Status, item.Outcome = true, status, "saved"
	item.Message = "STATE settings saved"
	if !input.Enabled {
		item.Message = "STATE disabled; existing plan and model preserved"
		return item
	}
	if !input.Harvest {
		return item
	}
	// This existing starter enforces process concurrency, duplicate suppression,
	// upstream rejection cooldown and the fixed-route verification requirement.
	s.startCodexAccountTicketJob(ctx, item.AccountID, true)
	status, err = s.GetCodexAccountTicketStatus(ctx, item.AccountID)
	if err != nil {
		item.Outcome, item.Message = "error", "Settings saved, but acquisition status could not be read; refresh to check"
		return item
	}
	item.Status = status
	switch {
	case status.State == "harvesting":
		item.Outcome, item.Message = "harvesting", "Acquisition is running; no successful ticket has been confirmed yet"
	case status.TicketUsable:
		item.Outcome, item.Message = "ready", "A verified ticket is available"
	case status.RetryAfter != nil && status.RetryAfter.After(time.Now()):
		item.Outcome, item.Message = "cooldown", "Settings saved; acquisition is cooling down after an upstream rejection"
	case status.State == "global_disabled" || !status.Enabled:
		item.Outcome, item.Message = "saved", "Settings saved; STATE was disabled before acquisition could start"
	case status.State == "error":
		item.Outcome, item.Message = "error", "Settings saved; acquisition could not complete, inspect the account STATE status"
	default:
		item.Outcome, item.Message = "waiting", "Settings saved; waiting for an available acquisition slot"
	}
	return item
}

func codexTicketBatchPublicError(err error) string {
	if apperrors.Code(err) >= http.StatusInternalServerError {
		return "Account STATE operation failed; refresh and retry"
	}
	return apperrors.Message(err)
}
