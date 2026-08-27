package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/google/uuid"
)

const (
	defaultLotteryPurchasePrice = 30.0
	lotteryPurchaseDailyLimit   = 5
	lotteryFreeTicketValidity   = 30 * 24 * time.Hour
	// Lottery probabilities are stored as decimals with at most six decimal
	// places, then converted to integers only for the secure random draw.
	lotteryProbabilityScale                = int64(1_000_000)
	lotteryPoolVersion                     = "v1"
	lotterySubscriptionGroupKey            = "lottery_subscription_group_id"
	lotteryPrizePoolKey                    = "lottery_prize_pool"
	lotteryEnabledKey                      = "lottery_enabled"
	lotteryInvitationFirstPaymentAmountKey = "lottery_invitation_first_payment_amount"
	lotteryInvitationConsumptionAmountKey  = "lottery_invitation_consumption_amount"
	lotteryPurchasePriceKey                = "lottery_purchase_price"
	lotteryPrizeIDLength                   = 32
	lotteryTicketSourceRefMaxLength        = 128
	lotteryPrizeCooldownMaxSeconds         = 365 * 24 * 60 * 60
	lotteryInternalBalancePaymentType      = "balance"
	lotteryRechargeRewardTierFirst         = 20
	lotteryRechargeRewardTierSecond        = 100
)

type LotteryStatus struct {
	Enabled                bool `json:"enabled"`
	AvailableTickets       int  `json:"available_tickets"`
	PityMisses             int  `json:"pity_misses"`
	PityRemaining          int  `json:"pity_remaining"`
	RemainingPurchases     int  `json:"remaining_purchases"`
	RechargeTicketsToday   int  `json:"recharge_tickets_today"`
	InvitationTicketsToday int  `json:"invitation_tickets_today"`
	PurchasedTicketsToday  int  `json:"purchased_tickets_today"`
	TicketDebt             int  `json:"ticket_debt"`
}

type LotteryTicketAdjustment struct {
	Operation string `json:"operation"`
	Count     int    `json:"count"`
	Reference string `json:"reference"`
	Reason    string `json:"reason"`
}

type LotteryTicketAdjustmentResult struct {
	AvailableTickets int `json:"available_tickets"`
}

type LotteryDrawResult struct {
	ID                       int64      `json:"id"`
	RequestID                string     `json:"request_id"`
	PrizeID                  string     `json:"prize_id"`
	PrizeLabel               string     `json:"prize_label"`
	PrizeType                string     `json:"prize_type"`
	Amount                   float64    `json:"amount"`
	BalanceBefore            *float64   `json:"balance_before,omitempty"`
	BalanceAfter             *float64   `json:"balance_after,omitempty"`
	Guaranteed               bool       `json:"guaranteed"`
	RedeemCode               string     `json:"redeem_code,omitempty"`
	RedeemStatus             string     `json:"redeem_status,omitempty"`
	RedeemExpiresAt          *time.Time `json:"redeem_expires_at,omitempty"`
	SubscriptionValidityDays *int       `json:"subscription_validity_days,omitempty"`
	CreatedAt                time.Time  `json:"created_at"`
}

// LotteryRecentWinner is the privacy-safe payload used by the public lottery
// ticker. It deliberately omits user IDs, emails, and reward references.
type LotteryRecentWinner struct {
	ID          int64     `json:"id"`
	DisplayName string    `json:"display_name"`
	PrizeID     string    `json:"prize_id"`
	PrizeLabel  string    `json:"prize_label"`
	PrizeType   string    `json:"prize_type"`
	Amount      float64   `json:"amount"`
	Probability float64   `json:"probability"`
	Guaranteed  bool      `json:"guaranteed"`
	CreatedAt   time.Time `json:"created_at"`
}

// LotteryPrizeConfig is the operator-managed source of truth for both the
// displayed odds and the server-side lottery draw.
type LotteryPrizeConfig struct {
	ID                  string     `json:"id"`
	Label               string     `json:"label"`
	Type                string     `json:"type"`
	Amount              float64    `json:"amount,omitempty"`
	Probability         float64    `json:"probability"`
	SubscriptionGroupID int64      `json:"subscription_group_id,omitempty"`
	SubscriptionPlanID  int64      `json:"subscription_plan_id,omitempty"`
	EligibleForPity     bool       `json:"eligible_for_pity"`
	CooldownSeconds     int        `json:"cooldown_seconds"`
	CooldownUntil       *time.Time `json:"cooldown_until,omitempty"`
}

type LotteryPrizePoolConfig struct {
	Enabled                      *bool                `json:"enabled,omitempty"`
	Prizes                       []LotteryPrizeConfig `json:"prizes"`
	InvitationFirstPaymentAmount float64              `json:"invitation_first_payment_amount"`
	InvitationConsumptionAmount  float64              `json:"invitation_consumption_amount"`
	PurchasePrice                float64              `json:"purchase_price"`
	BalanceRechargeMultiplier    float64              `json:"balance_recharge_multiplier"`
}

// LotteryBalanceTransaction is an auditable wallet movement caused by the
// lottery. It remains separate from payment orders, because neither a prize
// nor buying a ticket is a recharge.
type LotteryBalanceTransaction struct {
	ID              int64     `json:"id"`
	TransactionType string    `json:"transaction_type"`
	Amount          float64   `json:"amount"`
	Description     string    `json:"description"`
	CreatedAt       time.Time `json:"created_at"`
}

type lotteryPrize struct {
	ID                 string
	Label              string
	Type               string
	Amount             float64
	Weight             int64
	GroupID            int64
	SubscriptionPlanID int64
	Days               int
	CooldownSeconds    int
}

var lotteryPrizeIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{1,63}$`)

type lotteryUserState struct {
	AvailableTickets int
	PityMisses       int
	TicketDebt       int
	PurchaseDate     sql.NullTime
	PurchaseCount    int
}

// LotteryService owns all state-changing lottery operations. The database is
// the authority: the browser never supplies a prize, a probability, or a count.
type LotteryService struct {
	entClient            *dbent.Client
	billingCache         lotteryBalanceCacheInvalidator
	authCacheInvalidator APIKeyAuthCacheInvalidator
}

type lotteryBalanceCacheInvalidator interface {
	InvalidateUserBalance(ctx context.Context, userID int64) error
}

func NewLotteryService(entClient *dbent.Client, billingCache *BillingCacheService, authCacheInvalidator APIKeyAuthCacheInvalidator) *LotteryService {
	return &LotteryService{
		entClient:            entClient,
		billingCache:         billingCache,
		authCacheInvalidator: authCacheInvalidator,
	}
}

func defaultLotteryPrizePoolConfig() LotteryPrizePoolConfig {
	return LotteryPrizePoolConfig{
		Enabled:                      lotteryEnabledPointer(true),
		InvitationFirstPaymentAmount: 20,
		InvitationConsumptionAmount:  100,
		PurchasePrice:                defaultLotteryPurchasePrice,
		BalanceRechargeMultiplier:    defaultBalanceRechargeMultiplier,
		Prizes: []LotteryPrizeConfig{
			{ID: "none", Label: "谢谢参与", Type: "none", Probability: 0.529},
			{ID: "quota-10", Label: "$10", Type: "balance", Amount: 10, Probability: 0.31, EligibleForPity: true},
			{ID: "quota-30", Label: "$30", Type: "balance", Amount: 30, Probability: 0.11, EligibleForPity: true},
			{ID: "quota-100", Label: "$100", Type: "balance", Amount: 100, Probability: 0.05, EligibleForPity: true},
			{ID: "quota-1000", Label: "$1000", Type: "balance", Amount: 1000, Probability: 0.001},
		}}
}

func lotteryEnabledPointer(value bool) *bool {
	return &value
}

type legacyLotteryPrizeConfig struct {
	ID                  string  `json:"id"`
	Label               string  `json:"label"`
	Type                string  `json:"type"`
	Amount              float64 `json:"amount,omitempty"`
	Weight              int64   `json:"weight"`
	SubscriptionGroupID int64   `json:"subscription_group_id,omitempty"`
	EligibleForPity     bool    `json:"eligible_for_pity"`
	CooldownSeconds     int     `json:"cooldown_seconds"`
}

func decodeLotteryPrizePool(raw string) ([]LotteryPrizeConfig, error) {
	var prizes []LotteryPrizeConfig
	if err := json.Unmarshal([]byte(raw), &prizes); err != nil {
		return nil, err
	}
	for _, prize := range prizes {
		if prize.Probability != 0 {
			return prizes, nil
		}
	}

	// Configurations saved before probabilities were exposed used integer
	// weights. Read them once as relative values so existing installations keep
	// their exact odds until an operator saves the new representation.
	var legacy []legacyLotteryPrizeConfig
	if err := json.Unmarshal([]byte(raw), &legacy); err != nil {
		return nil, err
	}
	var total int64
	for _, prize := range legacy {
		total += prize.Weight
	}
	if total <= 0 {
		return prizes, nil
	}
	prizes = make([]LotteryPrizeConfig, 0, len(legacy))
	var assigned int64
	for index, prize := range legacy {
		probability := int64(math.Round(float64(prize.Weight) * float64(lotteryProbabilityScale) / float64(total)))
		if index == len(legacy)-1 {
			probability = lotteryProbabilityScale - assigned
		}
		assigned += probability
		prizes = append(prizes, LotteryPrizeConfig{
			ID: prize.ID, Label: prize.Label, Type: prize.Type, Amount: prize.Amount,
			Probability:         float64(probability) / float64(lotteryProbabilityScale),
			SubscriptionGroupID: prize.SubscriptionGroupID, EligibleForPity: prize.EligibleForPity,
			CooldownSeconds: prize.CooldownSeconds,
		})
	}
	return prizes, nil
}

// GetPrizePoolConfig returns the stored operator configuration, falling back to
// the historic pool for installations that have not saved it yet.
func (s *LotteryService) GetPrizePoolConfig(ctx context.Context) (*LotteryPrizePoolConfig, error) {
	if s == nil || s.entClient == nil {
		return nil, infraerrors.ServiceUnavailable("LOTTERY_UNAVAILABLE", "lottery service is unavailable")
	}
	config := defaultLotteryPrizePoolConfig()
	enabled, err := s.lotteryEnabled(ctx)
	if err != nil {
		return nil, err
	}
	config.Enabled = lotteryEnabledPointer(enabled)
	var raw string
	err = scanOne(ctx, s.entClient, `SELECT value FROM settings WHERE key = $1`, []any{lotteryPrizePoolKey}, &raw)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("get lottery prize pool: %w", err)
	}
	if err == nil && strings.TrimSpace(raw) != "" {
		prizes, err := decodeLotteryPrizePool(raw)
		if err != nil {
			return nil, infraerrors.ServiceUnavailable("LOTTERY_PRIZE_POOL_INVALID", "lottery prize pool is invalid")
		}
		config.Prizes = prizes
	}
	invitationRule, err := s.getLotteryInvitationRule(ctx)
	if err != nil {
		return nil, err
	}
	config.InvitationFirstPaymentAmount = invitationRule.FirstPaymentAmount
	config.InvitationConsumptionAmount = invitationRule.ConsumptionAmount
	purchasePrice, err := s.getLotteryPurchasePrice(ctx, s.entClient)
	if err != nil {
		return nil, err
	}
	config.PurchasePrice = purchasePrice
	rechargeMultiplier, err := s.lotteryBalanceRechargeMultiplier(ctx)
	if err != nil {
		return nil, err
	}
	config.BalanceRechargeMultiplier = rechargeMultiplier
	var groupRaw string
	groupErr := scanOne(ctx, s.entClient, `SELECT value FROM settings WHERE key = $1`, []any{lotterySubscriptionGroupKey}, &groupRaw)
	if groupErr != nil && groupErr != sql.ErrNoRows {
		return nil, fmt.Errorf("get lottery subscription group: %w", groupErr)
	}
	if groupErr == nil {
		legacyGroupID, _ := strconv.ParseInt(strings.TrimSpace(groupRaw), 10, 64)
		for index := range config.Prizes {
			if config.Prizes[index].Type == "subscription" && config.Prizes[index].SubscriptionGroupID == 0 && config.Prizes[index].SubscriptionPlanID == 0 {
				config.Prizes[index].SubscriptionGroupID = legacyGroupID
			}
		}
	}
	// Older installations stored one global subscription group and used display
	// labels such as "日卡 A". Resolve every subscription prize to a real active
	// group name; stale legacy entries are omitted rather than shown to users.
	prizes := make([]LotteryPrizeConfig, 0, len(config.Prizes))
	for _, prize := range config.Prizes {
		if prize.Type == "balance" {
			prize.Label = lotteryBalancePrizeLabel(prize.Amount)
		}
		if prize.Type == "subscription" {
			if prize.SubscriptionPlanID > 0 {
				name, groupID, _, err := s.lotterySubscriptionPlan(ctx, s.entClient, prize.SubscriptionPlanID)
				if err != nil {
					continue
				}
				prize.SubscriptionGroupID = groupID
				prize.Label = name
			} else {
				name, _, err := s.lotterySubscriptionGroup(ctx, s.entClient, prize.SubscriptionGroupID)
				if err != nil {
					continue
				}
				prize.Label = name
			}
		}
		prizes = append(prizes, prize)
	}
	config.Prizes = prizes
	if err := s.applyLotteryPrizeCooldowns(ctx, config.Prizes); err != nil {
		return nil, err
	}
	if err := validateLotteryPrizePoolConfig(config); err != nil {
		return nil, err
	}
	return &config, nil
}

func (s *LotteryService) UpdatePrizePoolConfig(ctx context.Context, config LotteryPrizePoolConfig) (*LotteryPrizePoolConfig, error) {
	if s == nil || s.entClient == nil {
		return nil, infraerrors.ServiceUnavailable("LOTTERY_UNAVAILABLE", "lottery service is unavailable")
	}
	// Older clients only submitted the prize list. Keep their current invitation
	// thresholds instead of treating omitted fields as an invalid zero value.
	if config.InvitationFirstPaymentAmount == 0 || config.InvitationConsumptionAmount == 0 {
		rule, err := s.getLotteryInvitationRule(ctx)
		if err != nil {
			return nil, err
		}
		if config.InvitationFirstPaymentAmount == 0 {
			config.InvitationFirstPaymentAmount = rule.FirstPaymentAmount
		}
		if config.InvitationConsumptionAmount == 0 {
			config.InvitationConsumptionAmount = rule.ConsumptionAmount
		}
	}
	if config.PurchasePrice == 0 {
		purchasePrice, err := s.getLotteryPurchasePrice(ctx, s.entClient)
		if err != nil {
			return nil, err
		}
		config.PurchasePrice = purchasePrice
	}
	if config.Enabled == nil {
		enabled, err := s.lotteryEnabled(ctx)
		if err != nil {
			return nil, err
		}
		config.Enabled = lotteryEnabledPointer(enabled)
	}
	if err := s.assignLotteryPrizeIDs(ctx, &config); err != nil {
		return nil, err
	}
	for index := range config.Prizes {
		if config.Prizes[index].Type == "balance" {
			config.Prizes[index].Label = lotteryBalancePrizeLabel(config.Prizes[index].Amount)
		}
	}
	if err := validateLotteryPrizePoolConfig(config); err != nil {
		return nil, err
	}
	for index := range config.Prizes {
		prize := &config.Prizes[index]
		if prize.Type != "subscription" {
			continue
		}
		planName, groupID, _, err := s.lotterySubscriptionPlan(ctx, s.entClient, prize.SubscriptionPlanID)
		if err != nil {
			return nil, err
		}
		if _, err := s.lotterySubscriptionWelfareAmount(ctx, s.entClient, groupID); err != nil {
			return nil, err
		}
		prize.SubscriptionGroupID = groupID
		prize.Label = planName
	}
	prizesForStorage := append([]LotteryPrizeConfig(nil), config.Prizes...)
	for index := range prizesForStorage {
		prizesForStorage[index].CooldownUntil = nil
	}
	raw, err := json.Marshal(prizesForStorage)
	if err != nil {
		return nil, err
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	client := tx.Client()
	if _, err := client.ExecContext(ctx, `INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, NOW()) ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`, lotteryPrizePoolKey, string(raw)); err != nil {
		return nil, fmt.Errorf("save lottery prize pool: %w", err)
	}
	for key, amount := range map[string]float64{
		lotteryInvitationFirstPaymentAmountKey: config.InvitationFirstPaymentAmount,
		lotteryInvitationConsumptionAmountKey:  config.InvitationConsumptionAmount,
		lotteryPurchasePriceKey:                config.PurchasePrice,
	} {
		if _, err := client.ExecContext(ctx, `INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, NOW()) ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`, key, strconv.FormatFloat(amount, 'f', -1, 64)); err != nil {
			return nil, fmt.Errorf("save lottery invitation rule: %w", err)
		}
	}
	if _, err := client.ExecContext(ctx, `INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, NOW()) ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`, lotteryEnabledKey, strconv.FormatBool(*config.Enabled)); err != nil {
		return nil, fmt.Errorf("save lottery enabled setting: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetPrizePoolConfig(ctx)
}

func (s *LotteryService) lotteryEnabled(ctx context.Context) (bool, error) {
	var raw string
	err := scanOne(ctx, s.entClient, `SELECT value FROM settings WHERE key = $1`, []any{lotteryEnabledKey}, &raw)
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("get lottery enabled setting: %w", err)
	}
	enabled, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false, infraerrors.ServiceUnavailable("LOTTERY_ENABLED_SETTING_INVALID", "lottery enabled setting is invalid")
	}
	return enabled, nil
}

func (s *LotteryService) lotteryBalanceRechargeMultiplier(ctx context.Context) (float64, error) {
	var raw string
	err := scanOne(ctx, s.entClient, `SELECT value FROM settings WHERE key = $1`, []any{SettingBalanceRechargeMult}, &raw)
	if err == sql.ErrNoRows {
		return defaultBalanceRechargeMultiplier, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get balance recharge multiplier: %w", err)
	}
	multiplier, parseErr := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if parseErr != nil {
		return defaultBalanceRechargeMultiplier, nil
	}
	return normalizeBalanceRechargeMultiplier(multiplier), nil
}

func (s *LotteryService) requireLotteryEnabled(ctx context.Context) error {
	enabled, err := s.lotteryEnabled(ctx)
	if err != nil {
		return err
	}
	if !enabled {
		return infraerrors.Forbidden("LOTTERY_DISABLED", "lottery is currently disabled")
	}
	return nil
}

func lotteryBalancePrizeLabel(amount float64) string {
	return "$" + strconv.FormatFloat(amount, 'f', -1, 64)
}

// assignLotteryPrizeIDs retains identifiers that the server has already
// issued, and creates a new one for every newly added prize. IDs are not a
// configurable operator field.
func (s *LotteryService) assignLotteryPrizeIDs(ctx context.Context, config *LotteryPrizePoolConfig) error {
	existingConfig, err := s.GetPrizePoolConfig(ctx)
	if err != nil {
		return err
	}
	existing := make(map[string]struct{}, len(existingConfig.Prizes))
	for _, prize := range existingConfig.Prizes {
		existing[prize.ID] = struct{}{}
	}
	assigned := make(map[string]struct{}, len(config.Prizes))
	for index := range config.Prizes {
		prize := &config.Prizes[index]
		id := strings.TrimSpace(prize.ID)
		if _, ok := existing[id]; ok {
			if _, duplicate := assigned[id]; !duplicate {
				prize.ID = id
				assigned[id] = struct{}{}
				continue
			}
		}
		for {
			id = newLotteryPrizeID()
			if _, used := assigned[id]; !used {
				break
			}
		}
		prize.ID = id
		assigned[id] = struct{}{}
	}
	return nil
}

func newLotteryPrizeID() string {
	// Keep generated IDs compatible with lottery_draws.prize_id while retaining
	// 104 bits of UUID entropy after the "prize-" prefix.
	return "prize-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:lotteryPrizeIDLength-len("prize-")]
}

func validateLotteryPrizePoolConfig(config LotteryPrizePoolConfig) error {
	if err := validateLotteryPurchasePrice(config.PurchasePrice); err != nil {
		return err
	}
	if err := validateLotteryInvitationRule(lotteryInvitationRule{
		FirstPaymentAmount: config.InvitationFirstPaymentAmount,
		ConsumptionAmount:  config.InvitationConsumptionAmount,
	}); err != nil {
		return err
	}
	if len(config.Prizes) < 2 || len(config.Prizes) > 12 {
		return infraerrors.BadRequest("LOTTERY_PRIZE_COUNT_INVALID", "lottery prize pool must contain 2 to 12 prizes")
	}
	seen := make(map[string]struct{}, len(config.Prizes))
	var normalProbability, pityProbability int64
	hasNonePrize := false
	for _, prize := range config.Prizes {
		prize.ID = strings.TrimSpace(prize.ID)
		prize.Label = strings.TrimSpace(prize.Label)
		if !lotteryPrizeIDPattern.MatchString(prize.ID) || prize.Label == "" || len([]rune(prize.Label)) > 24 {
			return infraerrors.BadRequest("LOTTERY_PRIZE_INVALID", "lottery prize id or label is invalid")
		}
		if _, exists := seen[prize.ID]; exists {
			return infraerrors.BadRequest("LOTTERY_PRIZE_DUPLICATE", "lottery prize ids must be unique")
		}
		seen[prize.ID] = struct{}{}
		if prize.CooldownSeconds < 0 || prize.CooldownSeconds > lotteryPrizeCooldownMaxSeconds {
			return infraerrors.BadRequest("LOTTERY_PRIZE_COOLDOWN_INVALID", "lottery prize cooldown must be between 0 and one year")
		}
		probability, ok := lotteryProbabilityUnits(prize.Probability)
		if !ok {
			return infraerrors.BadRequest("LOTTERY_PRIZE_PROBABILITY_INVALID", "lottery prize probability must be a decimal between 0 and 1 with at most six places")
		}
		switch prize.Type {
		case "none":
			if prize.Amount != 0 || prize.SubscriptionGroupID != 0 || prize.SubscriptionPlanID != 0 || prize.EligibleForPity || prize.CooldownSeconds != 0 {
				return infraerrors.BadRequest("LOTTERY_PRIZE_INVALID", "non-winning prize cannot have a reward, cooldown, or pity eligibility")
			}
			hasNonePrize = true
		case "balance":
			if prize.Amount <= 0 || prize.Amount > 1_000_000 || prize.SubscriptionGroupID != 0 || prize.SubscriptionPlanID != 0 {
				return infraerrors.BadRequest("LOTTERY_PRIZE_INVALID", "balance prize amount is invalid")
			}
		case "subscription":
			if prize.Amount != 0 || (prize.SubscriptionPlanID <= 0 && prize.SubscriptionGroupID <= 0) {
				return infraerrors.BadRequest("LOTTERY_PRIZE_INVALID", "subscription prize plan is invalid")
			}
		default:
			return infraerrors.BadRequest("LOTTERY_PRIZE_TYPE_INVALID", "lottery prize type is invalid")
		}
		normalProbability += probability
		if prize.EligibleForPity {
			pityProbability += probability
		}
	}
	if normalProbability != lotteryProbabilityScale {
		return infraerrors.BadRequest("LOTTERY_PRIZE_PROBABILITY_TOTAL_INVALID", "lottery prize probabilities must total 1")
	}
	if !hasNonePrize {
		return infraerrors.BadRequest("LOTTERY_PRIZE_POOL_INVALID", "lottery prize pool requires a non-winning prize")
	}
	if pityProbability <= 0 {
		return infraerrors.BadRequest("LOTTERY_PRIZE_POOL_INVALID", "lottery prize pool requires at least one pity reward")
	}
	return nil
}

type lotteryInvitationRule struct {
	FirstPaymentAmount float64
	ConsumptionAmount  float64
}

func (s *LotteryService) getLotteryInvitationRule(ctx context.Context) (lotteryInvitationRule, error) {
	defaults := defaultLotteryPrizePoolConfig()
	rule := lotteryInvitationRule{
		FirstPaymentAmount: defaults.InvitationFirstPaymentAmount,
		ConsumptionAmount:  defaults.InvitationConsumptionAmount,
	}
	rows, err := s.entClient.QueryContext(ctx, `SELECT key, value FROM settings WHERE key IN ($1, $2)`, lotteryInvitationFirstPaymentAmountKey, lotteryInvitationConsumptionAmountKey)
	if err != nil {
		return lotteryInvitationRule{}, fmt.Errorf("get lottery invitation rule: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var key, raw string
		if err := rows.Scan(&key, &raw); err != nil {
			return lotteryInvitationRule{}, fmt.Errorf("scan lottery invitation rule: %w", err)
		}
		amount, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err != nil {
			return lotteryInvitationRule{}, infraerrors.ServiceUnavailable("LOTTERY_INVITATION_RULE_INVALID", "lottery invitation rule is invalid")
		}
		switch key {
		case lotteryInvitationFirstPaymentAmountKey:
			rule.FirstPaymentAmount = amount
		case lotteryInvitationConsumptionAmountKey:
			rule.ConsumptionAmount = amount
		}
	}
	if err := rows.Err(); err != nil {
		return lotteryInvitationRule{}, fmt.Errorf("read lottery invitation rule: %w", err)
	}
	if err := validateLotteryInvitationRule(rule); err != nil {
		return lotteryInvitationRule{}, err
	}
	return rule, nil
}

func validateLotteryPurchasePrice(price float64) error {
	if math.IsNaN(price) || math.IsInf(price, 0) || price <= 0 || price > 1_000_000 || math.Abs(price*100-math.Round(price*100)) > 1e-8 {
		return infraerrors.BadRequest("LOTTERY_PURCHASE_PRICE_INVALID", "lottery purchase price must be between 0 and 1000000 with at most two decimal places")
	}
	return nil
}

func (s *LotteryService) getLotteryPurchasePrice(ctx context.Context, client interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}) (float64, error) {
	price := defaultLotteryPurchasePrice
	var raw string
	err := scanOne(ctx, client, `SELECT value FROM settings WHERE key = $1`, []any{lotteryPurchasePriceKey}, &raw)
	if err == sql.ErrNoRows {
		return price, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get lottery purchase price: %w", err)
	}
	price, err = strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, infraerrors.ServiceUnavailable("LOTTERY_PURCHASE_PRICE_INVALID", "lottery purchase price is invalid")
	}
	if err := validateLotteryPurchasePrice(price); err != nil {
		return 0, infraerrors.ServiceUnavailable("LOTTERY_PURCHASE_PRICE_INVALID", "lottery purchase price is invalid")
	}
	return price, nil
}

func validateLotteryInvitationRule(rule lotteryInvitationRule) error {
	if math.IsNaN(rule.FirstPaymentAmount) || math.IsInf(rule.FirstPaymentAmount, 0) || rule.FirstPaymentAmount <= 0 || rule.FirstPaymentAmount > 1_000_000 {
		return infraerrors.BadRequest("LOTTERY_INVITATION_FIRST_PAYMENT_INVALID", "lottery invitation cumulative recharge amount must be between 0 and 1000000")
	}
	if math.IsNaN(rule.ConsumptionAmount) || math.IsInf(rule.ConsumptionAmount, 0) || rule.ConsumptionAmount <= 0 || rule.ConsumptionAmount > 1_000_000 {
		return infraerrors.BadRequest("LOTTERY_INVITATION_CONSUMPTION_INVALID", "lottery invitation consumption amount must be between 0 and 1000000")
	}
	return nil
}

func lotteryProbabilityUnits(probability float64) (int64, bool) {
	if math.IsNaN(probability) || math.IsInf(probability, 0) || probability <= 0 || probability > 1 {
		return 0, false
	}
	units := int64(math.Round(probability * float64(lotteryProbabilityScale)))
	return units, math.Abs(probability-float64(units)/float64(lotteryProbabilityScale)) < 1e-10
}

func (s *LotteryService) configuredPrizes(ctx context.Context, client interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}) ([]lotteryPrize, []lotteryPrize, error) {
	config, err := s.GetPrizePoolConfig(ctx)
	if err != nil {
		return nil, nil, err
	}
	normal := make([]lotteryPrize, 0, len(config.Prizes))
	pity := make([]lotteryPrize, 0, len(config.Prizes))
	for _, item := range config.Prizes {
		weight, ok := lotteryProbabilityUnits(item.Probability)
		if !ok {
			return nil, nil, fmt.Errorf("invalid lottery prize probability")
		}
		prize := lotteryPrize{ID: item.ID, Label: item.Label, Type: item.Type, Amount: item.Amount, Weight: weight, GroupID: item.SubscriptionGroupID, SubscriptionPlanID: item.SubscriptionPlanID, CooldownSeconds: item.CooldownSeconds}
		if item.Type == "subscription" {
			if item.SubscriptionPlanID > 0 {
				_, groupID, validityDays, err := s.lotterySubscriptionPlan(ctx, client, item.SubscriptionPlanID)
				if err != nil {
					return nil, nil, err
				}
				prize.GroupID = groupID
				prize.Days = validityDays
			} else {
				_, validityDays, err := s.lotterySubscriptionGroup(ctx, client, item.SubscriptionGroupID)
				if err != nil {
					return nil, nil, err
				}
				prize.Days = validityDays
			}
		}
		normal = append(normal, prize)
		if item.EligibleForPity {
			pity = append(pity, prize)
		}
	}
	return normal, pity, nil
}

func (s *LotteryService) applyLotteryPrizeCooldowns(ctx context.Context, prizes []LotteryPrizeConfig) error {
	for index := range prizes {
		prizes[index].CooldownUntil = nil
		if prizes[index].Type == "none" || prizes[index].CooldownSeconds == 0 {
			continue
		}
		var until time.Time
		err := scanOne(ctx, s.entClient, `SELECT cooldown_until FROM lottery_prize_cooldowns WHERE prize_id = $1 AND cooldown_until > NOW()`, []any{prizes[index].ID}, &until)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return fmt.Errorf("get lottery prize cooldown: %w", err)
		}
		prizes[index].CooldownUntil = &until
	}
	return nil
}

func lockLotteryPrizeCooldowns(ctx context.Context, client interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, prizes []lotteryPrize) (map[string]bool, error) {
	active := make(map[string]bool)
	for _, prize := range prizes {
		if prize.Type == "none" || prize.CooldownSeconds == 0 {
			continue
		}
		if _, err := client.ExecContext(ctx, `INSERT INTO lottery_prize_cooldowns (prize_id, cooldown_until) VALUES ($1, '-infinity') ON CONFLICT (prize_id) DO NOTHING`, prize.ID); err != nil {
			return nil, fmt.Errorf("ensure lottery prize cooldown: %w", err)
		}
		var isActive bool
		if err := scanOne(ctx, client, `SELECT cooldown_until > NOW() FROM lottery_prize_cooldowns WHERE prize_id = $1 FOR UPDATE`, []any{prize.ID}, &isActive); err != nil {
			return nil, fmt.Errorf("lock lottery prize cooldown: %w", err)
		}
		active[prize.ID] = isActive
	}
	return active, nil
}

func filterLotteryPrizeCooldowns(normal, pity []lotteryPrize, active map[string]bool) ([]lotteryPrize, []lotteryPrize) {
	filteredNormal := make([]lotteryPrize, 0, len(normal))
	none := make([]lotteryPrize, 0, len(normal))
	for _, prize := range normal {
		if prize.Type == "none" {
			filteredNormal = append(filteredNormal, prize)
			none = append(none, prize)
			continue
		}
		if !active[prize.ID] {
			filteredNormal = append(filteredNormal, prize)
		}
	}
	filteredPity := make([]lotteryPrize, 0, len(pity))
	for _, prize := range pity {
		if !active[prize.ID] {
			filteredPity = append(filteredPity, prize)
		}
	}
	if len(filteredPity) == 0 {
		filteredPity = none
	}
	return filteredNormal, filteredPity
}

func (s *LotteryService) GetStatus(ctx context.Context, userID int64) (*LotteryStatus, error) {
	if s == nil || s.entClient == nil {
		return nil, infraerrors.ServiceUnavailable("LOTTERY_UNAVAILABLE", "lottery service is unavailable")
	}
	enabled, err := s.lotteryEnabled(ctx)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return &LotteryStatus{Enabled: false}, nil
	}
	// A qualified invitation can be released only once; this is idempotent and
	// keeps the trigger tied to server-side payment and usage facts.
	if err := s.maybeGrantInvitationTicket(ctx, userID); err != nil {
		return nil, err
	}
	if err := s.maybeGrantInvitationTicketsForInviter(ctx, userID); err != nil {
		return nil, err
	}
	if _, err := s.entClient.ExecContext(ctx, `
UPDATE lottery_ticket_ledger
SET remaining = 0
WHERE user_id = $1 AND remaining > 0 AND expires_at IS NOT NULL AND expires_at <= NOW()`, userID); err != nil {
		return nil, fmt.Errorf("expire lottery tickets: %w", err)
	}
	if _, err := s.entClient.ExecContext(ctx, `INSERT INTO lottery_user_states (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`, userID); err != nil {
		return nil, fmt.Errorf("ensure lottery state: %w", err)
	}
	state, err := s.readState(ctx, s.entClient, userID, false)
	if err != nil {
		return nil, err
	}
	return s.buildLotteryStatus(ctx, s.entClient, userID, state, timezone.Today())
}

func (s *LotteryService) buildLotteryStatus(ctx context.Context, client interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, userID int64, state lotteryUserState, today time.Time) (*LotteryStatus, error) {
	available, err := s.countAvailableTickets(ctx, client, userID)
	if err != nil {
		return nil, err
	}
	if available != state.AvailableTickets {
		if _, err := client.ExecContext(ctx, `UPDATE lottery_user_states SET available_tickets = $2, updated_at = NOW(), version = version + 1 WHERE user_id = $1`, userID, available); err != nil {
			return nil, err
		}
	}
	purchaseCount := state.PurchaseCount
	if !sameBusinessDate(state.PurchaseDate, today) {
		purchaseCount = 0
	}
	var rechargeTicketsToday, invitationTicketsToday int
	if err := scanOne(ctx, client, `
SELECT
  COALESCE(SUM(CASE WHEN source_type = 'recharge' THEN delta ELSE 0 END), 0),
  COALESCE(SUM(CASE WHEN source_type = 'invitation' THEN delta ELSE 0 END), 0)
FROM lottery_ticket_ledger
WHERE user_id = $1
  AND ((source_type = 'recharge' AND business_date = $2)
    OR (source_type = 'invitation' AND created_at >= $2 AND created_at < $3))`, []any{userID, today, today.AddDate(0, 0, 1)}, &rechargeTicketsToday, &invitationTicketsToday); err != nil {
		return nil, fmt.Errorf("read daily lottery ticket grants: %w", err)
	}
	return &LotteryStatus{
		Enabled:                true,
		AvailableTickets:       available,
		PityMisses:             state.PityMisses,
		PityRemaining:          lotteryMax(0, 4-state.PityMisses),
		RemainingPurchases:     lotteryMax(0, lotteryPurchaseDailyLimit-purchaseCount),
		RechargeTicketsToday:   rechargeTicketsToday,
		InvitationTicketsToday: invitationTicketsToday,
		PurchasedTicketsToday:  purchaseCount,
		TicketDebt:             state.TicketDebt,
	}, nil
}

func (s *LotteryService) PurchaseTicket(ctx context.Context, userID int64, requestID string) (*LotteryStatus, error) {
	if err := s.requireLotteryEnabled(ctx); err != nil {
		return nil, err
	}
	if err := validateLotteryRequestID(requestID); err != nil {
		return nil, err
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin lottery purchase transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	state, err := s.lockState(txCtx, client, userID)
	if err != nil {
		return nil, err
	}
	today := timezone.Today()
	if !sameBusinessDate(state.PurchaseDate, today) {
		state.PurchaseCount = 0
	}
	ref, err := lotteryPurchaseSourceRef(userID, requestID)
	if err != nil {
		return nil, err
	}
	var existing int64
	err = scanOne(txCtx, client, `SELECT id FROM lottery_ticket_ledger WHERE source_type = 'purchase' AND source_ref = $1`, []any{ref}, &existing)
	if err == nil {
		status, err := s.buildLotteryStatus(txCtx, client, userID, state, today)
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		s.invalidateBalanceCaches(userID)
		return status, nil
	}
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("check lottery purchase replay: %w", err)
	}
	if state.PurchaseCount >= lotteryPurchaseDailyLimit {
		return nil, infraerrors.TooManyRequests("LOTTERY_PURCHASE_LIMIT", "daily lottery ticket purchase limit reached")
	}
	purchasePrice, err := s.getLotteryPurchasePrice(txCtx, client)
	if err != nil {
		return nil, err
	}

	var before, after float64
	err = scanOne(txCtx, client, `
UPDATE users
SET balance = balance - $1, updated_at = NOW()
WHERE id = $2 AND deleted_at IS NULL AND status = 'active' AND balance >= $1
RETURNING balance + $1, balance`, []any{purchasePrice, userID}, &before, &after)
	if err == sql.ErrNoRows {
		return nil, infraerrors.BadRequest("INSUFFICIENT_BALANCE", "insufficient balance to purchase lottery ticket")
	}
	if err != nil {
		return nil, fmt.Errorf("deduct lottery purchase balance: %w", err)
	}
	if _, err := client.ExecContext(txCtx, `
INSERT INTO balance_transactions (user_id, transaction_type, amount, balance_before, balance_after, source_type, source_id, description)
VALUES ($1, 'lottery_ticket_purchase', $2, $3, $4, 'lottery_purchase', $5, '购买抽奖次数')`, userID, -purchasePrice, before, after, ref); err != nil {
		return nil, fmt.Errorf("record lottery purchase balance transaction: %w", err)
	}
	if _, err := s.addTicketsLocked(txCtx, client, userID, &state, 1, "purchase", ref, nil, nil, nil, nil); err != nil {
		return nil, err
	}
	state.PurchaseCount++
	if _, err := client.ExecContext(txCtx, `
UPDATE lottery_user_states
SET purchase_business_date = $2, purchase_count = $3, updated_at = NOW(), version = version + 1
WHERE user_id = $1`, userID, today, state.PurchaseCount); err != nil {
		return nil, err
	}
	state.PurchaseDate = sql.NullTime{Time: today, Valid: true}
	status, err := s.buildLotteryStatus(txCtx, client, userID, state, today)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit lottery purchase: %w", err)
	}
	s.invalidateBalanceCaches(userID)
	return status, nil
}

func (s *LotteryService) Draw(ctx context.Context, userID int64, requestID string) (*LotteryDrawResult, error) {
	if err := s.requireLotteryEnabled(ctx); err != nil {
		return nil, err
	}
	if err := validateLotteryRequestID(requestID); err != nil {
		return nil, err
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin lottery draw transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	if existing, err := s.findDraw(txCtx, client, userID, requestID); err == nil {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		if existing.PrizeType == "balance" {
			s.invalidateBalanceCaches(userID)
		}
		return existing, nil
	} else if err != sql.ErrNoRows {
		return nil, err
	}

	state, err := s.lockState(txCtx, client, userID)
	if err != nil {
		return nil, err
	}
	// A second request can have checked before the first transaction committed,
	// then waited on this state row. Check again after acquiring the row lock.
	if existing, err := s.findDraw(txCtx, client, userID, requestID); err == nil {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		if existing.PrizeType == "balance" {
			s.invalidateBalanceCaches(userID)
		}
		return existing, nil
	} else if err != sql.ErrNoRows {
		return nil, err
	}
	if state.TicketDebt > 0 {
		return nil, infraerrors.Forbidden("LOTTERY_TICKET_DEBT", "lottery ticket debt must be settled before drawing")
	}
	normalPrizes, guaranteedPrizes, err := s.configuredPrizes(txCtx, client)
	if err != nil {
		return nil, err
	}
	activeCooldowns, err := lockLotteryPrizeCooldowns(txCtx, client, normalPrizes)
	if err != nil {
		return nil, err
	}
	normalPrizes, guaranteedPrizes = filterLotteryPrizeCooldowns(normalPrizes, guaranteedPrizes, activeCooldowns)
	var email string
	if err := scanOne(txCtx, client, `SELECT email FROM users WHERE id = $1 AND deleted_at IS NULL AND status = 'active' FOR UPDATE`, []any{userID}, &email); err != nil {
		if err == sql.ErrNoRows {
			return nil, infraerrors.Forbidden("LOTTERY_USER_INELIGIBLE", "user is not eligible for lottery")
		}
		return nil, err
	}
	if err := s.consumeTicketLocked(txCtx, client, userID); err != nil {
		return nil, err
	}
	guaranteed := state.PityMisses >= 4
	prize, err := pickServerPrize(guaranteed, normalPrizes, guaranteedPrizes)
	if err != nil {
		return nil, err
	}
	if prize.Type == "none" {
		state.PityMisses = min(4, state.PityMisses+1)
	} else {
		state.PityMisses = 0
		if prize.CooldownSeconds > 0 {
			if _, err := client.ExecContext(txCtx, `UPDATE lottery_prize_cooldowns SET cooldown_until = NOW() + ($2 * INTERVAL '1 second'), updated_at = NOW() WHERE prize_id = $1`, prize.ID, prize.CooldownSeconds); err != nil {
				return nil, fmt.Errorf("start lottery prize cooldown: %w", err)
			}
		}
	}
	var drawID int64
	if err := scanOne(txCtx, client, `
INSERT INTO lottery_draws (user_id, request_id, prize_id, prize_label, prize_type, reward_amount, pool_version, is_guaranteed)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id`, []any{userID, requestID, prize.ID, prize.Label, prize.Type, prize.Amount, lotteryPoolVersion, guaranteed}, &drawID); err != nil {
		return nil, fmt.Errorf("create lottery draw: %w", err)
	}
	rewardRef, balanceBefore, balanceAfter, err := s.grantDrawReward(txCtx, client, drawID, userID, email, prize)
	if err != nil {
		return nil, err
	}
	if rewardRef != "" {
		if _, err := client.ExecContext(txCtx, `UPDATE lottery_draws SET reward_ref = $2 WHERE id = $1`, drawID, rewardRef); err != nil {
			return nil, err
		}
	}
	var redeemExpiresAt *time.Time
	var subscriptionValidityDays *int
	if rewardRef != "" {
		var expiresAt time.Time
		if err := scanOne(txCtx, client, `SELECT expires_at FROM redeem_codes WHERE code = $1 AND owner_user_id = $2`, []any{rewardRef, userID}, &expiresAt); err != nil {
			return nil, fmt.Errorf("read lottery redeem expiry: %w", err)
		}
		redeemExpiresAt = &expiresAt
		if prize.Type == "subscription" {
			subscriptionValidityDays = &prize.Days
		}
	}
	available, err := s.countAvailableTickets(txCtx, client, userID)
	if err != nil {
		return nil, err
	}
	if _, err := client.ExecContext(txCtx, `
UPDATE lottery_user_states
SET available_tickets = $2,
    pity_misses = $3,
    total_draw_attempts = total_draw_attempts + 1,
    total_wins = total_wins + CASE WHEN $4 THEN 1 ELSE 0 END,
    updated_at = NOW(),
    version = version + 1
WHERE user_id = $1`, userID, available, state.PityMisses, prize.Type != "none"); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit lottery draw: %w", err)
	}
	if prize.Type == "balance" {
		s.invalidateBalanceCaches(userID)
	}
	redeemStatus := ""
	if prize.Type == "subscription" {
		redeemStatus = StatusUnused
	}
	return &LotteryDrawResult{ID: drawID, RequestID: requestID, PrizeID: prize.ID, PrizeLabel: prize.Label, PrizeType: prize.Type, Amount: prize.Amount, BalanceBefore: balanceBefore, BalanceAfter: balanceAfter, Guaranteed: guaranteed, RedeemCode: rewardRef, RedeemStatus: redeemStatus, RedeemExpiresAt: redeemExpiresAt, SubscriptionValidityDays: subscriptionValidityDays, CreatedAt: timezone.Now()}, nil
}

func (s *LotteryService) ListDraws(ctx context.Context, userID int64, limit int) ([]LotteryDrawResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.entClient.QueryContext(ctx, `
		SELECT d.id, d.request_id, d.prize_id, d.prize_label, d.prize_type, d.reward_amount::double precision, d.is_guaranteed,
	       COALESCE(d.reward_ref, ''),
	       COALESCE(CASE WHEN r.status = 'used' THEN 'used' WHEN r.expires_at IS NOT NULL AND r.expires_at <= NOW() THEN 'expired' ELSE r.status END, ''),
	       r.expires_at, r.validity_days, d.created_at, bt.balance_before::double precision, bt.balance_after::double precision
FROM lottery_draws d
LEFT JOIN redeem_codes r ON r.code = d.reward_ref AND r.owner_user_id = d.user_id
LEFT JOIN balance_transactions bt ON bt.user_id = d.user_id AND bt.source_type = 'lottery_draw' AND bt.source_id = d.id::text
WHERE d.user_id = $1 ORDER BY d.created_at DESC, d.id DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]LotteryDrawResult, 0, limit)
	for rows.Next() {
		var item LotteryDrawResult
		var balanceBefore, balanceAfter sql.NullFloat64
		var redeemExpiresAt sql.NullTime
		var subscriptionValidityDays sql.NullInt64
		if err := rows.Scan(&item.ID, &item.RequestID, &item.PrizeID, &item.PrizeLabel, &item.PrizeType, &item.Amount, &item.Guaranteed, &item.RedeemCode, &item.RedeemStatus, &redeemExpiresAt, &subscriptionValidityDays, &item.CreatedAt, &balanceBefore, &balanceAfter); err != nil {
			return nil, err
		}
		item.BalanceBefore = nullableFloat64Pointer(balanceBefore)
		item.BalanceAfter = nullableFloat64Pointer(balanceAfter)
		if redeemExpiresAt.Valid {
			item.RedeemExpiresAt = &redeemExpiresAt.Time
		}
		if subscriptionValidityDays.Valid {
			validityDays := int(subscriptionValidityDays.Int64)
			item.SubscriptionValidityDays = &validityDays
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// ListRecentWinners returns recent non-empty prizes for the lottery ticker.
// User identity is masked before it leaves the service boundary.
func (s *LotteryService) ListRecentWinners(ctx context.Context, limit int) ([]LotteryRecentWinner, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	config, err := s.GetPrizePoolConfig(ctx)
	if err != nil {
		return nil, err
	}
	probabilities := make(map[string]float64, len(config.Prizes))
	amounts := make(map[string]float64, len(config.Prizes))
	for _, prize := range config.Prizes {
		probabilities[prize.ID] = prize.Probability
		amounts[prize.ID] = prize.Amount
		if prize.Type == "subscription" && prize.SubscriptionGroupID > 0 {
			if value, valueErr := s.lotterySubscriptionWelfareAmount(ctx, s.entClient, prize.SubscriptionGroupID); valueErr == nil {
				amounts[prize.ID] = value
			}
		}
	}
	rows, err := s.entClient.QueryContext(ctx, `
		SELECT d.id, COALESCE(u.email, ''), d.prize_id, d.prize_label, d.prize_type,
		       d.reward_amount::double precision, d.is_guaranteed, d.created_at
		FROM lottery_draws d
		JOIN users u ON u.id = d.user_id
		WHERE d.prize_type <> 'none' AND u.deleted_at IS NULL
		ORDER BY d.created_at DESC, d.id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent lottery winners: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]LotteryRecentWinner, 0, limit)
	for rows.Next() {
		var item LotteryRecentWinner
		var email string
		if err := rows.Scan(&item.ID, &email, &item.PrizeID, &item.PrizeLabel, &item.PrizeType, &item.Amount, &item.Guaranteed, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan recent lottery winner: %w", err)
		}
		item.DisplayName = maskLotteryWinnerEmail(email)
		item.Probability = probabilities[item.PrizeID]
		if value, ok := amounts[item.PrizeID]; ok {
			item.Amount = value
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent lottery winners: %w", err)
	}
	return items, nil
}

func maskLotteryWinnerEmail(email string) string {
	email = strings.TrimSpace(email)
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return "匿名用户"
	}
	local := strings.TrimSpace(parts[0])
	domain := strings.TrimSpace(parts[1])
	if local == "" || domain == "" {
		return "匿名用户"
	}

	domainParts := strings.Split(domain, ".")
	for index, part := range domainParts {
		if part == "" {
			return "匿名用户"
		}
		// Keep the TLD readable while reducing each remaining label to its
		// first character and one wildcard.
		if len(domainParts) == 1 || index < len(domainParts)-1 {
			domainParts[index] = maskLotteryEmailDomainSegment(part)
		}
	}
	return maskLotteryEmailSegment(local) + "@" + strings.Join(domainParts, ".")
}

func maskLotteryEmailSegment(value string) string {
	runes := []rune(value)
	if len(runes) <= 1 {
		return string(runes) + "***"
	}
	if len(runes) == 2 {
		return string(runes[0]) + "***" + string(runes[1])
	}
	return string(runes[0]) + "***" + string(runes[len(runes)-1])
}

func maskLotteryEmailDomainSegment(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return ""
	}
	return string(runes[0]) + "*"
}

// ListBalanceTransactions exposes auditable wallet changes that are surfaced in
// the user's balance history, including lottery movements and the QQ binding
// welcome bonus.
func (s *LotteryService) ListBalanceTransactions(ctx context.Context, userID int64, limit int) ([]LotteryBalanceTransaction, error) {
	if limit <= 0 || limit > 200 {
		limit = 25
	}
	rows, err := s.entClient.QueryContext(ctx, `
SELECT id, transaction_type, amount::double precision, description, created_at
FROM balance_transactions
WHERE user_id = $1 AND transaction_type IN ('lottery_reward', 'lottery_ticket_purchase', 'qq_bind_welcome_bonus')
ORDER BY created_at DESC, id DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]LotteryBalanceTransaction, 0, limit)
	for rows.Next() {
		var item LotteryBalanceTransaction
		if err := rows.Scan(&item.ID, &item.TransactionType, &item.Amount, &item.Description, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func isLotteryRechargePayment(order *dbent.PaymentOrder) bool {
	if order == nil || PaymentOrderCurrency(order) != payment.DefaultPaymentCurrency {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(order.PaymentType), lotteryInternalBalancePaymentType) {
		return false
	}
	return order.OrderType == payment.OrderTypeBalance || order.OrderType == payment.OrderTypeSubscription
}

func lotteryRechargeTotal(ctx context.Context, client *dbent.Client, userID int64, day time.Time, netOfRefunds bool) (float64, error) {
	query := lotteryRechargePaidTotalQuery
	if netOfRefunds {
		query = lotteryRechargeNetTotalQuery
	}
	var total float64
	err := scanOne(ctx, client, query, []any{userID, day, day.AddDate(0, 0, 1), lotteryInternalBalancePaymentType}, &total)
	if err != nil {
		return 0, fmt.Errorf("sum daily CNY recharge: %w", err)
	}
	return total, nil
}

const lotteryRechargePaidTotalQuery = `
SELECT COALESCE(SUM(pay_amount), 0)::double precision
FROM payment_orders
WHERE user_id = $1
  AND order_type IN ('balance', 'subscription')
  AND LOWER(payment_type) <> $4
  AND status IN ('PAID', 'RECHARGING', 'COMPLETED')
  AND paid_at >= $2 AND paid_at < $3
  AND UPPER(COALESCE(provider_snapshot->>'currency', 'CNY')) = 'CNY'`

const lotteryRechargeNetTotalQuery = `
SELECT COALESCE(SUM(CASE WHEN status = 'REFUNDED' THEN 0 ELSE GREATEST(pay_amount - refund_amount, 0) END), 0)::double precision
FROM payment_orders
WHERE user_id = $1
  AND order_type IN ('balance', 'subscription')
  AND LOWER(payment_type) <> $4
  AND status IN ('COMPLETED', 'PARTIALLY_REFUNDED', 'REFUNDED')
  AND paid_at >= $2 AND paid_at < $3
  AND UPPER(COALESCE(provider_snapshot->>'currency', 'CNY')) = 'CNY'`

// ApplyRechargeReward is called from third-party payment fulfillment.
// The partial unique index makes repeated callbacks and concurrent orders safe.
func (s *LotteryService) ApplyRechargeReward(ctx context.Context, order *dbent.PaymentOrder) error {
	if s == nil || !isLotteryRechargePayment(order) {
		return nil
	}
	enabled, err := s.lotteryEnabled(ctx)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	state, err := s.lockState(txCtx, client, order.UserID)
	if err != nil {
		return err
	}
	day := paymentOrderBusinessDay(order)
	total, err := lotteryRechargeTotal(txCtx, client, order.UserID, day, false)
	if err != nil {
		return err
	}
	for _, tier := range []int{lotteryRechargeRewardTierFirst, lotteryRechargeRewardTierSecond} {
		if total < float64(tier) {
			continue
		}
		ref := fmt.Sprintf("%d:%s:%d", order.UserID, day.Format("2006-01-02"), tier)
		expires := timezone.Now().Add(lotteryFreeTicketValidity)
		if _, err := s.addTicketsLocked(txCtx, client, order.UserID, &state, 1, "recharge", ref, &order.ID, &tier, &expires, &day); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// ReconcileRechargeRefund revokes unspent recharge tickets after a completed
// refund. A spent ticket becomes debt, which blocks future draws until offset.
func (s *LotteryService) ReconcileRechargeRefund(ctx context.Context, order *dbent.PaymentOrder) error {
	if s == nil || !isLotteryRechargePayment(order) {
		return nil
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	state, err := s.lockState(txCtx, client, order.UserID)
	if err != nil {
		return err
	}
	day := paymentOrderBusinessDay(order)
	total, err := lotteryRechargeTotal(txCtx, client, order.UserID, day, true)
	if err != nil {
		return err
	}
	for _, tier := range []int{lotteryRechargeRewardTierSecond, lotteryRechargeRewardTierFirst} {
		if total >= float64(tier) {
			continue
		}
		if err := s.revokeRechargeTicketTier(txCtx, client, order.UserID, day, tier, &state); err != nil {
			return err
		}
	}
	if err := s.syncLotteryStateTicketBalance(txCtx, client, order.UserID, &state); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *LotteryService) revokeRechargeTicketTier(ctx context.Context, client *dbent.Client, userID int64, day time.Time, tier int, state *lotteryUserState) error {
	var ledgerID int64
	var remaining int
	err := scanOne(ctx, client, `
SELECT id, remaining FROM lottery_ticket_ledger
WHERE user_id = $1 AND source_type = 'recharge' AND business_date = $2 AND reward_tier = $3
	  AND revoked_at IS NULL
FOR UPDATE`, []any{userID, day, tier}, &ledgerID, &remaining)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := client.ExecContext(ctx, `UPDATE lottery_ticket_ledger SET remaining = 0, revoked_at = NOW() WHERE id = $1`, ledgerID); err != nil {
		return err
	}
	if remaining == 0 {
		state.TicketDebt++
	}
	return nil
}

func (s *LotteryService) syncLotteryStateTicketBalance(ctx context.Context, client *dbent.Client, userID int64, state *lotteryUserState) error {
	available, err := s.countAvailableTickets(ctx, client, userID)
	if err != nil {
		return err
	}
	_, err = client.ExecContext(ctx, `UPDATE lottery_user_states SET available_tickets = $2, ticket_debt = $3, updated_at = NOW(), version = version + 1 WHERE user_id = $1`, userID, available, state.TicketDebt)
	return err
}

func (s *LotteryService) grantDrawReward(ctx context.Context, client interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, drawID, userID int64, email string, prize lotteryPrize) (string, *float64, *float64, error) {
	switch prize.Type {
	case "none":
		return "", nil, nil, nil
	case "balance":
		var before, after float64
		if err := scanOne(ctx, client, `UPDATE users SET balance = balance + $1, updated_at = NOW() WHERE id = $2 AND deleted_at IS NULL RETURNING balance - $1, balance`, []any{prize.Amount, userID}, &before, &after); err != nil {
			return "", nil, nil, err
		}
		sourceID := strconv.FormatInt(drawID, 10)
		if _, err := client.ExecContext(ctx, `INSERT INTO balance_transactions (user_id, transaction_type, amount, balance_before, balance_after, source_type, source_id, description) VALUES ($1, 'lottery_reward', $2, $3, $4, 'lottery_draw', $5, $6)`, userID, prize.Amount, before, after, sourceID, "抽奖奖励 "+prize.Label); err != nil {
			return "", nil, nil, err
		}
		if err := s.createLotteryWelfareRecord(ctx, client, userID, email, prize.Amount, prize.Type, sourceID, "", prize.Label); err != nil {
			return "", nil, nil, err
		}
		return "", &before, &after, nil
	case "subscription":
		if prize.GroupID <= 0 || prize.Days <= 0 {
			return "", nil, nil, infraerrors.ServiceUnavailable("LOTTERY_SUBSCRIPTION_NOT_CONFIGURED", "lottery subscription plan is unavailable")
		}
		validityDays := prize.Days
		welfareAmount, err := s.lotterySubscriptionWelfareAmount(ctx, client, prize.GroupID)
		if err != nil {
			return "", nil, nil, err
		}
		code, err := GenerateRedeemCode()
		if err != nil {
			return "", nil, nil, err
		}
		expiresAt := timezone.Now().AddDate(0, 0, 30)
		if _, err := client.ExecContext(ctx, `
INSERT INTO redeem_codes (code, type, value, status, notes, expires_at, group_id, validity_days, owner_user_id)
VALUES ($1, 'subscription', 1, 'unused', $2, $3, $4, $5, $6)`, code, fmt.Sprintf("抽奖记录 #%d", drawID), expiresAt, prize.GroupID, validityDays, userID); err != nil {
			return "", nil, nil, fmt.Errorf("create lottery subscription voucher: %w", err)
		}
		if err := s.createLotteryWelfareRecord(ctx, client, userID, email, welfareAmount, prize.Type, strconv.FormatInt(drawID, 10), code, prize.Label); err != nil {
			return "", nil, nil, err
		}
		return code, nil, nil, nil
	default:
		return "", nil, nil, fmt.Errorf("unsupported lottery prize type %q", prize.Type)
	}
}

func (s *LotteryService) createLotteryWelfareRecord(ctx context.Context, client interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, userID int64, email string, amount float64, rewardType, sourceID, rewardRef, prizeLabel string) error {
	_, err := client.ExecContext(ctx, `
INSERT INTO welfare_records (user_id, user_email, amount, remarks, status, source_type, source_id, reward_type, reward_ref)
VALUES ($1, $2, $3, $4, 'success', 'lottery_draw', $5, $6, $7)
ON CONFLICT (source_type, source_id) WHERE source_type IS NOT NULL AND source_id IS NOT NULL DO NOTHING`, userID, email, amount, "抽奖奖励 #"+sourceID+" · "+prizeLabel, sourceID, rewardType, nullableString(rewardRef))
	return err
}

func (s *LotteryService) lotterySubscriptionPlan(ctx context.Context, client interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, planID int64) (string, int64, int, error) {
	if planID <= 0 {
		return "", 0, 0, infraerrors.ServiceUnavailable("LOTTERY_SUBSCRIPTION_NOT_CONFIGURED", "lottery subscription plan is not configured")
	}
	var name, unit string
	var groupID int64
	var validityDays int
	err := scanOne(ctx, client, `
		SELECT p.name, p.group_id, p.validity_days, p.validity_unit
		FROM subscription_plans p
		JOIN groups g ON g.id = p.group_id
		WHERE p.id = $1 AND p.for_sale = true AND g.status = 'active'
			AND g.subscription_type = 'subscription' AND g.deleted_at IS NULL`, []any{planID}, &name, &groupID, &validityDays, &unit)
	if err == sql.ErrNoRows {
		return "", 0, 0, infraerrors.ServiceUnavailable("LOTTERY_SUBSCRIPTION_NOT_CONFIGURED", "lottery subscription plan is unavailable")
	}
	if err != nil {
		return "", 0, 0, err
	}
	validityDays = psComputeValidityDays(validityDays, unit)
	if validityDays <= 0 || validityDays > MaxValidityDays {
		return "", 0, 0, infraerrors.ServiceUnavailable("LOTTERY_SUBSCRIPTION_VALIDITY_INVALID", "lottery subscription plan validity is invalid")
	}
	return name, groupID, validityDays, nil
}

func (s *LotteryService) lotterySubscriptionGroup(ctx context.Context, client interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, groupID int64) (string, int, error) {
	if groupID <= 0 {
		return "", 0, infraerrors.ServiceUnavailable("LOTTERY_SUBSCRIPTION_NOT_CONFIGURED", "lottery subscription group is not configured")
	}
	var name string
	var validityDays int
	err := scanOne(ctx, client, `SELECT name, default_validity_days FROM groups WHERE id = $1 AND status = 'active' AND subscription_type = 'subscription' AND deleted_at IS NULL`, []any{groupID}, &name, &validityDays)
	if err == sql.ErrNoRows {
		return "", 0, infraerrors.ServiceUnavailable("LOTTERY_SUBSCRIPTION_NOT_CONFIGURED", "lottery subscription group is unavailable")
	}
	if err != nil {
		return "", 0, err
	}
	if validityDays <= 0 {
		validityDays = 30
	}
	return name, validityDays, nil
}

func (s *LotteryService) lotterySubscriptionWelfareAmount(ctx context.Context, client interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, groupID int64) (float64, error) {
	var price float64
	err := scanOne(ctx, client, `
SELECT price::double precision
FROM subscription_plans
WHERE group_id = $1 AND for_sale = true
ORDER BY sort_order, id
LIMIT 1`, []any{groupID}, &price)
	if err == sql.ErrNoRows {
		return 0, infraerrors.ServiceUnavailable("LOTTERY_SUBSCRIPTION_PRICE_UNAVAILABLE", "lottery subscription plan price is unavailable")
	}
	if err != nil {
		return 0, fmt.Errorf("get lottery subscription plan price: %w", err)
	}
	if price <= 0 {
		return 0, infraerrors.ServiceUnavailable("LOTTERY_SUBSCRIPTION_PRICE_INVALID", "lottery subscription plan price is invalid")
	}
	return price * 10, nil
}

func (s *LotteryService) AdjustTickets(ctx context.Context, userID int64, adjustment LotteryTicketAdjustment) (*LotteryTicketAdjustmentResult, error) {
	if err := validateLotteryTicketAdjustment(userID, &adjustment); err != nil {
		return nil, err
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin lottery ticket adjustment transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	state, err := s.lockState(txCtx, client, userID)
	if err != nil {
		return nil, err
	}
	if adjustment.Operation == "set" {
		available, err := s.countAvailableTickets(txCtx, client, userID)
		if err != nil {
			return nil, err
		}
		switch {
		case adjustment.Count > available:
			adjustment.Operation = "add"
			adjustment.Count -= available
		case adjustment.Count < available:
			adjustment.Operation = "subtract"
			adjustment.Count = available - adjustment.Count
		default:
			if err := tx.Commit(); err != nil {
				return nil, fmt.Errorf("commit unchanged lottery ticket target: %w", err)
			}
			return &LotteryTicketAdjustmentResult{AvailableTickets: available}, nil
		}
	}

	if adjustment.Operation == "add" {
		created, err := s.addAdminAdjustmentTicketsLocked(txCtx, client, userID, &state, adjustment.Count, adjustment.Reference, adjustment.Reason)
		if err != nil {
			return nil, err
		}
		if !created {
			available, err := s.countAvailableTickets(txCtx, client, userID)
			if err != nil {
				return nil, err
			}
			if err := tx.Commit(); err != nil {
				return nil, fmt.Errorf("commit lottery ticket adjustment replay: %w", err)
			}
			return &LotteryTicketAdjustmentResult{AvailableTickets: available}, nil
		}
	} else {
		marker, err := client.ExecContext(txCtx, `
			INSERT INTO lottery_ticket_ledger (user_id, delta, remaining, source_type, source_ref, adjustment_reason)
			VALUES ($1, $2, 0, 'admin_adjustment', $3, $4)
			ON CONFLICT (source_type, source_ref) DO NOTHING`, userID, -adjustment.Count, adjustment.Reference, adjustment.Reason)
		if err != nil {
			return nil, fmt.Errorf("create lottery ticket adjustment ledger marker: %w", err)
		}
		created, err := marker.RowsAffected()
		if err != nil {
			return nil, err
		}
		if created == 0 {
			available, err := s.countAvailableTickets(txCtx, client, userID)
			if err != nil {
				return nil, err
			}
			if err := tx.Commit(); err != nil {
				return nil, fmt.Errorf("commit lottery ticket adjustment replay: %w", err)
			}
			return &LotteryTicketAdjustmentResult{AvailableTickets: available}, nil
		}
		if err := s.consumeTicketsLocked(txCtx, client, userID, adjustment.Count); err != nil {
			return nil, err
		}
		available, err := s.countAvailableTickets(txCtx, client, userID)
		if err != nil {
			return nil, err
		}
		if _, err := client.ExecContext(txCtx, `UPDATE lottery_user_states SET available_tickets = $2, updated_at = NOW(), version = version + 1 WHERE user_id = $1`, userID, available); err != nil {
			return nil, err
		}
		state.AvailableTickets = available
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit lottery ticket adjustment: %w", err)
	}
	return &LotteryTicketAdjustmentResult{AvailableTickets: state.AvailableTickets}, nil
}

func validateLotteryTicketAdjustment(userID int64, adjustment *LotteryTicketAdjustment) error {
	if userID <= 0 {
		return infraerrors.BadRequest("LOTTERY_INVALID_USER", "invalid lottery user")
	}
	if adjustment == nil || adjustment.Count < 0 {
		return infraerrors.BadRequest("LOTTERY_INVALID_ADJUSTMENT", "ticket count must not be negative")
	}
	if adjustment.Operation != "add" && adjustment.Operation != "subtract" && adjustment.Operation != "set" {
		return infraerrors.BadRequest("LOTTERY_INVALID_ADJUSTMENT", "ticket operation must be set, add, or subtract")
	}
	if adjustment.Operation != "set" && adjustment.Count == 0 {
		return infraerrors.BadRequest("LOTTERY_INVALID_ADJUSTMENT", "ticket count must be positive for add or subtract")
	}
	adjustment.Reference = strings.TrimSpace(adjustment.Reference)
	adjustment.Reason = strings.TrimSpace(adjustment.Reason)
	if adjustment.Reference == "" || adjustment.Reason == "" {
		return infraerrors.BadRequest("LOTTERY_INVALID_ADJUSTMENT", "ticket adjustment reference and reason are required")
	}
	if len(adjustment.Reference) > lotteryTicketSourceRefMaxLength || utf8.RuneCountInString(adjustment.Reason) > 500 {
		return infraerrors.BadRequest("LOTTERY_INVALID_ADJUSTMENT", "ticket adjustment reference or reason is too long")
	}
	return nil
}

func (s *LotteryService) consumeTicketLocked(ctx context.Context, client interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, userID int64) error {
	if _, err := client.ExecContext(ctx, `UPDATE lottery_ticket_ledger SET remaining = 0 WHERE user_id = $1 AND remaining > 0 AND expires_at IS NOT NULL AND expires_at <= NOW()`, userID); err != nil {
		return err
	}
	var id int64
	var remaining int
	err := scanOne(ctx, client, `
SELECT id, remaining FROM lottery_ticket_ledger
WHERE user_id = $1 AND remaining > 0 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > NOW())
ORDER BY expires_at NULLS LAST, id FOR UPDATE LIMIT 1`, []any{userID}, &id, &remaining)
	if err == sql.ErrNoRows {
		return infraerrors.BadRequest("LOTTERY_NO_TICKETS", "no lottery tickets available")
	}
	if err != nil {
		return err
	}
	_, err = client.ExecContext(ctx, `UPDATE lottery_ticket_ledger SET remaining = $2 WHERE id = $1`, id, remaining-1)
	return err
}

func (s *LotteryService) consumeTicketsLocked(ctx context.Context, client interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, userID int64, count int) error {
	if count <= 0 {
		return infraerrors.BadRequest("LOTTERY_INVALID_ADJUSTMENT", "ticket count must be positive")
	}
	if _, err := client.ExecContext(ctx, `UPDATE lottery_ticket_ledger SET remaining = 0 WHERE user_id = $1 AND remaining > 0 AND expires_at IS NOT NULL AND expires_at <= NOW()`, userID); err != nil {
		return err
	}
	rows, err := client.QueryContext(ctx, `
		SELECT id, remaining
		FROM lottery_ticket_ledger
		WHERE user_id = $1 AND remaining > 0 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY expires_at NULLS LAST, id
		FOR UPDATE`, userID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	type ticket struct {
		id        int64
		remaining int
	}
	tickets := make([]ticket, 0)
	available := 0
	for rows.Next() {
		var item ticket
		if err := rows.Scan(&item.id, &item.remaining); err != nil {
			return err
		}
		tickets = append(tickets, item)
		available += item.remaining
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if available < count {
		return infraerrors.BadRequest("LOTTERY_INSUFFICIENT_TICKETS", "insufficient lottery tickets available")
	}

	remainingToConsume := count
	for _, item := range tickets {
		if remainingToConsume == 0 {
			break
		}
		consumed := lotteryMin(item.remaining, remainingToConsume)
		if _, err := client.ExecContext(ctx, `UPDATE lottery_ticket_ledger SET remaining = $2 WHERE id = $1`, item.id, item.remaining-consumed); err != nil {
			return err
		}
		remainingToConsume -= consumed
	}
	return nil
}

func (s *LotteryService) addAdminAdjustmentTicketsLocked(ctx context.Context, client interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, userID int64, state *lotteryUserState, count int, sourceRef, reason string) (bool, error) {
	if count <= 0 {
		return false, nil
	}
	remaining := count
	if state.TicketDebt > 0 {
		offset := lotteryMin(state.TicketDebt, remaining)
		state.TicketDebt -= offset
		remaining -= offset
	}
	result, err := client.ExecContext(ctx, `
		INSERT INTO lottery_ticket_ledger (user_id, delta, remaining, source_type, source_ref, adjustment_reason)
		VALUES ($1, $2, $3, 'admin_adjustment', $4, $5)
		ON CONFLICT (source_type, source_ref) DO NOTHING`, userID, count, remaining, sourceRef, reason)
	if err != nil {
		return false, fmt.Errorf("create lottery ticket adjustment ledger: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if affected == 0 {
		return false, nil
	}
	available, err := s.countAvailableTickets(ctx, client, userID)
	if err != nil {
		return false, err
	}
	state.AvailableTickets = available
	if _, err := client.ExecContext(ctx, `UPDATE lottery_user_states SET available_tickets = $2, ticket_debt = $3, updated_at = NOW(), version = version + 1 WHERE user_id = $1`, userID, available, state.TicketDebt); err != nil {
		return false, err
	}
	return true, nil
}

func (s *LotteryService) addTicketsLocked(ctx context.Context, client interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, userID int64, state *lotteryUserState, count int, sourceType, sourceRef string, sourceOrderID *int64, tier *int, expiresAt, businessDate *time.Time) (bool, error) {
	if count <= 0 {
		return false, nil
	}
	remaining := count
	if state.TicketDebt > 0 {
		offset := lotteryMin(state.TicketDebt, remaining)
		state.TicketDebt -= offset
		remaining -= offset
	}
	result, err := client.ExecContext(ctx, `
INSERT INTO lottery_ticket_ledger (user_id, delta, remaining, source_type, source_ref, source_order_id, business_date, reward_tier, expires_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	ON CONFLICT (source_type, source_ref) DO NOTHING`, userID, count, remaining, sourceType, sourceRef, sourceOrderID, businessDate, tier, expiresAt)
	if err != nil {
		return false, fmt.Errorf("create lottery ticket ledger: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if affected == 0 {
		return false, nil
	}
	available, err := s.countAvailableTickets(ctx, client, userID)
	if err != nil {
		return false, err
	}
	state.AvailableTickets = available
	if _, err := client.ExecContext(ctx, `UPDATE lottery_user_states SET available_tickets = $2, ticket_debt = $3, updated_at = NOW(), version = version + 1 WHERE user_id = $1`, userID, available, state.TicketDebt); err != nil {
		return false, err
	}
	return true, nil
}

func (s *LotteryService) lockState(ctx context.Context, client interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, userID int64) (lotteryUserState, error) {
	if _, err := client.ExecContext(ctx, `INSERT INTO lottery_user_states (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`, userID); err != nil {
		return lotteryUserState{}, err
	}
	return s.readState(ctx, client, userID, true)
}

func (s *LotteryService) readState(ctx context.Context, client interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, userID int64, lock bool) (lotteryUserState, error) {
	query := `SELECT available_tickets, pity_misses, ticket_debt, purchase_business_date, purchase_count FROM lottery_user_states WHERE user_id = $1`
	if lock {
		query += " FOR UPDATE"
	}
	var state lotteryUserState
	err := scanOne(ctx, client, query, []any{userID}, &state.AvailableTickets, &state.PityMisses, &state.TicketDebt, &state.PurchaseDate, &state.PurchaseCount)
	return state, err
}

func (s *LotteryService) countAvailableTickets(ctx context.Context, client interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, userID int64) (int, error) {
	var count int
	err := scanOne(ctx, client, `SELECT COALESCE(SUM(remaining), 0) FROM lottery_ticket_ledger WHERE user_id = $1 AND remaining > 0 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > NOW())`, []any{userID}, &count)
	return count, err
}

func (s *LotteryService) findDraw(ctx context.Context, client interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, userID int64, requestID string) (*LotteryDrawResult, error) {
	var item LotteryDrawResult
	var balanceBefore, balanceAfter sql.NullFloat64
	var redeemExpiresAt sql.NullTime
	var subscriptionValidityDays sql.NullInt64
	err := scanOne(ctx, client, `
		SELECT d.id, d.request_id, d.prize_id, d.prize_label, d.prize_type, d.reward_amount::double precision, d.is_guaranteed,
	       COALESCE(d.reward_ref, ''),
	       COALESCE(CASE WHEN r.status = 'used' THEN 'used' WHEN r.expires_at IS NOT NULL AND r.expires_at <= NOW() THEN 'expired' ELSE r.status END, ''),
	       r.expires_at, r.validity_days, d.created_at, bt.balance_before::double precision, bt.balance_after::double precision
FROM lottery_draws d
LEFT JOIN redeem_codes r ON r.code = d.reward_ref AND r.owner_user_id = d.user_id
LEFT JOIN balance_transactions bt ON bt.user_id = d.user_id AND bt.source_type = 'lottery_draw' AND bt.source_id = d.id::text
	WHERE d.user_id = $1 AND d.request_id = $2`, []any{userID, requestID}, &item.ID, &item.RequestID, &item.PrizeID, &item.PrizeLabel, &item.PrizeType, &item.Amount, &item.Guaranteed, &item.RedeemCode, &item.RedeemStatus, &redeemExpiresAt, &subscriptionValidityDays, &item.CreatedAt, &balanceBefore, &balanceAfter)
	if err != nil {
		return nil, err
	}
	item.BalanceBefore = nullableFloat64Pointer(balanceBefore)
	item.BalanceAfter = nullableFloat64Pointer(balanceAfter)
	if redeemExpiresAt.Valid {
		item.RedeemExpiresAt = &redeemExpiresAt.Time
	}
	if subscriptionValidityDays.Valid {
		validityDays := int(subscriptionValidityDays.Int64)
		item.SubscriptionValidityDays = &validityDays
	}
	return &item, nil
}

func (s *LotteryService) invalidateBalanceCaches(userID int64) {
	if s == nil || userID <= 0 {
		return
	}
	cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(cacheCtx, userID)
	}
	if s.billingCache != nil {
		if err := s.billingCache.InvalidateUserBalance(cacheCtx, userID); err != nil {
			slog.Error("invalidate lottery balance cache failed", "userID", userID, "error", err)
		}
	}
}

func (s *LotteryService) maybeGrantInvitationTicket(ctx context.Context, inviteeID int64) error {
	rule, err := s.getLotteryInvitationRule(ctx)
	if err != nil {
		return err
	}
	var inviterID int64
	err = scanOne(ctx, s.entClient, `SELECT inviter_id FROM user_affiliates WHERE user_id = $1 AND inviter_id IS NOT NULL`, []any{inviteeID}, &inviterID)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	var qq string
	err = scanOne(ctx, s.entClient, `
SELECT LOWER(BTRIM(v.value))
FROM user_attribute_values v JOIN user_attribute_definitions d ON d.id = v.attribute_id
WHERE v.user_id = $1 AND d.deleted_at IS NULL
  AND LOWER(d.key) = 'qq'
  AND NULLIF(BTRIM(v.value), '') IS NOT NULL
ORDER BY v.id LIMIT 1`, []any{inviteeID}, &qq)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	var qqUsers int
	if err := scanOne(ctx, s.entClient, `
SELECT COUNT(DISTINCT v.user_id)
FROM user_attribute_values v JOIN user_attribute_definitions d ON d.id = v.attribute_id
WHERE d.deleted_at IS NULL AND LOWER(d.key) = 'qq'
  AND LOWER(BTRIM(v.value)) = $1`, []any{qq}, &qqUsers); err != nil || qqUsers != 1 {
		return err
	}
	var rechargeTotal float64
	err = scanOne(ctx, s.entClient, `
SELECT COALESCE(SUM(pay_amount), 0)::double precision FROM payment_orders
WHERE user_id = $1 AND order_type IN ('balance', 'subscription') AND status = 'COMPLETED'
  AND UPPER(COALESCE(provider_snapshot->>'currency', 'CNY')) = 'CNY'`, []any{inviteeID}, &rechargeTotal)
	if err != nil {
		return err
	}
	if rechargeTotal < rule.FirstPaymentAmount {
		return nil
	}
	var consumed float64
	if err := scanOne(ctx, s.entClient, `SELECT COALESCE(SUM(actual_cost), 0)::double precision FROM usage_logs WHERE user_id = $1`, []any{inviteeID}, &consumed); err != nil || consumed < rule.ConsumptionAmount {
		return err
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	state, err := s.lockState(txCtx, tx.Client(), inviterID)
	if err != nil {
		return err
	}
	expires := timezone.Now().Add(lotteryFreeTicketValidity)
	_, err = s.addTicketsLocked(txCtx, tx.Client(), inviterID, &state, 2, "invitation", fmt.Sprintf("invitee:%d", inviteeID), nil, nil, &expires, nil)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// maybeGrantInvitationTicketsForInviter catches up rewards for qualified
// invitees when the inviter opens the lottery page. The ledger's unique source
// reference keeps repeated status requests and concurrent checks idempotent.
func (s *LotteryService) maybeGrantInvitationTicketsForInviter(ctx context.Context, inviterID int64) error {
	rows, err := s.entClient.QueryContext(ctx, `
SELECT ua.user_id
FROM user_affiliates ua
WHERE ua.inviter_id = $1
  AND NOT EXISTS (
    SELECT 1 FROM lottery_ticket_ledger l
    WHERE l.source_type = 'invitation'
      AND l.source_ref = CONCAT('invitee:', ua.user_id::text)
  )
ORDER BY ua.user_id`, inviterID)
	if err != nil {
		return fmt.Errorf("list invitation reward candidates: %w", err)
	}
	inviteeIDs := make([]int64, 0)
	for rows.Next() {
		var inviteeID int64
		if err := rows.Scan(&inviteeID); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan invitation reward candidate: %w", err)
		}
		inviteeIDs = append(inviteeIDs, inviteeID)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close invitation reward candidates: %w", err)
	}
	for _, inviteeID := range inviteeIDs {
		if err := s.maybeGrantInvitationTicket(ctx, inviteeID); err != nil {
			return err
		}
	}
	return nil
}

func pickServerPrize(guaranteed bool, normalPrizes, guaranteedPrizes []lotteryPrize) (lotteryPrize, error) {
	pool := normalPrizes
	if guaranteed {
		pool = guaranteedPrizes
	}
	var totalWeight int64
	for _, prize := range pool {
		totalWeight += prize.Weight
	}
	if totalWeight <= 0 {
		return lotteryPrize{}, fmt.Errorf("invalid lottery prize probabilities")
	}
	r, err := rand.Int(rand.Reader, big.NewInt(totalWeight))
	if err != nil {
		return lotteryPrize{}, fmt.Errorf("secure lottery random: %w", err)
	}
	remaining := r.Int64()
	for _, prize := range pool {
		if remaining < prize.Weight {
			return prize, nil
		}
		remaining -= prize.Weight
	}
	return lotteryPrize{}, fmt.Errorf("invalid lottery prize probabilities")
}

func validateLotteryRequestID(requestID string) error {
	requestID = strings.TrimSpace(requestID)
	if len(requestID) < 8 || len(requestID) > 128 {
		return infraerrors.BadRequest("LOTTERY_REQUEST_ID_INVALID", "valid idempotency key is required")
	}
	return nil
}

func lotteryPurchaseSourceRef(userID int64, requestID string) (string, error) {
	ref := fmt.Sprintf("%d:%s", userID, strings.TrimSpace(requestID))
	if len(ref) > lotteryTicketSourceRefMaxLength {
		return "", infraerrors.BadRequest("LOTTERY_REQUEST_ID_INVALID", "idempotency key is too long for lottery purchase")
	}
	return ref, nil
}

func sameBusinessDate(value sql.NullTime, day time.Time) bool {
	return value.Valid && timezone.StartOfDay(value.Time).Equal(timezone.StartOfDay(day))
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableFloat64Pointer(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	result := value.Float64
	return &result
}

func scanOne(ctx context.Context, client interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, query string, args []any, dest ...any) error {
	rows, err := client.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	if err := rows.Scan(dest...); err != nil {
		return err
	}
	return rows.Err()
}

func lotteryMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func lotteryMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func paymentOrderBusinessDay(order *dbent.PaymentOrder) time.Time {
	if order != nil && order.PaidAt != nil {
		return timezone.StartOfDay(*order.PaidAt)
	}
	return timezone.Today()
}
