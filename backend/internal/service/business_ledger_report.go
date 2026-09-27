package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"sort"
	"time"

	"github.com/shopspring/decimal"
)

func (s *BusinessLedgerService) Overview(ctx context.Context, start, end time.Time) (*BusinessLedgerOverview, error) {
	if !end.After(start) || end.Sub(start) > 366*24*time.Hour {
		return nil, fmt.Errorf("请选择不超过 366 天的有效区间")
	}
	if _, err := s.Project(ctx); err != nil {
		return nil, err
	}
	s.Start()
	var stateRaw []byte
	var revision int64
	var updated, enabled time.Time
	var zone string
	// Entries and checkpoint must come from one MVCC snapshot during projection.
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	err = tx.QueryRowContext(ctx, `SELECT p.state,p.revision,p.updated_at,c.enabled_at,c.reporting_timezone FROM business_projection_state p CROSS JOIN business_ledger_config c WHERE p.id=1 AND c.id=1`).Scan(&stateRaw, &revision, &updated, &enabled, &zone)
	if err != nil {
		return nil, err
	}
	state := NewBusinessProjection()
	if err = json.Unmarshal(stateRaw, state); err != nil {
		return nil, err
	}
	zone = timezone.Location().String()
	location, err := time.LoadLocation(zone)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,event_id,occurred_at,kind,COALESCE(user_id,0),COALESCE(account_id,0),COALESCE(group_id,0),model,COALESCE(plan_id,0),amount_cny::text,credits::text,quality,payload FROM business_ledger_entries WHERE occurred_at>=$1 AND occurred_at<$2 ORDER BY occurred_at,id`, start, end)
	if err != nil {
		return nil, err
	}
	entries := []BusinessEntry{}
	for rows.Next() {
		var e BusinessEntry
		var amount sql.NullString
		var credits string
		var raw []byte
		if err = rows.Scan(&e.ID, &e.EventID, &e.At, &e.Kind, &e.UserID, &e.AccountID, &e.GroupID, &e.Model, &e.PlanID, &amount, &credits, &e.Quality, &raw); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if amount.Valid {
			value, parseErr := decimal.NewFromString(amount.String)
			if parseErr != nil {
				_ = rows.Close()
				return nil, parseErr
			}
			e.Amount = &value
		}
		e.Credits, _ = decimal.NewFromString(credits)
		if err = json.Unmarshal(raw, &e.Detail); err != nil {
			_ = rows.Close()
			return nil, err
		}
		entries = append(entries, e)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	var pending int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM business_events e LEFT JOIN business_projection_processed p ON p.event_id=e.id WHERE p.event_id IS NULL`).Scan(&pending); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	report := BuildBusinessLedgerReport(state, entries, start, end, location, time.Now())
	report.Revision = revision
	report.UpdatedAt = updated
	report.EnabledAt = enabled
	if pending > 0 {
		report.Quality.MissingCount += pending
		report.Quality.Revenue = "pending"
		report.Quality.Cost = "pending"
		report.Profit = nil
		report.Margin = nil
	}
	if start.Before(enabled) {
		report.Quality.Revenue = "unknown"
		report.Quality.Cost = "unknown"
		report.Profit = nil
		report.Margin = nil
		report.Quality.MissingCount++
		report.Entries = append(report.Entries, BusinessEntry{ID: "before-cutover", At: start, Kind: "history_gap", Quality: "unknown", Detail: map[string]any{"reason": "所选区间包含新账启用前历史，请查看历史估算或补录凭据"}})
	}
	return report, nil
}

func BuildBusinessLedgerReport(state *BusinessProjection, source []BusinessEntry, start, end time.Time, zone *time.Location, now time.Time) *BusinessLedgerOverview {
	endAccrual := end
	if now.Before(endAccrual) {
		endAccrual = now
	}
	// Invoice verification is explicit and auditable, never inferred from a ratio.
	verified := map[string]bool{}
	verifiedAmounts := map[string]any{}
	verifiedEvents := map[int64]bool{}
	for _, bill := range state.Bills {
		raw, _ := json.Marshal(bill.Payload["covered_entry_ids"])
		var ids []string
		_ = json.Unmarshal(raw, &ids)
		for _, id := range ids {
			verified[id] = true
		}
		for id, amount := range bMap(bill.Payload, "covered_entry_amounts") {
			verifiedAmounts[id] = amount
		}
		raw, _ = json.Marshal(bill.Payload["covered_event_ids"])
		var eventIDs []int64
		_ = json.Unmarshal(raw, &eventIDs)
		for _, id := range eventIDs {
			verifiedEvents[id] = true
		}
	}
	checked := make([]BusinessEntry, 0, len(source))
	for _, entry := range source {
		if entry.At.Before(start) || !entry.At.Before(end) {
			continue
		}
		changed := false
		if entry.Kind == "usage_cost" || entry.Kind == "cost_gap" {
			if expected, ok := verifiedAmounts[entry.ID]; ok {
				changed = (expected == nil) != (entry.Amount == nil)
				if expected != nil && entry.Amount != nil {
					value, err := decimal.NewFromString(fmt.Sprint(expected))
					changed = err != nil || !entry.Amount.Equal(value)
				}
			} else if verifiedEvents[entry.EventID] {
				changed = true
			}
		}
		if changed {
			gap := entry
			gap.ID += ":invoice-stale"
			gap.Kind, gap.Quality, gap.Amount = "cost_gap", "unknown", nil
			gap.Detail = map[string]any{"reason": "账单核对后原始成本已修订，请冲销旧核对记录后重新核对"}
			checked = append(checked, gap)
		}
		if verified[entry.ID] && !changed {
			if entry.Kind == "cost_gap" {
				continue
			}
			entry.Quality = "confirmed"
		}
		checked = append(checked, entry)
	}
	source = checked
	// Enrich subscription gift attribution before allocating daily fixed costs.
	source = append([]BusinessEntry{}, source...)
	giftEntries := []BusinessEntry{}
	for i := range source {
		e := &source[i]
		if e.Kind != "usage_weight" && e.Kind != "usage_cost" {
			continue
		}
		subID := bInt(e.Detail, "subscription_id")
		if subID == 0 {
			continue
		}
		for _, term := range state.Terms {
			if term.SubscriptionID != subID || (bInt(e.Detail, "subscription_term_version") != 0 && bInt(e.Detail, "subscription_term_version") != term.Version) || e.At.Before(term.Start) || !e.At.Before(term.End) {
				continue
			}
			e.PlanID = term.PlanID
			e.Detail["plan_name"] = term.PlanName
			if term.GiftShare.IsPositive() {
				e.Detail["gift_share"] = term.GiftShare.String()
				gift := *e
				gift.ID += ":subscription-gift"
				if e.Kind == "usage_cost" && e.Amount != nil {
					value := e.Amount.Mul(term.GiftShare).Round(8)
					gift.Amount = &value
					gift.Kind = "gift_cost"
				} else {
					gift.Kind = "gift_use"
					gift.Credits = bDecimal(e.Detail, "subscription_credits").Mul(term.GiftShare).Round(8)
					gift.Amount = nil
				}
				giftEntries = append(giftEntries, gift)
			}
			break
		}
	}
	entries := append(append([]BusinessEntry{}, source...), giftEntries...)
	// A fixed-price account with no covering invoice is missing cost, not free.
	checkedFixed := map[string]bool{}
	for _, w := range source {
		if w.Kind != "usage_weight" || !bBool(w.Detail, "requires_fixed_cost") {
			continue
		}
		key := fmt.Sprintf("%d:%s", w.AccountID, w.At.In(zone).Format("2006-01-02"))
		if checkedFixed[key] {
			continue
		}
		checkedFixed[key] = true
		covered := false
		for _, expense := range state.Expenses {
			if bInt(expense.Event.Payload, "account_id") == w.AccountID && !w.At.Before(expense.Start) && w.At.Before(expense.End) {
				covered = true
				break
			}
		}
		if !covered {
			gap := w
			gap.ID += ":fixed-gap"
			gap.Kind = "cost_gap"
			gap.Amount = nil
			gap.Quality = "unknown"
			gap.Detail = map[string]any{"reason": "固定费用账号缺少覆盖当日的服务期费用"}
			entries = append(entries, gap)
		}
	}
	dayStart := func(t time.Time) time.Time {
		t = t.In(zone)
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, zone)
	}
	for _, term := range state.Terms {
		effectiveEnd := term.End
		if term.TerminatedAt != nil && term.TerminatedAt.Before(effectiveEnd) {
			effectiveEnd = *term.TerminatedAt
		}
		a := start
		if term.Start.After(a) {
			a = term.Start
		}
		b := endAccrual
		if effectiveEnd.Before(b) {
			b = effectiveEnd
		}
		for day := dayStart(a); day.Before(b); day = day.AddDate(0, 0, 1) {
			next := day.AddDate(0, 0, 1)
			left := day
			if a.After(left) {
				left = a
			}
			right := next
			if b.Before(right) {
				right = b
			}
			if !right.After(left) {
				continue
			}
			amount := businessAccrued(term.Value, term.Start, term.End, term.Start, right).Sub(businessAccrued(term.Value, term.Start, term.End, term.Start, left))
			event := BusinessEntry{ID: fmt.Sprintf("term:%s:%s", term.Key, day.Format("2006-01-02")), EventID: term.EventID, At: left, Kind: "subscription_revenue", UserID: term.UserID, GroupID: term.GroupID, PlanID: term.PlanID, Amount: &amount, Quality: term.Quality, Detail: map[string]any{"subscription_id": term.SubscriptionID, "term_version": term.Version, "order_id": term.OrderID, "group_name": term.GroupName, "plan_name": term.PlanName, "allocation": "服务期收入", "funding_event_ids": term.FundingSources}}
			if term.Quality == "unknown" {
				gap := event
				gap.ID += ":gap"
				gap.Kind = "revenue_gap"
				gap.Amount = nil
				gap.Detail = map[string]any{"reason": "订阅购买金额或期初仍有未知部分", "group_name": term.GroupName}
				entries = append(entries, gap)
				event.Quality = "confirmed"
			}
			weights := []BusinessEntry{}
			for _, w := range source {
				if w.Kind == "usage_weight" && bInt(w.Detail, "subscription_id") == term.SubscriptionID && (bInt(w.Detail, "subscription_term_version") == term.Version || bInt(w.Detail, "subscription_term_version") == 0) && !w.At.Before(left) && w.At.Before(right) {
					weights = append(weights, w)
				}
			}
			entries = append(entries, allocateBusinessEntry(event, weights, "subscription_credits")...)
		}
		if term.TerminatedAt != nil && !term.TerminatedAt.Before(start) && term.TerminatedAt.Before(endAccrual) {
			amount := term.Value.Sub(businessAccrued(term.Value, term.Start, term.End, term.Start, *term.TerminatedAt))
			e := BusinessEntry{ID: fmt.Sprintf("term-close:%s", term.Key), EventID: term.EventID, At: *term.TerminatedAt, Kind: "subscription_close_revenue", UserID: term.UserID, GroupID: term.GroupID, PlanID: term.PlanID, Amount: &amount, Quality: term.Quality, Detail: map[string]any{"reason": "订阅权益终止结转", "group_name": term.GroupName, "plan_name": term.PlanName}}
			if term.Quality == "unknown" {
				gap := e
				gap.ID += ":gap"
				gap.Amount = nil
				gap.Kind = "revenue_gap"
				entries = append(entries, gap)
				e.Quality = "confirmed"
			}
			entries = append(entries, e)
		}
	}
	for _, expense := range state.Expenses {
		a := start
		if expense.Start.After(a) {
			a = expense.Start
		}
		b := endAccrual
		if expense.End.Before(b) {
			b = expense.End
		}
		for day := dayStart(a); day.Before(b); day = day.AddDate(0, 0, 1) {
			next := day.AddDate(0, 0, 1)
			left := day
			if a.After(left) {
				left = a
			}
			right := next
			if b.Before(right) {
				right = b
			}
			if !right.After(left) {
				continue
			}
			amount := businessAccrued(expense.Amount, expense.Start, expense.End, expense.Start, right).Sub(businessAccrued(expense.Amount, expense.Start, expense.End, expense.Start, left))
			e := BusinessEntry{ID: fmt.Sprintf("expense:%d:%s", expense.Event.ID, day.Format("2006-01-02")), EventID: expense.Event.ID, At: left, Kind: "fixed_cost", AccountID: bInt(expense.Event.Payload, "account_id"), GroupID: bInt(expense.Event.Payload, "group_id"), Amount: &amount, Quality: "confirmed", Detail: map[string]any{"category": expense.Category, "account_name": expense.Event.Payload["account_name"], "group_name": expense.Event.Payload["group_name"], "source_key": expense.Event.SourceKey, "notes": bString(expense.Event.Payload, "notes")}}
			if e.AccountID == 0 && e.GroupID == 0 {
				e.Kind = "operating_cost"
				e.Detail["allocation"] = "公共费用"
				entries = append(entries, e)
				continue
			}
			weights := []BusinessEntry{}
			for _, w := range source {
				if w.Kind == "usage_weight" && !w.At.Before(left) && w.At.Before(right) && (e.AccountID == 0 || w.AccountID == e.AccountID) && (e.GroupID == 0 || w.GroupID == e.GroupID) {
					weights = append(weights, w)
				}
			}
			entries = append(entries, allocateBusinessEntry(e, weights, "")...)
		}
	}
	report := &BusinessLedgerOverview{Start: start, End: end, Currency: "CNY", Timezone: zone.String(), Entries: entries, Quality: BusinessDataQuality{Revenue: "confirmed", Cost: "confirmed", Attribution: "direct", Cash: "confirmed"}}
	for _, f := range state.Wallets {
		report.WalletDeferred = report.WalletDeferred.Add(f.Value)
		report.UnknownWalletCredits = report.UnknownWalletCredits.Add(f.Unknown)
		report.GiftOutstanding = report.GiftOutstanding.Add(f.Gift)
	}
	for _, f := range state.Held {
		report.WalletDeferred = report.WalletDeferred.Add(f.Value)
		report.UnknownWalletCredits = report.UnknownWalletCredits.Add(f.Unknown)
		report.GiftOutstanding = report.GiftOutstanding.Add(f.Gift)
	}
	for _, f := range state.RefundFunds {
		report.WalletDeferred = report.WalletDeferred.Add(f.Value)
		report.UnknownWalletCredits = report.UnknownWalletCredits.Add(f.Unknown)
	}
	for _, f := range state.BatchFunds {
		report.WalletDeferred = report.WalletDeferred.Add(f.Value)
		report.UnknownWalletCredits = report.UnknownWalletCredits.Add(f.Unknown)
		report.GiftOutstanding = report.GiftOutstanding.Add(f.Gift)
	}
	// Purchased tickets remain a customer prepayment until the ticket is used.
	// Ticket counts must not be added to the site's credit-denominated counters.
	for _, f := range state.TicketFunds {
		report.WalletDeferred = report.WalletDeferred.Add(f.Value)
	}
	for _, expense := range state.Expenses {
		report.PrepaidExpense = report.PrepaidExpense.Add(expense.Amount.Sub(businessAccrued(expense.Amount, expense.Start, expense.End, expense.Start, now)))
	}
	for _, f := range state.Pools {
		report.PrepaidSupplier = report.PrepaidSupplier.Add(f.Value)
	}
	for _, t := range state.Terms {
		if t.TerminatedAt == nil || t.TerminatedAt.After(now) {
			report.SubscriptionDeferred = report.SubscriptionDeferred.Add(t.Value.Sub(businessAccrued(t.Value, t.Start, t.End, t.Start, now)))
		}
	}
	for _, e := range entries {
		value := decimal.Zero
		if e.Amount != nil {
			value = *e.Amount
		}
		switch e.Kind {
		case "cash_in":
			report.CashIn = report.CashIn.Add(value)
		case "cash_refund":
			report.CashRefund = report.CashRefund.Add(value)
		case "cash_out":
			report.CashOut = report.CashOut.Add(value)
		case "wallet_revenue", "subscription_revenue", "subscription_close_revenue", "ticket_revenue", "refund_revenue":
			report.Revenue = report.Revenue.Add(value)
		case "usage_cost":
			report.UsageCost = report.UsageCost.Add(value)
		case "fixed_cost":
			report.FixedCost = report.FixedCost.Add(value)
		case "operating_cost":
			report.OperatingCost = report.OperatingCost.Add(value)
		case "gift_grant":
			report.GiftGranted = report.GiftGranted.Add(e.Credits)
		case "gift_use":
			report.GiftUsed = report.GiftUsed.Add(e.Credits)
		case "gift_cost":
			report.GiftCost = report.GiftCost.Add(value)
		case "discount":
			report.Discount = report.Discount.Add(value)
		}
		if e.Quality == "unknown" {
			report.Quality.MissingCount++
			switch e.Kind {
			case "cost_gap":
				report.Quality.Cost = "unknown"
			case "cash_gap":
				report.Quality.Cash = "unknown"
			default:
				report.Quality.Revenue = "unknown"
			}
		}
		if e.Quality == "estimated" && (e.Kind == "usage_cost" || e.Kind == "fixed_cost" || e.Kind == "operating_cost") {
			report.Quality.EstimatedCount++
			if report.Quality.Cost != "unknown" {
				report.Quality.Cost = "estimated"
			}
		}
		if bString(e.Detail, "allocation") == "按当日用量分摊" || e.Quality == "allocated" {
			report.Quality.Attribution = "allocated"
		}
	}
	report.KnownProfit = report.Revenue.Sub(report.UsageCost).Sub(report.FixedCost).Sub(report.OperatingCost)
	report.CashNet = report.CashIn.Sub(report.CashRefund).Sub(report.CashOut)
	if report.Quality.Revenue == "confirmed" && report.Quality.Cost == "confirmed" {
		profit := report.KnownProfit
		report.Profit = &profit
		if report.Revenue.IsPositive() {
			margin := profit.Div(report.Revenue)
			report.Margin = &margin
		}
	}
	report.Daily = businessBreakdowns(entries, "daily", zone)
	report.Groups = businessBreakdowns(entries, "groups", zone)
	report.Models = businessBreakdowns(entries, "models", zone)
	report.Accounts = businessBreakdowns(entries, "accounts", zone)
	report.Plans = businessBreakdowns(entries, "plans", zone)
	return report
}

func allocateBusinessEntry(base BusinessEntry, weights []BusinessEntry, field string) []BusinessEntry {
	total := decimal.Zero
	for _, w := range weights {
		v := w.Credits
		if field != "" {
			v = bDecimal(w.Detail, field)
		}
		total = total.Add(v)
	}
	if !total.IsPositive() || base.Amount == nil {
		if base.Kind == "fixed_cost" {
			base.Detail["allocation"] = "闲置账号成本"
		} else {
			base.Detail["allocation"] = "未使用订阅收入"
		}
		return []BusinessEntry{base}
	}
	output := []BusinessEntry{}
	allocated := decimal.Zero
	for i, w := range weights {
		v := w.Credits
		if field != "" {
			v = bDecimal(w.Detail, field)
		}
		value := base.Amount.Mul(v).Div(total).Round(8)
		if i == len(weights)-1 {
			value = base.Amount.Sub(allocated)
		}
		allocated = allocated.Add(value)
		e := base
		e.ID = fmt.Sprintf("%s:%d", base.ID, i)
		e.Amount = &value
		e.Model = w.Model
		e.AccountID = w.AccountID
		e.GroupID = w.GroupID
		if e.PlanID == 0 {
			e.PlanID = w.PlanID
		}
		e.Quality = "allocated"
		detail := map[string]any{}
		for k, v := range base.Detail {
			detail[k] = v
		}
		e.Detail = map[string]any{"allocation": "按当日用量分摊", "weight": v.String(), "weight_total": total.String(), "group_name": w.Detail["group_name"], "account_name": w.Detail["account_name"], "plan_name": w.Detail["plan_name"], "source_key": base.Detail["source_key"]}
		for k, v := range detail {
			if _, exists := e.Detail[k]; !exists {
				e.Detail[k] = v
			}
		}
		output = append(output, e)
		if base.Kind == "fixed_cost" {
			gift := value.Mul(bDecimal(w.Detail, "gift_share")).Round(8)
			if gift.IsPositive() {
				g := e
				g.ID += "gift"
				g.Kind = "gift_cost"
				g.Amount = &gift
				output = append(output, g)
			}
		}
	}
	return output
}

func businessBreakdowns(entries []BusinessEntry, dimension string, zone *time.Location) []BusinessBreakdown {
	groups := map[string]*BusinessBreakdown{}
	for _, e := range entries {
		isRevenue := e.Kind == "wallet_revenue" || e.Kind == "subscription_revenue" || e.Kind == "subscription_close_revenue" || e.Kind == "ticket_revenue" || e.Kind == "refund_revenue"
		isCost := e.Kind == "usage_cost" || e.Kind == "fixed_cost" || e.Kind == "operating_cost"
		isCash := dimension == "daily" && (e.Kind == "cash_in" || e.Kind == "cash_out" || e.Kind == "cash_refund" || e.Kind == "cash_gap")
		if !isRevenue && !isCost && !isCash && e.Kind != "revenue_gap" && e.Kind != "cost_gap" {
			continue
		}
		key, name := "", ""
		switch dimension {
		case "daily":
			key = e.At.In(zone).Format("2006-01-02")
			name = key
		case "groups":
			key = fmt.Sprint(e.GroupID)
			name = bString(e.Detail, "group_name")
			if e.GroupID == 0 {
				name = "公共 / 闲置 / 未归属"
			}
		case "accounts":
			key = fmt.Sprint(e.AccountID)
			name = bString(e.Detail, "account_name")
			if e.AccountID == 0 {
				name = "公共 / 未归属"
			}
		case "plans":
			key = fmt.Sprint(e.PlanID)
			name = bString(e.Detail, "plan_name")
			if name == "" {
				name = "套餐 #" + key
			}
			if e.PlanID == 0 {
				name = "余额 / 非套餐"
			}
		default:
			key = e.Model
			name = key
			if key == "" {
				name = "未使用订阅 / 公共费用"
			}
		}
		if name == "" {
			name = key
		}
		row := groups[key]
		if row == nil {
			row = &BusinessBreakdown{Key: key, Name: name, Allocation: "direct"}
			groups[key] = row
		}
		if e.Amount != nil {
			switch e.Kind {
			case "cash_in":
				row.CashIn = row.CashIn.Add(*e.Amount)
			case "cash_out":
				row.CashOut = row.CashOut.Add(*e.Amount)
			case "cash_refund":
				row.CashRefund = row.CashRefund.Add(*e.Amount)
			}
			if isRevenue {
				row.Revenue = row.Revenue.Add(*e.Amount)
			}
			if isCost {
				row.Cost = row.Cost.Add(*e.Amount)
			}
		}
		if e.Quality == "unknown" {
			row.Missing++
		}
		if isCost && e.Quality == "estimated" {
			row.Estimated++
		}
		if bString(e.Detail, "allocation") == "按当日用量分摊" || e.Quality == "allocated" {
			row.Allocation = "allocated"
		}
		row.Profit = row.Revenue.Sub(row.Cost)
		row.CashNet = row.CashIn.Sub(row.CashOut).Sub(row.CashRefund)
	}
	result := make([]BusinessBreakdown, 0, len(groups))
	for _, row := range groups {
		result = append(result, *row)
	}
	sort.Slice(result, func(i, j int) bool {
		if dimension == "daily" {
			return result[i].Key < result[j].Key
		}
		return result[i].Revenue.GreaterThan(result[j].Revenue)
	})
	return result
}
