package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// --- Dashboard & Analytics ---

const internalBalancePaymentType = "balance"

func (s *PaymentService) GetDashboardStats(ctx context.Context, days int, requestedCurrency string) (*DashboardStats, error) {
	if days <= 0 {
		days = 30
	}
	requestedCurrency = strings.TrimSpace(requestedCurrency)
	if requestedCurrency != "" {
		var err error
		requestedCurrency, err = payment.NormalizePaymentCurrency(requestedCurrency)
		if err != nil {
			return nil, infraerrors.BadRequest("INVALID_CURRENCY", err.Error())
		}
	}
	now := time.Now()
	since := now.AddDate(0, 0, -days)
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	paidStatuses := []string{OrderStatusCompleted, OrderStatusPaid, OrderStatusRecharging}

	orders, err := s.entClient.PaymentOrder.Query().
		Where(
			paymentorder.StatusIn(paidStatuses...),
			paymentorder.PaidAtGTE(since),
			paymentorder.PaymentTypeNEQ(internalBalancePaymentType),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}

	currencies := paymentOrderCurrencies(orders)
	selectedCurrency := selectDashboardCurrency(requestedCurrency, currencies)
	currencies = includeDashboardCurrency(currencies, selectedCurrency)
	selectedOrders := filterPaymentOrdersByCurrency(orders, selectedCurrency)
	st := &DashboardStats{Currency: selectedCurrency, Currencies: currencies}
	computeBasicStats(st, selectedOrders, todayStart)

	pendingOrders, err := s.entClient.PaymentOrder.Query().
		Where(
			paymentorder.StatusEQ(OrderStatusPending),
			paymentorder.PaymentTypeNEQ(internalBalancePaymentType),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}
	st.PendingOrders = len(filterPaymentOrdersByCurrency(pendingOrders, selectedCurrency))

	st.DailySeries = buildDailySeries(selectedOrders, since, days)
	st.PaymentMethods = buildMethodDistribution(selectedOrders)
	st.TopUsers = buildTopUsers(selectedOrders)

	return st, nil
}

func paymentOrderCurrencies(orders []*dbent.PaymentOrder) []string {
	seen := make(map[string]struct{})
	for _, order := range orders {
		if order == nil {
			continue
		}
		seen[PaymentOrderCurrency(order)] = struct{}{}
	}
	currencies := make([]string, 0, len(seen))
	for currency := range seen {
		currencies = append(currencies, currency)
	}
	sort.Strings(currencies)
	return currencies
}

func selectDashboardCurrency(requested string, available []string) string {
	if requested != "" {
		return requested
	}
	for _, currency := range available {
		if currency == payment.DefaultPaymentCurrency {
			return currency
		}
	}
	if len(available) > 0 {
		return available[0]
	}
	return payment.DefaultPaymentCurrency
}

func includeDashboardCurrency(currencies []string, selected string) []string {
	for _, currency := range currencies {
		if currency == selected {
			return currencies
		}
	}
	currencies = append(currencies, selected)
	sort.Strings(currencies)
	return currencies
}

func filterPaymentOrdersByCurrency(orders []*dbent.PaymentOrder, currency string) []*dbent.PaymentOrder {
	filtered := make([]*dbent.PaymentOrder, 0, len(orders))
	for _, order := range orders {
		if order != nil && PaymentOrderCurrency(order) == currency {
			filtered = append(filtered, order)
		}
	}
	return filtered
}

func computeBasicStats(st *DashboardStats, orders []*dbent.PaymentOrder, todayStart time.Time) {
	var totalAmount, todayAmount float64
	var todayCount int
	for _, o := range orders {
		totalAmount += o.PayAmount
		if o.PaidAt != nil && !o.PaidAt.Before(todayStart) {
			todayAmount += o.PayAmount
			todayCount++
		}
	}
	st.TotalAmount = math.Round(totalAmount*100) / 100
	st.TodayAmount = math.Round(todayAmount*100) / 100
	st.TotalCount = len(orders)
	st.TodayCount = todayCount
	if st.TotalCount > 0 {
		st.AvgAmount = math.Round(totalAmount/float64(st.TotalCount)*100) / 100
	}
}

func buildDailySeries(orders []*dbent.PaymentOrder, since time.Time, days int) []DailyStats {
	dailyMap := make(map[string]*DailyStats)
	for _, o := range orders {
		if o.PaidAt == nil {
			continue
		}
		date := o.PaidAt.Format("2006-01-02")
		ds, ok := dailyMap[date]
		if !ok {
			ds = &DailyStats{Date: date}
			dailyMap[date] = ds
		}
		ds.Amount += o.PayAmount
		ds.Count++
	}
	series := make([]DailyStats, 0, days)
	for i := 0; i < days; i++ {
		date := since.AddDate(0, 0, i+1).Format("2006-01-02")
		if ds, ok := dailyMap[date]; ok {
			ds.Amount = math.Round(ds.Amount*100) / 100
			series = append(series, *ds)
		} else {
			series = append(series, DailyStats{Date: date})
		}
	}
	return series
}

func buildMethodDistribution(orders []*dbent.PaymentOrder) []PaymentMethodStat {
	methodMap := make(map[string]*PaymentMethodStat)
	for _, o := range orders {
		ms, ok := methodMap[o.PaymentType]
		if !ok {
			ms = &PaymentMethodStat{Type: o.PaymentType}
			methodMap[o.PaymentType] = ms
		}
		ms.Amount += o.PayAmount
		ms.Count++
	}
	methods := make([]PaymentMethodStat, 0, len(methodMap))
	for _, ms := range methodMap {
		ms.Amount = math.Round(ms.Amount*100) / 100
		methods = append(methods, *ms)
	}
	return methods
}

func buildTopUsers(orders []*dbent.PaymentOrder) []TopUserStat {
	userMap := make(map[int64]*TopUserStat)
	for _, o := range orders {
		us, ok := userMap[o.UserID]
		if !ok {
			us = &TopUserStat{UserID: o.UserID, Email: o.UserEmail}
			userMap[o.UserID] = us
		}
		us.Amount += o.PayAmount
	}
	userList := make([]*TopUserStat, 0, len(userMap))
	for _, us := range userMap {
		us.Amount = math.Round(us.Amount*100) / 100
		userList = append(userList, us)
	}
	sort.Slice(userList, func(i, j int) bool {
		return userList[i].Amount > userList[j].Amount
	})
	limit := topUsersLimit
	if len(userList) < limit {
		limit = len(userList)
	}
	result := make([]TopUserStat, 0, limit)
	for i := 0; i < limit; i++ {
		result = append(result, *userList[i])
	}
	return result
}

// --- Audit Logs ---

func (s *PaymentService) writeAuditLog(ctx context.Context, oid int64, action, op string, detail map[string]any) {
	dj, _ := json.Marshal(detail)
	_, err := s.entClient.PaymentAuditLog.Create().SetOrderID(strconv.FormatInt(oid, 10)).SetAction(action).SetDetail(string(dj)).SetOperator(op).Save(ctx)
	if err != nil {
		slog.Error("audit log failed", "orderID", oid, "action", action, "error", err)
	}
}

func (s *PaymentService) GetOrderAuditLogs(ctx context.Context, oid int64) ([]*dbent.PaymentAuditLog, error) {
	return s.entClient.PaymentAuditLog.Query().Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(oid, 10))).Order(paymentauditlog.ByCreatedAt()).All(ctx)
}
