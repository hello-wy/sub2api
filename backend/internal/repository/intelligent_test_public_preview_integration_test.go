package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

const publicPreviewFixtureSVG = `<svg viewBox="0 0 80 80"><circle cx="40" cy="40" r="30" fill="orange"/></svg>`

func TestIntelligentPublicPreviewInvalidLegacyFallsBackWithoutChangingJudgment(t *testing.T) {
	db := intelligentTestDB(t)
	repo := &intelligentTestRepository{db: db}
	ctx := context.Background()
	invalid := `<svg><text>not a drawing</text></svg>`
	_, err := db.Exec(`UPDATE test_settings SET user_visible=true WHERE test_type='pelican'`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO account_tests(id,account_id,test_type,status,score,result,input,raw_response,config_snapshot,evaluation,anti_degradation,requested_by,finished_at)
VALUES(930,10,'pelican','success',17,$1,'private prompt','original raw','{"model":"legacy-model"}','{"answer_verdict":"incorrect","legacy_note":"keep this"}',true,1,NOW()),
(931,10,'pelican','success',93,$2,'private prompt','original raw','{"model":"legacy-model"}','{"answer_verdict":"correct","legacy_note":"keep this"}',true,1,NOW()),
(932,11,'pelican','success',42,$1,'private prompt','original raw','{}','{}',true,1,NOW())`, publicPreviewFixtureSVG, invalid)
	require.NoError(t, err)
	f := service.IntelligentTestFilter{AccountID: 10, Page: 1, PageSize: 12}
	items, total, err := repo.Capabilities(ctx, 2, f)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Len(t, items[0].Tests, 1)
	require.EqualValues(t, 930, items[0].Tests[0].ID)
	require.NotEmpty(t, items[0].Tests[0].ResultImage)
	history, err := repo.PublicRecords(ctx, 2, f)
	require.NoError(t, err)
	require.EqualValues(t, 1, history.Total)
	require.Len(t, history.Items, 1)
	require.EqualValues(t, 930, history.Items[0].ID)
	_, err = repo.PublicGet(ctx, 2, 931)
	require.ErrorIs(t, err, service.ErrIntelligentTestNotFound)
	_, err = repo.PublicGet(ctx, 2, 932)
	require.ErrorIs(t, err, service.ErrIntelligentTestNotFound)

	var status, result, raw, input, image string
	var score float64
	var cfg, evaluation []byte
	require.NoError(t, db.QueryRow(`SELECT status,score,result,raw_response,input,config_snapshot,evaluation,result_image FROM account_tests WHERE id=931`).Scan(&status, &score, &result, &raw, &input, &cfg, &evaluation, &image))
	require.Equal(t, "success", status)
	require.Equal(t, 93.0, score)
	require.Equal(t, invalid, result)
	require.Equal(t, "original raw", raw)
	require.Equal(t, "private prompt", input)
	require.JSONEq(t, `{"model":"legacy-model"}`, string(cfg))
	require.JSONEq(t, `{"answer_verdict":"correct","legacy_note":"keep this","image_state":"unavailable","image_preview_version":2}`, string(evaluation))
	require.Empty(t, image)
	require.NoError(t, db.QueryRow(`SELECT evaluation FROM account_tests WHERE id=932`).Scan(&evaluation))
	require.JSONEq(t, `{}`, string(evaluation), "public warmup cannot mutate an account outside the user's current group scope")

	var revision string
	require.NoError(t, db.QueryRow(`SELECT xmin::text FROM account_tests WHERE id=931`).Scan(&revision))
	_, err = repo.PublicGet(ctx, 2, 931)
	require.ErrorIs(t, err, service.ErrIntelligentTestNotFound)
	var unchanged string
	require.NoError(t, db.QueryRow(`SELECT xmin::text FROM account_tests WHERE id=931`).Scan(&unchanged))
	require.Equal(t, revision, unchanged, "invalid legacy data is classified once rather than parsed on every request")
}

func TestIntelligentAdminPreviewWarmsOnlySelectedPageAndKeepsOlderPicture(t *testing.T) {
	db := intelligentTestDB(t)
	repo := &intelligentTestRepository{db: db}
	ctx := context.Background()
	_, err := db.Exec(`INSERT INTO account_tests(id,account_id,test_type,status,result,input,config_snapshot,evaluation,anti_degradation,requested_by,finished_at)
VALUES(940,10,'pelican','success',$1,'private prompt','{}','{}',true,1,NOW()),
(941,10,'pelican','success','<svg><text>no drawing</text></svg>','private prompt','{}','{}',true,1,NOW()),
(942,11,'pelican','success',$1,'private prompt','{}','{}',true,1,NOW())`, publicPreviewFixtureSVG)
	require.NoError(t, err)
	accounts, err := repo.Accounts(ctx, service.IntelligentTestFilter{AccountID: 10, TestType: "pelican", Page: 1, PageSize: 1})
	require.NoError(t, err)
	require.Len(t, accounts.Items, 1)
	require.Len(t, accounts.Items[0].Tests, 1)
	preview := accounts.Items[0].Tests[0]
	require.EqualValues(t, 941, preview.Latest.ID)
	require.NotNil(t, preview.LatestCompleted)
	require.EqualValues(t, 940, preview.LatestCompleted.ID)
	require.NotEmpty(t, preview.LatestCompleted.ResultImage)
	_, err = repo.PublicGet(ctx, 2, 940)
	require.ErrorIs(t, err, service.ErrIntelligentTestNotFound, "admin warmup does not publish an unpublished test")
	var state string
	require.NoError(t, db.QueryRow(`SELECT COALESCE(evaluation->>'image_state','') FROM account_tests WHERE id=942`).Scan(&state))
	require.Empty(t, state, "admin warmup must not scan accounts outside the selected page")
}

func TestIntelligentPublicPreviewBatchLimitKeepsModernPictureAvailable(t *testing.T) {
	db := intelligentTestDB(t)
	repo := &intelligentTestRepository{db: db}
	ctx := context.Background()
	_, err := db.Exec(`UPDATE test_settings SET user_visible=true WHERE test_type='pelican'`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO account_tests(id,account_id,test_type,status,result,result_image,input,config_snapshot,evaluation,anti_degradation,requested_by,finished_at)
VALUES(950,10,'pelican','completed',$1,$1,'private prompt','{}','{"image_state":"ready"}',true,1,NOW())`, publicPreviewFixtureSVG)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO account_tests(id,account_id,test_type,status,result,input,config_snapshot,evaluation,anti_degradation,requested_by,finished_at)
SELECT id,10,'pelican','success','<svg><text>legacy text</text></svg>','private prompt','{}','{}',true,1,NOW()
FROM generate_series(951,967) AS id`)
	require.NoError(t, err)
	items, total, err := repo.Capabilities(ctx, 2, service.IntelligentTestFilter{Page: 1, PageSize: 24})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.EqualValues(t, 950, items[0].Tests[0].ID, "a large legacy backlog must not hide a persisted modern picture")
	var processed int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM account_tests WHERE evaluation->>'image_state'='unavailable'`).Scan(&processed))
	require.Equal(t, publicPelicanPreviewBatchSize, processed)
}

func TestIntelligentPublicPreviewAggregateByteLimit(t *testing.T) {
	db := intelligentTestDB(t)
	repo := &intelligentTestRepository{db: db}
	ctx := context.Background()
	largeSource := strings.Repeat("x", publicPelicanPreviewSourceBytes-len(publicPreviewFixtureSVG)) + publicPreviewFixtureSVG
	_, err := db.Exec(`UPDATE test_settings SET user_visible=true WHERE test_type='pelican'`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO account_tests(id,account_id,test_type,status,result,input,config_snapshot,evaluation,anti_degradation,requested_by,finished_at)
SELECT id,10,'pelican','success',$1,'private prompt','{}','{}',true,1,NOW()
FROM generate_series(970,974) AS id`, largeSource)
	require.NoError(t, err)
	_, _, err = repo.Capabilities(ctx, 2, service.IntelligentTestFilter{Page: 1, PageSize: 24})
	require.NoError(t, err)
	var processed int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM account_tests WHERE evaluation->>'image_state'='ready'`).Scan(&processed))
	require.Equal(t, publicPelicanPreviewBatchBytes/publicPelicanPreviewSourceBytes, processed)
	var evaluation []byte
	require.NoError(t, db.QueryRow(`SELECT evaluation FROM account_tests WHERE id=970`).Scan(&evaluation))
	var pending map[string]any
	require.NoError(t, json.Unmarshal(evaluation, &pending))
	require.NotContains(t, pending, "image_state", "the oldest record is left for a subsequent bounded request")
}

func TestIntelligentPublicPreviewHandlesNonObjectLegacyEvaluation(t *testing.T) {
	db := intelligentTestDB(t)
	repo := &intelligentTestRepository{db: db}
	ctx := context.Background()
	_, err := db.Exec(`UPDATE test_settings SET user_visible=true WHERE test_type='pelican'`)
	require.NoError(t, err)
	for i, evaluation := range []string{`null`, `[]`, `"legacy"`} {
		id := int64(980 + i)
		_, err := db.Exec(`INSERT INTO account_tests(id,account_id,test_type,status,score,result,input,config_snapshot,evaluation,anti_degradation,requested_by,finished_at)
VALUES($1,10,'pelican','success',23,$2,'private prompt','{}',$3::jsonb,true,1,NOW())`, id, publicPreviewFixtureSVG, evaluation)
		require.NoError(t, err)
		picture, err := repo.PublicGet(ctx, 2, id)
		require.NoError(t, err)
		require.NotEmpty(t, picture.ResultImage)
		var score float64
		var result, status, state string
		require.NoError(t, db.QueryRow(`SELECT score,result,status,evaluation->>'image_state' FROM account_tests WHERE id=$1`, id).Scan(&score, &result, &status, &state))
		require.Equal(t, 23.0, score)
		require.Equal(t, publicPreviewFixtureSVG, result)
		require.Equal(t, "success", status)
		require.Equal(t, "ready", state)
	}
}

func TestIntelligentPublicPreviewConcurrentResultChangeWins(t *testing.T) {
	db := intelligentTestDB(t)
	repo := &intelligentTestRepository{db: db}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.ExecContext(ctx, `UPDATE test_settings SET user_visible=true WHERE test_type='pelican'`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO account_tests(id,account_id,test_type,status,score,result,input,config_snapshot,evaluation,anti_degradation,requested_by,finished_at)
VALUES(990,10,'pelican','success',23,$1,'private prompt','{}','{}',true,1,NOW())`, publicPreviewFixtureSVG)
	require.NoError(t, err)
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()
	var id, lockPID int64
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT id FROM account_tests WHERE id=990 FOR UPDATE`).Scan(&id))
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&lockPID))
	done := make(chan error, 1)
	go func() {
		_, err := repo.PublicGet(ctx, 2, 990)
		done <- err
	}()
	// The public read can inspect the old committed source, but its cache
	// update waits for the row lock held by a concurrent re-evaluation.
	require.Eventually(t, func() bool {
		var blocked bool
		err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, lockPID).Scan(&blocked)
		return err == nil && blocked
	}, 2*time.Second, 10*time.Millisecond)
	newSource := `<svg><text>updated answer has no drawing</text></svg>`
	_, err = tx.ExecContext(ctx, `UPDATE account_tests SET score=99,result=$1,evaluation='{"review":"new judgment"}' WHERE id=990`, newSource)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	require.ErrorIs(t, <-done, service.ErrIntelligentTestNotFound)
	var score float64
	var result, image string
	var evaluation []byte
	require.NoError(t, db.QueryRowContext(ctx, `SELECT score,result,result_image,evaluation FROM account_tests WHERE id=990`).Scan(&score, &result, &image, &evaluation))
	require.Equal(t, 99.0, score)
	require.Equal(t, newSource, result)
	require.Empty(t, image, "the preview derived from the stale source cannot overwrite a concurrent update")
	require.JSONEq(t, `{"review":"new judgment"}`, string(evaluation))
}

func TestIntelligentPublicPreviewRetriesOldUnavailableOnce(t *testing.T) {
	db := intelligentTestDB(t)
	repo := &intelligentTestRepository{db: db}
	ctx := context.Background()
	_, err := db.Exec(`UPDATE test_settings SET user_visible=true WHERE test_type='pelican'`)
	require.NoError(t, err)
	const pixels = `<svg viewBox="0 0 100 100"><circle cx="50" cy="50" r="40px"/></svg>`
	const implicit = `<svg viewBox="0 0 100 100"><path d="M0 0 100 0 100 100 0 100z"/></svg>`
	_, err = db.Exec(`INSERT INTO account_tests(id,account_id,test_type,status,score,result,input,config_snapshot,evaluation,anti_degradation,requested_by,finished_at)
VALUES(1000,10,'pelican','completed',17,$1,'private prompt','{}','{"image_state":"unavailable","legacy_note":"keep"}',true,1,NOW()),
(1001,10,'pelican','completed',18,$2,'private prompt','{}','{"image_state":"unavailable","image_preview_version":1}',true,1,NOW()),
(1002,10,'pelican','completed',19,'<svg><text>no drawing</text></svg>','private prompt','{}','{"image_state":"unavailable"}',true,1,NOW()),
(1003,11,'pelican','completed',20,$1,'private prompt','{}','{"image_state":"unavailable"}',true,1,NOW())`, pixels, implicit)
	require.NoError(t, err)
	for id, source := range map[int64]string{1000: pixels, 1001: implicit} {
		picture, err := repo.PublicGet(ctx, 2, id)
		require.NoError(t, err)
		require.NotEmpty(t, picture.ResultImage)
		var original, state string
		var version int
		require.NoError(t, db.QueryRow(`SELECT result,evaluation->>'image_state',(evaluation->>'image_preview_version')::int FROM account_tests WHERE id=$1`, id).Scan(&original, &state, &version))
		require.Equal(t, source, original)
		require.Equal(t, "ready", state)
		require.Equal(t, service.IntelligentSVGPreviewVersion, version)
	}
	var score float64
	var note string
	require.NoError(t, db.QueryRow(`SELECT score,evaluation->>'legacy_note' FROM account_tests WHERE id=1000`).Scan(&score, &note))
	require.Equal(t, 17.0, score)
	require.Equal(t, "keep", note)
	_, err = repo.PublicGet(ctx, 2, 1002)
	require.ErrorIs(t, err, service.ErrIntelligentTestNotFound)
	var revision, unchanged string
	var version int
	require.NoError(t, db.QueryRow(`SELECT xmin::text,(evaluation->>'image_preview_version')::int FROM account_tests WHERE id=1002`).Scan(&revision, &version))
	require.Equal(t, service.IntelligentSVGPreviewVersion, version)
	_, err = repo.PublicGet(ctx, 2, 1002)
	require.ErrorIs(t, err, service.ErrIntelligentTestNotFound)
	require.NoError(t, db.QueryRow(`SELECT xmin::text FROM account_tests WHERE id=1002`).Scan(&unchanged))
	require.Equal(t, revision, unchanged, "an unavailable drawing is attempted at most once per renderer version")
	_, err = repo.PublicGet(ctx, 2, 1003)
	require.ErrorIs(t, err, service.ErrIntelligentTestNotFound)
	var evaluation []byte
	require.NoError(t, db.QueryRow(`SELECT evaluation FROM account_tests WHERE id=1003`).Scan(&evaluation))
	require.JSONEq(t, `{"image_state":"unavailable"}`, string(evaluation), "the cache retry still respects current account visibility")
}

func TestIntelligentPublicPreviewOldUnavailableRetryRemainsBounded(t *testing.T) {
	db := intelligentTestDB(t)
	repo := &intelligentTestRepository{db: db}
	ctx := context.Background()
	_, err := db.Exec(`UPDATE test_settings SET user_visible=true WHERE test_type='pelican'`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO account_tests(id,account_id,test_type,status,result,result_image,input,config_snapshot,evaluation,anti_degradation,requested_by,finished_at)
VALUES(1100,10,'pelican','completed',$1,$1,'private prompt','{}','{"image_state":"ready"}',true,1,NOW())`, publicPreviewFixtureSVG)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO account_tests(id,account_id,test_type,status,result,input,config_snapshot,evaluation,anti_degradation,requested_by,finished_at)
SELECT id,10,'pelican','completed','<svg><text>still empty</text></svg>','private prompt','{}','{"image_state":"unavailable"}',true,1,NOW()
FROM generate_series(1101,1117) AS id`)
	require.NoError(t, err)
	for pass, want := range []int{publicPelicanPreviewBatchSize, 17, 17} {
		items, total, err := repo.Capabilities(ctx, 2, service.IntelligentTestFilter{Page: 1, PageSize: 24})
		require.NoError(t, err)
		require.EqualValues(t, 1, total)
		require.Len(t, items, 1)
		require.EqualValues(t, 1100, items[0].Tests[0].ID, "existing picture survives the retry backlog")
		var processed int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM account_tests WHERE evaluation->>'image_preview_version'=$1`, fmt.Sprint(service.IntelligentSVGPreviewVersion)).Scan(&processed))
		require.Equal(t, want, processed, "pass %d", pass)
	}
}
