package service

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type manualAccountTestContextKey struct{}
type manualAccountTestContext struct {
	prompt          string
	reasoningEffort string
	cancel          context.CancelFunc
	accountID       int64
	proxyID         *int64
}

func manualAccountTest(ctx context.Context) *manualAccountTestContext {
	v, _ := ctx.Value(manualAccountTestContextKey{}).(*manualAccountTestContext)
	return v
}

func ValidateAccountTestOptions(opts AccountTestOptions) error {
	if err := ValidateIntelligentReasoningEffort(opts.ReasoningEffort); err != nil {
		return err
	}
	if opts.TimeoutSeconds != 0 && (opts.TimeoutSeconds < 10 || opts.TimeoutSeconds > 300) {
		return fmt.Errorf("测试超时必须为 10 到 300 秒")
	}
	return nil
}

func validateManualTestReasoning(a *Account, model, mode string, opts AccountTestOptions) error {
	if opts.ReasoningEffort == "" {
		return nil
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == AccountTestModeCompact || (mode != "" && mode != AccountTestModeDefault && mode != AccountTestModeGrokText) || isOpenAIImageModel(a.GetMappedModel(model)) {
		return fmt.Errorf("当前测试模式不支持指定思考强度，请选择默认")
	}
	if a.IsCNProvider() && a.GetAPIProtocol() == APIProtocolAdaptive {
		return fmt.Errorf("自适应测试包含 Anthropic 端点，请使用默认思考强度")
	}
	return validateIntelligentAccountReasoning(a, model, opts.ReasoningEffort)
}

// Test the selected physical channel only. A retired logical root is not a
// usable route, and a test must never silently move to another IP.
func (s *AccountTestService) validateTestIPChannel(ctx context.Context, a *Account) error {
	r, ok := s.accountRepo.(interface {
		GetAccountIPChannels(context.Context, []int64) (map[int64][]AccountIPChannel, error)
	})
	if !ok || a.Platform != PlatformOpenAI || a.Type != AccountTypeOAuth {
		return nil
	}
	groups, err := r.GetAccountIPChannels(ctx, []int64{a.ID})
	if err != nil {
		return fmt.Errorf("读取 IP 通道失败: %w", err)
	}
	channels := groups[a.ID]
	if len(channels) == 0 {
		return nil
	}
	for _, channel := range channels {
		if channel.Account == nil || channel.Account.ID != a.ID {
			continue
		}
		if !channel.Enabled || !channel.LogicalEnabled {
			return fmt.Errorf("所选 IP 通道已停用，请先启用后测试")
		}
		p := channel.Account.Proxy
		if p == nil || !p.IsActive() || p.IsExpired(time.Now()) {
			return fmt.Errorf("所选 IP 通道代理不可用，请修复后测试")
		}
		a.ProxyID, a.Proxy = channel.Account.ProxyID, p
		return nil
	}
	return fmt.Errorf("逻辑账号的原 IP 通道已移除，请选择一个有效 IP 通道进行测试")
}
