package service

import (
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
)

func TestResolvePaymentLoyaltyRuleBoundaries(t *testing.T) {
	t.Parallel()

	if got := resolvePaymentLoyaltyRule(19, paymentWeeklyLoyaltyRules); got != nil {
		t.Fatalf("weekly 19 points rule = %+v, want nil", got)
	}
	if got := resolvePaymentLoyaltyRule(20, paymentWeeklyLoyaltyRules); got == nil || got.Level != "L1" || got.Discount != 2 {
		t.Fatalf("weekly 20 points rule = %+v, want L1 2%%", got)
	}
	if got := resolvePaymentLoyaltyRule(800, paymentWeeklyLoyaltyRules); got == nil || got.Level != "L4" || got.Discount != 8 {
		t.Fatalf("weekly 800 points rule = %+v, want L4 8%%", got)
	}
	if got := resolvePaymentLoyaltyRule(799, paymentPermanentLoyaltyRules); got != nil {
		t.Fatalf("permanent 799 points rule = %+v, want nil", got)
	}
	if got := resolvePaymentLoyaltyRule(800, paymentPermanentLoyaltyRules); got == nil || got.Level != "L2" || got.Discount != 4 {
		t.Fatalf("permanent 800 points rule = %+v, want L2 4%%", got)
	}
}

func TestBetterPaymentLoyaltyRulePrefersHigherDiscount(t *testing.T) {
	t.Parallel()

	weekly := resolvePaymentLoyaltyRule(800, paymentWeeklyLoyaltyRules)
	permanent := resolvePaymentLoyaltyRule(800, paymentPermanentLoyaltyRules)

	if got := betterPaymentLoyaltyRule(weekly, permanent); got != weekly {
		t.Fatalf("best rule = %+v, want weekly L4", got)
	}
}

func TestPaymentLoyaltySnapshotCarriesPointsDelta(t *testing.T) {
	t.Parallel()

	info := &PaymentLoyaltyInfo{
		Enabled:           true,
		WeeklyPoints:      820,
		PermanentPoints:   1200,
		WeeklyDiscount:    8,
		PermanentDiscount: 4,
		DiscountPercent:   8,
		DiscountScope:     "weekly",
		DiscountLevel:     "L4",
	}
	snapshot := buildPaymentLoyaltySnapshot(info, 100, 92, 100, "CNY")
	order := &dbent.PaymentOrder{
		ProviderSnapshot: map[string]any{
			paymentLoyaltyProviderSnapshot: snapshot,
		},
	}

	if got := paymentLoyaltyPointsDeltaFromOrder(order); got != 100 {
		t.Fatalf("points delta = %v, want 100", got)
	}
	if got := snapshot["weekly_attribute"]; got != LoyaltyWeeklyPointsAttributeKey {
		t.Fatalf("weekly attribute = %v, want %s", got, LoyaltyWeeklyPointsAttributeKey)
	}
	if got := snapshot["permanent_attribute"]; got != LoyaltyPermanentPointsAttributeKey {
		t.Fatalf("permanent attribute = %v, want %s", got, LoyaltyPermanentPointsAttributeKey)
	}
	if got := snapshot["discount_amount"]; got != 8.0 {
		t.Fatalf("discount amount = %v, want 8", got)
	}
}
