package repository

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Uses full database accounts (including fixed proxy and server-owned ticket
// metadata). Scheduler/Redis projections intentionally never contain raw STATE.
func (r *accountRepository) LoadCodexTicketSelectionAccounts(ctx context.Context, ids []int64) ([]*service.Account, error) {
	return r.GetByIDs(ctx, ids)
}

var _ service.CodexTicketSelectionRepository = (*accountRepository)(nil)
