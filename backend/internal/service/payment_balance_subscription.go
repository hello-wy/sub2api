package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	dbuser "github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type BalanceSubscriptionPurchaseRequest struct {
	UserID         int64
	PlanID         int64
	ClientIP       string
	SrcHost        string
	SrcURL         string
	Locale         string
	IdempotencyKey string
}

type BalanceSubscriptionPurchaseResult struct {
	OrderID      int64
	Amount       float64
	NewBalance   float64
	Subscription *UserSubscription
}

func (s *PaymentService) PurchaseSubscriptionWithBalance(ctx context.Context, req BalanceSubscriptionPurchaseRequest) (*BalanceSubscriptionPurchaseResult, error) {
	key, err := NormalizeIdempotencyKey(req.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	if key == "" {
		return nil, ErrIdempotencyKeyRequired
	}
	idempotencyRef := "idem:" + HashIdempotencyKey(key)
	if recovered, err := s.recoverBalanceSubscriptionPurchase(ctx, req.UserID, req.PlanID, idempotencyRef); err != nil {
		return nil, err
	} else if recovered != nil {
		return recovered, nil
	}
	if s.subscriptionSvc == nil {
		return nil, fmt.Errorf("subscription service is unavailable")
	}

	cfg, err := s.configService.GetPaymentConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("get payment config: %w", err)
	}
	if !cfg.Enabled {
		return nil, infraerrors.Forbidden("PAYMENT_DISABLED", "payment system is disabled")
	}

	plan, err := s.validateSubOrder(ctx, CreateOrderRequest{
		UserID:    req.UserID,
		OrderType: payment.OrderTypeSubscription,
		PlanID:    req.PlanID,
	})
	if err != nil {
		return nil, err
	}
	balancePrice := balanceSubscriptionPlanPrice(plan, cfg.BalanceRechargeMultiplier, cfg.SubscriptionUSDToCNYRate)
	if math.IsNaN(balancePrice) || math.IsInf(balancePrice, 0) || balancePrice <= 0 {
		return nil, infraerrors.BadRequest("INVALID_AMOUNT", "subscription plan price must be positive")
	}

	user, err := s.userRepo.GetByID(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user.Status != payment.EntityStatusActive {
		return nil, infraerrors.Forbidden("USER_INACTIVE", "user account is disabled")
	}
	if s.notificationEmailService != nil {
		s.notificationEmailService.RememberRecipientLocale(ctx, req.UserID, user.Email, req.Locale)
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin balance subscription transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	txClient := tx.Client()

	updated, err := txClient.User.Update().
		Where(dbuser.IDEQ(req.UserID), dbuser.BalanceGTE(balancePrice)).
		AddBalance(-balancePrice).
		Save(txCtx)
	if err != nil {
		return nil, fmt.Errorf("deduct subscription balance: %w", err)
	}
	if updated == 0 {
		return nil, ErrInsufficientBalance.WithMetadata(map[string]string{
			"required":  fmt.Sprintf("%.2f", balancePrice),
			"available": fmt.Sprintf("%.2f", user.Balance),
		})
	}

	outTradeNo, err := s.allocateOutTradeNo(txCtx, tx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	order, err := txClient.PaymentOrder.Create().
		SetUserID(req.UserID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetNillableUserNotes(psNilIfEmpty(user.Notes)).
		SetAmount(balancePrice).
		SetPayAmount(balancePrice).
		SetFeeRate(0).
		SetRechargeCode("").
		SetOutTradeNo(outTradeNo).
		SetPaymentType("balance").
		SetPaymentTradeNo(idempotencyRef).
		SetOrderType(payment.OrderTypeSubscription).
		SetPlanID(plan.ID).
		SetSubscriptionGroupID(plan.GroupID).
		SetSubscriptionDays(psComputeValidityDays(plan.ValidityDays, plan.ValidityUnit)).
		SetStatus(OrderStatusCompleted).
		SetPaidAt(now).
		SetCompletedAt(now).
		SetExpiresAt(now.Add(time.Duration(max(cfg.OrderTimeoutMin, 1)) * time.Minute)).
		SetClientIP(req.ClientIP).
		SetSrcHost(req.SrcHost).
		SetNillableSrcURL(psNilIfEmpty(req.SrcURL)).
		Save(txCtx)
	if err != nil {
		if dbent.IsConstraintError(err) {
			_ = tx.Rollback()
			recovered, recoverErr := s.recoverBalanceSubscriptionPurchase(ctx, req.UserID, req.PlanID, idempotencyRef)
			if recoverErr != nil {
				return nil, recoverErr
			}
			if recovered != nil {
				return recovered, nil
			}
		}
		return nil, fmt.Errorf("create balance subscription order: %w", err)
	}
	if err := recordBalanceSubscriptionPayment(txCtx, txClient, req.UserID, balancePrice, plan.Name, now); err != nil {
		return nil, err
	}
	orderNote := paymentSubscriptionOrderNote(order.ID)
	subscription, err := s.subscriptionSvc.renewSubscriptionForPayment(txCtx, &AssignSubscriptionInput{
		UserID:       req.UserID,
		GroupID:      plan.GroupID,
		ValidityDays: psComputeValidityDays(plan.ValidityDays, plan.ValidityUnit),
		AssignedBy:   0,
		Notes:        orderNote,
	})
	if err != nil {
		return nil, fmt.Errorf("assign balance-funded subscription: %w", err)
	}
	if err := recordCompletedBalanceSubscriptionAudits(txCtx, txClient, order.ID, plan.GroupID); err != nil {
		return nil, err
	}
	updatedUser, err := txClient.User.Get(txCtx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("reload balance in transaction: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit balance subscription transaction: %w", err)
	}

	if s.subscriptionSvc.billingCacheService != nil {
		_ = s.subscriptionSvc.billingCacheService.InvalidateUserBalance(ctx, req.UserID)
	}
	if err := s.subscriptionSvc.invalidateSubscriptionCaches(req.UserID, plan.GroupID); err != nil {
		slog.Error("invalidate subscription cache after balance purchase failed",
			"user_id", req.UserID, "group_id", plan.GroupID, "order_id", order.ID, "error", err)
	}
	s.dispatchPaymentFulfillmentNotification(order, "SUBSCRIPTION_SUCCESS")

	return &BalanceSubscriptionPurchaseResult{
		OrderID:      order.ID,
		Amount:       balancePrice,
		NewBalance:   updatedUser.Balance,
		Subscription: subscription,
	}, nil
}

func (s *PaymentService) recoverBalanceSubscriptionPurchase(ctx context.Context, userID, planID int64, idempotencyRef string) (*BalanceSubscriptionPurchaseResult, error) {
	if s == nil || s.entClient == nil || s.subscriptionSvc == nil || userID <= 0 || idempotencyRef == "" {
		return nil, nil
	}
	order, err := s.entClient.PaymentOrder.Query().Where(
		paymentorder.UserIDEQ(userID),
		paymentorder.PaymentTypeEQ("balance"),
		paymentorder.OrderTypeEQ(payment.OrderTypeSubscription),
		paymentorder.PaymentTradeNoEQ(idempotencyRef),
		paymentorder.StatusEQ(OrderStatusCompleted),
	).Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("recover balance subscription purchase: %w", err)
	}
	if order.PlanID == nil || *order.PlanID != planID {
		return nil, ErrIdempotencyKeyConflict
	}
	if order.SubscriptionGroupID == nil {
		return nil, fmt.Errorf("recover balance subscription purchase: order %d missing group", order.ID)
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("recover user balance: %w", err)
	}
	subscription, err := s.subscriptionSvc.GetActiveSubscription(ctx, userID, *order.SubscriptionGroupID)
	if err != nil {
		return nil, fmt.Errorf("recover active subscription: %w", err)
	}
	return &BalanceSubscriptionPurchaseResult{
		OrderID: order.ID, Amount: order.Amount, NewBalance: user.Balance, Subscription: subscription,
	}, nil
}

func recordCompletedBalanceSubscriptionAudits(ctx context.Context, client *dbent.Client, orderID, groupID int64) error {
	detail, _ := json.Marshal(map[string]any{"groupID": groupID, "funding": "balance"})
	for _, action := range []string{"SUBSCRIPTION_ASSIGNED", "SUBSCRIPTION_SUCCESS"} {
		if _, err := client.PaymentAuditLog.Create().
			SetOrderID(strconv.FormatInt(orderID, 10)).
			SetAction(action).
			SetDetail(string(detail)).
			SetOperator("system").
			Save(ctx); err != nil {
			return fmt.Errorf("record %s audit: %w", action, err)
		}
	}
	return nil
}

func recordBalanceSubscriptionPayment(ctx context.Context, client *dbent.Client, userID int64, amount float64, planName string, at time.Time) error {
	if client == nil || amount <= 0 {
		return nil
	}
	code, err := GenerateRedeemCode()
	if err != nil {
		return fmt.Errorf("generate subscription payment history code: %w", err)
	}
	if _, err := client.RedeemCode.Create().
		SetCode(code).
		SetType(AdjustmentTypeSubscriptionPay).
		SetValue(-amount).
		SetStatus(StatusUsed).
		SetUsedBy(userID).
		SetUsedAt(at).
		SetNotes(planName).
		Save(ctx); err != nil {
		return fmt.Errorf("record subscription balance payment: %w", err)
	}
	return nil
}

func balanceSubscriptionPlanPrice(plan *dbent.SubscriptionPlan, rechargeMultiplier, usdToCNYRate float64) float64 {
	if plan == nil {
		return 0
	}
	gatewayPrice := calculateSubscriptionGatewayBaseAmount(plan.Price, usdToCNYRate, payment.DefaultPaymentCurrency)
	return calculateCreditedBalance(gatewayPrice, rechargeMultiplier)
}
