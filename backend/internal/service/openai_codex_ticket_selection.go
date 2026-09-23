package service

import (
	"context"
	"errors"
	"sync"
	"time"
)

type CodexTicketSelectionRepository interface {
	LoadCodexTicketSelectionAccounts(context.Context, []int64) ([]*Account, error)
}

type codexTicketSelectionCacheKey struct{}
type codexTicketSelectionReadKey struct {
	id              int64
	revision, route string
}
type codexTicketSelectionRead struct {
	once    sync.Once
	account *Account
	err     error
}
type codexTicketSelectionCache struct {
	mu       sync.Mutex
	deadline time.Time
	reads    map[codexTicketSelectionReadKey]*codexTicketSelectionRead
}

// The cache and two-second budget belong to one scheduler pass, never a queued
// wait or the forwarding stage. Injection still makes an authoritative reread.
func withCodexTicketSelectionCache(ctx context.Context) context.Context {
	if _, ok := ctx.Value(codexTicketSelectionCacheKey{}).(*codexTicketSelectionCache); ok {
		return ctx
	}
	return context.WithValue(ctx, codexTicketSelectionCacheKey{}, &codexTicketSelectionCache{
		deadline: time.Now().Add(2 * time.Second), reads: make(map[codexTicketSelectionReadKey]*codexTicketSelectionRead),
	})
}

func (s *OpenAIGatewayService) codexTicketSelectionAccount(ctx context.Context, account *Account) (*Account, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cache, _ := ctx.Value(codexTicketSelectionCacheKey{}).(*codexTicketSelectionCache)
	if cache == nil {
		readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		return s.codexTicketLiveAccount(readCtx, account)
	}
	key := codexTicketSelectionReadKey{account.ID, codexAccountTicketConfigOf(account).Revision, codexTicketFixedProxyFingerprint(account)}
	cache.mu.Lock()
	read := cache.reads[key]
	if read == nil {
		read = &codexTicketSelectionRead{}
		cache.reads[key] = read
	}
	cache.mu.Unlock()
	read.once.Do(func() {
		readCtx, cancel := context.WithDeadline(ctx, cache.deadline)
		defer cancel()
		if err := readCtx.Err(); err != nil {
			read.err = err
			return
		}
		read.account, read.err = s.codexTicketLiveAccount(readCtx, account)
	})
	return read.account, read.err
}

func cacheCodexTicketSelectionAccount(ctx context.Context, snapshot, live *Account, err error) {
	cache, _ := ctx.Value(codexTicketSelectionCacheKey{}).(*codexTicketSelectionCache)
	if cache == nil || snapshot == nil {
		return
	}
	key := codexTicketSelectionReadKey{snapshot.ID, codexAccountTicketConfigOf(snapshot).Revision, codexTicketFixedProxyFingerprint(snapshot)}
	cache.mu.Lock()
	read := cache.reads[key]
	if read == nil {
		read = &codexTicketSelectionRead{}
		cache.reads[key] = read
	}
	cache.mu.Unlock()
	read.once.Do(func() { read.account, read.err = live, err })
}

// Only opted-in accounts for the actual outbound model require ticket state.
// Batch those authoritative reads before candidate filtering; ordinary accounts
// and non-target models retain the original snapshot-only fast path.
func (s *OpenAIGatewayService) primeCodexTicketSelectionAccounts(ctx context.Context, accounts []Account, requestedModel string, compact bool) {
	cache, _ := ctx.Value(codexTicketSelectionCacheKey{}).(*codexTicketSelectionCache)
	repo, ok := s.accountRepo.(CodexTicketSelectionRepository)
	if cache == nil || !ok {
		return
	}
	pending := make(map[int64]*Account)
	ids := make([]int64, 0)
	for i := range accounts {
		a := &accounts[i]
		ac := codexAccountTicketConfigOf(a)
		if !isOpenAICodexTicketAccount(a) || !a.IsSchedulable() || !ac.Enabled || (!ac.RequireVerified && ac.Model != s.openAICodexTicketOutboundModel(a, requestedModel, compact)) {
			continue
		}
		key := codexTicketSelectionReadKey{a.ID, ac.Revision, codexTicketFixedProxyFingerprint(a)}
		cache.mu.Lock()
		_, exists := cache.reads[key]
		cache.mu.Unlock()
		if exists || pending[a.ID] != nil {
			continue
		}
		pending[a.ID] = a
		ids = append(ids, a.ID)
	}
	if len(ids) == 0 {
		return
	}
	enabled, err := s.openAICodexTicketRuntimeEnabled(ctx)
	if err != nil || !enabled {
		return
	}
	for start := 0; start < len(ids); start += 128 {
		batch := ids[start:min(start+128, len(ids))]
		readCtx, cancel := context.WithDeadline(ctx, cache.deadline)
		var loaded []*Account
		loadErr := readCtx.Err()
		if loadErr == nil {
			loaded, loadErr = repo.LoadCodexTicketSelectionAccounts(readCtx, batch)
		}
		cancel()
		byID := make(map[int64]*Account, len(loaded))
		for _, a := range loaded {
			if a != nil {
				byID[a.ID] = a
			}
		}
		for _, id := range batch {
			accountErr := loadErr
			if accountErr == nil && byID[id] == nil {
				accountErr = errors.New("STATE account unavailable")
			}
			cacheCodexTicketSelectionAccount(ctx, pending[id], byID[id], accountErr)
		}
	}
}
