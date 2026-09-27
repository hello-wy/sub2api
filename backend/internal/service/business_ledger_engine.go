package service

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// ApplyTransaction consumes an entire committed transaction. Domain evidence
// can be written after the wallet UPDATE, so event-by-event classification is
// deliberately forbidden. Missing evidence remains unknown.
func (p *BusinessProjection) ApplyTransaction(events []BusinessEvent) {
	sort.SliceStable(events, func(i, j int) bool { return events[i].ID < events[j].ID })
	consumed := map[int64]BusinessFunds{}
	transfers := map[int64]BusinessFunds{}
	ticketTransfers := map[int64]BusinessFunds{}
	for _, e := range events {
		f := businessFund(p.Wallets, e.UserID)
		switch e.Type {
		case "opening_unknown":
			f.Sources = append(f.Sources, e.ID)
			total := bDecimal(e.Payload, "credits")
			frozen := bDecimal(e.Payload, "frozen")
			if bBool(e.Payload, "resolved") {
				f.Add(BusinessFunds{Paid: bDecimal(e.Payload, "paid_credits"), Gift: bDecimal(e.Payload, "gift_credits"), Unknown: bDecimal(e.Payload, "unknown_credits"), Value: bDecimal(e.Payload, "amount_cny")})
			} else if total.IsPositive() {
				f.Unknown = f.Unknown.Add(total)
			}
			if frozen.IsPositive() {
				businessFund(p.Held, e.UserID).Add(f.Take(frozen))
			}
		case "wallet", "signup_wallet":
			before := bDecimal(e.Payload, "before")
			after := bDecimal(e.Payload, "after")
			frozenDelta := bDecimal(e.Payload, "frozen_after").Sub(bDecimal(e.Payload, "frozen_before"))
			totalDelta := after.Sub(before).Add(frozenDelta)
			// Refund reservation/release carries its explicit order identity.
			context := bMap(e.Payload, "context")
			orderID := bInt(context, "order_id")
			if orderID > 0 && bString(context, "operation") == "refund_reserve" && totalDelta.IsNegative() {
				businessFund(p.RefundFunds, orderID).Add(f.Take(totalDelta.Neg()))
				continue
			}
			if orderID > 0 && bString(context, "operation") == "refund_release" && totalDelta.IsPositive() {
				f.Add(businessFund(p.RefundFunds, orderID).Take(totalDelta))
				continue
			}
			var batch *BusinessEvent
			for i := range events {
				if events[i].UserID == e.UserID && events[i].Type == "batch_operation" {
					batch = &events[i]
					break
				}
			}
			if batch != nil {
				key := fmt.Sprintf("%d:%s", e.UserID, bString(batch.Payload, "batch_id"))
				if p.BatchFunds[key] == nil {
					p.BatchFunds[key] = &BusinessFunds{}
				}
				held := p.BatchFunds[key]
				if frozenDelta.IsPositive() {
					held.Add(f.Take(frozenDelta))
					continue
				}
				if frozenDelta.IsNegative() {
					released := held.Take(frozenDelta.Neg())
					if bString(batch.Payload, "action") == "release" {
						f.Add(released)
						continue
					}
					amount := decimal.Max(totalDelta.Neg(), decimal.Zero)
					taken := released.Take(decimal.Min(amount, released.Total()))
					if amount.GreaterThan(taken.Total()) {
						taken.Add(f.Take(amount.Sub(taken.Total())))
					}
					f.Add(released)
					for _, source := range events {
						if source.Type == "usage" && bString(source.Payload, "batch_id") == bString(batch.Payload, "batch_id") {
							consumed[source.ID] = taken
						}
					}
					continue
				}
			}
			if frozenDelta.IsPositive() {
				businessFund(p.Held, e.UserID).Add(f.Take(frozenDelta))
			}
			if totalDelta.IsZero() && frozenDelta.IsNegative() {
				f.Add(businessFund(p.Held, e.UserID).Take(frozenDelta.Neg()))
			}
			if totalDelta.IsPositive() {
				incoming := p.walletFunding(e, events, totalDelta)
				incoming.Sources = []int64{e.ID}
				if incoming.Unknown.IsPositive() {
					p.entry(e, "funding_unknown", nil, incoming.Unknown, "unverified", map[string]any{"reason": "余额增加的收款或赠送来源待补录"})
				}
				if incoming.Gift.IsPositive() {
					p.entry(e, "gift_grant", nil, incoming.Gift, "confirmed", map[string]any{"source": rewardSource(events)})
				}
				// A recharge first settles an existing overdraft. Its cash basis
				// cannot also remain available for the next wallet consumption.
				priorTotal := before.Add(bDecimal(e.Payload, "frozen_before"))
				if priorTotal.IsNegative() {
					repaid := incoming.Take(decimal.Min(incoming.Total(), priorTotal.Neg()))
					p.revenue(e, repaid, "wallet_revenue")
					if repaid.Gift.IsPositive() {
						p.entry(e, "gift_use", nil, repaid.Gift, "confirmed", map[string]any{"reason": "赠送额度补足原有透支"})
					}
				}
				f.Add(incoming)
			} else if totalDelta.IsNegative() {
				amount := totalDelta.Neg()
				var taken BusinessFunds
				if frozenDelta.IsNegative() {
					released := businessFund(p.Held, e.UserID).Take(frozenDelta.Neg())
					taken = released.Take(decimal.Min(amount, frozenDelta.Neg()))
					f.Add(released)
					if amount.GreaterThan(taken.Total()) {
						taken.Add(f.Take(amount.Sub(taken.Total())))
					}
				} else {
					taken = f.Take(amount)
				}
				assigned := false
				for _, source := range events {
					if source.UserID != e.UserID {
						continue
					}
					if source.Type == "usage" && bDecimal(source.Payload, "balance_cost").IsPositive() {
						already := consumed[source.ID]
						needed := bDecimal(source.Payload, "balance_cost").Sub(already.Total())
						if needed.IsPositive() {
							already.Add(taken.Take(decimal.Min(needed, taken.Total())))
							consumed[source.ID] = already
							assigned = true
						}
						if taken.Total().IsZero() {
							break
						}
						continue
					}
					if source.Type == "payment_orders" && bString(source.Payload, "payment_type") == "balance" && bString(source.Payload, "order_type") == "subscription" {
						transfers[e.UserID] = taken
						assigned = true
						break
					}
					if source.Type == "balance_transactions" && strings.Contains(bString(source.Payload, "transaction_type"), "purchase") {
						ticketTransfers[e.UserID] = taken
						assigned = true
						break
					}
					if source.Type == "payment_orders" && isBusinessRefund(bString(source.Payload, "status")) {
						businessFund(p.RefundFunds, bInt(source.Payload, "id")).Add(taken)
						assigned = true
						break
					}
				}
				if !assigned {
					if bString(e.Payload, "classification") == "usage" {
						p.revenue(e, taken, "wallet_revenue")
					} else {
						p.entry(e, "wallet_adjustment", nil, taken.Total(), "unknown", map[string]any{"reason": "余额减少缺少消费、退款或转移凭据", "removed_value_cny": taken.Value.String()})
					}
				}
			}
		}
	}
	for _, e := range events {
		switch e.Type {
		case "payment_orders":
			p.payment(e)
		case "usage":
			slice, ok := consumed[e.ID]
			if !ok && !bBool(e.Payload, "billing_applied") && bInt(e.Payload, "billing_type") == 0 && bDecimal(e.Payload, "actual_cost").IsPositive() {
				// A legacy log is not proof of a successful wallet deduction. Keep cost,
				// explicitly flag revenue rather than charging the shadow wallet twice.
				slice.Unknown = bDecimal(e.Payload, "actual_cost")
			}
			p.usage(e, slice)
		case "user_subscriptions":
			p.subscription(e, events, transfers[e.UserID])
		case "lottery_ticket_ledger":
			p.ticket(e, ticketTransfers[e.UserID])
		case "receipt":
			amount, known := bMoney(e.Payload)
			p.cash(e, "cash_in", amount, known)
			if d := bDecimal(e.Payload, "discount_cny"); d.IsPositive() {
				p.entry(e, "discount", &d, decimal.Zero, "confirmed", nil)
			}
		case "purchase", "opening_pool":
			amount, _ := bMoney(e.Payload)
			credits := bDecimal(e.Payload, "credits")
			businessFund(p.Pools, bInt(e.Payload, "pool_id")).Add(BusinessFunds{Paid: credits, Value: amount, Sources: []int64{e.ID}})
			if e.Type == "purchase" {
				p.cash(e, "cash_out", amount, true)
			}
		case "expense":
			amount, _ := bMoney(e.Payload)
			if !bBool(e.Payload, "opening") {
				p.cash(e, "cash_out", amount, true)
			}
			if bBool(e.Payload, "cash_only") {
				continue
			}
			start := bTime(e.Payload, "starts_at", e.OccurredAt)
			end := bTime(e.Payload, "ends_at", start)
			if end.After(start) {
				p.Expenses = append(p.Expenses, BusinessExpense{Event: e, Amount: amount, Start: start, End: end, Category: bString(e.Payload, "category")})
			} else {
				p.entry(e, "operating_cost", &amount, decimal.Zero, "confirmed", nil)
			}
		case "expense_stop":
			target := bInt(e.Payload, "source_event_id")
			found := false
			for i := range p.Expenses {
				expense := &p.Expenses[i]
				if expense.Event.ID != target {
					continue
				}
				found = true
				earned := businessAccrued(expense.Amount, expense.Start, expense.End, expense.Start, e.OccurredAt)
				loss := expense.Amount.Sub(earned)
				expense.Amount = earned
				if e.OccurredAt.Before(expense.End) {
					expense.End = e.OccurredAt
				}
				lossEvent := e
				lossEvent.Payload = map[string]any{"account_id": bInt(expense.Event.Payload, "account_id"), "group_id": bInt(expense.Event.Payload, "group_id"), "account_name": expense.Event.Payload["account_name"], "group_name": expense.Event.Payload["group_name"]}
				p.entry(lossEvent, "operating_cost", &loss, decimal.Zero, "confirmed", map[string]any{"reason": "服务提前终止，剩余预付费用结转损失", "source_event_id": target})
				break
			}
			if !found {
				p.entry(e, "cost_gap", nil, decimal.Zero, "unknown", map[string]any{"reason": "被终止的原始预付费用不存在"})
			}
		case "reconciliation", "adjustment":
			if e.Type == "reconciliation" && e.Payload["bill_amount_cny"] != nil {
				p.Bills = append(p.Bills, e)
			}
			amount, _ := bMoney(e.Payload)
			kind := bString(e.Payload, "entry_kind")
			if kind == "" {
				kind = "usage_cost"
			}
			p.entry(e, kind, &amount, decimal.Zero, "confirmed", map[string]any{"reason": bString(e.Payload, "notes"), "adjusts_event_id": bInt(e.Payload, "adjusts_event_id")})
		case "supplier_refund", "supplier_loss":
			taken := businessFund(p.Pools, bInt(e.Payload, "pool_id")).Take(bDecimal(e.Payload, "credits"))
			amount, _ := bMoney(e.Payload)
			if e.Type == "supplier_refund" {
				p.cash(e, "cash_out", amount.Neg(), true)
				difference := taken.Value.Sub(amount)
				p.entry(e, "operating_cost", &difference, decimal.Zero, "confirmed", nil)
			} else {
				loss := taken.Value
				p.entry(e, "operating_cost", &loss, decimal.Zero, "confirmed", nil)
			}
			if taken.Unknown.IsPositive() {
				p.entry(e, "cost_gap", nil, taken.Unknown, "unknown", map[string]any{"reason": "采购剩余额度不足，待核对"})
			}
		}
	}
}

func (p *BusinessProjection) walletFunding(e BusinessEvent, events []BusinessEvent, n decimal.Decimal) BusinessFunds {
	if e.Type == "signup_wallet" {
		return BusinessFunds{Gift: n}
	}
	if bString(e.Payload, "classification") == "gift" {
		return BusinessFunds{Gift: n}
	}
	if bString(e.Payload, "classification") == "paid" {
		return BusinessFunds{Paid: n, Value: bDecimal(e.Payload, "amount_cny")}
	}
	for _, s := range events {
		if s.UserID != e.UserID {
			continue
		}
		if s.Type == "receipt" && bBool(s.Payload, "apply_balance") {
			amount, ok := bMoney(s.Payload)
			if ok {
				return BusinessFunds{Paid: n, Value: amount}
			}
		}
		if s.Type == "redeem_codes" && bString(s.Payload, "type") == "balance" {
			if order := bMap(s.Payload, "order"); order != nil {
				amount, ok := bMoney(order)
				if ok {
					return BusinessFunds{Paid: n, Value: amount}
				}
			}
		}
		if s.Type == "daily_checkin_records" || s.Type == "welfare_records" || s.Type == "user_affiliate_ledger" {
			return BusinessFunds{Gift: n}
		}
		if s.Type == "balance_transactions" && (strings.Contains(bString(s.Payload, "transaction_type"), "reward") || strings.Contains(bString(s.Payload, "source_type"), "binding")) {
			return BusinessFunds{Gift: n}
		}
		if s.Type == "redeem_codes" {
			switch bString(s.Payload, "type") {
			case "daily_checkin", "welfare", "affiliate", "promo", "registration", "qq_binding_reward", "admin_gift":
				return BusinessFunds{Gift: n}
			}
		}
	}
	return BusinessFunds{Unknown: n}
}

func rewardSource(events []BusinessEvent) string {
	for _, e := range events {
		switch e.Type {
		case "daily_checkin_records":
			return "签到"
		case "lottery_draws":
			return "抽奖"
		case "user_affiliate_ledger":
			return "邀请返利"
		case "welfare_records":
			return "活动奖励"
		}
	}
	return "赠送"
}
func isBusinessRefund(status string) bool {
	return status == "REFUNDED" || status == "PARTIALLY_REFUNDED"
}
func businessRefundValue(data map[string]any) (decimal.Decimal, bool) {
	amount, ok := bMoney(data)
	base := bDecimal(data, "amount")
	if !base.IsPositive() {
		return decimal.Zero, false
	}
	return amount.Mul(bDecimal(data, "refund_amount")).Div(base).Round(8), ok
}
func (p *BusinessProjection) cash(e BusinessEvent, kind string, amount decimal.Decimal, known bool) {
	if !known {
		p.entry(e, "cash_gap", nil, decimal.Zero, "unknown", map[string]any{"reason": "外币实际结算人民币金额待补录", "cash_kind": kind})
		return
	}
	p.entry(e, kind, &amount, decimal.Zero, "confirmed", nil)
}
func (p *BusinessProjection) payment(e BusinessEvent) {
	id := bInt(e.Payload, "id")
	if bString(e.Payload, "payment_type") == "balance" {
		return
	}
	receiptKey := fmt.Sprintf("payment:%d", id)
	if e.Payload["paid_at"] != nil && !p.Seen[receiptKey] {
		p.Seen[receiptKey] = true
		cashEvent := e
		cashEvent.OccurredAt = bTime(e.Payload, "paid_at", e.OccurredAt)
		amount, known := bMoney(e.Payload)
		p.cash(cashEvent, "cash_in", amount, known)
		loyalty := bMap(e.Payload, "loyalty")
		discount := bDecimal(loyalty, "discount_amount")
		if discount.IsPositive() {
			if c := bString(e.Payload, "currency"); c == "" || c == "CNY" {
				p.entry(cashEvent, "discount", &discount, decimal.Zero, "confirmed", nil)
			}
		}
	}
	if isBusinessRefund(bString(e.Payload, "status")) {
		key := fmt.Sprintf("refund:%d:%s", id, bString(e.Payload, "refund_at"))
		if p.Seen[key] {
			return
		}
		p.Seen[key] = true
		refund, known := businessRefundValue(e.Payload)
		previous := p.RefundAmounts[id]
		totalRefund := refund
		refund = refund.Sub(previous)
		if !refund.IsPositive() {
			return
		}
		p.RefundAmounts[id] = totalRefund
		e.OccurredAt = bTime(e.Payload, "refund_at", e.OccurredAt)
		p.cash(e, "cash_refund", refund, known)
		deferred := businessFund(p.RefundFunds, id)
		recovered := deferred.Value
		unknown := deferred.Unknown
		*deferred = BusinessFunds{}
		if bString(e.Payload, "order_type") == "subscription" {
			for _, term := range p.Terms {
				if term.OrderID == id {
					if term.Quality == "unknown" {
						unknown = decimal.NewFromInt(1)
					}
					// Refunds reverse consideration at refund time; earned past
					// service stays untouched, the unearned balance is extinguished.
					at := e.OccurredAt
					if term.TerminatedAt == nil && at.Before(term.End) {
						earned := businessAccrued(term.Value, term.Start, term.End, term.Start, at)
						recovered = recovered.Add(term.Value.Sub(earned))
						term.Value = earned
						term.End = at
						term.OriginalEnd = at
					}
				}
			}
		}
		if !known || unknown.IsPositive() {
			p.entry(e, "revenue_gap", nil, unknown, "unknown", map[string]any{"reason": "退款对应资金来源待核实"})
		} else {
			adjustment := recovered.Sub(refund)
			p.entry(e, "refund_revenue", &adjustment, decimal.Zero, "confirmed", nil)
		}
	}
}

func (p *BusinessProjection) usage(e BusinessEvent, funds BusinessFunds) {
	if funds.Total().IsPositive() {
		p.revenue(e, funds, "wallet_revenue")
	}
	if funds.Gift.IsPositive() {
		p.entry(e, "gift_use", nil, funds.Gift, "confirmed", nil)
	}
	weight := businessUsageWeight(e.Payload)
	zero := decimal.Zero
	p.entry(e, "usage_weight", &zero, weight, "internal", map[string]any{"gift_share": giftFraction(funds).String(), "subscription_id": bInt(e.Payload, "subscription_id"), "subscription_term_version": bInt(e.Payload, "subscription_term_version"), "subscription_credits": bString(e.Payload, "actual_cost")})
	if e.Payload["actual_supplier_cost_cny"] != nil {
		cost := bDecimal(e.Payload, "actual_supplier_cost_cny")
		detail := map[string]any{"usage_weight": weight.String(), "subscription_id": bInt(e.Payload, "subscription_id"), "subscription_term_version": bInt(e.Payload, "subscription_term_version")}
		pool, rule := bMap(e.Payload, "pool"), bMap(e.Payload, "rule")
		if bString(pool, "mode") == "prepaid" && rule != nil {
			units := businessSupplierUnits(e.Payload, rule)
			taken := businessFund(p.Pools, bInt(pool, "id")).Take(units)
			detail["funding_event_ids"] = taken.Sources
			detail["supplier_units"] = units.String()
			if taken.Unknown.IsPositive() {
				p.entry(e, "cost_gap", nil, taken.Unknown, "unknown", map[string]any{"reason": "已补录实际成本，但采购额度或期初仍不足，待核对"})
			}
		}
		p.entry(e, "usage_cost", &cost, weight, "confirmed", detail)
		giftCost := cost.Mul(giftFraction(funds)).Round(8)
		if giftCost.IsPositive() {
			p.entry(e, "gift_cost", &giftCost, funds.Gift, "confirmed", nil)
		}
		return
	}
	pool := bMap(e.Payload, "pool")
	rule := bMap(e.Payload, "rule")
	if pool == nil {
		p.entry(e, "cost_gap", nil, bDecimal(e.Payload, "total_cost"), "unknown", map[string]any{"reason": "账号未配置成本池"})
		return
	}
	if bString(pool, "mode") == "fixed" {
		p.Entries[len(p.Entries)-1].Detail["requires_fixed_cost"] = true
		return
	}
	if rule == nil {
		p.entry(e, "cost_gap", nil, businessUsageWeight(e.Payload), "unknown", map[string]any{"reason": "缺少有效期内上游价格规则"})
		return
	}
	units := businessSupplierUnits(e.Payload, rule)
	quality := "contract"
	if bString(pool, "mode") == "postpaid" {
		quality = "estimated"
	}
	if bString(rule, "quality") == "estimated" || bString(rule, "basis") == "account_stats" {
		quality = "estimated"
	}
	cost := decimal.Zero
	var purchases []int64
	if bString(pool, "mode") == "prepaid" {
		taken := businessFund(p.Pools, bInt(pool, "id")).Take(units)
		cost = taken.Value
		purchases = taken.Sources
		if taken.Unknown.IsPositive() {
			p.entry(e, "cost_gap", nil, taken.Unknown, "unknown", map[string]any{"reason": "上游采购额度不足或期初未录入", "pool_id": bInt(pool, "id")})
		}
	} else {
		cost = units.Mul(bDecimal(rule, "cny_per_unit")).Round(8)
	}
	detail := map[string]any{"pool_id": bInt(pool, "id"), "rule_id": bInt(rule, "id"), "supplier_units": units.String(), "usage_weight": businessUsageWeight(e.Payload).String(), "gift_share": giftFraction(funds).String(), "subscription_id": bInt(e.Payload, "subscription_id"), "subscription_term_version": bInt(e.Payload, "subscription_term_version"), "subscription_credits": bString(e.Payload, "actual_cost")}
	detail["funding_event_ids"] = purchases
	p.entry(e, "usage_cost", &cost, units, quality, detail)
	giftCost := cost.Mul(giftFraction(funds)).Round(8)
	if giftCost.IsPositive() {
		p.entry(e, "gift_cost", &giftCost, funds.Gift, quality, nil)
	}
}
func giftFraction(f BusinessFunds) decimal.Decimal {
	if f.Total().IsPositive() {
		return f.Gift.Div(f.Total())
	}
	return decimal.Zero
}
func businessUsageWeight(data map[string]any) decimal.Decimal {
	if data["account_stats_cost"] != nil {
		return bDecimal(data, "account_stats_cost")
	}
	multiplier := decimal.NewFromInt(1)
	if data["account_rate_multiplier"] != nil {
		multiplier = bDecimal(data, "account_rate_multiplier")
	}
	return bDecimal(data, "total_cost").Mul(multiplier)
}
func businessSupplierUnits(data, rule map[string]any) decimal.Decimal {
	switch bString(rule, "basis") {
	case "tokens":
		cache1h := bDecimal(data, "cache_creation_1h_tokens")
		cacheShort := decimal.Max(decimal.Zero, bDecimal(data, "cache_creation_tokens").Sub(cache1h))
		cacheCost := cacheShort.Mul(bDecimal(rule, "cache_write_price")).Add(cache1h.Mul(bDecimal(rule, "cache_write_1h_price")))
		return bDecimal(data, "input_tokens").Mul(bDecimal(rule, "input_price")).Add(bDecimal(data, "output_tokens").Mul(bDecimal(rule, "output_price"))).Add(bDecimal(data, "cache_read_tokens").Mul(bDecimal(rule, "cache_read_price"))).Add(cacheCost).Div(decimal.NewFromInt(1000000))
	case "request":
		return bDecimal(rule, "unit_price")
	case "image":
		return bDecimal(data, "image_count").Mul(bDecimal(rule, "unit_price"))
	case "video_second":
		return bDecimal(data, "video_duration_seconds").Mul(bDecimal(rule, "unit_price"))
	default:
		return businessUsageWeight(data).Mul(bDecimal(rule, "unit_price"))
	}
}

func (p *BusinessProjection) subscription(e BusinessEvent, events []BusinessEvent, transfer BusinessFunds) {
	id := bInt(e.Payload, "id")
	version := bInt(e.Payload, "term_version")
	key := fmt.Sprintf("%d:%d", id, version)
	start := bTime(e.Payload, "starts_at", e.OccurredAt)
	end := bTime(e.Payload, "expires_at", start)
	if !end.After(start) {
		return
	}
	for oldKey, old := range p.Terms {
		if old.SubscriptionID != id || old.TerminatedAt != nil {
			continue
		}
		if oldKey != key && old.End.After(e.OccurredAt) {
			at := e.OccurredAt
			old.TerminatedAt = &at
		}
	}
	if old := p.Terms[key]; old != nil {
		if end.Equal(old.End) && e.Payload["deleted_at"] == nil && bString(e.Payload, "status") != "revoked" {
			return
		}
		at := e.OccurredAt
		if e.Payload["deleted_at"] != nil || bString(e.Payload, "status") == "revoked" {
			old.TerminatedAt = &at
			return
		}
		// Extension: keep earned revenue unchanged; spread only the remaining
		// consideration over the revised remaining service period.
		earned := businessAccrued(old.Value, old.Start, old.End, old.Start, at)
		remaining := old.Value.Sub(earned)
		old.Value = earned
		old.End = at
		old.OriginalEnd = at
		clone := *old
		clone.Key = key + fmt.Sprintf(":revision:%d", e.ID)
		clone.EventID = e.ID
		clone.Start = at
		clone.End = end
		clone.OriginalEnd = end
		clone.Value = remaining
		delete(p.Terms, key)
		p.Terms[old.Key+fmt.Sprintf(":closed:%d", e.ID)] = old
		p.Terms[key] = &clone
		return
	}
	term := &BusinessTerm{Key: key, EventID: e.ID, UserID: e.UserID, SubscriptionID: id, Version: version, GroupID: bInt(e.Payload, "group_id"), GroupName: bString(e.Payload, "group_name"), Start: start, End: end, OriginalEnd: end, Quality: "unknown"}
	order := bMap(e.Payload, "order")
	if order != nil {
		term.OrderID = bInt(order, "id")
		term.PlanID = bInt(order, "plan_id")
		term.PlanName = bString(order, "plan_name")
		if bString(order, "payment_type") == "balance" {
			term.Value = transfer.Value
			term.FundingSources = transfer.Sources
			term.GiftShare = giftFraction(transfer)
			if transfer.Unknown.IsZero() && transfer.Total().IsPositive() {
				term.Quality = "confirmed"
			}
		} else if value, ok := bMoney(order); ok {
			term.Value = value
			term.Quality = "confirmed"
		}
	} else {
		for _, s := range events {
			if s.UserID == e.UserID && (s.Type == "lottery_draws" || s.Type == "welfare_records" || (s.Type == "redeem_codes" && bString(s.Payload, "business_classification") == "gift")) {
				term.Quality = "confirmed"
				term.GiftShare = decimal.NewFromInt(1)
			}
		}
	}
	if bBool(e.Payload, "resolved") {
		term.Value = bDecimal(e.Payload, "amount_cny")
		term.Quality = "confirmed"
		term.GiftShare = bDecimal(e.Payload, "gift_share")
	}
	p.Terms[key] = term
}

func (p *BusinessProjection) ticket(e BusinessEvent, transfer BusinessFunds) {
	id := bInt(e.Payload, "id")
	remaining := bDecimal(e.Payload, "remaining")
	delta := bDecimal(e.Payload, "delta")
	existing := p.TicketFunds[id]
	if existing == nil {
		f := BusinessFunds{Gift: remaining}
		if transfer.Total().IsPositive() {
			ratio := remaining.Div(transfer.Total())
			f = BusinessFunds{Paid: transfer.Paid.Mul(ratio), Gift: transfer.Gift.Mul(ratio), Unknown: transfer.Unknown.Mul(ratio), Value: transfer.Value, Sources: transfer.Sources}
		}
		if bString(e.Payload, "source_type") == "purchase" && transfer.Total().IsZero() {
			f = BusinessFunds{Unknown: remaining}
		}
		p.TicketFunds[id] = &f
		return
	}
	used := existing.Total().Sub(remaining)
	if used.IsPositive() {
		slice := existing.Take(used)
		if e.Payload["revoked_at"] == nil {
			p.revenue(e, slice, "ticket_revenue")
		} else if slice.Value.IsPositive() {
			p.entry(e, "revenue_gap", nil, delta, "unknown", map[string]any{"reason": "付费抽奖券撤销，待核对退回"})
		}
	}
}

func businessAccrued(amount decimal.Decimal, from, to, start, end time.Time) decimal.Decimal {
	if !to.After(from) || !end.After(start) {
		return decimal.Zero
	}
	a := from
	if start.After(a) {
		a = start
	}
	b := to
	if end.Before(b) {
		b = end
	}
	if !b.After(a) {
		return decimal.Zero
	}
	return amount.Mul(decimal.NewFromInt(b.Sub(a).Nanoseconds())).Div(decimal.NewFromInt(to.Sub(from).Nanoseconds())).Round(8)
}
