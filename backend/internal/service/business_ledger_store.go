package service

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/lib/pq"
)

// A historical repair can regenerate many entries. Stream them through COPY
// in the projector transaction instead of one database round-trip per entry.
func storeBusinessProjection(ctx context.Context, tx *sql.Tx, entries []BusinessEntry, events []BusinessEvent) error {
	if _, err := tx.ExecContext(ctx, `CREATE TEMP TABLE business_projected_entries (LIKE business_ledger_entries INCLUDING DEFAULTS) ON COMMIT DROP; CREATE TEMP TABLE business_projected_ids(event_id BIGINT) ON COMMIT DROP`); err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, pq.CopyIn("business_projected_entries", "id", "event_id", "occurred_at", "kind", "user_id", "account_id", "group_id", "model", "plan_id", "amount_cny", "credits", "quality", "payload"))
	if err != nil {
		return err
	}
	nullableID := func(id int64) any {
		if id == 0 {
			return nil
		}
		return id
	}
	for _, entry := range entries {
		detail, err := json.Marshal(entry.Detail)
		if err != nil {
			_ = stmt.Close()
			return err
		}
		var amount any
		if entry.Amount != nil {
			amount = entry.Amount.String()
		}
		if _, err = stmt.ExecContext(ctx, entry.ID, entry.EventID, entry.At, entry.Kind, nullableID(entry.UserID), nullableID(entry.AccountID), nullableID(entry.GroupID), entry.Model, nullableID(entry.PlanID), amount, entry.Credits.String(), entry.Quality, string(detail)); err != nil {
			_ = stmt.Close()
			return err
		}
	}
	if _, err = stmt.ExecContext(ctx); err != nil {
		_ = stmt.Close()
		return err
	}
	if err = stmt.Close(); err != nil {
		return err
	}
	stmt, err = tx.PrepareContext(ctx, pq.CopyIn("business_projected_ids", "event_id"))
	if err != nil {
		return err
	}
	for _, event := range events {
		if _, err = stmt.ExecContext(ctx, event.ID); err != nil {
			_ = stmt.Close()
			return err
		}
	}
	if _, err = stmt.ExecContext(ctx); err != nil {
		_ = stmt.Close()
		return err
	}
	if err = stmt.Close(); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO business_ledger_entries SELECT * FROM business_projected_entries ON CONFLICT(id) DO NOTHING; INSERT INTO business_projection_processed SELECT event_id FROM business_projected_ids ON CONFLICT(event_id) DO NOTHING`)
	return err
}
