package service

import (
	"context"
	"errors"
	"sync"
	"time"
)

// OpenAICodexTicketMutationRepository serializes ticket changes with ordinary
// account edits and other instances. The callback runs with the account row
// locked and a freshly loaded fixed proxy. Only managed extra keys may change.
type OpenAICodexTicketMutationRepository interface {
	MutateOpenAICodexTicketExtra(context.Context, int64, func(*Account) (map[string]any, error)) error
}

var errCodexTicketSourceChanged = errors.New("STATE configuration or fixed route changed")

type codexTicketCandidateKey struct{}
type codexTicketFreshReplacementKey struct{}

func (s *OpenAIGatewayService) mutateCodexTicket(ctx context.Context, id int64, mutate func(*Account) (map[string]any, error)) error {
	if s == nil || s.accountRepo == nil {
		return errors.New("STATE repository unavailable")
	}
	if repo, ok := s.accountRepo.(OpenAICodexTicketMutationRepository); ok {
		return repo.MutateOpenAICodexTicketExtra(ctx, id, mutate)
	}
	// Small in-memory repositories used by tests still receive local serialization.
	lock, _ := s.openaiCodexTicketMutationLocks.LoadOrStore(id, &sync.Mutex{})
	mu := lock.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()
	account, err := s.accountRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	updates, err := mutate(account)
	if err != nil || len(updates) == 0 {
		return err
	}
	return s.accountRepo.UpdateExtra(ctx, id, updates)
}

func (s *OpenAIGatewayService) publishCodexTicket(ctx context.Context, ticket *openAICodexTicket, pool string) error {
	if codexTicketPublicationExpired(ctx) || !s.openAICodexTicketEnabledContext(ctx) || s.openAICodexTicketHarvestProxyURLContext(ctx) != pool {
		return errCodexTicketSourceChanged
	}
	err := s.mutateCodexTicket(ctx, ticket.AccountID, func(live *Account) (map[string]any, error) {
		if codexTicketPublicationExpired(ctx) || !codexAccountTicketEligible(live) || ticket.RuntimeFingerprint != s.codexTicketRuntimeFingerprint() || !ticket.validFor(live, codexAccountTicketConfigOf(live), time.Now()) || s.codexTicketRejectedByWatchdog(ticket) {
			return nil, errCodexTicketSourceChanged
		}
		current := parseOpenAICodexTicketFromAny(live.ID, ticket.Model, live.Extra[openAICodexTicketExtraKey(ticket.Model)])
		if candidate, _ := ctx.Value(codexTicketCandidateKey{}).(*openAICodexTicket); candidate != nil {
			// Revocation/replacement during the probe must not be resurrected by
			// the candidate's fresh proof. Its original deadline is never extended.
			freshState, _ := ctx.Value(codexTicketFreshReplacementKey{}).(string)
			freshReplacement := freshState != "" && freshState != candidate.State && freshState == ticket.State
			if !receiptForCodexTicket(candidate).matches(current) || s.codexTicketRejectedByWatchdog(candidate) || !time.Now().Before(candidate.ExpiresAt) || (!freshReplacement && ticket.ExpiresAt.After(candidate.ExpiresAt)) {
				return nil, errCodexTicketSourceChanged
			}
		}
		if current != nil && !current.CapturedAt.Before(ticket.CapturedAt) {
			return nil, errCodexTicketSourceChanged
		}
		return map[string]any{openAICodexTicketExtraKey(ticket.Model): ticket}, nil
	})
	if err == nil {
		s.openaiCodexTickets.Store(openAICodexTicketKey(ticket.AccountID, ticket.Model), ticket)
	}
	return err
}

func codexTicketPublicationExpired(ctx context.Context) bool {
	if ctx.Err() != nil {
		return true
	}
	// A suspended process can resume before the timer goroutine has run.
	deadline, ok := ctx.Deadline()
	return ok && !time.Now().Before(deadline)
}
