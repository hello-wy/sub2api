package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type codexSelectionCountingRepo struct {
	*stateTicketTestRepo
	reads int
}

type codexSelectionBatchRepo struct {
	*codexSelectionCountingRepo
	batches int
	fail    bool
}

func (r *codexSelectionBatchRepo) LoadCodexTicketSelectionAccounts(ctx context.Context, ids []int64) ([]*Account, error) {
	r.batches++
	if r.fail {
		return nil, errors.New("simulated batch read outage")
	}
	accounts := make([]*Account, 0, len(ids))
	for _, id := range ids {
		a, err := r.stateTicketTestRepo.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	return accounts, nil
}

func TestCodexTicketSelectionBatchesCandidatesWithoutPerAccountFallback(t *testing.T) {
	a := stateTicketTestAccount(8000)
	s, base := stateTicketTestService(t, a)
	repo := &codexSelectionBatchRepo{codexSelectionCountingRepo: &codexSelectionCountingRepo{stateTicketTestRepo: base}}
	s.accountRepo = repo
	accounts := make([]Account, 0, 300)
	for id := int64(8000); id < 8300; id++ {
		live := stateTicketTestAccount(id)
		ticket := stateTicketVerified(live, time.Now())
		live.Extra[openAICodexTicketExtraKey(ticket.Model)] = ticket
		base.accounts[id] = live
		accounts = append(accounts, *cloneStateTicketAccount(live))
	}
	ctx := withCodexTicketSelectionCache(context.Background())
	s.primeCodexTicketSelectionAccounts(ctx, accounts, openAICodexTicketDefaultModel, false)
	for i := range accounts {
		require.False(t, s.openAICodexTicketBlocksAccount(ctx, &accounts[i], openAICodexTicketDefaultModel))
	}
	require.LessOrEqual(t, repo.batches, 3, "hundreds of candidates use bounded batch reads")
	require.Zero(t, repo.reads, "filtering must not reread each candidate")
	repo.fail = true
	repo.batches = 0
	ctx = withCodexTicketSelectionCache(context.Background())
	s.primeCodexTicketSelectionAccounts(ctx, accounts, openAICodexTicketDefaultModel, false)
	for i := range accounts {
		require.True(t, s.openAICodexTicketBlocksAccount(ctx, &accounts[i], openAICodexTicketDefaultModel))
	}
	require.Zero(t, repo.reads, "failed batch must not explode into one fallback query per account")
	repo.batches = 0
	s.primeCodexTicketSelectionAccounts(withCodexTicketSelectionCache(context.Background()), accounts, "other-model", false)
	require.Zero(t, repo.batches, "unmanaged models keep the snapshot fast path")
}

func (r *codexSelectionCountingRepo) GetByID(ctx context.Context, id int64) (*Account, error) {
	r.reads++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r.stateTicketTestRepo.GetByID(ctx, id)
}

func TestCodexTicketSelectionCachesOneReadButInjectionRechecks(t *testing.T) {
	a := stateTicketTestAccount(7951)
	ticket := stateTicketVerified(a, time.Now())
	a.Extra[openAICodexTicketExtraKey(ticket.Model)] = ticket
	s, base := stateTicketTestService(t, a)
	repo := &codexSelectionCountingRepo{stateTicketTestRepo: base}
	s.accountRepo = repo
	ctx := withCodexTicketSelectionCache(context.Background())
	for i := 0; i < 4; i++ {
		require.False(t, s.openAICodexTicketBlocksAccount(ctx, a, ticket.Model))
	}
	require.Equal(t, 1, repo.reads, "candidate filter, channel expansion and final checks reuse a pass read")
	base.mu.Lock()
	base.accounts[a.ID].Extra[openAICodexTicketExtraKey(ticket.Model)] = nil
	base.mu.Unlock()
	require.ErrorIs(t, s.applyOpenAICodexTicket(ctx, a, ticket.Model, http.Header{}), ErrOpenAICodexTicketUnavailable, "send must see revocation despite selection cache")
	require.Equal(t, 2, repo.reads)
}

func TestCodexTicketSelectionHonorsCancellationAndOnePassBudget(t *testing.T) {
	a := stateTicketTestAccount(7952)
	s, base := stateTicketTestService(t, a)
	repo := &codexSelectionCountingRepo{stateTicketTestRepo: base}
	s.accountRepo = repo
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.True(t, s.openAICodexTicketBlocksAccount(withCodexTicketSelectionCache(ctx), a, openAICodexTicketDefaultModel))
	require.Zero(t, repo.reads)
	ctx = withCodexTicketSelectionCache(context.Background())
	cache, ok := ctx.Value(codexTicketSelectionCacheKey{}).(*codexTicketSelectionCache)
	require.True(t, ok)
	cache.deadline = time.Now().Add(-time.Second)
	require.True(t, s.openAICodexTicketBlocksAccount(ctx, a, openAICodexTicketDefaultModel))
	require.Zero(t, repo.reads, "an exhausted pass must not start another two-second DB read")
	_, err := s.codexTicketSelectionAccount(withCodexTicketSelectionCache(context.Background()), a)
	require.NoError(t, err)
	require.Equal(t, 1, repo.reads, "queued admission gets a new pass")
}
