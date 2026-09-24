package service

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type codexCoordinatorFixture struct {
	GatewayCache
	mu        sync.Mutex
	owners    map[int64]string
	cooldowns map[int64]CodexTicketHarvestState
	sources   map[int64]string
}

func newCodexCoordinatorFixture() *codexCoordinatorFixture {
	return &codexCoordinatorFixture{owners: map[int64]string{}, cooldowns: map[int64]CodexTicketHarvestState{}, sources: map[int64]string{}}
}
func (c *codexCoordinatorFixture) ClaimCodexTicketHarvest(_ context.Context, id int64, source, owner string, limit int, _ time.Duration) (CodexTicketHarvestState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if state := c.cooldowns[id]; c.sources[id] == source && time.Now().Before(state.RetryAfter) {
		return state, nil
	}
	if c.owners[id] != "" {
		return CodexTicketHarvestState{Running: true}, nil
	}
	if len(c.owners) >= limit {
		return CodexTicketHarvestState{}, nil
	}
	c.owners[id] = owner
	return CodexTicketHarvestState{Acquired: true, Running: true}, nil
}
func (c *codexCoordinatorFixture) RefreshCodexTicketHarvest(_ context.Context, id int64, owner string, _ time.Duration) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.owners[id] == owner, nil
}
func (c *codexCoordinatorFixture) FinishCodexTicketHarvest(_ context.Context, id int64, source, owner string, cooldown time.Duration, reason string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.owners[id] != owner {
		return false, nil
	}
	delete(c.owners, id)
	if cooldown > 0 {
		c.sources[id] = source
		c.cooldowns[id] = CodexTicketHarvestState{RetryAfter: time.Now().Add(cooldown), LastError: reason}
	} else {
		delete(c.cooldowns, id)
	}
	return true, nil
}
func (c *codexCoordinatorFixture) GetCodexTicketHarvestState(_ context.Context, id int64, source string) (CodexTicketHarvestState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if state := c.cooldowns[id]; c.sources[id] == source && time.Now().Before(state.RetryAfter) {
		return state, nil
	}
	return CodexTicketHarvestState{Running: c.owners[id] != ""}, nil
}

func TestCodexTicketCoordinatedJobsDeduplicateAcrossInstances(t *testing.T) {
	a := stateTicketTestAccount(7991)
	first, repo := stateTicketTestService(t, a)
	second, _ := stateTicketTestService(t, a)
	second.accountRepo = repo
	coordinator := newCodexCoordinatorFixture()
	first.cache = coordinator
	second.cache = coordinator
	started := make(chan struct{}, 2)
	probe := func(ctx context.Context, _ *Account, _, _, _, _ string, _ time.Duration) (string, int, error) {
		started <- struct{}{}
		<-ctx.Done()
		return "", 0, ctx.Err()
	}
	first.openaiCodexTicketProbe = probe
	second.openaiCodexTicketProbe = probe
	require.NotNil(t, first.startCodexAccountTicketJob(context.Background(), a.ID, false))
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first probe did not start")
	}
	waitStateTicketJob(t, second.startCodexAccountTicketJob(context.Background(), a.ID, false))
	select {
	case <-started:
		t.Fatal("duplicate upstream probe")
	default:
	}
	status, err := second.GetCodexAccountTicketStatus(context.Background(), a.ID)
	require.NoError(t, err)
	require.Equal(t, "harvesting", status.State, "another instance's job is visible")
}

func TestCodexTicketCoordinatedCooldownSurvivesRestart(t *testing.T) {
	a := stateTicketTestAccount(7992)
	first, repo := stateTicketTestService(t, a)
	coordinator := newCodexCoordinatorFixture()
	first.cache = coordinator
	var calls atomic.Int32
	probe := func(context.Context, *Account, string, string, string, string, time.Duration) (string, int, error) {
		calls.Add(1)
		return "", 401, nil
	}
	first.openaiCodexTicketProbe = probe
	waitStateTicketJob(t, first.startCodexAccountTicketJob(context.Background(), a.ID, false))
	second, _ := stateTicketTestService(t, a)
	second.accountRepo = repo
	second.cache = coordinator
	second.openaiCodexTicketProbe = probe
	waitStateTicketJob(t, second.startCodexAccountTicketJob(context.Background(), a.ID, false))
	require.EqualValues(t, 1, calls.Load())
	status, err := second.GetCodexAccountTicketStatus(context.Background(), a.ID)
	require.NoError(t, err)
	require.NotNil(t, status.RetryAfter)
	require.True(t, status.RetryAfter.After(time.Now()))
	require.Contains(t, status.LastError, "401")
}

func TestCodexTicketLostLeaseCannotPublishOrReleaseReplacement(t *testing.T) {
	a := stateTicketTestAccount(7993)
	s, repo := stateTicketTestService(t, a)
	coordinator := newCodexCoordinatorFixture()
	s.cache = coordinator
	s.openaiCodexTicketProbe = func(_ context.Context, account *Account, _, _, _, state string, _ time.Duration) (string, int, error) {
		if state == "" {
			return stateTicketVerified(account, time.Now()).State, 200, nil
		}
		coordinator.mu.Lock()
		coordinator.owners[a.ID] = "replacement-owner"
		coordinator.mu.Unlock()
		return "", 200, nil
	}
	waitStateTicketJob(t, s.startCodexAccountTicketJob(context.Background(), a.ID, false))
	live, err := repo.GetByID(context.Background(), a.ID)
	require.NoError(t, err)
	require.Nil(t, live.Extra[openAICodexTicketExtraKey(openAICodexTicketDefaultModel)])
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	require.Equal(t, "replacement-owner", coordinator.owners[a.ID])
	require.Empty(t, coordinator.cooldowns)
}

type delayedCodexRenewal struct{ *codexCoordinatorFixture }

type codexPolicyReleaseFixture struct {
	*codexCoordinatorFixture
	finishEnforced bool
}

func (c *codexPolicyReleaseFixture) FinishCodexTicketHarvest(ctx context.Context, id int64, source, owner string, cooldown time.Duration, reason string) (bool, error) {
	c.finishEnforced = Context429Enforcement(ctx)
	return c.codexCoordinatorFixture.FinishCodexTicketHarvest(ctx, id, source, owner, cooldown, reason)
}

func TestCodexTicketLeaseReleaseRetainsJob429Policy(t *testing.T) {
	old := Context429Enforcement(context.TODO())
	t.Cleanup(func() { Set429EnforcementEnabled(old) })
	Set429EnforcementEnabled(false)
	a := stateTicketTestAccount(7995)
	s, _ := stateTicketTestService(t, a)
	coordinator := &codexPolicyReleaseFixture{codexCoordinatorFixture: newCodexCoordinatorFixture()}
	s.cache = coordinator
	s.openaiCodexTicketProbe = func(context.Context, *Account, string, string, string, string, time.Duration) (string, int, error) {
		return "", 429, nil
	}
	ctx, cancel := context.WithCancel(With429Enforcement(context.Background(), true))
	defer cancel()
	pool := s.openAICodexTicketHarvestProxyURLContext(ctx)
	job := &codexAccountTicketJob{revision: codexAccountTicketConfigOf(a).Revision, fixedFingerprint: codexTicketFixedProxyFingerprint(a), harvestProxyURL: pool, owner: "policy-owner", source: codexTicketHarvestSource(a, pool), running: true}
	s.runCoordinatedCodexTicketJob(ctx, cancel, a.ID, job)
	require.True(t, coordinator.finishEnforced, "detaching shutdown cancellation must preserve the job's explicit policy")
	require.True(t, coordinator.cooldowns[a.ID].RetryAfter.After(time.Now()))
}

func (c *delayedCodexRenewal) RefreshCodexTicketHarvest(ctx context.Context, _ int64, _ string, _ time.Duration) (bool, error) {
	// Simulate a successful Redis renewal whose caller was then suspended past
	// the entire publication budget before it could inspect the response.
	<-ctx.Done()
	return true, nil
}

func TestCodexTicketPublicationBudgetStartsBeforeLeaseRenewal(t *testing.T) {
	a := stateTicketTestAccount(7994)
	s, repo := stateTicketTestService(t, a)
	s.cache = &delayedCodexRenewal{newCodexCoordinatorFixture()}
	s.openaiCodexTicketProbe = func(_ context.Context, account *Account, _, _, _, state string, _ time.Duration) (string, int, error) {
		if state == "" {
			return stateTicketVerified(account, time.Now()).State, 200, nil
		}
		return "", 200, nil
	}
	job := s.startCodexAccountTicketJob(context.Background(), a.ID, false)
	require.NotNil(t, job)
	select {
	case <-job.done:
	case <-time.After(8 * time.Second):
		t.Fatal("publication deadline did not stop job")
	}
	live, err := repo.GetByID(context.Background(), a.ID)
	require.NoError(t, err)
	require.Nil(t, live.Extra[openAICodexTicketExtraKey(openAICodexTicketDefaultModel)])
	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Zero(t, repo.mutations, "expired publication must stop before repository write")
}
