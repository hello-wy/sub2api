package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"time"
)

const codexTicketHarvestLeaseTTL = 90 * time.Second

// Coordination contains only ownership and cooldown metadata, never STATE or
// proxy credentials. Redis persists it across gateway restarts and blue/green
// overlap; every mutation compares the lease owner to prevent stale unlocks.
type CodexTicketHarvestState struct {
	Acquired   bool
	Running    bool
	RetryAfter time.Time
	LastError  string
}
type CodexTicketHarvestCoordinator interface {
	ClaimCodexTicketHarvest(context.Context, int64, string, string, int, time.Duration) (CodexTicketHarvestState, error)
	RefreshCodexTicketHarvest(context.Context, int64, string, time.Duration) (bool, error)
	FinishCodexTicketHarvest(context.Context, int64, string, string, time.Duration, string) (bool, error)
	GetCodexTicketHarvestState(context.Context, int64, string) (CodexTicketHarvestState, error)
}

func codexTicketHarvestSource(account *Account, pool string) string {
	raw, _ := json.Marshal([]string{codexAccountTicketConfigOf(account).Revision, codexTicketFixedProxyFingerprint(account), pool})
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func (s *OpenAIGatewayService) codexTicketCoordinator() (CodexTicketHarvestCoordinator, error) {
	if s.cache == nil {
		// Production wiring always supplies the shared GatewayCache.
		return nil, nil
	}
	coordinator, ok := s.cache.(CodexTicketHarvestCoordinator)
	if !ok {
		return nil, errors.New("STATE shared coordination is unavailable")
	}
	return coordinator, nil
}

func refreshCodexTicketJobLease(ctx context.Context, id int64, job *codexAccountTicketJob) error {
	if job.coordinator == nil {
		return ctx.Err()
	}
	leaseCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ok, err := job.coordinator.RefreshCodexTicketHarvest(leaseCtx, id, job.owner, codexTicketHarvestLeaseTTL)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("STATE harvest lease was replaced or expired")
	}
	// A successful remote operation returned after its deadline is not a
	// usable ownership proof, even if a test/dialer discarded ctx.Err().
	return leaseCtx.Err()
}

func (s *OpenAIGatewayService) runCoordinatedCodexTicketJob(ctx context.Context, cancel context.CancelFunc, id int64, job *codexAccountTicketJob) {
	coordinator, coordErr := s.codexTicketCoordinator()
	job.coordinator = coordinator
	if coordErr != nil || coordinator != nil {
		state := CodexTicketHarvestState{}
		if coordErr == nil {
			claimCtx, end := context.WithTimeout(ctx, 5*time.Second)
			state, coordErr = coordinator.ClaimCodexTicketHarvest(claimCtx, id, job.source, job.owner, s.openAICodexTicketConfig().MaxConcurrentHarvests, codexTicketHarvestLeaseTTL)
			end()
		}
		if coordErr != nil || !state.Acquired {
			s.openaiCodexAccountMu.Lock()
			job.running = false
			job.remoteRunning = state.Running
			job.remoteUntil = time.Now().Add(30 * time.Second)
			job.retryAfter = state.RetryAfter
			job.lastError = state.LastError
			if coordErr != nil && ctx.Err() == nil {
				job.lastError = "STATE shared coordination is unavailable; acquisition paused"
				job.retryAfter = time.Now().Add(30 * time.Second)
			}
			s.openaiCodexAccountMu.Unlock()
			return
		}
	}
	renewDone := make(chan struct{})
	stopRenew := make(chan struct{})
	go func() {
		defer close(renewDone)
		ticker := time.NewTicker(codexTicketHarvestLeaseTTL / 3)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stopRenew:
				return
			case <-ticker.C:
				if refreshCodexTicketJobLease(ctx, id, job) != nil {
					cancel()
					return
				}
			}
		}
	}()
	s.runCodexAccountTicketJob(ctx, id, job)
	close(stopRenew)
	<-renewDone
	if job.coordinator != nil {
		s.openaiCodexAccountMu.Lock()
		cooldown, reason := time.Duration(0), job.lastError
		if !job.retryAfter.IsZero() && (!codexTicket429Observation(reason) || Context429Enforcement(ctx)) {
			cooldown = time.Until(job.retryAfter)
		}
		s.openaiCodexAccountMu.Unlock()
		// Release does not inherit client cancellation or server drain. A lost
		// owner cannot remove the replacement's lease or write its cooldown.
		releaseCtx, end := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer end()
		if _, err := job.coordinator.FinishCodexTicketHarvest(releaseCtx, id, job.source, job.owner, cooldown, reason); err != nil {
			// The account's local cooldown is retained. The lease still expires
			// safely, and the failed shared write is observable without secrets.
			slog.Warn("codex_ticket_coordination_finish_failed", "account_id", id)
		}
	}
}

// Detail/batch operations consult shared state. List summaries intentionally
// remain zero-extra-I/O and use the local mirror updated by the harvester.
func (s *OpenAIGatewayService) syncCodexTicketCoordinationStatus(ctx context.Context, account *Account) {
	coordinator, err := s.codexTicketCoordinator()
	if err != nil || coordinator == nil || account == nil {
		return
	}
	ac := codexAccountTicketConfigOf(account)
	if !ac.Enabled {
		return
	}
	pool := s.openAICodexTicketHarvestProxyURLContext(ctx)
	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	state, err := coordinator.GetCodexTicketHarvestState(readCtx, account.ID, codexTicketHarvestSource(account, pool))
	if err != nil {
		return
	}
	s.openaiCodexAccountMu.Lock()
	defer s.openaiCodexAccountMu.Unlock()
	job := s.openaiCodexAccountJobs[account.ID]
	if job != nil && job.running {
		return
	}
	if s.openaiCodexAccountJobs == nil {
		s.openaiCodexAccountJobs = make(map[int64]*codexAccountTicketJob)
	}
	if job == nil || job.revision != ac.Revision || job.fixedFingerprint != codexTicketFixedProxyFingerprint(account) || job.harvestProxyURL != pool {
		job = &codexAccountTicketJob{revision: ac.Revision, fixedFingerprint: codexTicketFixedProxyFingerprint(account), harvestProxyURL: pool}
		s.openaiCodexAccountJobs[account.ID] = job
	}
	job.remoteRunning = state.Running
	job.remoteUntil = time.Now().Add(30 * time.Second)
	if !Context429Enforcement(ctx) && (codexTicket429Observation(job.lastError) || codexTicket429Observation(state.LastError)) {
		job.retryAfter = time.Time{}
		if state.LastError != "" {
			job.lastError = state.LastError
		}
		return
	}
	if !state.RetryAfter.IsZero() {
		job.retryAfter = state.RetryAfter
		job.lastError = state.LastError
	}
}
