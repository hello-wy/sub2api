package service

import (
	"context"
	"errors"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/ent/usersubscription"
	"github.com/stretchr/testify/require"
)

type balancePurchaseUserRepo struct {
	UserRepository
	client *dbent.Client
}

func (r *balancePurchaseUserRepo) GetByID(ctx context.Context, id int64) (*User, error) {
	client := r.client
	if tx := dbent.TxFromContext(ctx); tx != nil {
		client = tx.Client()
	}
	u, err := client.User.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return &User{ID: u.ID, Email: u.Email, Username: u.Username, Notes: u.Notes, Balance: u.Balance, Status: u.Status}, nil
}

type balancePurchaseGroupRepo struct {
	GroupRepository
	group *Group
}

func (r *balancePurchaseGroupRepo) GetByID(context.Context, int64) (*Group, error) {
	return r.group, nil
}

type balancePurchaseSubRepo struct {
	UserSubscriptionRepository
	client     *dbent.Client
	failCreate bool
}

func (r *balancePurchaseSubRepo) txClient(ctx context.Context) *dbent.Client {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return tx.Client()
	}
	return r.client
}

func (r *balancePurchaseSubRepo) ListActiveByUserID(ctx context.Context, userID int64) ([]UserSubscription, error) {
	entities, err := r.txClient(ctx).UserSubscription.Query().Where(
		usersubscription.UserIDEQ(userID),
		usersubscription.StatusEQ(SubscriptionStatusActive),
		usersubscription.ExpiresAtGT(time.Now()),
	).All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]UserSubscription, 0, len(entities))
	for _, entity := range entities {
		result = append(result, balancePurchaseSubscription(entity))
	}
	return result, nil
}

func (r *balancePurchaseSubRepo) GetByUserIDAndGroupID(ctx context.Context, userID, groupID int64) (*UserSubscription, error) {
	entity, err := r.txClient(ctx).UserSubscription.Query().Where(
		usersubscription.UserIDEQ(userID), usersubscription.GroupIDEQ(groupID),
	).Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, ErrSubscriptionNotFound
	}
	if err != nil {
		return nil, err
	}
	sub := balancePurchaseSubscription(entity)
	return &sub, nil
}

func (r *balancePurchaseSubRepo) GetByID(ctx context.Context, id int64) (*UserSubscription, error) {
	entity, err := r.txClient(ctx).UserSubscription.Get(ctx, id)
	if dbent.IsNotFound(err) {
		return nil, ErrSubscriptionNotFound
	}
	if err != nil {
		return nil, err
	}
	sub := balancePurchaseSubscription(entity)
	return &sub, nil
}

func (r *balancePurchaseSubRepo) GetActiveByUserIDAndGroupID(ctx context.Context, userID, groupID int64) (*UserSubscription, error) {
	entity, err := r.txClient(ctx).UserSubscription.Query().Where(
		usersubscription.UserIDEQ(userID),
		usersubscription.GroupIDEQ(groupID),
		usersubscription.StatusEQ(SubscriptionStatusActive),
		usersubscription.ExpiresAtGT(time.Now()),
	).Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, ErrSubscriptionNotFound
	}
	if err != nil {
		return nil, err
	}
	sub := balancePurchaseSubscription(entity)
	return &sub, nil
}

func (r *balancePurchaseSubRepo) Create(ctx context.Context, sub *UserSubscription) error {
	if r.failCreate {
		return errors.New("injected subscription write failure")
	}
	created, err := r.txClient(ctx).UserSubscription.Create().
		SetUserID(sub.UserID).
		SetGroupID(sub.GroupID).
		SetStartsAt(sub.StartsAt).
		SetExpiresAt(sub.ExpiresAt).
		SetStatus(sub.Status).
		SetAssignedAt(sub.AssignedAt).
		SetNotes(sub.Notes).
		Save(ctx)
	if err != nil {
		return err
	}
	*sub = balancePurchaseSubscription(created)
	return nil
}

func balancePurchaseSubscription(entity *dbent.UserSubscription) UserSubscription {
	notes := ""
	if entity.Notes != nil {
		notes = *entity.Notes
	}
	return UserSubscription{
		ID: entity.ID, UserID: entity.UserID, GroupID: entity.GroupID,
		StartsAt: entity.StartsAt, ExpiresAt: entity.ExpiresAt, Status: entity.Status,
		AssignedAt: entity.AssignedAt, Notes: notes,
	}
}

func newBalancePurchaseService(t *testing.T, failSubscription bool) (*PaymentService, *dbent.Client, int64, int64) {
	t.Helper()
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	user, err := client.User.Create().
		SetEmail("balance-purchase@example.com").
		SetPasswordHash("hash").
		SetUsername("balance-purchase").
		SetStatus(StatusActive).
		SetBalance(100).
		Save(ctx)
	require.NoError(t, err)
	groupEntity, err := client.Group.Create().
		SetName("balance purchase group").
		SetPlatform(PlatformOpenAI).
		SetStatus(StatusActive).
		SetSubscriptionType(SubscriptionTypeSubscription).
		Save(ctx)
	require.NoError(t, err)
	plan, err := client.SubscriptionPlan.Create().
		SetGroupID(groupEntity.ID).
		SetName("monthly").
		SetPrice(10).
		SetValidityDays(30).
		SetValidityUnit("day").
		SetForSale(true).
		Save(ctx)
	require.NoError(t, err)

	groupRepo := &balancePurchaseGroupRepo{group: &Group{
		ID: groupEntity.ID, Name: groupEntity.Name, Platform: groupEntity.Platform,
		Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription,
	}}
	subRepo := &balancePurchaseSubRepo{client: client, failCreate: failSubscription}
	subscriptionSvc := NewSubscriptionService(groupRepo, subRepo, nil, client, nil)
	configService := &PaymentConfigService{
		entClient: client,
		settingRepo: &paymentConfigSettingRepoStub{values: map[string]string{
			SettingPaymentEnabled:           "true",
			SettingBalanceRechargeMult:      "1",
			SettingSubscriptionUSDToCNYRate: "0",
		}},
	}
	service := &PaymentService{
		entClient: client, subscriptionSvc: subscriptionSvc, configService: configService,
		userRepo: &balancePurchaseUserRepo{client: client}, groupRepo: groupRepo,
	}
	return service, client, user.ID, plan.ID
}

func TestPurchaseSubscriptionWithBalanceRequiresIdempotencyKey(t *testing.T) {
	_, err := (&PaymentService{}).PurchaseSubscriptionWithBalance(context.Background(), BalanceSubscriptionPurchaseRequest{})
	require.ErrorIs(t, err, ErrIdempotencyKeyRequired)
}

func TestPurchaseSubscriptionWithBalanceRollsBackAllFinancialWrites(t *testing.T) {
	service, client, userID, planID := newBalancePurchaseService(t, true)
	_, err := service.PurchaseSubscriptionWithBalance(context.Background(), BalanceSubscriptionPurchaseRequest{
		UserID: userID, PlanID: planID, IdempotencyKey: "rollback-key", SrcHost: "test",
	})
	require.ErrorContains(t, err, "injected subscription write failure")

	user, err := client.User.Get(context.Background(), userID)
	require.NoError(t, err)
	require.InDelta(t, 100, user.Balance, 1e-9)
	orderCount, err := client.PaymentOrder.Query().Count(context.Background())
	require.NoError(t, err)
	require.Zero(t, orderCount)
	historyCount, err := client.RedeemCode.Query().Count(context.Background())
	require.NoError(t, err)
	require.Zero(t, historyCount)
	subscriptionCount, err := client.UserSubscription.Query().Count(context.Background())
	require.NoError(t, err)
	require.Zero(t, subscriptionCount)
	auditCount, err := client.PaymentAuditLog.Query().Count(context.Background())
	require.NoError(t, err)
	require.Zero(t, auditCount)
}

func TestPurchaseSubscriptionWithBalanceRecoversSameIdempotentOrder(t *testing.T) {
	service, client, userID, planID := newBalancePurchaseService(t, false)
	req := BalanceSubscriptionPurchaseRequest{
		UserID: userID, PlanID: planID, IdempotencyKey: "stable-key", SrcHost: "test",
	}
	first, err := service.PurchaseSubscriptionWithBalance(context.Background(), req)
	require.NoError(t, err)
	second, err := service.PurchaseSubscriptionWithBalance(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, first.OrderID, second.OrderID)
	_, err = service.PurchaseSubscriptionWithBalance(context.Background(), BalanceSubscriptionPurchaseRequest{
		UserID: userID, PlanID: planID + 1, IdempotencyKey: req.IdempotencyKey, SrcHost: "test",
	})
	require.ErrorIs(t, err, ErrIdempotencyKeyConflict)

	user, err := client.User.Get(context.Background(), userID)
	require.NoError(t, err)
	require.InDelta(t, 90, user.Balance, 1e-9)
	orderCount, err := client.PaymentOrder.Query().Count(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, orderCount)
	order, err := client.PaymentOrder.Query().Only(context.Background())
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, order.Status)
	require.Equal(t, "idem:"+HashIdempotencyKey(req.IdempotencyKey), order.PaymentTradeNo)
	require.NotEqual(t, req.IdempotencyKey, order.PaymentTradeNo)
	historyCount, err := client.RedeemCode.Query().Count(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, historyCount)
	subscriptionCount, err := client.UserSubscription.Query().Count(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, subscriptionCount)
	auditCount, err := client.PaymentAuditLog.Query().Count(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, auditCount)
}

func TestPurchaseSubscriptionWithBalanceDoesNotGrantLoyaltyPoints(t *testing.T) {
	ctx := context.Background()
	service, client, userID, planID := newBalancePurchaseService(t, false)
	defs, configured, err := service.ensureLoyaltyAttributeDefinitions(ctx)
	require.NoError(t, err)
	require.True(t, configured)
	createPaymentLoyaltyTestValue(t, client, userID, defs[LoyaltyWeeklyPointsAttributeKey].ID, "40", time.Now())
	createPaymentLoyaltyTestValue(t, client, userID, defs[LoyaltyPermanentPointsAttributeKey].ID, "400", time.Now())

	_, err = service.PurchaseSubscriptionWithBalance(ctx, BalanceSubscriptionPurchaseRequest{
		UserID: userID, PlanID: planID, IdempotencyKey: "balance-no-loyalty", SrcHost: "test",
	})
	require.NoError(t, err)
	require.Equal(t, "40", readPaymentLoyaltyTestValue(t, client, userID, defs[LoyaltyWeeklyPointsAttributeKey].ID))
	require.Equal(t, "400", readPaymentLoyaltyTestValue(t, client, userID, defs[LoyaltyPermanentPointsAttributeKey].ID))

	pointsAuditCount, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.ActionEQ(paymentLoyaltyAuditAction)).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, pointsAuditCount)
}
