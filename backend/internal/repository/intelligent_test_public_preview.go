package repository

import (
	"context"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

const (
	publicPelicanPreviewBatchSize   = 16
	publicPelicanPreviewBatchBytes  = 2 << 20
	publicPelicanPreviewSourceBytes = 512 << 10
	publicPelicanPreviewImageBytes  = 128 << 10
)

// Old results may have no derived image or image_state. Cache only the safe
// preview and its rendering state, leaving the historical judgment and source
// untouched. Modern ready images require no work, and an invalid old source is
// marked unavailable once per renderer version so fixed renderers can recover
// old drawings without reparsing failures on every request.
func (r *intelligentTestRepository) preparePublicPelicanPreviews(ctx context.Context, user, accountID, recordID int64) error {
	w := &intelligentWhere{parts: []string{publicIntelligentAccessSQL}, args: []any{user}}
	if accountID > 0 {
		w.add("t.account_id=$%d", accountID)
	}
	if recordID > 0 {
		w.add("t.id=$%d", recordID)
	}
	return r.preparePelicanPreviews(ctx, w)
}

// Accounts has already been authorized by the service. Scope admin warmup to
// the account IDs on this page; unpublished legacy pictures need the same
// validated latest-picture fallback without scanning other accounts.
func (r *intelligentTestRepository) prepareAdminPelicanPreviews(ctx context.Context, accountIDs []int64) error {
	if len(accountIDs) == 0 {
		return nil
	}
	return r.preparePelicanPreviews(ctx, &intelligentWhere{
		parts: []string{`a.deleted_at IS NULL`, `t.account_id=ANY($1)`},
		args:  []any{pq.Array(accountIDs)},
	})
}

func (r *intelligentTestRepository) preparePelicanPreviews(ctx context.Context, w *intelligentWhere) error {
	w.parts = append(w.parts,
		`t.test_type='pelican' AND t.status IN ('completed','success','suspected_degradation')`,
		`COALESCE(t.evaluation->>'image_state','') NOT IN ('ready','sanitized')`,
		fmt.Sprintf(`(COALESCE(t.evaluation->>'image_state','')<>'unavailable' OR COALESCE(t.evaluation->>'image_preview_version','')<>'%d')`, service.IntelligentSVGPreviewVersion),
	)
	// Both the number of candidate rows and the bytes returned from PostgreSQL
	// are bounded. Oversized historical sources are classified without copying
	// their entire contents into the request handler. Do not hold row locks
	// while parsing XML; xmin below guards against concurrent result changes.
	query := fmt.Sprintf(`WITH candidates AS (
SELECT t.id,t.xmin::text AS revision,
CASE WHEN t.result_image<>'' THEN
  CASE WHEN OCTET_LENGTH(t.result_image)<=%d THEN t.result_image ELSE '' END
ELSE CASE WHEN OCTET_LENGTH(t.result)<=%d THEN t.result ELSE '' END END AS source
FROM account_tests t JOIN accounts a ON a.id=t.account_id JOIN test_settings s ON s.test_type=t.test_type
WHERE %s ORDER BY t.id DESC LIMIT %d),
bounded AS (SELECT id,revision,source,SUM(OCTET_LENGTH(source)) OVER (ORDER BY id DESC ROWS UNBOUNDED PRECEDING) AS bytes FROM candidates)
SELECT id,revision,source FROM bounded WHERE bytes<=%d ORDER BY id DESC`, publicPelicanPreviewImageBytes, publicPelicanPreviewSourceBytes, w.sql(), publicPelicanPreviewBatchSize, publicPelicanPreviewBatchBytes)
	rows, err := r.db.QueryContext(ctx, query, w.args...)
	if err != nil {
		return err
	}
	type candidate struct {
		id               int64
		revision, source string
	}
	candidates := make([]candidate, 0, publicPelicanPreviewBatchSize)
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.id, &item.revision, &item.source); err != nil {
			rows.Close()
			return err
		}
		candidates = append(candidates, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, item := range candidates {
		preview, adjusted, previewErr := service.PrepareIntelligentSVGPreview(item.source)
		state := "ready"
		if previewErr != nil {
			preview, state = "", "unavailable"
		} else if adjusted {
			state = "sanitized"
		}
		// The revision and current access checks make this a conditional cache
		// fill: another worker, re-evaluation or publication change wins without
		// losing any updated source, score, status or unrelated evaluation keys.
		n := len(w.args)
		update := fmt.Sprintf(`UPDATE account_tests t SET result_image=$%d,
evaluation=jsonb_set(jsonb_set(CASE WHEN jsonb_typeof(t.evaluation)='object' THEN t.evaluation ELSE '{}'::jsonb END,'{image_state}',to_jsonb($%d::text),true),'{image_preview_version}',to_jsonb($%d::int),true)
WHERE t.id=$%d AND t.xmin::text=$%d
AND EXISTS(SELECT 1 FROM accounts a JOIN test_settings s ON s.test_type=t.test_type WHERE a.id=t.account_id AND %s)`, n+2, n+3, n+5, n+1, n+4, w.sql())
		args := append(append([]any{}, w.args...), item.id, preview, state, item.revision, service.IntelligentSVGPreviewVersion)
		_, err := r.db.ExecContext(ctx, update, args...)
		if err != nil {
			return err
		}
	}
	return nil
}
