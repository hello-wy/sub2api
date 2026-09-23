package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type planPurchaseCooldownCheck struct {
	Client *dbent.Client
	UserID int64
	Plan   *dbent.SubscriptionPlan
	Now    time.Time
}

const secondsPerHour int64 = 60 * 60

func checkPlanPurchaseCooldown(ctx context.Context, check planPurchaseCooldownCheck) error {
	if check.Plan.RepurchaseCooldownHours == 0 {
		return nil
	}
	latest, err := check.Client.PaymentOrder.Query().
		Where(
			paymentorder.UserIDEQ(check.UserID),
			paymentorder.PlanIDEQ(check.Plan.ID),
			paymentorder.OrderTypeEQ(payment.OrderTypeSubscription),
			paymentorder.CompletedAtNotNil(),
		).
		Order(paymentorder.ByCompletedAt(sql.OrderDesc())).
		First(ctx)
	if dbent.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("query latest plan purchase: %w", err)
	}

	availableAt := addPlanCooldownHours(*latest.CompletedAt, check.Plan.RepurchaseCooldownHours)
	if !check.Now.Before(availableAt) {
		return nil
	}
	remainingSeconds := availableAt.Unix() - check.Now.Unix()
	if availableAt.Nanosecond() > check.Now.Nanosecond() {
		remainingSeconds++
	}
	return infraerrors.TooManyRequests("PLAN_PURCHASE_COOLDOWN", "plan purchase is cooling down").
		WithMetadata(map[string]string{
			"available_at":      availableAt.UTC().Format(time.RFC3339),
			"remaining_seconds": strconv.FormatInt(remainingSeconds, 10),
		})
}

func addPlanCooldownHours(start time.Time, hours int) time.Time {
	seconds := int64(hours) * secondsPerHour
	return time.Unix(start.Unix()+seconds, int64(start.Nanosecond())).In(start.Location())
}
