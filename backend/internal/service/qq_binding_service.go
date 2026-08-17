package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/ent/userattributedefinition"
	"github.com/Wei-Shaw/sub2api/ent/userattributevalue"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

const (
	qqAttributeKey              = "qq"
	qqBindingWelcomeBonus       = 10.0
	qqBindingWelcomeSourceType  = "qq_bind_welcome_bonus"
	qqBindingWelcomeDescription = "QQ 绑定新人礼金"
)

var qqNumberPattern = regexp.MustCompile(`^[1-9][0-9]{4,11}$`)

type QQBindingConfirmInput struct {
	Email string
	QQ    string
}

type QQBindingConfirmResult struct {
	UserID              int64   `json:"user_id"`
	QQ                  string  `json:"qq"`
	BindingCreated      bool    `json:"binding_created"`
	WelcomeBonusGranted bool    `json:"welcome_bonus_granted"`
	WelcomeBonusAmount  float64 `json:"welcome_bonus_amount"`
	Balance             float64 `json:"balance"`
}

// QQBindingService owns the irreversible part of a bot QQ binding. The bot
// validates its temporary verification code before calling this service.
type QQBindingService struct {
	entClient            *dbent.Client
	billingCache         lotteryBalanceCacheInvalidator
	authCacheInvalidator APIKeyAuthCacheInvalidator
}

func NewQQBindingService(entClient *dbent.Client, billingCache *BillingCacheService, authCacheInvalidator APIKeyAuthCacheInvalidator) *QQBindingService {
	return &QQBindingService{
		entClient:            entClient,
		billingCache:         billingCache,
		authCacheInvalidator: authCacheInvalidator,
	}
}

// Confirm binds a normalized QQ number and grants the welcome bonus exactly
// once. The unique balance-transaction source is the durable idempotency
// record; it is deliberately independent of the user's balance.
func (s *QQBindingService) Confirm(ctx context.Context, input QQBindingConfirmInput) (*QQBindingConfirmResult, error) {
	if s == nil || s.entClient == nil {
		return nil, infraerrors.ServiceUnavailable("QQ_BINDING_UNAVAILABLE", "QQ binding service is unavailable")
	}
	email := strings.TrimSpace(input.Email)
	qq, err := normalizeQQNumber(input.QQ)
	if err != nil {
		return nil, err
	}
	if email == "" {
		return nil, infraerrors.BadRequest("QQ_BINDING_INVALID_EMAIL", "email is required")
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin QQ binding transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	definition, err := client.UserAttributeDefinition.Query().
		Where(userattributedefinition.KeyEQ(qqAttributeKey), userattributedefinition.DeletedAtIsNil()).
		Only(txCtx)
	if dbent.IsNotFound(err) {
		return nil, infraerrors.ServiceUnavailable("QQ_ATTRIBUTE_NOT_CONFIGURED", "QQ attribute is not configured")
	}
	if err != nil {
		return nil, fmt.Errorf("get QQ attribute definition: %w", err)
	}
	// Serialize confirmations for this user before observing its QQ attribute.
	// Without this row lock, two first-time confirmations can both observe an
	// empty attribute and the later upsert would overwrite the earlier QQ.
	userEntity, err := client.User.Query().
		Where(user.EmailEqualFold(email)).
		ForUpdate().
		Only(txCtx)
	if dbent.IsNotFound(err) {
		return nil, infraerrors.NotFound("QQ_BINDING_USER_NOT_FOUND", "user was not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get QQ binding user: %w", err)
	}

	current, err := client.UserAttributeValue.Query().
		Where(userattributevalue.UserIDEQ(userEntity.ID), userattributevalue.AttributeIDEQ(definition.ID)).
		Only(txCtx)
	if err != nil && !dbent.IsNotFound(err) {
		return nil, fmt.Errorf("get current QQ binding: %w", err)
	}
	currentQQ := ""
	if err == nil {
		currentQQ = strings.TrimSpace(current.Value)
	}
	if currentQQ != "" && currentQQ != qq {
		return nil, infraerrors.Conflict("QQ_ALREADY_BOUND_TO_USER", "this user is already bound to a different QQ number")
	}

	holderID, err := findQQBindingHolder(txCtx, client, definition.ID, qq)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err == nil && holderID != userEntity.ID {
		return nil, infraerrors.Conflict("QQ_ALREADY_BOUND", "QQ number is already bound to another user")
	}

	bindingCreated := currentQQ == ""
	if bindingCreated {
		if err := client.UserAttributeValue.Create().
			SetUserID(userEntity.ID).
			SetAttributeID(definition.ID).
			SetValue(qq).
			OnConflictColumns(userattributevalue.FieldUserID, userattributevalue.FieldAttributeID).
			UpdateValue().
			UpdateUpdatedAt().
			Exec(txCtx); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "qq value is already bound") {
				return nil, infraerrors.Conflict("QQ_ALREADY_BOUND", "QQ number is already bound to another user")
			}
			return nil, fmt.Errorf("save QQ binding: %w", err)
		}
	}

	result := &QQBindingConfirmResult{
		UserID:             userEntity.ID,
		QQ:                 qq,
		BindingCreated:     bindingCreated,
		WelcomeBonusAmount: qqBindingWelcomeBonus,
		Balance:            userEntity.Balance,
	}
	if bindingCreated {
		granted, balance, err := grantQQBindingWelcomeBonus(txCtx, client, userEntity.ID)
		if err != nil {
			return nil, err
		}
		result.WelcomeBonusGranted = granted
		result.Balance = balance
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit QQ binding transaction: %w", err)
	}
	s.invalidateUserBalance(ctx, userEntity.ID)
	return result, nil
}

func normalizeQQNumber(value string) (string, error) {
	qq := strings.TrimSpace(value)
	if !qqNumberPattern.MatchString(qq) {
		return "", infraerrors.BadRequest("QQ_BINDING_INVALID_QQ", "QQ must be a 5 to 12 digit number")
	}
	return qq, nil
}

func findQQBindingHolder(ctx context.Context, client interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, attributeID int64, qq string) (int64, error) {
	var userID int64
	err := scanOne(ctx, client, `
SELECT user_id
FROM user_attribute_values
WHERE attribute_id = $1 AND LOWER(BTRIM(value)) = LOWER($2)
LIMIT 1`, []any{attributeID, qq}, &userID)
	if err != nil && err != sql.ErrNoRows {
		return 0, fmt.Errorf("find QQ binding holder: %w", err)
	}
	return userID, err
}

func grantQQBindingWelcomeBonus(ctx context.Context, client *dbent.Client, userID int64) (bool, float64, error) {
	sourceID := strconv.FormatInt(userID, 10)
	marker, err := client.ExecContext(ctx, `
INSERT INTO balance_transactions (user_id, transaction_type, amount, source_type, source_id, description)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (source_type, source_id) DO NOTHING`, userID, qqBindingWelcomeSourceType, qqBindingWelcomeBonus, qqBindingWelcomeSourceType, sourceID, qqBindingWelcomeDescription)
	if err != nil {
		return false, 0, fmt.Errorf("create QQ binding welcome bonus marker: %w", err)
	}
	created, err := marker.RowsAffected()
	if err != nil {
		return false, 0, fmt.Errorf("read QQ binding welcome bonus marker: %w", err)
	}
	if created == 0 {
		userEntity, err := client.User.Get(ctx, userID)
		if err != nil {
			return false, 0, fmt.Errorf("get user after QQ welcome bonus replay: %w", err)
		}
		return false, userEntity.Balance, nil
	}

	var balanceAfter float64
	if err := scanOne(ctx, client, `UPDATE users SET balance = balance + $2 WHERE id = $1 RETURNING balance`, []any{userID, qqBindingWelcomeBonus}, &balanceAfter); err != nil {
		return false, 0, fmt.Errorf("grant QQ binding welcome bonus: %w", err)
	}
	if _, err := client.ExecContext(ctx, `
UPDATE balance_transactions
SET balance_before = $3, balance_after = $2
WHERE source_type = $1 AND source_id = $4`, qqBindingWelcomeSourceType, balanceAfter, balanceAfter-qqBindingWelcomeBonus, sourceID); err != nil {
		return false, 0, fmt.Errorf("complete QQ binding welcome bonus record: %w", err)
	}
	return true, balanceAfter, nil
}

func (s *QQBindingService) invalidateUserBalance(ctx context.Context, userID int64) {
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if s.billingCache != nil {
		if err := s.billingCache.InvalidateUserBalance(ctx, userID); err != nil {
			logger.LegacyPrintf("service.qq_binding", "invalidate user balance cache failed: user_id=%d err=%v", userID, err)
		}
	}
}
