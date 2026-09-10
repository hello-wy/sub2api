package service

import (
	"context"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

const quotaTransitionCacheTimeout = 30 * time.Second

// Carry rolling usage forward only when entering lifetime quota mode.
func (s *adminServiceImpl) updateGroupQuota(ctx context.Context, group *Group, wasLifetimeQuota bool) error {
	if wasLifetimeQuota || !group.UsesSubscriptionLifetimeQuota() {
		return s.groupRepo.Update(ctx, group)
	}
	if s.groupQuotaRepo == nil {
		return errors.New("group repository does not support subscription quota transitions")
	}
	userIDs, err := s.groupQuotaRepo.UpdateWithSubscriptionQuotaTransition(ctx, group)
	if err != nil {
		return err
	}
	s.invalidateGroupQuotaSubscriptions(group.ID, userIDs)
	return nil
}

func (s *adminServiceImpl) invalidateGroupQuotaSubscriptions(groupID int64, userIDs []int64) {
	if len(userIDs) == 0 || s.billingCacheService == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), quotaTransitionCacheTimeout)
	defer cancel()
	for _, userID := range userIDs {
		if err := s.billingCacheService.InvalidateSubscription(ctx, userID, groupID); err != nil {
			logger.LegacyPrintf("service.admin", "invalidate subscription cache after quota transition failed: user_id=%d group_id=%d err=%v", userID, groupID, err)
		}
		if err := s.billingCacheService.PublishSubscriptionCacheInvalidation(ctx, subCacheKey(userID, groupID)); err != nil {
			logger.LegacyPrintf("service.admin", "publish subscription cache invalidation after quota transition failed: user_id=%d group_id=%d err=%v", userID, groupID, err)
		}
	}
}
