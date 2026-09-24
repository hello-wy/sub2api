package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// This predicate is shared by list, detail, history and image reads. Visibility
// is checked from current settings at request time, never from a cached snapshot.
// Subscription access follows APIKeyService.canUserBindGroupInternal: an active
// subscription grants access without an additional standard-group allowlist entry.
const publicIntelligentAccessSQL = `a.deleted_at IS NULL AND s.user_visible
 AND EXISTS(SELECT 1 FROM users u WHERE u.id=$1 AND u.status='active' AND u.deleted_at IS NULL
 AND EXISTS(SELECT 1 FROM account_groups ag JOIN groups g ON g.id=ag.group_id
 WHERE ag.account_id=a.id AND g.deleted_at IS NULL AND g.status='active'
 AND ((g.subscription_type='subscription' AND EXISTS(SELECT 1 FROM user_subscriptions us WHERE us.user_id=u.id AND us.group_id=g.id AND us.status='active' AND us.expires_at>NOW() AND us.deleted_at IS NULL))
 OR (g.subscription_type<>'subscription' AND ((NOT g.is_exclusive AND NOT u.restrict_public_groups) OR EXISTS(SELECT 1 FROM user_allowed_groups ug WHERE ug.user_id=u.id AND ug.group_id=g.id))))))`

// Pelican is a gallery. A failed, pending or unrenderable attempt must not
// replace the last published picture. Legacy previews are validated in bounded
// batches before this shared predicate is used for lists, counts and detail.
const publicIntelligentResultSQL = `t.status NOT IN ('queued','running','cancelled')
 AND (t.test_type<>'pelican' OR (t.status IN ('completed','success','suspected_degradation')
 AND t.result_image<>'' AND t.evaluation->>'image_state' IN ('ready','sanitized')))`
const publicIntelligentColumns = `t.id,t.account_id,t.test_type,t.status,t.score,t.result,t.result_image,t.duration_ms,t.model,t.created_at,t.finished_at,t.evaluation`

func scanPublicIntelligent(row intelligentScanner) (*service.PublicAccountTest, error) {
	p := &service.PublicAccountTest{}
	var raw []byte
	err := row.Scan(&p.ID, &p.AccountID, &p.TestType, &p.Status, &p.Score, &p.Result, &p.ResultImage, &p.DurationMS, &p.Model, &p.CreatedAt, &p.FinishedAt, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrIntelligentTestNotFound
	}
	if err == nil {
		var evaluation map[string]any
		if err = json.Unmarshal(raw, &evaluation); err == nil {
			p.Evaluation = service.PublicIntelligentAssessment(evaluation)
		}
	}
	if err == nil && p.TestType == "pelican" {
		// Publish only the inert picture, including historical answers wrapped
		// in HTML/Markdown. Do not expose HTML, private explanatory output or
		// historical success/failure judgments as a picture-quality verdict.
		source := p.ResultImage
		if source == "" {
			source = p.Result
		}
		p.ResultImage, _, _ = service.PrepareIntelligentSVGPreview(source)
		p.Result = ""
		p.Score = nil
		p.Evaluation = nil
	}
	return p, err
}
func (r *intelligentTestRepository) PublicGet(ctx context.Context, user, id int64) (*service.PublicAccountTest, error) {
	if err := r.preparePublicPelicanPreviews(ctx, user, 0, id); err != nil {
		return nil, err
	}
	return scanPublicIntelligent(r.db.QueryRowContext(ctx, `SELECT `+publicIntelligentColumns+` FROM account_tests t JOIN accounts a ON a.id=t.account_id JOIN test_settings s ON s.test_type=t.test_type WHERE `+publicIntelligentAccessSQL+` AND `+publicIntelligentResultSQL+` AND t.id=$2`, user, id))
}
func (r *intelligentTestRepository) PublicRecords(ctx context.Context, user int64, f service.IntelligentTestFilter) (*service.PublicAccountTests, error) {
	if f.TestType == "" || f.TestType == "pelican" {
		if err := r.preparePublicPelicanPreviews(ctx, user, f.AccountID, 0); err != nil {
			return nil, err
		}
	}
	out := &service.PublicAccountTests{Items: []service.PublicAccountTest{}, Page: f.Page, PageSize: f.PageSize}
	w := &intelligentWhere{parts: []string{publicIntelligentAccessSQL, publicIntelligentResultSQL}, args: []any{user}}
	if f.AccountID > 0 {
		w.add("t.account_id=$%d", f.AccountID)
	}
	if f.TestType != "" {
		w.add("t.test_type=$%d", f.TestType)
	}
	if f.Status != "" {
		w.add("t.status=$%d", f.Status)
	}
	if f.From != nil {
		w.add("t.created_at >= $%d", *f.From)
	}
	if f.To != nil {
		w.add("t.created_at <= $%d", *f.To)
	}
	base := ` FROM account_tests t JOIN accounts a ON a.id=t.account_id JOIN test_settings s ON s.test_type=t.test_type WHERE ` + w.sql()
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*)`+base, w.args...).Scan(&out.Total); err != nil {
		return nil, err
	}
	args := append(append([]any{}, w.args...), f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := r.db.QueryContext(ctx, `SELECT `+publicIntelligentColumns+base+fmt.Sprintf(` ORDER BY t.id DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		p, err := scanPublicIntelligent(rows)
		if err != nil {
			return nil, err
		}
		out.Items = append(out.Items, *p)
	}
	return out, rows.Err()
}
func (r *intelligentTestRepository) Capabilities(ctx context.Context, user int64, f service.IntelligentTestFilter) ([]service.AccountCapability, int64, error) {
	if err := r.preparePublicPelicanPreviews(ctx, user, f.AccountID, 0); err != nil {
		return nil, 0, err
	}
	out := []service.AccountCapability{}
	w := &intelligentWhere{parts: []string{publicIntelligentAccessSQL, publicIntelligentResultSQL}, args: []any{user}}
	if f.AccountID > 0 {
		w.add("a.id=$%d", f.AccountID)
	}
	base := ` FROM account_tests t JOIN accounts a ON a.id=t.account_id JOIN test_settings s ON s.test_type=t.test_type WHERE ` + w.sql()
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT a.id)`+base, w.args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args := append(append([]any{}, w.args...), f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT a.id,a.platform,a.type`+base+fmt.Sprintf(` ORDER BY a.id DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	for rows.Next() {
		var item service.AccountCapability
		if err := rows.Scan(&item.AccountID, &item.Platform, &item.AccountType); err != nil {
			_ = rows.Close()
			return nil, 0, err
		}
		item.Tests = []service.PublicAccountTest{}
		out = append(out, item)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, 0, err
	}
	for i := range out {
		query := `SELECT DISTINCT ON(t.test_type) ` + publicIntelligentColumns + base + fmt.Sprintf(` AND a.id=$%d ORDER BY t.test_type,t.id DESC`, len(w.args)+1)
		queryArgs := append(append([]any{}, w.args...), out[i].AccountID)
		tests, err := r.db.QueryContext(ctx, query, queryArgs...)
		if err != nil {
			return nil, 0, err
		}
		for tests.Next() {
			p, err := scanPublicIntelligent(tests)
			if err != nil {
				tests.Close()
				return nil, 0, err
			}
			out[i].Tests = append(out[i].Tests, *p)
		}
		err = tests.Err()
		tests.Close()
		if err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}
