package service

import (
	"context"
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
)

type lotteryBillingCacheInvalidatorStub struct {
	userIDs []int64
}

func (s *lotteryBillingCacheInvalidatorStub) InvalidateUserBalance(_ context.Context, userID int64) error {
	s.userIDs = append(s.userIDs, userID)
	return nil
}

type lotteryAuthCacheInvalidatorStub struct {
	userIDs []int64
}

func (s *lotteryAuthCacheInvalidatorStub) InvalidateAuthCacheByKey(context.Context, string)    {}
func (s *lotteryAuthCacheInvalidatorStub) InvalidateAuthCacheByGroupID(context.Context, int64) {}
func (s *lotteryAuthCacheInvalidatorStub) InvalidateAuthCacheByUserID(_ context.Context, userID int64) {
	s.userIDs = append(s.userIDs, userID)
}

func TestIsLotteryRechargePayment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		order *dbent.PaymentOrder
		want  bool
	}{
		{
			name:  "third-party balance recharge qualifies",
			order: &dbent.PaymentOrder{PaymentType: payment.TypeAlipay, OrderType: payment.OrderTypeBalance},
			want:  true,
		},
		{
			name:  "third-party subscription purchase qualifies",
			order: &dbent.PaymentOrder{PaymentType: payment.TypeEasyPay, OrderType: payment.OrderTypeSubscription},
			want:  true,
		},
		{
			name:  "balance-funded subscription is excluded",
			order: &dbent.PaymentOrder{PaymentType: payment.OrderTypeBalance, OrderType: payment.OrderTypeSubscription},
			want:  false,
		},
		{
			name: "non-CNY payment is excluded",
			order: &dbent.PaymentOrder{
				PaymentType: payment.TypeStripe,
				OrderType:   payment.OrderTypeSubscription,
				ProviderSnapshot: map[string]any{
					"currency": "USD",
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isLotteryRechargePayment(tt.order); got != tt.want {
				t.Fatalf("isLotteryRechargePayment() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLotteryBalanceChangesInvalidateBillingAndAuthCaches(t *testing.T) {
	billing := &lotteryBillingCacheInvalidatorStub{}
	auth := &lotteryAuthCacheInvalidatorStub{}
	service := &LotteryService{billingCache: billing, authCacheInvalidator: auth}

	service.invalidateBalanceCaches(42)

	if len(billing.userIDs) != 1 || billing.userIDs[0] != 42 {
		t.Fatalf("billing invalidations = %v, want [42]", billing.userIDs)
	}
	if len(auth.userIDs) != 1 || auth.userIDs[0] != 42 {
		t.Fatalf("auth invalidations = %v, want [42]", auth.userIDs)
	}
}

func TestDefaultLotteryPrizePoolProbabilitiesTotalOne(t *testing.T) {
	pool := defaultLotteryPrizePoolConfig()
	if pool.Enabled == nil || !*pool.Enabled {
		t.Fatal("default lottery pool must be enabled")
	}
	if pool.InvitationFirstPaymentAmount != 20 || pool.InvitationConsumptionAmount != 100 {
		t.Fatalf("default invitation rule = (%v, %v), want (20, 100)", pool.InvitationFirstPaymentAmount, pool.InvitationConsumptionAmount)
	}
	if pool.PurchasePrice != defaultLotteryPurchasePrice {
		t.Fatalf("default purchase price = %v, want %v", pool.PurchasePrice, defaultLotteryPurchasePrice)
	}
	var total int64
	for _, prize := range pool.Prizes {
		units, ok := lotteryProbabilityUnits(prize.Probability)
		if !ok {
			t.Fatalf("prize %s has invalid probability %v", prize.ID, prize.Probability)
		}
		total += units
	}
	if total != lotteryProbabilityScale {
		t.Fatalf("probability units = %d, want %d", total, lotteryProbabilityScale)
	}
}

func TestValidateLotteryPurchasePrice(t *testing.T) {
	for _, price := range []float64{0.01, 12.5, 30, 1_000_000} {
		if err := validateLotteryPurchasePrice(price); err != nil {
			t.Fatalf("valid purchase price %v rejected: %v", price, err)
		}
	}
	for _, price := range []float64{0, -1, math.NaN(), math.Inf(1), 1_000_000.01, 12.345} {
		if err := validateLotteryPurchasePrice(price); err == nil {
			t.Fatalf("invalid purchase price %v accepted", price)
		}
	}
}

func TestValidateLotteryInvitationRule(t *testing.T) {
	if err := validateLotteryInvitationRule(lotteryInvitationRule{FirstPaymentAmount: 20, ConsumptionAmount: 100}); err != nil {
		t.Fatalf("valid invitation rule rejected: %v", err)
	}
	if err := validateLotteryInvitationRule(lotteryInvitationRule{FirstPaymentAmount: 0, ConsumptionAmount: 100}); err == nil {
		t.Fatal("zero first payment amount must be rejected")
	}
	if err := validateLotteryInvitationRule(lotteryInvitationRule{FirstPaymentAmount: 20, ConsumptionAmount: 0}); err == nil {
		t.Fatal("zero consumption amount must be rejected")
	}
}

func TestValidateLotteryRequestID(t *testing.T) {
	if err := validateLotteryRequestID("short"); err == nil {
		t.Fatal("short idempotency key must be rejected")
	}
	if err := validateLotteryRequestID("lottery-request-123"); err != nil {
		t.Fatalf("valid idempotency key rejected: %v", err)
	}
}

func TestValidateLotteryTicketAdjustment(t *testing.T) {
	validReference := strings.Repeat("a", lotteryTicketSourceRefMaxLength)
	validReason := strings.Repeat("原", 500)
	adjustment := LotteryTicketAdjustment{Operation: "add", Count: 1, Reference: validReference, Reason: validReason}
	if err := validateLotteryTicketAdjustment(42, &adjustment); err != nil {
		t.Fatalf("valid ticket adjustment rejected: %v", err)
	}
	if utf8.RuneCountInString(adjustment.Reason) != 500 {
		t.Fatal("reason should preserve its rune length")
	}
	if err := validateLotteryTicketAdjustment(42, &LotteryTicketAdjustment{Operation: "set", Count: 0, Reference: "ref", Reason: "reason"}); err != nil {
		t.Fatalf("zero target ticket count rejected: %v", err)
	}
	for _, adjustment := range []*LotteryTicketAdjustment{
		{Operation: "add", Count: 0, Reference: "ref", Reason: "reason"},
		{Operation: "set", Count: -1, Reference: "ref", Reason: "reason"},
		{Operation: "subtract", Count: 1, Reference: strings.Repeat("a", lotteryTicketSourceRefMaxLength+1), Reason: "reason"},
		{Operation: "subtract", Count: 1, Reference: "ref", Reason: strings.Repeat("原", 501)},
	} {
		if err := validateLotteryTicketAdjustment(42, adjustment); err == nil {
			t.Fatalf("invalid ticket adjustment accepted: %+v", adjustment)
		}
	}
}

func TestLotteryPurchaseSourceRefFitsLedgerColumn(t *testing.T) {
	const userID int64 = 42
	prefixLength := len("42:")
	requestID := strings.Repeat("a", lotteryTicketSourceRefMaxLength-prefixLength)

	ref, err := lotteryPurchaseSourceRef(userID, requestID)
	if err != nil {
		t.Fatalf("maximum purchase request id rejected: %v", err)
	}
	if len(ref) != lotteryTicketSourceRefMaxLength {
		t.Fatalf("source ref length = %d, want %d", len(ref), lotteryTicketSourceRefMaxLength)
	}

	if _, err := lotteryPurchaseSourceRef(userID, requestID+"a"); err == nil {
		t.Fatal("purchase request id exceeding source_ref must be rejected")
	}
}

func TestComputeLotterySubscriptionPlanValidity(t *testing.T) {
	tests := []struct {
		name string
		days int
		unit string
		want int
	}{
		{name: "day card", days: 1, unit: "day", want: 1},
		{name: "week card", days: 1, unit: "week", want: 7},
		{name: "month card", days: 1, unit: "month", want: 30},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := psComputeValidityDays(tt.days, tt.unit); got != tt.want {
				t.Fatalf("psComputeValidityDays(%d, %q) = %d, want %d", tt.days, tt.unit, got, tt.want)
			}
		})
	}
}

func TestValidateLotteryPrizePoolConfig(t *testing.T) {
	valid := defaultLotteryPrizePoolConfig()
	if err := validateLotteryPrizePoolConfig(valid); err != nil {
		t.Fatalf("default pool should be valid: %v", err)
	}

	withoutPity := LotteryPrizePoolConfig{Prizes: []LotteryPrizeConfig{
		{ID: "none-a", Label: "谢谢参与 A", Type: "none", Probability: 0.5},
		{ID: "none-b", Label: "谢谢参与 B", Type: "none", Probability: 0.5},
	}}
	if err := validateLotteryPrizePoolConfig(withoutPity); err == nil {
		t.Fatal("pool without a pity-eligible reward must be rejected")
	}

	invalidSubscription := LotteryPrizePoolConfig{Prizes: []LotteryPrizeConfig{
		{ID: "none", Label: "谢谢参与", Type: "none", Probability: 0.5},
		{ID: "sub-card", Label: "订阅", Type: "subscription", Probability: 0.5, EligibleForPity: true},
	}}
	if err := validateLotteryPrizePoolConfig(invalidSubscription); err == nil {
		t.Fatal("subscription prize without a plan must be rejected")
	}

	validSubscription := LotteryPrizePoolConfig{
		InvitationFirstPaymentAmount: 20,
		InvitationConsumptionAmount:  100,
		PurchasePrice:                defaultLotteryPurchasePrice,
		Prizes: []LotteryPrizeConfig{
			{ID: "none", Label: "谢谢参与", Type: "none", Probability: 0.5},
			{ID: "sub-card", Label: "日卡", Type: "subscription", SubscriptionPlanID: 1, Probability: 0.5, EligibleForPity: true},
		},
	}
	if err := validateLotteryPrizePoolConfig(validSubscription); err != nil {
		t.Fatalf("subscription prize with a plan rejected: %v", err)
	}

	invalidPurchasePrice := defaultLotteryPrizePoolConfig()
	invalidPurchasePrice.PurchasePrice = 12.345
	if err := validateLotteryPrizePoolConfig(invalidPurchasePrice); err == nil {
		t.Fatal("pool with invalid purchase price must be rejected")
	}

	invalidTotal := defaultLotteryPrizePoolConfig()
	invalidTotal.Prizes[0].Probability = 0.528999
	if err := validateLotteryPrizePoolConfig(invalidTotal); err == nil {
		t.Fatal("pool whose probabilities do not total one must be rejected")
	}
}

func TestDecodeLegacyLotteryPrizePool(t *testing.T) {
	prizes, err := decodeLotteryPrizePool(`[
        {"id":"none","label":"谢谢参与","type":"none","weight":3},
        {"id":"quota-10","label":"$10","type":"balance","amount":10,"weight":7,"eligible_for_pity":true}
    ]`)
	if err != nil {
		t.Fatalf("decode legacy pool: %v", err)
	}
	if len(prizes) != 2 || math.Abs(prizes[0].Probability-0.3) > 1e-9 || math.Abs(prizes[1].Probability-0.7) > 1e-9 {
		t.Fatalf("legacy weights were not converted to probabilities: %#v", prizes)
	}
}

func TestValidateLotteryPrizeCooldown(t *testing.T) {
	valid := defaultLotteryPrizePoolConfig()
	valid.Prizes[1].CooldownSeconds = 60
	if err := validateLotteryPrizePoolConfig(valid); err != nil {
		t.Fatalf("valid cooldown rejected: %v", err)
	}

	invalid := defaultLotteryPrizePoolConfig()
	invalid.Prizes[1].CooldownSeconds = lotteryPrizeCooldownMaxSeconds + 1
	if err := validateLotteryPrizePoolConfig(invalid); err == nil {
		t.Fatal("cooldown exceeding maximum must be rejected")
	}

	nonePrize := defaultLotteryPrizePoolConfig()
	nonePrize.Prizes[0].CooldownSeconds = 60
	if err := validateLotteryPrizePoolConfig(nonePrize); err == nil {
		t.Fatal("non-winning prize cooldown must be rejected")
	}

	withoutNone := defaultLotteryPrizePoolConfig()
	withoutNone.Prizes[0] = LotteryPrizeConfig{ID: "quota-1", Label: "$1", Type: "balance", Amount: 1, Probability: 0.529, EligibleForPity: true}
	if err := validateLotteryPrizePoolConfig(withoutNone); err == nil {
		t.Fatal("pool without a non-winning prize must be rejected")
	}
}

func TestFilterLotteryPrizeCooldowns(t *testing.T) {
	none := lotteryPrize{ID: "none", Type: "none", Weight: 1}
	first := lotteryPrize{ID: "first", Type: "balance", Weight: 1}
	second := lotteryPrize{ID: "second", Type: "balance", Weight: 1}
	normal, pity := filterLotteryPrizeCooldowns([]lotteryPrize{none, first, second}, []lotteryPrize{first, second}, map[string]bool{"first": true})
	if len(normal) != 2 || normal[0].ID != "none" || normal[1].ID != "second" {
		t.Fatalf("normal prizes = %#v", normal)
	}
	if len(pity) != 1 || pity[0].ID != "second" {
		t.Fatalf("pity prizes = %#v", pity)
	}

	normal, pity = filterLotteryPrizeCooldowns([]lotteryPrize{none, first, second}, []lotteryPrize{first, second}, map[string]bool{"first": true, "second": true})
	if len(normal) != 1 || normal[0].ID != "none" || len(pity) != 1 || pity[0].ID != "none" {
		t.Fatalf("fully cooled pools = normal %#v, pity %#v", normal, pity)
	}
}

func TestLotteryBalancePrizeLabel(t *testing.T) {
	if got := lotteryBalancePrizeLabel(10); got != "$10" {
		t.Fatalf("label = %q, want $10", got)
	}
	if got := lotteryBalancePrizeLabel(10.5); got != "$10.5" {
		t.Fatalf("label = %q, want $10.5", got)
	}
}

func TestMaskLotteryWinnerEmail(t *testing.T) {
	if got := maskLotteryWinnerEmail("alice@example.com"); got != "a***e@e*.com" {
		t.Fatalf("maskLotteryWinnerEmail() = %q", got)
	}
	if got := maskLotteryWinnerEmail("bob@mail.example.co.uk"); got != "b***b@m*.e*.c*.uk" {
		t.Fatalf("multi-level maskLotteryWinnerEmail() = %q", got)
	}
	if got := maskLotteryWinnerEmail("x@domain"); got != "x***@d*" {
		t.Fatalf("single-character maskLotteryWinnerEmail() = %q", got)
	}
	if got := maskLotteryWinnerEmail("匿名用户"); got != "匿名用户" {
		t.Fatalf("invalid email should be anonymous, got %q", got)
	}
}

func TestNewLotteryPrizeIDFitsDrawStorage(t *testing.T) {
	id := newLotteryPrizeID()
	if len(id) != lotteryPrizeIDLength {
		t.Fatalf("id length = %d, want %d", len(id), lotteryPrizeIDLength)
	}
	if !lotteryPrizeIDPattern.MatchString(id) {
		t.Fatalf("generated id is invalid: %q", id)
	}
}
