package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"entgo.io/ent/dialect"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/ent/userattributedefinition"
	"github.com/Wei-Shaw/sub2api/ent/userattributevalue"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/shopspring/decimal"
)

const (
	LoyaltyWeeklyPointsAttributeKey    = "loyalty_weekly_points"
	LoyaltyPermanentPointsAttributeKey = "loyalty_permanent_points"

	paymentLoyaltyAuditAction      = "LOYALTY_POINTS_APPLIED"
	paymentLoyaltyProviderSnapshot = "loyalty"
	paymentLoyaltyDefaultCurrency  = payment.DefaultPaymentCurrency
	paymentLoyaltyDefinitionType   = "number"
)

type PaymentLoyaltyRule struct {
	Scope    string  `json:"scope"`
	Level    string  `json:"level"`
	Points   float64 `json:"points"`
	Discount float64 `json:"discount"`
}

var paymentWeeklyLoyaltyRules = []PaymentLoyaltyRule{
	{Scope: "weekly", Level: "L1", Points: 20, Discount: 2},
	{Scope: "weekly", Level: "L2", Points: 200, Discount: 4},
	{Scope: "weekly", Level: "L3", Points: 400, Discount: 6},
	{Scope: "weekly", Level: "L4", Points: 800, Discount: 8},
}

var paymentPermanentLoyaltyRules = []PaymentLoyaltyRule{
	{Scope: "permanent", Level: "L2", Points: 800, Discount: 4},
	{Scope: "permanent", Level: "L3", Points: 4000, Discount: 6},
	{Scope: "permanent", Level: "L4", Points: 8000, Discount: 8},
}

func clonePaymentLoyaltyRules(rules []PaymentLoyaltyRule) []PaymentLoyaltyRule {
	return append([]PaymentLoyaltyRule(nil), rules...)
}

func defaultPaymentLoyaltyRules(scope string) []PaymentLoyaltyRule {
	if normalizePaymentLoyaltyScope(scope) == "permanent" {
		return clonePaymentLoyaltyRules(paymentPermanentLoyaltyRules)
	}
	return clonePaymentLoyaltyRules(paymentWeeklyLoyaltyRules)
}

func DefaultPaymentLoyaltyRulesJSON(scope string) string {
	data, err := json.Marshal(defaultPaymentLoyaltyRules(scope))
	if err != nil {
		return "[]"
	}
	return string(data)
}

func normalizePaymentLoyaltyScope(scope string) string {
	if strings.TrimSpace(scope) == "permanent" {
		return "permanent"
	}
	return "weekly"
}

func NormalizePaymentLoyaltyRulesJSON(scope string, raw string) (string, error) {
	rules, err := parsePaymentLoyaltyRulesJSON(scope, raw)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(rules)
	if err != nil {
		return "", fmt.Errorf("marshal loyalty %s rules: %w", normalizePaymentLoyaltyScope(scope), err)
	}
	return string(data), nil
}

func parsePaymentLoyaltyRulesJSON(scope string, raw string) ([]PaymentLoyaltyRule, error) {
	scope = normalizePaymentLoyaltyScope(scope)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultPaymentLoyaltyRules(scope), nil
	}
	var rules []PaymentLoyaltyRule
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return nil, fmt.Errorf("loyalty_%s_rules must be valid JSON array", scope)
	}
	return normalizePaymentLoyaltyRules(scope, rules)
}

func normalizePaymentLoyaltyRules(scope string, rules []PaymentLoyaltyRule) ([]PaymentLoyaltyRule, error) {
	scope = normalizePaymentLoyaltyScope(scope)
	normalized := make([]PaymentLoyaltyRule, 0, len(rules))
	for _, rule := range rules {
		if rule.Points <= 0 || math.IsNaN(rule.Points) || math.IsInf(rule.Points, 0) {
			return nil, fmt.Errorf("loyalty_%s_rules points must be greater than 0", scope)
		}
		if rule.Discount < 0 || rule.Discount > 100 || math.IsNaN(rule.Discount) || math.IsInf(rule.Discount, 0) {
			return nil, fmt.Errorf("loyalty_%s_rules discount must be between 0 and 100", scope)
		}
		normalized = append(normalized, PaymentLoyaltyRule{
			Scope:    scope,
			Level:    strings.TrimSpace(rule.Level),
			Points:   rule.Points,
			Discount: rule.Discount,
		})
	}
	if len(normalized) == 0 {
		return nil, fmt.Errorf("loyalty_%s_rules must include at least one rule", scope)
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		return normalized[i].Points < normalized[j].Points
	})
	for i := range normalized {
		if normalized[i].Level == "" {
			normalized[i].Level = "L" + strconv.Itoa(i+1)
		}
	}
	return normalized, nil
}

func parsePaymentLoyaltyRulesJSONOrDefault(scope string, raw string) []PaymentLoyaltyRule {
	rules, err := parsePaymentLoyaltyRulesJSON(scope, raw)
	if err != nil {
		return defaultPaymentLoyaltyRules(scope)
	}
	return rules
}

func (s *PaymentService) loadPaymentLoyaltyRules(ctx context.Context) ([]PaymentLoyaltyRule, []PaymentLoyaltyRule) {
	weeklyRules := defaultPaymentLoyaltyRules("weekly")
	permanentRules := defaultPaymentLoyaltyRules("permanent")
	if s == nil || s.configService == nil || s.configService.settingRepo == nil {
		return weeklyRules, permanentRules
	}
	values, err := s.configService.settingRepo.GetMultiple(ctx, []string{
		SettingKeyLoyaltyWeeklyRules,
		SettingKeyLoyaltyPermanentRules,
	})
	if err != nil {
		return weeklyRules, permanentRules
	}
	weeklyRules = parsePaymentLoyaltyRulesJSONOrDefault("weekly", values[SettingKeyLoyaltyWeeklyRules])
	permanentRules = parsePaymentLoyaltyRulesJSONOrDefault("permanent", values[SettingKeyLoyaltyPermanentRules])
	return weeklyRules, permanentRules
}

type PaymentLoyaltyInfo struct {
	Enabled               bool                 `json:"enabled"`
	DefinitionsConfigured bool                 `json:"definitions_configured"`
	WeeklyPoints          float64              `json:"weekly_points"`
	PermanentPoints       float64              `json:"permanent_points"`
	WeeklyDiscount        float64              `json:"weekly_discount"`
	PermanentDiscount     float64              `json:"permanent_discount"`
	DiscountPercent       float64              `json:"discount_percent"`
	DiscountScope         string               `json:"discount_scope,omitempty"`
	DiscountLevel         string               `json:"discount_level,omitempty"`
	NextWeeklyResetAt     *time.Time           `json:"next_weekly_reset_at,omitempty"`
	WeeklyRules           []PaymentLoyaltyRule `json:"weekly_rules"`
	PermanentRules        []PaymentLoyaltyRule `json:"permanent_rules"`
}

type paymentLoyaltyAttributeSpec struct {
	Key          string
	Name         string
	Description  string
	DisplayOrder int
}

var paymentLoyaltyAttributeSpecs = []paymentLoyaltyAttributeSpec{
	{
		Key:          LoyaltyWeeklyPointsAttributeKey,
		Name:         "周积分",
		Description:  "会员计划本周累计积分。每周一按服务端时区重新计算有效周期。",
		DisplayOrder: 900,
	},
	{
		Key:          LoyaltyPermanentPointsAttributeKey,
		Name:         "永久积分",
		Description:  "会员计划永久累计积分。用于解锁长期充值折扣。",
		DisplayOrder: 901,
	},
}

func (s *PaymentService) GetLoyaltyInfo(ctx context.Context, userID int64) (*PaymentLoyaltyInfo, error) {
	return s.getLoyaltyInfoAt(ctx, userID, timezone.Now())
}

func (s *PaymentService) getLoyaltyInfoAt(ctx context.Context, userID int64, now time.Time) (*PaymentLoyaltyInfo, error) {
	weeklyRules, permanentRules := s.loadPaymentLoyaltyRules(ctx)
	info := &PaymentLoyaltyInfo{
		WeeklyRules:    weeklyRules,
		PermanentRules: permanentRules,
	}
	if s == nil || s.entClient == nil || userID <= 0 {
		return info, nil
	}

	defs, configured, err := s.ensureLoyaltyAttributeDefinitions(ctx)
	if err != nil {
		return nil, err
	}
	info.DefinitionsConfigured = configured
	if !configured {
		return info, nil
	}

	weekStart := timezone.StartOfWeek(now)
	nextWeeklyReset := weekStart.AddDate(0, 0, 7)
	info.NextWeeklyResetAt = &nextWeeklyReset

	if def := defs[LoyaltyWeeklyPointsAttributeKey]; def != nil {
		points, err := s.readLoyaltyPoints(ctx, userID, def.ID, &weekStart)
		if err != nil {
			return nil, err
		}
		info.WeeklyPoints = points
	}
	if def := defs[LoyaltyPermanentPointsAttributeKey]; def != nil {
		points, err := s.readLoyaltyPoints(ctx, userID, def.ID, nil)
		if err != nil {
			return nil, err
		}
		info.PermanentPoints = points
	}

	weeklyRule := resolvePaymentLoyaltyRule(info.WeeklyPoints, weeklyRules)
	permanentRule := resolvePaymentLoyaltyRule(info.PermanentPoints, permanentRules)
	if weeklyRule != nil {
		info.WeeklyDiscount = weeklyRule.Discount
	}
	if permanentRule != nil {
		info.PermanentDiscount = permanentRule.Discount
	}
	best := betterPaymentLoyaltyRule(weeklyRule, permanentRule)
	if best != nil {
		info.Enabled = true
		info.DiscountPercent = best.Discount
		info.DiscountScope = best.Scope
		info.DiscountLevel = best.Level
	}
	return info, nil
}

func resolvePaymentLoyaltyRule(points float64, rules []PaymentLoyaltyRule) *PaymentLoyaltyRule {
	if points <= 0 || math.IsNaN(points) || math.IsInf(points, 0) {
		return nil
	}
	var current *PaymentLoyaltyRule
	for i := range rules {
		rule := &rules[i]
		if points >= rule.Points && (current == nil || rule.Points > current.Points) {
			current = rule
		}
	}
	return current
}

func betterPaymentLoyaltyRule(a, b *PaymentLoyaltyRule) *PaymentLoyaltyRule {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if b.Discount > a.Discount {
		return b
	}
	if b.Discount == a.Discount && b.Points > a.Points {
		return b
	}
	return a
}

func applyPaymentLoyaltyDiscount(amount float64, discountPercent float64, currency string) float64 {
	if amount <= 0 || discountPercent <= 0 {
		return amount
	}
	if discountPercent >= 100 {
		return 0
	}
	if strings.TrimSpace(currency) == "" {
		currency = paymentLoyaltyDefaultCurrency
	}
	fractionDigits := int32(payment.CurrencyMaxFractionDigits(currency))
	return decimal.NewFromFloat(amount).
		Mul(decimal.NewFromFloat(100 - discountPercent)).
		Div(decimal.NewFromInt(100)).
		Round(fractionDigits).
		InexactFloat64()
}

func paymentLoyaltyDiscountAmount(originalAmount, discountedAmount float64, currency string) float64 {
	if originalAmount <= discountedAmount {
		return 0
	}
	if strings.TrimSpace(currency) == "" {
		currency = paymentLoyaltyDefaultCurrency
	}
	fractionDigits := int32(payment.CurrencyMaxFractionDigits(currency))
	return decimal.NewFromFloat(originalAmount).
		Sub(decimal.NewFromFloat(discountedAmount)).
		Round(fractionDigits).
		InexactFloat64()
}

func (s *PaymentService) ensureLoyaltyAttributeDefinitions(ctx context.Context) (map[string]*dbent.UserAttributeDefinition, bool, error) {
	defs := make(map[string]*dbent.UserAttributeDefinition, len(paymentLoyaltyAttributeSpecs))
	if s == nil || s.entClient == nil {
		return defs, false, nil
	}
	for _, spec := range paymentLoyaltyAttributeSpecs {
		def, err := s.ensureLoyaltyAttributeDefinition(ctx, spec)
		if err != nil {
			return nil, false, err
		}
		if def != nil && def.Type == paymentLoyaltyDefinitionType && def.Enabled {
			defs[spec.Key] = def
		}
	}
	return defs, defs[LoyaltyWeeklyPointsAttributeKey] != nil && defs[LoyaltyPermanentPointsAttributeKey] != nil, nil
}

func (s *PaymentService) ensureLoyaltyAttributeDefinition(ctx context.Context, spec paymentLoyaltyAttributeSpec) (*dbent.UserAttributeDefinition, error) {
	client := paymentServiceClientFromContext(ctx, s.entClient)
	def, err := client.UserAttributeDefinition.Query().
		Where(userattributedefinition.KeyEQ(spec.Key)).
		Only(ctx)
	if err == nil {
		return def, nil
	}
	if err != nil && !dbent.IsNotFound(err) {
		return nil, err
	}

	if err := s.insertLoyaltyAttributeDefinition(ctx, spec); err != nil {
		return nil, err
	}
	return client.UserAttributeDefinition.Query().
		Where(userattributedefinition.KeyEQ(spec.Key)).
		Only(ctx)
}

func (s *PaymentService) insertLoyaltyAttributeDefinition(ctx context.Context, spec paymentLoyaltyAttributeSpec) error {
	client := paymentServiceClientFromContext(ctx, s.entClient)
	if client.Driver() != nil && client.Driver().Dialect() == dialect.SQLite {
		_, err := client.ExecContext(ctx, `
INSERT INTO user_attribute_definitions
	(key, name, description, type, options, required, validation, placeholder, display_order, enabled, created_at, updated_at)
VALUES (?, ?, ?, 'number', '[]', FALSE, ?, '0', ?, TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT DO NOTHING`,
			spec.Key, spec.Name, spec.Description, `{"min":0}`, spec.DisplayOrder)
		return err
	}
	_, err := client.ExecContext(ctx, `
INSERT INTO user_attribute_definitions
	(key, name, description, type, options, required, validation, placeholder, display_order, enabled, created_at, updated_at)
VALUES ($1, $2, $3, 'number', '[]'::jsonb, FALSE, '{"min":0}'::jsonb, '0', $4, TRUE, NOW(), NOW())
ON CONFLICT DO NOTHING`,
		spec.Key, spec.Name, spec.Description, spec.DisplayOrder)
	return err
}

func (s *PaymentService) readLoyaltyPoints(ctx context.Context, userID, attributeID int64, resetBefore *time.Time) (float64, error) {
	client := paymentServiceClientFromContext(ctx, s.entClient)
	value, err := client.UserAttributeValue.Query().
		Where(
			userattributevalue.UserIDEQ(userID),
			userattributevalue.AttributeIDEQ(attributeID),
		).
		Only(ctx)
	if dbent.IsNotFound(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if resetBefore != nil && value.UpdatedAt.Before(*resetBefore) {
		return 0, nil
	}
	return parsePositiveLoyaltyPoints(value.Value), nil
}

func parsePositiveLoyaltyPoints(raw string) float64 {
	points, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || points <= 0 || math.IsNaN(points) || math.IsInf(points, 0) {
		return 0
	}
	return points
}

func (s *PaymentService) applyLoyaltyPointsForOrder(ctx context.Context, o *dbent.PaymentOrder) error {
	if s == nil || s.entClient == nil || o == nil {
		return nil
	}
	if o.OrderType != payment.OrderTypeBalance && o.OrderType != payment.OrderTypeSubscription {
		return nil
	}
	pointsDelta := paymentLoyaltyPointsDeltaFromOrder(o)
	if pointsDelta <= 0 {
		return nil
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin loyalty points transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	txCtx := dbent.NewTxContext(ctx, tx)
	claimed, err := s.tryClaimLoyaltyPointsAudit(txCtx, tx.Client(), o.ID, pointsDelta)
	if err != nil {
		return fmt.Errorf("claim loyalty points audit: %w", err)
	}
	if !claimed {
		return nil
	}

	defs, configured, err := s.ensureLoyaltyAttributeDefinitions(txCtx)
	if err != nil {
		return fmt.Errorf("ensure loyalty attribute definitions: %w", err)
	}
	if !configured {
		return nil
	}

	weekStart := timezone.StartOfWeek(timezone.Now())
	if def := defs[LoyaltyWeeklyPointsAttributeKey]; def != nil {
		if err := s.incrementLoyaltyPoints(txCtx, o.UserID, def.ID, pointsDelta, &weekStart); err != nil {
			return fmt.Errorf("increment weekly loyalty points: %w", err)
		}
	}
	if def := defs[LoyaltyPermanentPointsAttributeKey]; def != nil {
		if err := s.incrementLoyaltyPoints(txCtx, o.UserID, def.ID, pointsDelta, nil); err != nil {
			return fmt.Errorf("increment permanent loyalty points: %w", err)
		}
	}

	if err := s.updateClaimedLoyaltyPointsAudit(txCtx, tx.Client(), o.ID, map[string]any{
		"pointsDelta":           pointsDelta,
		"weeklyAttributeKey":    LoyaltyWeeklyPointsAttributeKey,
		"permanentAttributeKey": LoyaltyPermanentPointsAttributeKey,
	}); err != nil {
		return fmt.Errorf("update loyalty points audit: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit loyalty points transaction: %w", err)
	}
	return nil
}

func (s *PaymentService) incrementLoyaltyPoints(ctx context.Context, userID, attributeID int64, delta float64, resetBefore *time.Time) error {
	client := paymentServiceClientFromContext(ctx, s.entClient)
	value := formatLoyaltyPointNumber(delta)
	if client.Driver() != nil && client.Driver().Dialect() == dialect.SQLite {
		return s.incrementLoyaltyPointsSQLite(ctx, client, userID, attributeID, delta, value, resetBefore)
	}
	return s.incrementLoyaltyPointsPostgres(ctx, client, userID, attributeID, delta, value, resetBefore)
}

func (s *PaymentService) incrementLoyaltyPointsPostgres(ctx context.Context, client *dbent.Client, userID, attributeID int64, delta float64, value string, resetBefore *time.Time) error {
	_, err := client.ExecContext(ctx, `
INSERT INTO user_attribute_values (user_id, attribute_id, value, created_at, updated_at)
VALUES ($1, $2, $3, NOW(), NOW())
ON CONFLICT (user_id, attribute_id) DO UPDATE SET
	value = (
		CASE
			WHEN $5::timestamptz IS NOT NULL AND user_attribute_values.updated_at < $5::timestamptz THEN $4::numeric
			ELSE (
				CASE
					WHEN user_attribute_values.value ~ '^-?[0-9]+(\.[0-9]+)?$' THEN user_attribute_values.value::numeric
					ELSE 0
				END
			) + $4::numeric
		END
	)::text,
	updated_at = NOW()`,
		userID, attributeID, value, delta, resetBefore)
	return err
}

func (s *PaymentService) incrementLoyaltyPointsSQLite(ctx context.Context, client *dbent.Client, userID, attributeID int64, delta float64, value string, resetBefore *time.Time) error {
	_, err := client.ExecContext(ctx, `
INSERT INTO user_attribute_values (user_id, attribute_id, value, created_at, updated_at)
VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (user_id, attribute_id) DO UPDATE SET
	value = (
		CASE
			WHEN ? IS NOT NULL AND user_attribute_values.updated_at < ? THEN ?
			ELSE CAST(COALESCE(NULLIF(user_attribute_values.value, ''), '0') AS REAL) + ?
		END
	),
	updated_at = CURRENT_TIMESTAMP`,
		userID, attributeID, value, resetBefore, resetBefore, value, delta)
	return err
}

func paymentLoyaltyPointsDeltaFromOrder(o *dbent.PaymentOrder) float64 {
	snapshot := paymentLoyaltySnapshotFromOrder(o)
	return positiveFloatFromAny(snapshot["points_delta"])
}

func paymentLoyaltySnapshotFromOrder(o *dbent.PaymentOrder) map[string]any {
	if o == nil || len(o.ProviderSnapshot) == 0 {
		return nil
	}
	raw, ok := o.ProviderSnapshot[paymentLoyaltyProviderSnapshot]
	if !ok {
		return nil
	}
	snapshot, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	return snapshot
}

func buildPaymentLoyaltySnapshot(info *PaymentLoyaltyInfo, originalAmount, discountedAmount, pointsDelta float64, currency string) map[string]any {
	if info == nil {
		return nil
	}
	snapshot := map[string]any{
		"weekly_points":       info.WeeklyPoints,
		"permanent_points":    info.PermanentPoints,
		"weekly_discount":     info.WeeklyDiscount,
		"permanent_discount":  info.PermanentDiscount,
		"discount_percent":    info.DiscountPercent,
		"discount_scope":      info.DiscountScope,
		"discount_level":      info.DiscountLevel,
		"original_amount":     originalAmount,
		"discounted_amount":   discountedAmount,
		"discount_amount":     paymentLoyaltyDiscountAmount(originalAmount, discountedAmount, currency),
		"points_delta":        pointsDelta,
		"weekly_attribute":    LoyaltyWeeklyPointsAttributeKey,
		"permanent_attribute": LoyaltyPermanentPointsAttributeKey,
	}
	if info.NextWeeklyResetAt != nil {
		snapshot["next_weekly_reset_at"] = info.NextWeeklyResetAt.Format(time.RFC3339)
	}
	return snapshot
}

func positiveFloatFromAny(value any) float64 {
	switch v := value.(type) {
	case float64:
		if v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0) {
			return v
		}
	case float32:
		f := float64(v)
		if f > 0 && !math.IsNaN(f) && !math.IsInf(f, 0) {
			return f
		}
	case int:
		if v > 0 {
			return float64(v)
		}
	case int64:
		if v > 0 {
			return float64(v)
		}
	case json.Number:
		f, err := v.Float64()
		if err == nil && f > 0 && !math.IsNaN(f) && !math.IsInf(f, 0) {
			return f
		}
	case string:
		return parsePositiveLoyaltyPoints(v)
	}
	return 0
}

func formatLoyaltyPointNumber(value float64) string {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return "0"
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func (s *PaymentService) tryClaimLoyaltyPointsAudit(ctx context.Context, client *dbent.Client, orderID int64, pointsDelta float64) (bool, error) {
	oid := strconv.FormatInt(orderID, 10)
	detail, _ := json.Marshal(map[string]any{
		"pointsDelta": pointsDelta,
		"status":      "reserved",
	})
	query, args := buildLoyaltyPointsAuditClaimQuery(client, oid, string(detail))
	rows, err := client.QueryContext(ctx, query, args...)
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, err
		}
		return false, nil
	}
	var claimID int64
	if err := rows.Scan(&claimID); err != nil {
		return false, err
	}
	return true, nil
}

func buildLoyaltyPointsAuditClaimQuery(client *dbent.Client, orderID, detail string) (string, []any) {
	nowExpr := paymentAuditCurrentTimestampExpr(client)
	if paymentAuditDialect(client) == dialect.Postgres {
		return fmt.Sprintf(`
INSERT INTO payment_audit_logs (order_id, action, detail, operator, created_at)
VALUES ($1::text, '%s', $2::text, 'system', %s)
ON CONFLICT (order_id, action) DO NOTHING
RETURNING id`, paymentLoyaltyAuditAction, nowExpr), []any{orderID, detail}
	}
	return fmt.Sprintf(`
INSERT INTO payment_audit_logs (order_id, action, detail, operator, created_at)
VALUES (?, '%s', ?, 'system', %s)
ON CONFLICT (order_id, action) DO NOTHING
RETURNING id`, paymentLoyaltyAuditAction, nowExpr), []any{orderID, detail}
}

func (s *PaymentService) updateClaimedLoyaltyPointsAudit(ctx context.Context, client *dbent.Client, orderID int64, detail map[string]any) error {
	oid := strconv.FormatInt(orderID, 10)
	detailJSON, _ := json.Marshal(detail)
	updated, err := client.PaymentAuditLog.Update().
		Where(
			paymentauditlog.OrderIDEQ(oid),
			paymentauditlog.ActionEQ(paymentLoyaltyAuditAction),
		).
		SetDetail(string(detailJSON)).
		SetOperator("system").
		Save(ctx)
	if err != nil {
		return err
	}
	if updated == 0 {
		return fmt.Errorf("loyalty points audit claim not found")
	}
	return nil
}

func paymentServiceClientFromContext(ctx context.Context, defaultClient *dbent.Client) *dbent.Client {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return tx.Client()
	}
	return defaultClient
}
