package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type subscriptionTermCacheStub struct {
	BillingCache

	mu          sync.Mutex
	data        *SubscriptionCacheData
	setData     *SubscriptionCacheData
	usageCalls  int
	usageUserID int64
	usageGroup  int64
	usageTerm   int64
	usageCost   float64
}

func (s *subscriptionTermCacheStub) GetSubscriptionCache(context.Context, int64, int64) (*SubscriptionCacheData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		return nil, nil
	}
	copy := *s.data
	return &copy, nil
}

func (s *subscriptionTermCacheStub) SetSubscriptionCache(_ context.Context, _, _ int64, data *SubscriptionCacheData) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if data == nil {
		s.setData = nil
		return nil
	}
	copy := *data
	s.setData = &copy
	return nil
}

func (s *subscriptionTermCacheStub) UpdateSubscriptionUsage(_ context.Context, userID, groupID, termVersion int64, cost float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.usageCalls++
	s.usageUserID = userID
	s.usageGroup = groupID
	s.usageTerm = termVersion
	s.usageCost = cost
	return nil
}

type subscriptionTermRepoStub struct {
	UserSubscriptionRepository

	subscription *UserSubscription
	calls        int
}

type renewingSubscriptionTermRepoStub struct {
	*subscriptionUserSubRepoStub

	locked    *UserSubscription
	lockCalls int
}

func (s *renewingSubscriptionTermRepoStub) GetByIDForUpdate(context.Context, int64) (*UserSubscription, error) {
	s.lockCalls++
	copy := *s.locked
	return &copy, nil
}

func (s *subscriptionTermRepoStub) GetActiveByUserIDAndGroupID(context.Context, int64, int64) (*UserSubscription, error) {
	s.calls++
	copy := *s.subscription
	return &copy, nil
}

func TestCheckSubscriptionEligibility_RefreshesStaleCacheForAuthorizedTerm(t *testing.T) {
	now := time.Now()
	cache := &subscriptionTermCacheStub{data: &SubscriptionCacheData{
		Status:      SubscriptionStatusActive,
		ExpiresAt:   now.Add(time.Hour),
		TermVersion: 1,
	}}
	repo := &subscriptionTermRepoStub{subscription: &UserSubscription{
		Status:      SubscriptionStatusActive,
		ExpiresAt:   now.Add(time.Hour),
		TermVersion: 2,
	}}
	svc := &BillingCacheService{cache: cache, subRepo: repo}

	err := svc.checkSubscriptionEligibility(context.Background(), 10, &Group{ID: 20}, &UserSubscription{TermVersion: 2})

	require.NoError(t, err)
	require.Equal(t, 1, repo.calls)
	require.NotNil(t, cache.setData)
	require.Equal(t, int64(2), cache.setData.TermVersion)
}

func TestAssignOrExtendSubscription_IncrementsLatestLockedTermVersion(t *testing.T) {
	baseRepo := newSubscriptionUserSubRepoStub()
	stale := &UserSubscription{
		ID:          50,
		UserID:      10,
		GroupID:     20,
		Status:      SubscriptionStatusActive,
		TermVersion: 7,
		StartsAt:    time.Now().Add(-time.Hour),
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	baseRepo.seed(stale)
	repo := &renewingSubscriptionTermRepoStub{
		subscriptionUserSubRepoStub: baseRepo,
		locked: &UserSubscription{
			ID:          stale.ID,
			UserID:      stale.UserID,
			GroupID:     stale.GroupID,
			Status:      SubscriptionStatusActive,
			TermVersion: 8,
			StartsAt:    stale.StartsAt,
			ExpiresAt:   stale.ExpiresAt,
		},
	}
	svc := NewSubscriptionService(&subscriptionGroupRepoStub{group: &Group{
		ID:               20,
		SubscriptionType: SubscriptionTypeSubscription,
	}}, repo, nil, nil, nil)

	renewed, reused, err := svc.AssignOrExtendSubscription(context.Background(), &AssignSubscriptionInput{
		UserID:       10,
		GroupID:      20,
		ValidityDays: 30,
	})

	require.NoError(t, err)
	require.True(t, reused)
	require.Equal(t, 1, repo.lockCalls)
	require.Equal(t, int64(9), renewed.TermVersion)
}

func TestCheckSubscriptionEligibility_RejectsReplacedAuthorizedTerm(t *testing.T) {
	now := time.Now()
	cache := &subscriptionTermCacheStub{data: &SubscriptionCacheData{
		Status:      SubscriptionStatusActive,
		ExpiresAt:   now.Add(time.Hour),
		TermVersion: 2,
	}}
	repo := &subscriptionTermRepoStub{subscription: &UserSubscription{
		Status:      SubscriptionStatusActive,
		ExpiresAt:   now.Add(time.Hour),
		TermVersion: 2,
	}}
	svc := &BillingCacheService{cache: cache, subRepo: repo}

	err := svc.checkSubscriptionEligibility(context.Background(), 10, &Group{ID: 20}, &UserSubscription{TermVersion: 1})

	require.ErrorIs(t, err, ErrSubscriptionInvalid)
	require.Equal(t, 1, repo.calls)
	require.Nil(t, cache.setData)
}

func TestFinalizePostUsageBilling_SubscriptionTermControlsCacheUpdate(t *testing.T) {
	groupID := int64(20)
	newParams := func() *postUsageBillingParams {
		return &postUsageBillingParams{
			Cost:               &CostBreakdown{ActualCost: 1.25},
			User:               &User{ID: 10},
			APIKey:             &APIKey{ID: 30, GroupID: &groupID},
			Account:            &Account{ID: 40},
			Subscription:       &UserSubscription{ID: 50, TermVersion: 7},
			IsSubscriptionBill: true,
		}
	}

	t.Run("stale term skips cache increment", func(t *testing.T) {
		cache := &subscriptionTermCacheStub{}
		finalizePostUsageBilling(context.Background(), newParams(), &billingDeps{
			billingCacheService: &BillingCacheService{cache: cache},
			deferredService:     NewDeferredService(nil, nil, time.Second),
		}, &UsageBillingApplyResult{SubscriptionTermStale: true})

		require.Zero(t, cache.usageCalls)
	})

	t.Run("matching term forwards identity and cost", func(t *testing.T) {
		cache := &subscriptionTermCacheStub{}
		finalizePostUsageBilling(context.Background(), newParams(), &billingDeps{
			billingCacheService: &BillingCacheService{cache: cache},
			deferredService:     NewDeferredService(nil, nil, time.Second),
		}, &UsageBillingApplyResult{})

		require.Equal(t, 1, cache.usageCalls)
		require.Equal(t, int64(10), cache.usageUserID)
		require.Equal(t, int64(20), cache.usageGroup)
		require.Equal(t, int64(7), cache.usageTerm)
		require.InDelta(t, 1.25, cache.usageCost, 0.000001)
	})
}

func TestUsageBillingFingerprint_IncludesSubscriptionTermVersion(t *testing.T) {
	subscriptionID := int64(50)
	first := &UsageBillingCommand{
		RequestID:               "request-1",
		APIKeyID:                30,
		SubscriptionID:          &subscriptionID,
		SubscriptionTermVersion: 1,
		SubscriptionCost:        1,
	}
	second := *first
	second.SubscriptionTermVersion = 2

	first.Normalize()
	second.Normalize()

	require.NotEqual(t, first.RequestFingerprint, second.RequestFingerprint)
}
