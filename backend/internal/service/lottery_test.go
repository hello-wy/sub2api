package service

import (
	"context"
	"math"
	"testing"
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
		t.Fatal("subscription prize without a group must be rejected")
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

func TestLotteryBalancePrizeLabel(t *testing.T) {
	if got := lotteryBalancePrizeLabel(10); got != "$10" {
		t.Fatalf("label = %q, want $10", got)
	}
	if got := lotteryBalancePrizeLabel(10.5); got != "$10.5" {
		t.Fatalf("label = %q, want $10.5", got)
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
