package service

import (
	"context"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/userattributevalue"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/require"
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

func TestResetExpiredWeeklyLoyaltyPointsClearsOnlyPreviousWeek(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := &PaymentService{entClient: client}
	defs, configured, err := svc.ensureLoyaltyAttributeDefinitions(ctx)
	require.NoError(t, err)
	require.True(t, configured)

	now := time.Date(2026, 7, 22, 12, 0, 0, 0, timezone.Location())
	weekStart := timezone.StartOfWeek(now)
	oldUpdatedAt := weekStart.Add(-time.Hour)
	currentUpdatedAt := weekStart.Add(time.Hour)

	oldWeeklyUser := createPaymentLoyaltyTestUser(t, client, "old-weekly@example.com")
	currentWeeklyUser := createPaymentLoyaltyTestUser(t, client, "current-weekly@example.com")
	permanentUser := createPaymentLoyaltyTestUser(t, client, "permanent@example.com")

	createPaymentLoyaltyTestValue(t, client, oldWeeklyUser.ID, defs[LoyaltyWeeklyPointsAttributeKey].ID, "120", oldUpdatedAt)
	createPaymentLoyaltyTestValue(t, client, currentWeeklyUser.ID, defs[LoyaltyWeeklyPointsAttributeKey].ID, "80", currentUpdatedAt)
	createPaymentLoyaltyTestValue(t, client, permanentUser.ID, defs[LoyaltyPermanentPointsAttributeKey].ID, "500", oldUpdatedAt)

	updated, err := svc.resetExpiredWeeklyLoyaltyPoints(ctx, now)
	require.NoError(t, err)
	require.Equal(t, int64(1), updated)

	require.Equal(t, "0", readPaymentLoyaltyTestValue(t, client, oldWeeklyUser.ID, defs[LoyaltyWeeklyPointsAttributeKey].ID))
	require.Equal(t, "80", readPaymentLoyaltyTestValue(t, client, currentWeeklyUser.ID, defs[LoyaltyWeeklyPointsAttributeKey].ID))
	require.Equal(t, "500", readPaymentLoyaltyTestValue(t, client, permanentUser.ID, defs[LoyaltyPermanentPointsAttributeKey].ID))
}

func createPaymentLoyaltyTestUser(t *testing.T, client *dbent.Client, email string) *dbent.User {
	t.Helper()

	user, err := client.User.Create().
		SetEmail(email).
		SetPasswordHash("hash").
		SetUsername(email).
		Save(context.Background())
	require.NoError(t, err)
	return user
}

func createPaymentLoyaltyTestValue(t *testing.T, client *dbent.Client, userID, attributeID int64, value string, updatedAt time.Time) {
	t.Helper()

	_, err := client.UserAttributeValue.Create().
		SetUserID(userID).
		SetAttributeID(attributeID).
		SetValue(value).
		SetCreatedAt(updatedAt).
		SetUpdatedAt(updatedAt).
		Save(context.Background())
	require.NoError(t, err)
}

func readPaymentLoyaltyTestValue(t *testing.T, client *dbent.Client, userID, attributeID int64) string {
	t.Helper()

	value, err := client.UserAttributeValue.Query().
		Where(
			userattributevalue.UserIDEQ(userID),
			userattributevalue.AttributeIDEQ(attributeID),
		).
		Only(context.Background())
	require.NoError(t, err)
	return value.Value
}
