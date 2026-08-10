package admin

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestParseAdminOrderID(t *testing.T) {
	tests := []struct {
		value      string
		sourceKind string
		id         int64
		ok         bool
	}{
		{value: "payment:12", sourceKind: service.AdminOrderSourcePayment, id: 12, ok: true},
		{value: "lottery:34", sourceKind: service.AdminOrderSourceLottery, id: 34, ok: true},
		{value: "12", ok: false},
		{value: "lottery:0", ok: false},
		{value: "other:12", ok: false},
	}
	for _, tt := range tests {
		sourceKind, id, ok := parseAdminOrderID(tt.value)
		if sourceKind != tt.sourceKind || id != tt.id || ok != tt.ok {
			t.Errorf("parseAdminOrderID(%q) = (%q, %d, %t), want (%q, %d, %t)", tt.value, sourceKind, id, ok, tt.sourceKind, tt.id, tt.ok)
		}
	}
}

func TestSanitizeAdminLotteryOrderForResponse(t *testing.T) {
	now := time.Now()
	before, after := 50.0, 20.0
	got := sanitizeAdminOrderForResponse(service.AdminOrder{
		ID: "lottery:3", SourceKind: service.AdminOrderSourceLottery, UserID: 2,
		Amount: 30, PayAmount: 30, Currency: "USD", OutTradeNo: "lottery-purchase-ref",
		PaymentType: "balance", OrderType: service.AdminOrderTypeLottery, Status: service.OrderStatusCompleted,
		TicketCount: 1, BalanceBefore: &before, BalanceAfter: &after, CreatedAt: now, UpdatedAt: now,
	})
	if got.ID != "lottery:3" || got.SourceKind != service.AdminOrderSourceLottery || got.OrderType != service.AdminOrderTypeLottery {
		t.Fatalf("unexpected lottery response: %#v", got)
	}
	if got.PaymentType != "balance" || got.Amount != 30 || got.PayAmount != 30 || got.TicketCount != 1 {
		t.Fatalf("unexpected lottery payment fields: %#v", got)
	}
	if got.PayURL != nil || got.ProviderKey != nil || got.RefundReason != nil {
		t.Fatalf("lottery order exposed payment-only fields: %#v", got)
	}
}

func TestSanitizeAdminPaymentOrderForResponseAddsCurrency(t *testing.T) {
	now := time.Now()
	order := &dbent.PaymentOrder{
		ID:          1,
		UserID:      2,
		Amount:      100,
		PayAmount:   108,
		FeeRate:     8,
		OutTradeNo:  "sub2_202606250001",
		PaymentType: "stripe",
		OrderType:   "subscription",
		Status:      "COMPLETED",
		ExpiresAt:   now,
		CreatedAt:   now,
		UpdatedAt:   now,
		ProviderSnapshot: map[string]any{
			"schema_version": 2,
			"currency":       "USD",
		},
	}

	got := sanitizeAdminPaymentOrderForResponse(order)
	if got == nil {
		t.Fatal("expected sanitized order")
		return
	}
	if got.Currency != "USD" {
		t.Fatalf("expected currency USD, got %q", got.Currency)
	}

	body, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal sanitized order: %v", err)
	}
	if strings.Contains(string(body), "provider_snapshot") {
		t.Fatalf("expected provider_snapshot to be omitted, got %s", string(body))
	}
}

func TestAdminSubscriptionPlansForResponseIncludesCompositeGroupInfo(t *testing.T) {
	weekly := 25.0
	total := 100.0
	now := time.Now()
	plans := []*dbent.SubscriptionPlan{
		{
			ID:           11,
			GroupID:      7,
			Name:         "All models",
			Description:  "Composite access",
			Price:        19.99,
			Currency:     "CNY",
			ValidityDays: 30,
			ValidityUnit: "days",
			Features:     "OpenAI\nClaude\nGemini\nGrok",
			ProductName:  "Sub2API",
			ForSale:      true,
			SortOrder:    1,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}
	groupInfo := map[int64]service.PlanGroupInfo{
		7: {
			Platform:                   service.PlatformComposite,
			Name:                       "Bucket 2 composite",
			RateMultiplier:             1.5,
			SubscriptionQuotaResetMode: service.SubscriptionQuotaResetModeUntilSubscriptionExpires,
			SubscriptionTotalLimitUSD:  &total,
			WeeklyLimitUSD:             &weekly,
			ModelScopes:                []string{"openai", "claude", "gemini", "grok"},
		},
	}

	got := adminSubscriptionPlansForResponse(plans, groupInfo)

	if len(got) != 1 {
		t.Fatalf("expected one plan, got %d", len(got))
	}
	if got[0].GroupPlatform != service.PlatformComposite {
		t.Fatalf("expected composite group platform, got %q", got[0].GroupPlatform)
	}
	if got[0].GroupName != "Bucket 2 composite" {
		t.Fatalf("expected group name to be included, got %q", got[0].GroupName)
	}
	if got[0].WeeklyLimitUSD == nil || *got[0].WeeklyLimitUSD != weekly {
		t.Fatalf("expected weekly limit to be included, got %#v", got[0].WeeklyLimitUSD)
	}
	if got[0].SubscriptionQuotaResetMode != service.SubscriptionQuotaResetModeUntilSubscriptionExpires || got[0].SubscriptionTotalLimitUSD == nil || *got[0].SubscriptionTotalLimitUSD != total {
		t.Fatalf("expected lifetime quota details to be included, got mode=%q total=%#v", got[0].SubscriptionQuotaResetMode, got[0].SubscriptionTotalLimitUSD)
	}
	if strings.Join(got[0].ModelScopes, ",") != "openai,claude,gemini,grok" {
		t.Fatalf("expected model scopes to be preserved, got %#v", got[0].ModelScopes)
	}
	// 投影必须保留 ent 原始响应的全部套餐字段：currency 丢失曾导致编辑保存时
	// 静默清空套餐货币（PlanEditDialog 回传空串 → SetCurrency("")）。
	if got[0].Currency != "CNY" {
		t.Fatalf("expected currency to be preserved, got %q", got[0].Currency)
	}
	if !got[0].CreatedAt.Equal(now) || !got[0].UpdatedAt.Equal(now) {
		t.Fatalf("expected created_at/updated_at to be preserved, got %v / %v", got[0].CreatedAt, got[0].UpdatedAt)
	}
}
