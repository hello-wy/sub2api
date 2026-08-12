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
	"github.com/Wei-Shaw/sub2api/ent/subscriptionplan"
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
	subscriptionOrders, err := s.entClient.PaymentOrder.Query().
		Where(
			paymentorder.StatusIn(paidStatuses...),
			paymentorder.PaidAtGTE(since),
			paymentorder.OrderTypeEQ(payment.OrderTypeSubscription),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}
	planNames, err := s.subscriptionPlanNames(ctx, subscriptionOrders)
	if err != nil {
		return nil, err
	}
	st.SubscriptionPlans = buildSubscriptionPlanDistribution(subscriptionOrders, planNames)

	return st, nil
}

func (s *PaymentService) subscriptionPlanNames(ctx context.Context, orders []*dbent.PaymentOrder) (map[int64]string, error) {
	planIDs := make(map[int64]struct{})
	for _, order := range orders {
		if order != nil && order.OrderType == payment.OrderTypeSubscription && order.PlanID != nil {
			planIDs[*order.PlanID] = struct{}{}
		}
	}
	if len(planIDs) == 0 {
		return map[int64]string{}, nil
	}
	ids := make([]int64, 0, len(planIDs))
	for id := range planIDs {
		ids = append(ids, id)
	}
	plans, err := s.entClient.SubscriptionPlan.Query().Where(subscriptionplan.IDIn(ids...)).All(ctx)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(plans))
	for _, plan := range plans {
		names[plan.ID] = plan.Name
	}
	return names, nil
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
	st.TotalAmount = make(CurrencyAmounts)
	st.TodayAmount = make(CurrencyAmounts)
	st.AvgAmount = make(CurrencyAmounts)
	currencyCounts := make(map[string]int)
	var todayCount int
	for _, o := range orders {
		currency := PaymentOrderCurrency(o)
		st.TotalAmount[currency] += o.PayAmount
		currencyCounts[currency]++
		if o.PaidAt != nil && !o.PaidAt.Before(todayStart) {
			st.TodayAmount[currency] += o.PayAmount
			todayCount++
		}
	}
	st.TotalCount = len(orders)
	st.TodayCount = todayCount
	for currency, totalAmount := range st.TotalAmount {
		st.AvgAmount[currency] = roundAmount(totalAmount / float64(currencyCounts[currency]))
	}
	roundCurrencyAmounts(st.TotalAmount)
	roundCurrencyAmounts(st.TodayAmount)
}

func buildSubscriptionPlanDistribution(orders []*dbent.PaymentOrder, planNames map[int64]string) []SubscriptionPlanPurchaseStat {
	counts := make(map[int64]int)
	for _, order := range orders {
		if order == nil || order.OrderType != payment.OrderTypeSubscription || order.PlanID == nil {
			continue
		}
		counts[*order.PlanID]++
	}
	plans := make([]SubscriptionPlanPurchaseStat, 0, len(counts))
	for planID, count := range counts {
		plans = append(plans, SubscriptionPlanPurchaseStat{PlanID: planID, PlanName: planNames[planID], Count: count})
	}
	sort.Slice(plans, func(i, j int) bool {
		if plans[i].Count != plans[j].Count {
			return plans[i].Count > plans[j].Count
		}
		return plans[i].PlanID < plans[j].PlanID
	})
	return plans
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
			ds = &DailyStats{Date: date, Amount: make(CurrencyAmounts)}
			dailyMap[date] = ds
		}
		ds.Amount[PaymentOrderCurrency(o)] += o.PayAmount
		ds.Count++
	}
	series := make([]DailyStats, 0, days)
	for i := 0; i < days; i++ {
		date := since.AddDate(0, 0, i+1).Format("2006-01-02")
		if ds, ok := dailyMap[date]; ok {
			roundCurrencyAmounts(ds.Amount)
			series = append(series, *ds)
		} else {
			series = append(series, DailyStats{Date: date, Amount: make(CurrencyAmounts)})
		}
	}
	return series
}

func buildMethodDistribution(orders []*dbent.PaymentOrder) []PaymentMethodStat {
	methodMap := make(map[string]*PaymentMethodStat)
	for _, o := range orders {
		ms, ok := methodMap[o.PaymentType]
		if !ok {
			ms = &PaymentMethodStat{Type: o.PaymentType, Amount: make(CurrencyAmounts)}
			methodMap[o.PaymentType] = ms
		}
		ms.Amount[PaymentOrderCurrency(o)] += o.PayAmount
		ms.Count++
	}
	methods := make([]PaymentMethodStat, 0, len(methodMap))
	for _, ms := range methodMap {
		roundCurrencyAmounts(ms.Amount)
		methods = append(methods, *ms)
	}
	sort.Slice(methods, func(i, j int) bool {
		return methods[i].Type < methods[j].Type
	})
	return methods
}

func buildTopUsers(orders []*dbent.PaymentOrder) TopUsersByCurrency {
	userMap := make(map[string]map[int64]*TopUserStat)
	for _, o := range orders {
		currency := PaymentOrderCurrency(o)
		users, ok := userMap[currency]
		if !ok {
			users = make(map[int64]*TopUserStat)
			userMap[currency] = users
		}
		us, ok := users[o.UserID]
		if !ok {
			us = &TopUserStat{UserID: o.UserID, Email: o.UserEmail}
			users[o.UserID] = us
		}
		us.Amount += o.PayAmount
	}
	result := make(TopUsersByCurrency, len(userMap))
	for currency, users := range userMap {
		userList := make([]*TopUserStat, 0, len(users))
		for _, us := range users {
			us.Amount = roundAmount(us.Amount)
			userList = append(userList, us)
		}
		sort.Slice(userList, func(i, j int) bool {
			return userList[i].Amount > userList[j].Amount
		})
		limit := topUsersLimit
		if len(userList) < limit {
			limit = len(userList)
		}
		result[currency] = make([]TopUserStat, 0, limit)
		for i := 0; i < limit; i++ {
			result[currency] = append(result[currency], *userList[i])
		}
	}
	return result
}

func roundCurrencyAmounts(amounts CurrencyAmounts) {
	for currency, amount := range amounts {
		amounts[currency] = roundAmount(amount)
	}
}

func roundAmount(amount float64) float64 {
	return math.Round(amount*100) / 100
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
