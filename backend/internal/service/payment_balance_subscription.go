package service

import (
	"context"
	"fmt"
	"math"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbuser "github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type BalanceSubscriptionPurchaseRequest struct {
	UserID   int64
	PlanID   int64
	ClientIP string
	SrcHost  string
	SrcURL   string
	Locale   string
}

type BalanceSubscriptionPurchaseResult struct {
	OrderID      int64
	Amount       float64
	NewBalance   float64
	Subscription *UserSubscription
}

func (s *PaymentService) PurchaseSubscriptionWithBalance(ctx context.Context, req BalanceSubscriptionPurchaseRequest) (*BalanceSubscriptionPurchaseResult, error) {
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

	updated, err := tx.User.Update().
		Where(dbuser.IDEQ(req.UserID), dbuser.BalanceGTE(balancePrice)).
		AddBalance(-balancePrice).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("deduct subscription balance: %w", err)
	}
	if updated == 0 {
		return nil, ErrInsufficientBalance.WithMetadata(map[string]string{
			"required":  fmt.Sprintf("%.2f", balancePrice),
			"available": fmt.Sprintf("%.2f", user.Balance),
		})
	}

	outTradeNo, err := s.allocateOutTradeNo(ctx, tx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	order, err := tx.PaymentOrder.Create().
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
		SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeSubscription).
		SetPlanID(plan.ID).
		SetSubscriptionGroupID(plan.GroupID).
		SetSubscriptionDays(psComputeValidityDays(plan.ValidityDays, plan.ValidityUnit)).
		SetStatus(OrderStatusPaid).
		SetPaidAt(now).
		SetExpiresAt(now.Add(time.Duration(max(cfg.OrderTimeoutMin, 1)) * time.Minute)).
		SetClientIP(req.ClientIP).
		SetSrcHost(req.SrcHost).
		SetNillableSrcURL(psNilIfEmpty(req.SrcURL)).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create balance subscription order: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit balance subscription transaction: %w", err)
	}

	if s.subscriptionSvc != nil && s.subscriptionSvc.billingCacheService != nil {
		_ = s.subscriptionSvc.billingCacheService.InvalidateUserBalance(ctx, req.UserID)
	}
	if err := s.ExecuteSubscriptionFulfillment(ctx, order.ID); err != nil {
		return nil, fmt.Errorf("fulfill balance subscription order: %w", err)
	}

	updatedUser, err := s.userRepo.GetByID(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("reload user balance: %w", err)
	}
	subscription, err := s.subscriptionSvc.GetActiveSubscription(ctx, req.UserID, plan.GroupID)
	if err != nil {
		return nil, fmt.Errorf("reload subscription: %w", err)
	}

	return &BalanceSubscriptionPurchaseResult{
		OrderID:      order.ID,
		Amount:       balancePrice,
		NewBalance:   updatedUser.Balance,
		Subscription: subscription,
	}, nil
}

func balanceSubscriptionPlanPrice(plan *dbent.SubscriptionPlan, rechargeMultiplier, usdToCNYRate float64) float64 {
	if plan == nil {
		return 0
	}
	gatewayPrice := calculateSubscriptionGatewayBaseAmount(plan.Price, usdToCNYRate, payment.DefaultPaymentCurrency)
	return calculateCreditedBalance(gatewayPrice, rechargeMultiplier)
}
