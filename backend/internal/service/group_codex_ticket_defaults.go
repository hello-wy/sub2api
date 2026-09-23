package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
)

type GroupCodexTicketDefaults = domain.GroupCodexTicketDefaults

type GroupCodexTicketDefaultsAdmin interface {
	ValidateCodexTicketGroupDefaults(context.Context, []int64) error
}

type AccountIPChannelTicketDefaultsRepository interface {
	AddAccountIPChannelsWithTicketDefaults(context.Context, int64, []int64, *int, *int, *GroupCodexTicketDefaults) ([]int64, error)
}

func normalizeGroupCodexTicketDefaults(platform string, value GroupCodexTicketDefaults) (GroupCodexTicketDefaults, error) {
	value.TicketPlan = strings.ToLower(strings.TrimSpace(value.TicketPlan))
	value.Model = strings.TrimSpace(value.Model)
	if value.TicketPlan == "" {
		value.TicketPlan = codexTicketPlanPro
	}
	if value.Model == "" {
		value.Model = openAICodexTicketDefaultModel
	}
	if platform != PlatformOpenAI {
		value.Enabled = false
	}
	if value.TicketPlan != codexTicketPlanPro && value.TicketPlan != codexTicketPlanTeam {
		return value, infraerrors.BadRequest("GROUP_CODEX_TICKET_PLAN", "STATE 套餐必须为 Pro（292）或 Team（332）")
	}
	if value.Model != openAICodexTicketDefaultModel && value.Model != openAICodexTicketDefaultSolModel {
		return value, infraerrors.BadRequest("GROUP_CODEX_TICKET_MODEL", "STATE 模型必须为 gpt-6-astra 或 gpt-5.6-sol")
	}
	return value, nil
}

func (s *adminServiceImpl) resolveCodexTicketGroupDefaults(ctx context.Context, ids []int64) (*GroupCodexTicketDefaults, error) {
	// The global new-account policy intentionally takes precedence over group
	// defaults. This resolver is used only by new-account/new-IP/restore paths;
	// saving this setting never rewrites existing accounts.
	if s.settingService != nil {
		global, err := s.settingService.GetNewAccountCodexTicketDefaults(ctx)
		if err != nil {
			return nil, err
		}
		if global.Enabled {
			return &GroupCodexTicketDefaults{Enabled: true, TicketPlan: global.TicketPlan, Model: global.Model, RequireVerified: true}, nil
		}
	}
	var result *GroupCodexTicketDefaults
	var source string
	seen := map[int64]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		group, err := s.groupRepo.GetByIDLite(ctx, id)
		if err != nil {
			return nil, err
		}
		if group == nil || group.Platform != PlatformOpenAI || !group.CodexTicketDefaults.Enabled {
			continue
		}
		value, err := normalizeGroupCodexTicketDefaults(group.Platform, group.CodexTicketDefaults)
		if err != nil {
			return nil, err
		}
		if result != nil && (result.TicketPlan != value.TicketPlan || result.Model != value.Model) {
			return nil, infraerrors.BadRequest("GROUP_CODEX_TICKET_CONFLICT", fmt.Sprintf("分组 %s 与 %s 的 STATE 套餐或模型配置冲突，请统一设置后再导入", source, group.Name))
		}
		result = &value
		source = group.Name
	}
	return result, nil
}

func (s *adminServiceImpl) ValidateCodexTicketGroupDefaults(ctx context.Context, ids []int64) error {
	_, err := s.resolveCodexTicketGroupDefaults(ctx, ids)
	return err
}

func PrepareNewAccountCodexTicketDefaults(account *Account) bool {
	if account == nil {
		return false
	}
	defaults := account.InitialCodexTicketDefaults
	if defaults == nil || !defaults.Enabled || !isOpenAICodexTicketAccount(account) {
		return false
	}
	// Called only on newly created records after untrusted import state is stripped.
	if account.Extra == nil {
		account.Extra = map[string]any{}
	}
	account.Extra[codexAccountTicketConfigKey] = codexAccountTicketConfig{Enabled: true, RequireVerified: defaults.RequireVerified, TicketPlan: defaults.TicketPlan, Model: defaults.Model, Revision: uuid.NewString()}
	return true
}

func (s *adminServiceImpl) scheduleInheritedCodexTicket(ctx context.Context, id int64) {
	starter, ok := s.runtimeBlocker.(interface {
		HarvestCodexAccountTicket(context.Context, int64) (*CodexAccountTicketStatus, error)
	})
	if !ok {
		return
	}
	// Import transactions run this hook only after commit. No unavailable pool,
	// disabled master switch or upstream failure can roll back saved accounts.
	RunAccountImportSideEffect(ctx, func() {
		// Do not carry the already-committed import transaction into the worker.
		jobCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = starter.HarvestCodexAccountTicket(jobCtx, id)
	})
}
