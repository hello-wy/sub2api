package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Ordinary imports, credential refreshes and account edits must not publish or
// erase STATE material. Only MutateOpenAICodexTicketExtra writes these keys.
func preserveCodexTicketExtraSQL(expression string) string {
	const predicate = "key IN ('codex_ticket_config','codex_ticket_watchdog','codex_harvest_proxy_url') OR left(key,18) = 'codex_turn_ticket:'"
	return "COALESCE((SELECT jsonb_object_agg(key,value) FROM jsonb_each(" + expression + ") WHERE NOT (" + predicate + ")), '{}'::jsonb) || COALESCE((SELECT jsonb_object_agg(key,value) FROM jsonb_each(COALESCE(extra,'{}'::jsonb)) WHERE " + predicate + "), '{}'::jsonb)"
}

// Mutation re-reads the configuration, ticket and fixed proxy under a row lock.
// This makes stale completions harmless even when workers run in different
// processes. The callback must only compute updates; it must not perform I/O.
func (r *accountRepository) MutateOpenAICodexTicketExtra(ctx context.Context, id int64, mutate func(*service.Account) (map[string]any, error)) error {
	if mutate == nil {
		return errors.New("STATE mutation callback is required")
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	txctx := dbent.NewTxContext(ctx, tx)
	repo := newAccountRepositoryWithSQL(tx.Client(), tx.Client(), nil)
	rows, err := tx.Client().QueryContext(txctx, "SELECT id FROM accounts WHERE id=$1 AND deleted_at IS NULL FOR NO KEY UPDATE", id)
	if err != nil {
		return err
	}
	exists := rows.Next()
	readErr := rows.Err()
	_ = rows.Close()
	if readErr != nil {
		return readErr
	}
	if !exists {
		return service.ErrAccountNotFound
	}
	account, err := repo.GetByID(txctx, id)
	if err != nil {
		return err
	}
	updates, err := mutate(account)
	if err != nil {
		return err
	}
	if len(updates) == 0 {
		return tx.Commit()
	}
	for key := range updates {
		if !service.IsOpenAICodexTicketExtraKey(key) {
			return errors.New("STATE mutation contains an unmanaged field")
		}
	}
	payload, err := json.Marshal(updates)
	if err != nil {
		return err
	}
	if _, err = tx.Client().ExecContext(txctx, "UPDATE accounts SET extra=COALESCE(extra,'{}'::jsonb)||$2::jsonb, updated_at=GREATEST(clock_timestamp(),updated_at+interval '1 microsecond') WHERE id=$1", id, string(payload)); err != nil {
		return err
	}
	if err = enqueueSchedulerOutbox(txctx, tx.Client(), service.SchedulerOutboxEventAccountChanged, &id, nil, nil); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	refreshCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	r.syncSchedulerAccountSnapshot(refreshCtx, id)
	return nil
}
