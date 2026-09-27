package service

import (
	"fmt"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestBusinessLedgerMixedWalletUsesCashBasis(t *testing.T) {
	f := BusinessFunds{Paid: decimal.NewFromInt(200), Gift: decimal.NewFromInt(100), Value: decimal.NewFromInt(180)}
	slice := f.Take(decimal.NewFromInt(30))
	require.Equal(t, "20", slice.Paid.String())
	require.Equal(t, "10", slice.Gift.String())
	require.Equal(t, "18", slice.Value.String())
	require.Equal(t, "162", f.Value.String())
	// Proportional promo attribution is a slice of cost, not an extra expense.
	require.Equal(t, "2", decimal.NewFromInt(6).Mul(giftFraction(slice)).Round(8).String())
}
func TestBusinessLedgerFundsConserveEverySource(t *testing.T) {
	f := BusinessFunds{Paid: decimal.RequireFromString("0.12345678"), Gift: decimal.RequireFromString("0.87654322"), Value: decimal.RequireFromString("0.10123456")}
	paid, gift, value := decimal.Zero, decimal.Zero, decimal.Zero
	for i := 0; i < 100; i++ {
		s := f.Take(decimal.RequireFromString("0.01"))
		paid = paid.Add(s.Paid)
		gift = gift.Add(s.Gift)
		value = value.Add(s.Value)
		require.False(t, s.Unknown.IsPositive())
		require.False(t, f.Paid.IsNegative())
		require.False(t, f.Gift.IsNegative())
	}
	require.Equal(t, "0.12345678", paid.String())
	require.Equal(t, "0.87654322", gift.String())
	require.Equal(t, "0.10123456", value.String())
	require.True(t, f.Total().IsZero())
}
func TestBusinessLedgerUnknownOpeningDoesNotBecomeRevenue(t *testing.T) {
	f := BusinessFunds{Paid: decimal.NewFromInt(100), Unknown: decimal.NewFromInt(100), Value: decimal.NewFromInt(80)}
	slice := f.Take(decimal.NewFromInt(20))
	require.Equal(t, "8", slice.Value.String())
	require.Equal(t, "10", slice.Unknown.String())
	p := NewBusinessProjection()
	p.revenue(BusinessEvent{ID: 1, OccurredAt: time.Now()}, slice, "wallet_revenue")
	report := BuildBusinessLedgerReport(p, p.Entries, time.Now().Add(-time.Hour), time.Now().Add(time.Hour), time.UTC, time.Now())
	require.Nil(t, report.Profit)
	require.Equal(t, "8", report.KnownProfit.String())
	require.Equal(t, "unknown", report.Quality.Revenue)
}
func TestBusinessLedgerProcurementMovingAverageIsProspective(t *testing.T) {
	f := BusinessFunds{Paid: decimal.NewFromInt(100), Value: decimal.NewFromInt(20)}
	first := f.Take(decimal.NewFromInt(50))
	require.Equal(t, "10", first.Value.String())
	f.Add(BusinessFunds{Paid: decimal.NewFromInt(50), Value: decimal.NewFromInt(30)})
	second := f.Take(decimal.NewFromInt(50))
	require.Equal(t, "20", second.Value.String())
	require.Equal(t, "10", first.Value.String())
}
func TestBusinessLedgerSubscriptionAccrualAndUnconsumedRevenue(t *testing.T) {
	start := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 30)
	p := NewBusinessProjection()
	p.Terms["1:1"] = &BusinessTerm{Key: "1:1", EventID: 1, GroupID: 9, Start: start, End: end, Value: decimal.NewFromInt(90), Quality: "confirmed"}
	report := BuildBusinessLedgerReport(p, nil, start, start.AddDate(0, 0, 1), time.UTC, end)
	require.Equal(t, "3", report.Revenue.String())
	require.Len(t, report.Models, 1)
	require.Equal(t, "未使用订阅 / 公共费用", report.Models[0].Name)
	full := BuildBusinessLedgerReport(p, nil, start, end, time.UTC, end)
	require.Equal(t, "90", full.Revenue.String())
}
func TestBusinessLedgerNoDoubleCountingGifts(t *testing.T) {
	at := time.Now().Add(-time.Minute)
	p := NewBusinessProjection()
	e := BusinessEvent{ID: 1, OccurredAt: at, UserID: 1, Payload: map[string]any{"account_id": 1, "pool": map[string]any{"id": 1, "mode": "postpaid"}, "rule": map[string]any{"id": 1, "basis": "request", "unit_price": "1", "cny_per_unit": "6", "quality": "contract"}}}
	p.usage(e, BusinessFunds{Paid: decimal.NewFromInt(20), Gift: decimal.NewFromInt(10), Value: decimal.NewFromInt(18)})
	report := BuildBusinessLedgerReport(p, p.Entries, at.Add(-time.Hour), at.Add(time.Hour), time.UTC, time.Now())
	require.Equal(t, "18", report.Revenue.String())
	require.Equal(t, "6", report.UsageCost.String())
	require.Equal(t, "2", report.GiftCost.String())
	require.Equal(t, "12", report.KnownProfit.String())
	require.Nil(t, report.Profit)
	require.Equal(t, "estimated", report.Quality.Cost)
}
func TestBusinessLedgerIdleCostsRemainInTotal(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	p := NewBusinessProjection()
	p.Expenses = []BusinessExpense{{Event: BusinessEvent{ID: 7, Payload: map[string]any{"account_id": 3}}, Amount: decimal.NewFromInt(300), Start: start, End: start.AddDate(0, 0, 30)}}
	report := BuildBusinessLedgerReport(p, nil, start, start.AddDate(0, 0, 1), time.UTC, start.AddDate(0, 0, 30))
	require.Equal(t, "10", report.FixedCost.String())
	require.Equal(t, "-10", report.KnownProfit.String())
	require.Len(t, report.Groups, 1)
	require.Equal(t, "10", report.Groups[0].Cost.String())
}
func TestBusinessLedgerSnapshotPricingIgnoresSaleMultiplier(t *testing.T) {
	data := map[string]any{"input_tokens": "1000000", "output_tokens": "1000000", "rate_multiplier": "999", "total_cost": "1000"}
	rule := map[string]any{"basis": "tokens", "input_price": "2", "output_price": "3"}
	require.Equal(t, "5", businessSupplierUnits(data, rule).String())
}
func TestBusinessLedgerCashRefundUsesRefundDay(t *testing.T) {
	paid := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	refund := paid.AddDate(0, 1, 0)
	p := NewBusinessProjection()
	payload := map[string]any{"id": 1, "status": "PAID", "amount": "100", "pay_amount": "80", "paid_at": paid.Format(time.RFC3339), "currency": "CNY"}
	p.payment(BusinessEvent{ID: 1, OccurredAt: paid, Payload: payload})
	payload["status"] = "REFUNDED"
	payload["refund_amount"] = "100"
	payload["refund_at"] = refund.Format(time.RFC3339)
	p.payment(BusinessEvent{ID: 2, OccurredAt: refund, Payload: payload})
	require.Len(t, p.Entries, 3)
	require.Equal(t, paid, p.Entries[0].At)
	require.Equal(t, refund, p.Entries[1].At)
	require.Equal(t, "80", p.Entries[1].Amount.String())
}
func TestBusinessLedgerAllocationConservesTinyAmounts(t *testing.T) {
	amount := decimal.RequireFromString("0.00000001")
	base := BusinessEntry{Amount: &amount, Detail: map[string]any{}}
	result := allocateBusinessEntry(base, []BusinessEntry{{Credits: decimal.NewFromInt(1)}, {Credits: decimal.NewFromInt(1)}, {Credits: decimal.NewFromInt(1)}}, "")
	total := decimal.Zero
	for _, v := range result {
		total = total.Add(*v.Amount)
	}
	require.True(t, total.Equal(amount))
}

func ledgerEvent(id int64, typ string, at time.Time, data map[string]any) BusinessEvent {
	return BusinessEvent{ID: id, Type: typ, UserID: 1, OccurredAt: at, Payload: data}
}
func TestBusinessLedgerBatchFundingPreservesEachReservation(t *testing.T) {
	p := NewBusinessProjection()
	at := time.Now()
	p.Wallets[1] = &BusinessFunds{Paid: decimal.NewFromInt(100), Value: decimal.NewFromInt(80)}
	p.ApplyTransaction([]BusinessEvent{
		ledgerEvent(1, "batch_operation", at, map[string]any{"batch_id": "one", "action": "hold"}),
		ledgerEvent(2, "wallet", at, map[string]any{"before": "100", "after": "50", "frozen_before": "0", "frozen_after": "50"}),
	})
	p.Wallets[1].Add(BusinessFunds{Gift: decimal.NewFromInt(100)})
	p.ApplyTransaction([]BusinessEvent{
		ledgerEvent(3, "batch_operation", at, map[string]any{"batch_id": "two", "action": "hold"}),
		ledgerEvent(4, "wallet", at, map[string]any{"before": "150", "after": "120", "frozen_before": "50", "frozen_after": "80"}),
	})
	p.ApplyTransaction([]BusinessEvent{
		ledgerEvent(5, "batch_operation", at, map[string]any{"batch_id": "one", "action": "capture"}),
		ledgerEvent(6, "wallet", at, map[string]any{"before": "120", "after": "140", "frozen_before": "80", "frozen_after": "30"}),
		ledgerEvent(7, "usage", at, map[string]any{"batch_id": "one", "billing_applied": true, "actual_supplier_cost_cny": "6"}),
	})
	require.Equal(t, "24", p.Entries[0].Amount.String())
	require.Equal(t, "48", p.Wallets[1].Value.String())
	require.Equal(t, "8", p.BatchFunds["1:two"].Value.String())
	// Releasing the second hold restores exactly the paid/gift mix it reserved.
	p.ApplyTransaction([]BusinessEvent{
		ledgerEvent(8, "batch_operation", at, map[string]any{"batch_id": "two", "action": "release"}),
		ledgerEvent(9, "wallet", at, map[string]any{"before": "140", "after": "170", "frozen_before": "30", "frozen_after": "0"}),
	})
	require.Equal(t, "100", p.Wallets[1].Gift.String())
	require.Equal(t, "56", p.Wallets[1].Value.String())
}

func TestBusinessLedgerRefundReleasesBasisAndOnlyReversesEarnedDifference(t *testing.T) {
	p := NewBusinessProjection()
	at := time.Now()
	p.Wallets[1] = &BusinessFunds{Paid: decimal.NewFromInt(50), Value: decimal.NewFromInt(40)}
	p.ApplyTransaction([]BusinessEvent{ledgerEvent(1, "wallet", at, map[string]any{"before": "50", "after": "0", "context": map[string]any{"order_id": 10, "operation": "refund_reserve"}})})
	require.Equal(t, "40", p.RefundFunds[10].Value.String())
	p.ApplyTransaction([]BusinessEvent{ledgerEvent(2, "payment_orders", at, map[string]any{"id": 10, "status": "REFUNDED", "order_type": "balance", "amount": "100", "pay_amount": "80", "refund_amount": "100", "refund_at": at.Format(time.RFC3339)})})
	report := BuildBusinessLedgerReport(p, p.Entries, at.Add(-time.Hour), at.Add(time.Hour), time.UTC, at)
	require.Equal(t, "80", report.CashRefund.String())
	require.Equal(t, "-40", report.Revenue.String())
	require.Zero(t, report.Quality.MissingCount)
	p.payment(ledgerEvent(3, "payment_orders", at, map[string]any{"id": 10, "status": "REFUNDED", "amount": "100", "pay_amount": "80", "refund_amount": "100", "refund_at": at.Add(time.Minute).Format(time.RFC3339)}))
	require.Len(t, p.Entries, 2)
}

func TestBusinessLedgerRefundFailureRestoresOriginalGiftComposition(t *testing.T) {
	p := NewBusinessProjection()
	at := time.Now()
	p.Wallets[1] = &BusinessFunds{Paid: decimal.NewFromInt(20), Gift: decimal.NewFromInt(10), Value: decimal.NewFromInt(18)}
	p.ApplyTransaction([]BusinessEvent{ledgerEvent(1, "wallet", at, map[string]any{"before": "30", "after": "0", "context": map[string]any{"order_id": 10, "operation": "refund_reserve"}})})
	p.ApplyTransaction([]BusinessEvent{ledgerEvent(2, "wallet", at, map[string]any{"before": "0", "after": "30", "context": map[string]any{"order_id": 10, "operation": "refund_release"}})})
	require.Equal(t, "18", p.Wallets[1].Value.String())
	require.Equal(t, "10", p.Wallets[1].Gift.String())
	require.Empty(t, p.Entries)
}

func TestBusinessLedgerFixedModeRequiresInvoiceAndConservesGroupCosts(t *testing.T) {
	p := NewBusinessProjection()
	at := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	p.usage(ledgerEvent(1, "usage", at, map[string]any{"account_id": 5, "group_id": 7, "total_cost": "1", "pool": map[string]any{"mode": "fixed"}}), BusinessFunds{})
	report := BuildBusinessLedgerReport(p, p.Entries, at, at.AddDate(0, 0, 1), time.UTC, at.AddDate(0, 0, 1))
	require.Nil(t, report.Profit)
	require.Equal(t, "unknown", report.Quality.Cost)
	p.Expenses = append(p.Expenses, BusinessExpense{Event: BusinessEvent{ID: 2, Payload: map[string]any{"account_id": 5}}, Amount: decimal.NewFromInt(10), Start: at, End: at.AddDate(0, 0, 1)})
	report = BuildBusinessLedgerReport(p, p.Entries, at, at.AddDate(0, 0, 1), time.UTC, at.AddDate(0, 0, 1))
	require.NotNil(t, report.Profit)
	require.Equal(t, "10", report.Groups[0].Cost.String())
}

func TestBusinessLedgerSubscriptionReplacementAndRefundDoNotRewriteEarnedIncome(t *testing.T) {
	p := NewBusinessProjection()
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	p.subscription(ledgerEvent(1, "user_subscriptions", start, map[string]any{"id": 4, "term_version": 1, "starts_at": start.Format(time.RFC3339), "expires_at": start.AddDate(0, 0, 30).Format(time.RFC3339), "order": map[string]any{"id": 8, "pay_amount": "90"}}), nil, BusinessFunds{})
	replacement := start.AddDate(0, 0, 10)
	p.subscription(ledgerEvent(2, "user_subscriptions", replacement, map[string]any{"id": 4, "term_version": 2, "starts_at": replacement.Format(time.RFC3339), "expires_at": replacement.AddDate(0, 0, 30).Format(time.RFC3339), "order": map[string]any{"id": 9, "pay_amount": "60"}}), nil, BusinessFunds{})
	report := BuildBusinessLedgerReport(p, nil, start, replacement.AddDate(0, 0, 1), time.UTC, replacement.AddDate(0, 0, 1))
	require.Equal(t, "92", report.Revenue.String()) // 30 earned + 60 termination + 2 new term.
	p.payment(ledgerEvent(3, "payment_orders", replacement.AddDate(0, 0, 1), map[string]any{"id": 9, "order_type": "subscription", "status": "REFUNDED", "amount": "60", "pay_amount": "60", "refund_amount": "60"}))
	report = BuildBusinessLedgerReport(p, p.Entries, start, replacement.AddDate(0, 0, 2), time.UTC, replacement.AddDate(0, 0, 2))
	require.Equal(t, "90", report.Revenue.String())
}

func TestBusinessLedgerMultipleCallsInOneTransactionSplitConsumption(t *testing.T) {
	p := NewBusinessProjection()
	at := time.Now()
	p.Wallets[1] = &BusinessFunds{Paid: decimal.NewFromInt(100), Value: decimal.NewFromInt(80)}
	p.ApplyTransaction([]BusinessEvent{
		ledgerEvent(1, "wallet", at, map[string]any{"before": "100", "after": "70"}),
		ledgerEvent(2, "usage", at, map[string]any{"balance_cost": "10", "actual_supplier_cost_cny": "1", "model": "first"}),
		ledgerEvent(3, "usage", at, map[string]any{"balance_cost": "20", "actual_supplier_cost_cny": "2", "model": "second"}),
	})
	report := BuildBusinessLedgerReport(p, p.Entries, at.Add(-time.Hour), at.Add(time.Hour), time.UTC, at)
	require.Equal(t, "24", report.Revenue.String())
	require.Len(t, report.Models, 2)
	require.Equal(t, "second", report.Models[0].Name)
	require.Equal(t, "16", report.Models[0].Revenue.String())
}

func TestBusinessLedgerExpenseTerminationKeepsAccrualAndRecognizesRemainder(t *testing.T) {
	p := NewBusinessProjection()
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	stop := start.AddDate(0, 0, 10)
	p.ApplyTransaction([]BusinessEvent{ledgerEvent(1, "expense", start, map[string]any{
		"amount_cny": "300", "account_id": 9,
		"starts_at": start.Format(time.RFC3339), "ends_at": start.AddDate(0, 0, 30).Format(time.RFC3339),
	})})
	p.ApplyTransaction([]BusinessEvent{ledgerEvent(2, "expense_stop", stop, map[string]any{"source_event_id": 1})})
	before := BuildBusinessLedgerReport(p, p.Entries, start, start.AddDate(0, 0, 5), time.UTC, start.AddDate(0, 0, 40))
	require.Equal(t, "50", before.FixedCost.String())
	require.Equal(t, "0", before.OperatingCost.String())
	after := BuildBusinessLedgerReport(p, p.Entries, stop, stop.AddDate(0, 0, 1), time.UTC, start.AddDate(0, 0, 40))
	require.Equal(t, "200", after.OperatingCost.String())
	require.Equal(t, "0", after.FixedCost.String())
	full := BuildBusinessLedgerReport(p, p.Entries, start, start.AddDate(0, 0, 40), time.UTC, start.AddDate(0, 0, 40))
	require.Equal(t, "300", full.CashOut.String())
	require.Equal(t, "-300", full.KnownProfit.String())
	require.True(t, full.PrepaidExpense.IsZero())
}

func TestBusinessLedgerPartiallyUnknownSubscriptionShowsKnownIncomeAndUniqueGaps(t *testing.T) {
	p := NewBusinessProjection()
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	terminated := start.AddDate(0, 0, 10)
	p.Terms["4:1"] = &BusinessTerm{Key: "4:1", EventID: 1, SubscriptionID: 4, Version: 1,
		Start: start, End: start.AddDate(0, 0, 30), Value: decimal.NewFromInt(60), Quality: "unknown", TerminatedAt: &terminated}
	report := BuildBusinessLedgerReport(p, nil, start, start.AddDate(0, 0, 11), time.UTC, start.AddDate(0, 0, 11))
	require.Equal(t, "60", report.Revenue.String())
	require.Nil(t, report.Profit)
	require.Equal(t, "unknown", report.Quality.Revenue)
	ids := map[string]bool{}
	for _, e := range report.Entries {
		require.False(t, ids[e.ID], "duplicate entry ID: %s", e.ID)
		ids[e.ID] = true
	}
}

func TestBusinessLedgerSupplierUnitsSeparateCacheDurationsAndMedia(t *testing.T) {
	rule := map[string]any{"basis": "tokens", "input_price": "2", "output_price": "3", "cache_read_price": "0.5", "cache_write_price": "4", "cache_write_1h_price": "8"}
	data := map[string]any{"input_tokens": "1000000", "output_tokens": "2000000", "cache_read_tokens": "1000000", "cache_creation_tokens": "3000000", "cache_creation_1h_tokens": "1000000", "rate_multiplier": "999"}
	require.Equal(t, "24.5", businessSupplierUnits(data, rule).String())
	require.Equal(t, "1.2", businessSupplierUnits(map[string]any{"image_count": 3}, map[string]any{"basis": "image", "unit_price": "0.4"}).String())
	require.Equal(t, "2.5", businessSupplierUnits(map[string]any{"video_duration_seconds": 5}, map[string]any{"basis": "video_second", "unit_price": "0.5"}).String())
}

func TestBusinessLedgerBalanceSubscriptionTransfersBasisWithoutCashIncome(t *testing.T) {
	p := NewBusinessProjection()
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	p.Wallets[1] = &BusinessFunds{Paid: decimal.NewFromInt(100), Gift: decimal.NewFromInt(100), Value: decimal.NewFromInt(80)}
	p.ApplyTransaction([]BusinessEvent{
		ledgerEvent(1, "wallet", start, map[string]any{"before": "200", "after": "100"}),
		ledgerEvent(2, "payment_orders", start, map[string]any{"id": 3, "payment_type": "balance", "order_type": "subscription", "paid_at": start.Format(time.RFC3339), "pay_amount": "100"}),
		ledgerEvent(3, "user_subscriptions", start, map[string]any{"id": 4, "term_version": 1, "starts_at": start.Format(time.RFC3339), "expires_at": start.AddDate(0, 0, 10).Format(time.RFC3339), "order": map[string]any{"id": 3, "payment_type": "balance", "plan_id": 8, "plan_name": "十日卡"}}),
	})
	p.usage(ledgerEvent(4, "usage", start.Add(time.Hour), map[string]any{"subscription_id": 4, "subscription_term_version": 1, "actual_cost": "10", "total_cost": "1", "actual_supplier_cost_cny": "1"}), BusinessFunds{})
	report := BuildBusinessLedgerReport(p, p.Entries, start, start.AddDate(0, 0, 1), time.UTC, start.AddDate(0, 0, 1))
	require.True(t, report.CashIn.IsZero())
	require.Equal(t, "4", report.Revenue.String())
	require.Equal(t, "40", report.WalletDeferred.String())
	require.Equal(t, "36", report.SubscriptionDeferred.String())
	require.Equal(t, "0.5", report.GiftCost.String())
	require.Len(t, report.Plans, 1)
	require.Equal(t, "8", report.Plans[0].Key)
	require.Equal(t, "1", report.Plans[0].Cost.String())
}

func TestBusinessLedgerInvoiceChangesRequireReconciliation(t *testing.T) {
	p := NewBusinessProjection()
	at := time.Now()
	p.usage(ledgerEvent(1, "usage", at, map[string]any{"actual_supplier_cost_cny": "8"}), BusinessFunds{})
	p.Bills = []BusinessEvent{{ID: 2, Payload: map[string]any{
		"covered_entry_ids": []string{"1:usage_cost:0"}, "covered_event_ids": []int64{1},
		"covered_entry_amounts": map[string]any{"1:usage_cost:0": "6"},
	}}}
	report := BuildBusinessLedgerReport(p, p.Entries, at.Add(-time.Hour), at.Add(time.Hour), time.UTC, at)
	require.Nil(t, report.Profit)
	require.Equal(t, "unknown", report.Quality.Cost)
}

func TestBusinessLedgerSharedAccountCostsReconcileWithPublicAndIdleCosts(t *testing.T) {
	p := NewBusinessProjection()
	at := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	for i, weight := range []string{"1", "3"} {
		p.usage(ledgerEvent(int64(i+1), "usage", at.Add(time.Hour), map[string]any{"account_id": 5, "group_id": i + 7, "model": fmt.Sprintf("model-%d", i), "total_cost": weight, "actual_supplier_cost_cny": "0", "rate_multiplier": 999}), BusinessFunds{})
	}
	p.Expenses = []BusinessExpense{
		{Event: ledgerEvent(3, "expense", at, map[string]any{"account_id": 5}), Amount: decimal.NewFromInt(40), Start: at, End: at.AddDate(0, 0, 1)},
		{Event: ledgerEvent(4, "expense", at, map[string]any{"account_id": 6}), Amount: decimal.NewFromInt(10), Start: at, End: at.AddDate(0, 0, 1)},
		{Event: ledgerEvent(5, "expense", at, map[string]any{}), Amount: decimal.NewFromInt(20), Start: at, End: at.AddDate(0, 0, 1)},
	}
	report := BuildBusinessLedgerReport(p, p.Entries, at, at.AddDate(0, 0, 1), time.UTC, at.AddDate(0, 0, 1))
	sum := decimal.Zero
	costs := map[string]string{}
	for _, g := range report.Groups {
		sum = sum.Add(g.Cost)
		costs[g.Key] = g.Cost.String()
	}
	require.Equal(t, "70", sum.String())
	require.Equal(t, "10", costs["7"])
	require.Equal(t, "30", costs["8"])
	require.Equal(t, "30", costs["0"])
	require.Equal(t, "-70", report.Profit.String())
}

func TestBusinessLedgerTicketPurchaseDefersRevenueUntilUsed(t *testing.T) {
	p := NewBusinessProjection()
	at := time.Now()
	p.Wallets[1] = &BusinessFunds{Paid: decimal.NewFromInt(20), Gift: decimal.NewFromInt(10), Value: decimal.NewFromInt(18)}
	p.ApplyTransaction([]BusinessEvent{
		ledgerEvent(1, "wallet", at, map[string]any{"before": "30", "after": "0"}),
		ledgerEvent(2, "balance_transactions", at, map[string]any{"transaction_type": "lottery_ticket_purchase"}),
		ledgerEvent(3, "lottery_ticket_ledger", at, map[string]any{"id": 9, "source_type": "purchase", "remaining": "1"}),
	})
	report := BuildBusinessLedgerReport(p, p.Entries, at.Add(-time.Hour), at.Add(time.Hour), time.UTC, at)
	require.True(t, report.Revenue.IsZero())
	require.Equal(t, "18", report.WalletDeferred.String())
	p.ApplyTransaction([]BusinessEvent{ledgerEvent(4, "lottery_ticket_ledger", at, map[string]any{"id": 9, "source_type": "purchase", "remaining": "0"})})
	report = BuildBusinessLedgerReport(p, p.Entries, at.Add(-time.Hour), at.Add(time.Hour), time.UTC, at)
	require.Equal(t, "18", report.Revenue.String())
	require.True(t, report.WalletDeferred.IsZero())
}

func TestBusinessLedgerRechargeSettlesOverdraftBeforeDeferringNewCredits(t *testing.T) {
	p := NewBusinessProjection()
	at := time.Now()
	p.ApplyTransaction([]BusinessEvent{
		ledgerEvent(1, "wallet", at, map[string]any{"before": "-5", "after": "95"}),
		ledgerEvent(2, "receipt", at, map[string]any{"apply_balance": true, "amount_cny": "80", "credits": "100"}),
	})
	report := BuildBusinessLedgerReport(p, p.Entries, at.Add(-time.Hour), at.Add(time.Hour), time.UTC, at)
	require.Equal(t, "80", report.CashIn.String())
	require.Equal(t, "4", report.Revenue.String())
	require.Equal(t, "76", report.WalletDeferred.String())
	require.Equal(t, "95", p.Wallets[1].Paid.String())
}

func TestBusinessLedgerActualCostStillConsumesPrepaidInventory(t *testing.T) {
	p := NewBusinessProjection()
	p.Pools[1] = &BusinessFunds{Paid: decimal.NewFromInt(100), Value: decimal.NewFromInt(20), Sources: []int64{10}}
	p.usage(ledgerEvent(1, "usage", time.Now(), map[string]any{
		"actual_supplier_cost_cny": "10",
		"pool":                     map[string]any{"id": 1, "mode": "prepaid"},
		"rule":                     map[string]any{"basis": "request", "unit_price": "50"},
	}), BusinessFunds{})
	require.Equal(t, "50", p.Pools[1].Paid.String())
	require.Equal(t, "10", p.Pools[1].Value.String())
}
