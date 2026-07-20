package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestPaymentDashboardExcludesBalanceConsumptionAndSeparatesCurrencies(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	user, err := client.User.Create().
		SetEmail("payment-dashboard@example.com").
		SetPasswordHash("hash").
		SetUsername("payment-dashboard").
		SetStatus(StatusActive).
		Save(ctx)
	require.NoError(t, err)

	now := time.Now()
	createDashboardPaymentOrder(t, ctx, client, user.ID, "alipay", "CNY", OrderStatusCompleted, 20, now, 1)
	createDashboardPaymentOrder(t, ctx, client, user.ID, internalBalancePaymentType, "CNY", OrderStatusCompleted, 200, now, 2)
	createDashboardPaymentOrder(t, ctx, client, user.ID, "stripe", "USD", OrderStatusCompleted, 10, now, 3)
	createDashboardPaymentOrder(t, ctx, client, user.ID, "stripe", "USD", OrderStatusPending, 15, now, 4)
	createDashboardPaymentOrder(t, ctx, client, user.ID, internalBalancePaymentType, "CNY", OrderStatusPending, 300, now, 5)

	svc := &PaymentService{entClient: client}
	cnyStats, err := svc.GetDashboardStats(ctx, 30, "")
	require.NoError(t, err)
	require.Equal(t, "CNY", cnyStats.Currency)
	require.Equal(t, []string{"CNY", "USD"}, cnyStats.Currencies)
	require.Equal(t, 1, cnyStats.TotalCount)
	require.Equal(t, 20.0, cnyStats.TotalAmount)
	require.Zero(t, cnyStats.PendingOrders)
	require.Len(t, cnyStats.PaymentMethods, 1)
	require.Equal(t, "alipay", cnyStats.PaymentMethods[0].Type)

	usdStats, err := svc.GetDashboardStats(ctx, 30, "usd")
	require.NoError(t, err)
	require.Equal(t, "USD", usdStats.Currency)
	require.Equal(t, 1, usdStats.TotalCount)
	require.Equal(t, 10.0, usdStats.TotalAmount)
	require.Equal(t, 1, usdStats.PendingOrders)
	require.Len(t, usdStats.PaymentMethods, 1)
	require.Equal(t, "stripe", usdStats.PaymentMethods[0].Type)
}

func TestPaymentDashboardRejectsInvalidCurrency(t *testing.T) {
	svc := &PaymentService{entClient: newPaymentConfigServiceTestClient(t)}
	_, err := svc.GetDashboardStats(context.Background(), 30, "US")
	require.Error(t, err)
	require.Equal(t, "INVALID_CURRENCY", infraerrors.Reason(err))
}

func createDashboardPaymentOrder(
	t *testing.T,
	ctx context.Context,
	client *dbent.Client,
	userID int64,
	paymentType string,
	currency string,
	status string,
	payAmount float64,
	at time.Time,
	sequence int,
) {
	t.Helper()
	builder := client.PaymentOrder.Create().
		SetUserID(userID).
		SetUserEmail("payment-dashboard@example.com").
		SetUserName("payment-dashboard").
		SetAmount(payAmount).
		SetPayAmount(payAmount).
		SetRechargeCode("").
		SetOutTradeNo(fmt.Sprintf("stats-%d-%d", at.UnixNano(), sequence)).
		SetPaymentType(paymentType).
		SetPaymentTradeNo(fmt.Sprintf("trade-%d", sequence)).
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(status).
		SetExpiresAt(at.Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("test")
	if status != OrderStatusPending {
		builder.SetPaidAt(at)
	}
	if currency != payment.DefaultPaymentCurrency {
		builder.SetProviderSnapshot(map[string]any{"schema_version": 2, "currency": currency})
	}
	_, err := builder.Save(ctx)
	require.NoError(t, err)
}
