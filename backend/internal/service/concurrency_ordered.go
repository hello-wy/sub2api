package service

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"
)

// OrderedAccountSlotCache selects and reserves the first channel with capacity
// in one operation. Reading cached load followed by a single-channel acquire is
// insufficient: simultaneous requests could bypass a newly available first IP.
type OrderedAccountSlotCache interface {
	AcquireOrderedAccountSlot(context.Context, []AccountWithConcurrency, string) (int64, error)
}

// LogicalAccountConcurrency separates account selection from fixed-IP order.
// LoadAccountIDs includes busy disabled channels: removing a channel from the
// eligible set must not make its still-running requests disappear from load.
type LogicalAccountConcurrency struct {
	ID             int64
	Priority       int
	Channels       []AccountWithConcurrency
	LoadAccountIDs []int64
}

type BalancedLogicalAccountSlotCache interface {
	AcquireBalancedLogicalAccountSlot(context.Context, []LogicalAccountConcurrency, string) (int64, bool, error)
}

func (s *ConcurrencyService) AcquireBalancedLogicalAccountSlot(ctx context.Context, accounts []LogicalAccountConcurrency) (int64, *AcquireResult, error) {
	if s == nil || s.cache == nil {
		return 0, nil, errors.New("logical account concurrency cache is unavailable")
	}
	cache, ok := s.cache.(BalancedLogicalAccountSlotCache)
	if !ok {
		return 0, nil, errors.New("balanced logical account concurrency is unsupported by this cache")
	}
	requestID := generateRequestID()
	id, acquired, err := cache.AcquireBalancedLogicalAccountSlot(ctx, accounts, requestID)
	if err != nil || !acquired {
		return id, &AcquireResult{}, err
	}
	var once sync.Once
	return id, &AcquireResult{Acquired: true, ReleaseFunc: func() {
		once.Do(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = s.cache.ReleaseAccountSlot(ctx, id, requestID)
		})
	}}, nil
}

func (s *ConcurrencyService) AcquireOrderedAccountSlot(ctx context.Context, accounts []AccountWithConcurrency) (int64, *AcquireResult, error) {
	if len(accounts) == 0 {
		return 0, &AcquireResult{}, nil
	}
	if s == nil || s.cache == nil {
		return 0, nil, errors.New("ordered IP channel concurrency cache is unavailable")
	}
	cache, ok := s.cache.(OrderedAccountSlotCache)
	if !ok {
		return 0, nil, errors.New("ordered IP channel concurrency is unsupported by this cache")
	}
	limits := append([]AccountWithConcurrency(nil), accounts...)
	for i := range limits {
		if limits[i].MaxConcurrency <= 0 {
			limits[i].MaxConcurrency = math.MaxInt32
		}
	}
	requestID := generateRequestID()
	id, err := cache.AcquireOrderedAccountSlot(ctx, limits, requestID)
	if err != nil || id == 0 {
		return id, &AcquireResult{}, err
	}
	var releaseOnce sync.Once
	return id, &AcquireResult{Acquired: true, ReleaseFunc: func() {
		releaseOnce.Do(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = s.cache.ReleaseAccountSlot(ctx, id, requestID)
		})
	}}, nil
}
