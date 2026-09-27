package service

import (
	"fmt"
	"strconv"
	"time"

	"github.com/shopspring/decimal"
)

// Amounts are decimal strings on the wire. Credits and currency are never
// inferred from a display symbol or a legacy *_usd column name.
type BusinessEvent struct {
	ID            int64          `json:"id"`
	SourceKey     string         `json:"source_key"`
	Type          string         `json:"event_type"`
	TransactionID int64          `json:"transaction_id"`
	UserID        int64          `json:"user_id"`
	OccurredAt    time.Time      `json:"occurred_at"`
	RecordedAt    time.Time      `json:"recorded_at"`
	Payload       map[string]any `json:"payload"`
	ActorID       int64          `json:"actor_id"`
	ReversesID    int64          `json:"reverses_id"`
}

type BusinessEntry struct {
	ID        string           `json:"id"`
	EventID   int64            `json:"event_id"`
	At        time.Time        `json:"occurred_at"`
	Kind      string           `json:"kind"`
	UserID    int64            `json:"user_id"`
	AccountID int64            `json:"account_id"`
	GroupID   int64            `json:"group_id"`
	PlanID    int64            `json:"plan_id"`
	Model     string           `json:"model"`
	Amount    *decimal.Decimal `json:"amount_cny"`
	Credits   decimal.Decimal  `json:"credits"`
	Quality   string           `json:"quality"`
	Detail    map[string]any   `json:"detail"`
}

type BusinessFunds struct {
	Sources []int64         `json:"source_event_ids,omitempty"`
	Paid    decimal.Decimal `json:"paid"`
	Gift    decimal.Decimal `json:"gift"`
	Unknown decimal.Decimal `json:"unknown"`
	Value   decimal.Decimal `json:"value_cny"`
}

func (f BusinessFunds) Total() decimal.Decimal { return f.Paid.Add(f.Gift).Add(f.Unknown) }
func (f *BusinessFunds) Add(v BusinessFunds) {
	f.Paid = f.Paid.Add(v.Paid)
	f.Gift = f.Gift.Add(v.Gift)
	f.Unknown = f.Unknown.Add(v.Unknown)
	f.Value = f.Value.Add(v.Value)
	seen := map[int64]bool{}
	for _, id := range f.Sources {
		seen[id] = true
	}
	for _, id := range v.Sources {
		if !seen[id] {
			f.Sources = append(f.Sources, id)
			seen[id] = true
		}
	}
}

// Take preserves the cash basis and splits paid/gift/unknown proportionally.
// An overdraft is unknown revenue, never an invented cash receipt.
func (f *BusinessFunds) Take(amount decimal.Decimal) BusinessFunds {
	if !amount.IsPositive() {
		return BusinessFunds{}
	}
	total := f.Total()
	if !total.IsPositive() {
		return BusinessFunds{Unknown: amount}
	}
	n := decimal.Min(amount, total)
	if n.Equal(total) {
		result := *f
		*f = BusinessFunds{}
		result.Unknown = result.Unknown.Add(amount.Sub(n))
		return result
	}
	ratio := n.Div(total)
	paid := decimal.Min(n, decimal.Min(f.Paid, f.Paid.Mul(ratio).Round(8)))
	gift := decimal.Min(f.Gift, decimal.Min(n.Sub(paid), f.Gift.Mul(ratio).Round(8)))
	unknown := decimal.Min(f.Unknown, n.Sub(paid).Sub(gift))
	residue := n.Sub(paid).Sub(gift).Sub(unknown)
	addPaid := decimal.Min(residue, f.Paid.Sub(paid))
	paid = paid.Add(addPaid)
	residue = residue.Sub(addPaid)
	gift = gift.Add(decimal.Min(residue, f.Gift.Sub(gift)))
	value := decimal.Zero
	if f.Paid.IsPositive() {
		value = decimal.Min(f.Value, f.Value.Mul(paid).Div(f.Paid).Round(8))
	}
	f.Paid = f.Paid.Sub(paid)
	f.Gift = f.Gift.Sub(gift)
	f.Unknown = f.Unknown.Sub(unknown)
	f.Value = f.Value.Sub(value)
	return BusinessFunds{Paid: paid, Gift: gift, Unknown: unknown.Add(amount.Sub(n)), Value: value, Sources: append([]int64{}, f.Sources...)}
}

type BusinessTerm struct {
	FundingSources []int64         `json:"funding_event_ids"`
	Key            string          `json:"key"`
	EventID        int64           `json:"event_id"`
	UserID         int64           `json:"user_id"`
	SubscriptionID int64           `json:"subscription_id"`
	Version        int64           `json:"version"`
	OrderID        int64           `json:"order_id"`
	GroupID        int64           `json:"group_id"`
	GroupName      string          `json:"group_name"`
	PlanID         int64           `json:"plan_id"`
	PlanName       string          `json:"plan_name"`
	Start          time.Time       `json:"starts_at"`
	End            time.Time       `json:"ends_at"`
	OriginalEnd    time.Time       `json:"original_ends_at"`
	Value          decimal.Decimal `json:"value_cny"`
	GiftShare      decimal.Decimal `json:"gift_share"`
	Quality        string          `json:"quality"`
	TerminatedAt   *time.Time      `json:"terminated_at,omitempty"`
}

type BusinessExpense struct {
	Event    BusinessEvent   `json:"event"`
	Amount   decimal.Decimal `json:"amount_cny"`
	Start    time.Time       `json:"starts_at"`
	End      time.Time       `json:"ends_at"`
	Category string          `json:"category"`
}

type BusinessProjection struct {
	entryCounts   map[string]int
	Bills         []BusinessEvent           `json:"bills"`
	Wallets       map[int64]*BusinessFunds  `json:"wallets"`
	Held          map[int64]*BusinessFunds  `json:"held"`
	Pools         map[int64]*BusinessFunds  `json:"pools"`
	Terms         map[string]*BusinessTerm  `json:"terms"`
	Expenses      []BusinessExpense         `json:"expenses"`
	Seen          map[string]bool           `json:"seen"`
	RefundFunds   map[int64]*BusinessFunds  `json:"refund_funds"`
	BatchFunds    map[string]*BusinessFunds `json:"batch_funds"`
	RefundAmounts map[int64]decimal.Decimal `json:"refund_amounts"`
	TicketFunds   map[int64]*BusinessFunds  `json:"ticket_funds"`
	Entries       []BusinessEntry           `json:"-"`
}

func NewBusinessProjection() *BusinessProjection {
	return &BusinessProjection{Wallets: map[int64]*BusinessFunds{}, Held: map[int64]*BusinessFunds{}, Pools: map[int64]*BusinessFunds{}, Terms: map[string]*BusinessTerm{}, Seen: map[string]bool{}, TicketFunds: map[int64]*BusinessFunds{}, Expenses: []BusinessExpense{}, RefundFunds: map[int64]*BusinessFunds{}, BatchFunds: map[string]*BusinessFunds{}, RefundAmounts: map[int64]decimal.Decimal{}}
}
func businessFund(m map[int64]*BusinessFunds, id int64) *BusinessFunds {
	if m[id] == nil {
		m[id] = &BusinessFunds{}
	}
	return m[id]
}
func bString(m map[string]any, k string) string {
	if m == nil || m[k] == nil {
		return ""
	}
	return fmt.Sprint(m[k])
}
func bDecimal(m map[string]any, k string) decimal.Decimal {
	d, _ := decimal.NewFromString(bString(m, k))
	return d
}
func bInt(m map[string]any, k string) int64 {
	n, _ := strconv.ParseInt(bString(m, k), 10, 64)
	return n
}
func bMap(m map[string]any, k string) map[string]any { v, _ := m[k].(map[string]any); return v }
func bTime(m map[string]any, k string, fallback time.Time) time.Time {
	v, err := time.Parse(time.RFC3339Nano, bString(m, k))
	if err != nil {
		return fallback
	}
	return v
}
func bBool(m map[string]any, k string) bool { return bString(m, k) == "true" }
func bMoney(m map[string]any) (decimal.Decimal, bool) {
	if m["amount_cny"] != nil {
		d, err := decimal.NewFromString(bString(m, "amount_cny"))
		return d, err == nil
	}
	currency := bString(m, "currency")
	if currency == "" || currency == "CNY" {
		d, err := decimal.NewFromString(bString(m, "pay_amount"))
		return d, err == nil
	}
	if bDecimal(m, "fx_rate").IsPositive() {
		return bDecimal(m, "pay_amount").Mul(bDecimal(m, "fx_rate")).Round(8), true
	}
	return decimal.Zero, false
}
func (p *BusinessProjection) entry(e BusinessEvent, kind string, amount *decimal.Decimal, credits decimal.Decimal, quality string, detail map[string]any) {
	if detail == nil {
		detail = map[string]any{}
	}
	detail["source_key"] = e.SourceKey
	for _, key := range []string{"account_name", "group_name", "pool", "rule", "currency", "pay_amount", "category"} {
		if v := e.Payload[key]; v != nil {
			detail[key] = v
		}
	}
	if p.entryCounts == nil {
		p.entryCounts = map[string]int{}
	}
	key := fmt.Sprintf("%d:%s", e.ID, kind)
	index := p.entryCounts[key]
	p.entryCounts[key] = index + 1
	p.Entries = append(p.Entries, BusinessEntry{ID: fmt.Sprintf("%s:%d", key, index), EventID: e.ID, At: e.OccurredAt, Kind: kind, UserID: e.UserID, AccountID: bInt(e.Payload, "account_id"), GroupID: bInt(e.Payload, "group_id"), PlanID: bInt(e.Payload, "plan_id"), Model: bString(e.Payload, "model"), Amount: amount, Credits: credits, Quality: quality, Detail: detail})
}
func (p *BusinessProjection) revenue(e BusinessEvent, f BusinessFunds, kind string) {
	v := f.Value
	p.entry(e, kind, &v, f.Paid, "confirmed", map[string]any{"gift_credits": f.Gift.String(), "paid_credits": f.Paid.String(), "unknown_credits": f.Unknown.String(), "funding_event_ids": f.Sources, "allocation": "按消费时余额组成比例分配"})
	if f.Unknown.IsPositive() {
		p.entry(e, "revenue_gap", nil, f.Unknown, "unknown", map[string]any{"reason": "资金来源未核实"})
	}
}

type BusinessRecordInput struct {
	IdempotencyKey string         `json:"idempotency_key"`
	Type           string         `json:"type"`
	At             time.Time      `json:"occurred_at"`
	UserID         int64          `json:"user_id"`
	SourceEventID  int64          `json:"source_event_id"`
	Payload        map[string]any `json:"payload"`
}

type BusinessDataQuality struct {
	Cash           string `json:"cash"`
	Revenue        string `json:"revenue"`
	Cost           string `json:"cost"`
	Attribution    string `json:"attribution"`
	MissingCount   int    `json:"missing_count"`
	EstimatedCount int    `json:"estimated_count"`
}

type BusinessLedgerOverview struct {
	Start                time.Time           `json:"start_at"`
	End                  time.Time           `json:"end_at"`
	EnabledAt            time.Time           `json:"enabled_at"`
	UpdatedAt            time.Time           `json:"updated_at"`
	Revision             int64               `json:"revision"`
	Currency             string              `json:"currency"`
	Timezone             string              `json:"timezone"`
	CashIn               decimal.Decimal     `json:"cash_in_cny"`
	CashRefund           decimal.Decimal     `json:"cash_refund_cny"`
	CashOut              decimal.Decimal     `json:"cash_out_cny"`
	CashNet              decimal.Decimal     `json:"cash_net_cny"`
	Revenue              decimal.Decimal     `json:"recognized_revenue_cny"`
	UsageCost            decimal.Decimal     `json:"usage_cost_cny"`
	FixedCost            decimal.Decimal     `json:"fixed_cost_cny"`
	OperatingCost        decimal.Decimal     `json:"operating_cost_cny"`
	KnownProfit          decimal.Decimal     `json:"known_profit_cny"`
	Profit               *decimal.Decimal    `json:"profit_cny"`
	Margin               *decimal.Decimal    `json:"margin"`
	GiftGranted          decimal.Decimal     `json:"gift_granted_credits"`
	GiftUsed             decimal.Decimal     `json:"gift_used_credits"`
	GiftCost             decimal.Decimal     `json:"gift_cost_cny"`
	Discount             decimal.Decimal     `json:"discount_cny"`
	WalletDeferred       decimal.Decimal     `json:"wallet_deferred_cny"`
	SubscriptionDeferred decimal.Decimal     `json:"subscription_deferred_cny"`
	PrepaidSupplier      decimal.Decimal     `json:"prepaid_supplier_cny"`
	PrepaidExpense       decimal.Decimal     `json:"prepaid_expense_cny"`
	GiftOutstanding      decimal.Decimal     `json:"gift_outstanding_credits"`
	UnknownWalletCredits decimal.Decimal     `json:"unknown_wallet_credits"`
	Quality              BusinessDataQuality `json:"quality"`
	Entries              []BusinessEntry     `json:"entries"`
	Daily                []BusinessBreakdown `json:"daily"`
	Groups               []BusinessBreakdown `json:"groups"`
	Models               []BusinessBreakdown `json:"models"`
	Accounts             []BusinessBreakdown `json:"accounts"`
	Plans                []BusinessBreakdown `json:"plans"`
}

type BusinessBreakdown struct {
	CashIn     decimal.Decimal `json:"cash_in_cny"`
	CashRefund decimal.Decimal `json:"cash_refund_cny"`
	CashOut    decimal.Decimal `json:"cash_out_cny"`
	CashNet    decimal.Decimal `json:"cash_net_cny"`
	Key        string          `json:"key"`
	Name       string          `json:"name"`
	Revenue    decimal.Decimal `json:"revenue_cny"`
	Cost       decimal.Decimal `json:"cost_cny"`
	Profit     decimal.Decimal `json:"profit_cny"`
	Missing    int             `json:"missing_count"`
	Estimated  int             `json:"estimated_count"`
	Allocation string          `json:"allocation"`
}
