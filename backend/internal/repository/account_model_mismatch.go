package repository

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

const accountModelMismatchRawEvidenceSQL = "COALESCE(jsonb_typeof(extra -> 'model_mismatch') = 'object' AND extra -> 'model_mismatch' <> '{}'::jsonb, false)"
const accountModelMismatchEvidenceSQL = "(" + accountModelMismatchRawEvidenceSQL + " AND COALESCE(lower(btrim(extra #>> '{model_mismatch,expected_model}'))='gpt-6-astra' AND lower(btrim(extra #>> '{model_mismatch,actual_model}'))='gpt-5.6-luna', false))"
const accountModelMismatchSQL = "(" + accountModelMismatchEvidenceSQL + " AND COALESCE(extra #> '{model_mismatch,quarantined}' <> 'false'::jsonb, true))"

func accountModelMismatchEvidenceSQLFor(extraColumn string) string {
	return strings.ReplaceAll(accountModelMismatchEvidenceSQL, "extra", extraColumn)
}

func accountModelMismatchSQLFor(extraColumn string) string {
	return strings.ReplaceAll(accountModelMismatchSQL, "extra", extraColumn)
}

func rejectQuarantinedBulkResume(ctx context.Context, exec sqlExecutor, ids []int64) error {
	rows, err := exec.QueryContext(ctx, "SELECT "+accountModelMismatchSQL+" FROM accounts WHERE id=ANY($1) AND deleted_at IS NULL ORDER BY id FOR NO KEY UPDATE", pq.Array(ids))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var quarantined bool
		if err := rows.Scan(&quarantined); err != nil {
			return err
		}
		if quarantined {
			return infraerrors.Conflict("MODEL_MISMATCH_CONFIRM_REQUIRED", "包含降智账号，请先单独确认恢复，再批量开启调度")
		}
	}
	return rows.Err()
}

func preserveModelMismatchExtraSQL(expression string) string {
	return "((" + expression + ") - 'model_mismatch') || CASE WHEN " + accountModelMismatchRawEvidenceSQL + " THEN jsonb_build_object('model_mismatch', extra -> 'model_mismatch') ELSE '{}'::jsonb END"
}

// The channel mutation lock serializes quarantine/resume with channel additions,
// removal, import and changes to the logical switch. Row locks also serialize
// ordinary account/credential edits, whose writes preserve this marker.
func (r *accountRepository) modelMismatchFamily(ctx context.Context, id int64) (int64, []int64, error) {
	if _, err := r.sql.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", accountIPChannelMutationLock); err != nil {
		return 0, nil, err
	}
	rows, err := r.sql.QueryContext(ctx, `SELECT COALESCE(c.logical_account_id,a.id) FROM accounts a LEFT JOIN account_ip_channels c ON c.account_id=a.id WHERE a.id=$1 AND a.deleted_at IS NULL`, id)
	if err != nil {
		return 0, nil, err
	}
	var rootID int64
	if !rows.Next() {
		err = rows.Err()
		rows.Close()
		if err != nil {
			return 0, nil, err
		}
		return 0, nil, service.ErrAccountNotFound
	}
	err = rows.Scan(&rootID)
	rows.Close()
	if err != nil {
		return 0, nil, err
	}
	rows, err = r.sql.QueryContext(ctx, `SELECT a.id FROM accounts a WHERE a.deleted_at IS NULL AND (a.id=$1 OR a.id IN (SELECT account_id FROM account_ip_channels WHERE logical_account_id=$1)) ORDER BY a.id FOR NO KEY UPDATE`, rootID)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var member int64
		if err = rows.Scan(&member); err != nil {
			return 0, nil, err
		}
		ids = append(ids, member)
	}
	return rootID, ids, rows.Err()
}

func (r *accountRepository) MarkAccountModelMismatch(ctx context.Context, id int64, expectedModel, actualModel, requestID string) error {
	_, err := r.RecordAccountModelMismatch(ctx, id, expectedModel, actualModel, requestID)
	return err
}

// RecordAccountModelMismatch keeps the detection visible independently of the
// scheduling policy. The returned bool reports durable quarantine, including a
// previous quarantine that must survive disabling automatic protection.
func (r *accountRepository) RecordAccountModelMismatch(ctx context.Context, id int64, expectedModel, actualModel, requestID string, gates ...func([]int64)) (quarantined bool, recordErr error) {
	expectedModel, actualModel = strings.TrimSpace(expectedModel), strings.TrimSpace(actualModel)
	if !service.IsAccountModelDegradation(expectedModel, actualModel) {
		return false, nil
	}
	gate := func(ids []int64) {
		for _, callback := range gates {
			if callback != nil {
				callback(ids)
			}
		}
	}
	policyKnown := false
	// If the policy itself could not be read, retain the historic fail-closed
	// behavior. A known observation-only policy never creates a temporary gate,
	// even when recording the evidence or refreshing caches is slow or fails.
	defer func() {
		if recordErr != nil && !policyKnown {
			gate([]int64{id})
		}
	}()
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	txctx := dbent.NewTxContext(ctx, tx)
	repo := newAccountRepositoryWithSQL(tx.Client(), tx.Client(), nil)
	// Read the saved setting for every detection (no per-process policy cache).
	// A settings/database failure rolls back and retains the caller's temporary
	// protection until retry; it must never silently disable the default guard.
	settings, err := NewSettingRepository(tx.Client()).GetMultiple(txctx, []string{service.SettingKeyOpenAIModelMismatchAutoQuarantineEnabled})
	if err != nil {
		return false, err
	}
	policyKnown = true
	quarantine := service.ModelMismatchAutoQuarantineEnabled(settings[service.SettingKeyOpenAIModelMismatchAutoQuarantineEnabled])
	rootID, ids, err := repo.modelMismatchFamily(txctx, id)
	if err != nil {
		return false, err
	}
	source, err := repo.GetByID(txctx, id)
	if err != nil {
		return false, err
	}
	// modelMismatchFamily holds the source and sibling row locks. A stale
	// inference/probe must not write evidence or create a pending gate for a
	// credential generation that was replaced while it waited for those locks.
	if !service.ModelMismatchCredentialExpectationMatches(ctx, source) {
		return false, nil
	}
	if quarantine {
		gate([]int64{id})
	}
	accounts, err := repo.GetByIDs(txctx, ids)
	if err != nil {
		return false, err
	}
	for _, account := range accounts {
		quarantine = quarantine || account.IsModelMismatchQuarantined()
	}
	if quarantine {
		gate(ids)
	}
	marker, err := json.Marshal(map[string]any{
		"expected_model": expectedModel, "actual_model": actualModel,
		"request_id": requestID, "detected_at": time.Now().UTC().Format(time.RFC3339Nano),
		"source_account_id": id, "proxy_id": source.ProxyID,
		"quarantined": quarantine,
	})
	if err != nil {
		return false, err
	}
	_, err = repo.sql.ExecContext(txctx, `UPDATE accounts SET extra=jsonb_set(COALESCE(extra,'{}'::jsonb),'{model_mismatch}', CASE WHEN `+accountModelMismatchSQL+` THEN extra->'model_mismatch' ELSE $2::jsonb END,true),schedulable=CASE WHEN $3 THEN false ELSE schedulable END,updated_at=GREATEST(clock_timestamp(),updated_at+interval '1 microsecond') WHERE id=ANY($1) AND deleted_at IS NULL`, pq.Array(ids), string(marker), quarantine)
	if err != nil {
		return false, err
	}
	if quarantine {
		if _, err = repo.sql.ExecContext(txctx, `UPDATE account_ip_logical_accounts SET enabled=false WHERE account_id=$1`, rootID); err != nil {
			return false, err
		}
	}
	if err = enqueueSchedulerOutbox(txctx, repo.sql, service.SchedulerOutboxEventAccountBulkChanged, nil, nil, map[string]any{"account_ids": ids}); err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	propagationCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	r.syncSchedulerAccountSnapshots(propagationCtx, ids)
	return quarantine, nil
}

// ResumeAccountModelMismatch is only called by the administrator's explicit
// schedulable=true action. Generic SetSchedulable and editing cannot clear it.
func (r *accountRepository) ResumeAccountModelMismatch(ctx context.Context, id int64) (bool, error) {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return true, err
	}
	defer func() { _ = tx.Rollback() }()
	txctx := dbent.NewTxContext(ctx, tx)
	repo := newAccountRepositoryWithSQL(tx.Client(), tx.Client(), nil)
	rootID, ids, err := repo.modelMismatchFamily(txctx, id)
	if err != nil {
		return true, err
	}
	clearPending := service.CapturePendingAccountModelMismatchReset(ids...)
	accounts, err := repo.GetByIDs(txctx, ids)
	if err != nil {
		return true, err
	}
	quarantined := false
	for _, a := range accounts {
		quarantined = quarantined || a.IsModelMismatchQuarantined() || a.IsLegacyModelMismatchQuarantined()
	}
	if !quarantined {
		if service.ModelMismatchResumeConfirmed(ctx) && rootID == id {
			if err = tx.Commit(); err != nil {
				return true, err
			}
			clearPending()
		}
		return false, nil
	}
	if !service.ModelMismatchResumeConfirmed(ctx) {
		return true, infraerrors.Conflict("MODEL_MISMATCH_CONFIRM_REQUIRED", "账号存在模型停调记录，请核实后单独确认恢复")
	}
	if rootID != id {
		return true, infraerrors.Conflict("MODEL_MISMATCH_USE_PARENT", "请从逻辑账号确认恢复降智账号")
	}
	if _, err = repo.sql.ExecContext(txctx, `UPDATE account_ip_logical_accounts SET enabled=true WHERE account_id=$1`, rootID); err != nil {
		return true, err
	}
	_, err = repo.sql.ExecContext(txctx, `UPDATE accounts a SET extra=COALESCE(a.extra,'{}'::jsonb)-'model_mismatch',schedulable=CASE WHEN NOT EXISTS(SELECT 1 FROM account_ip_channels c WHERE c.account_id=a.id) THEN true ELSE EXISTS(SELECT 1 FROM account_ip_channels c JOIN proxies p ON p.id=a.proxy_id WHERE c.account_id=a.id AND c.retired_at IS NULL AND c.enabled=true AND p.deleted_at IS NULL AND p.status='active' AND (p.expires_at IS NULL OR p.expires_at>NOW())) END,updated_at=GREATEST(clock_timestamp(),updated_at+interval '1 microsecond') WHERE a.id=ANY($1) AND a.deleted_at IS NULL`, pq.Array(ids))
	if err != nil {
		return true, err
	}
	if err = enqueueSchedulerOutbox(txctx, repo.sql, service.SchedulerOutboxEventAccountBulkChanged, nil, nil, map[string]any{"account_ids": ids}); err != nil {
		return true, err
	}
	if err = tx.Commit(); err != nil {
		return true, err
	}
	propagationCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	r.syncSchedulerAccountSnapshots(propagationCtx, ids)
	clearPending()
	return true, nil
}
