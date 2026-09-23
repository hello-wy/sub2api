package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func intelligentDeleteEngine(repo *intelligentTestRepository, actor int64, role string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := service.NewIntelligentTestService(repo, nil)
	adminHandler := admin.NewIntelligentTestHandler(svc)
	publicHandler := handler.NewAccountCapabilityHandler(svc)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		if actor > 0 {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: actor})
			c.Set(string(middleware.ContextKeyUserRole), role)
		}
	})
	engine.GET("/admin/records", adminHandler.Records)
	engine.GET("/admin/records/:id", adminHandler.Get)
	engine.GET("/admin/records/:id/image", adminHandler.Image)
	engine.DELETE("/admin/records/:id", adminHandler.Delete)
	engine.POST("/admin/records/delete-batch", adminHandler.DeleteBatch)
	engine.GET("/public/results/:id", publicHandler.Get)
	engine.GET("/public/results/:id/image", publicHandler.Image)
	engine.GET("/public/accounts/:account_id/history", publicHandler.History)
	return engine
}

func intelligentDeleteRequest(engine *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

func intelligentDeleteFixture(t *testing.T, db *sql.DB, id, account int64, status string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO account_tests(id,account_id,test_type,status,result_image,input,config_snapshot,anti_degradation,requested_by,finished_at)
VALUES($1,$2,'pelican',$3,'<svg viewBox="0 0 10 10"><circle cx="5" cy="5" r="4"/></svg>','private prompt','{}',true,1,NOW())`, id, account, status)
	require.NoError(t, err)
}

func intelligentDeleteCount(t *testing.T, db *sql.DB, query string, args ...any) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.QueryRow(query, args...).Scan(&count))
	return count
}

func intelligentDeleteTableSnapshot(t *testing.T, db *sql.DB, table string) string {
	t.Helper()
	// The caller passes fixed fixture table names, never request data.
	var snapshot string
	require.NoError(t, db.QueryRow(`SELECT COALESCE(jsonb_agg(to_jsonb(t) ORDER BY to_jsonb(t)::text),'[]'::jsonb)::text FROM `+table+` t`).Scan(&snapshot))
	return snapshot
}

func TestIntelligentDeleteHTTPAuthorizationAndDatabaseRevocation(t *testing.T) {
	db := intelligentTestDB(t)
	repo := &intelligentTestRepository{db: db}
	intelligentDeleteFixture(t, db, 901, 10, "completed")
	for _, route := range []struct{ method, path, body string }{
		{http.MethodDelete, "/admin/records/901", ""},
		{http.MethodPost, "/admin/records/delete-batch", `{"ids":[901]}`},
	} {
		for _, tc := range []struct {
			name, role string
			actor      int64
			status     int
		}{
			{"anonymous", "", 0, http.StatusUnauthorized},
			{"user", "user", 2, http.StatusForbidden},
			{"forged-admin-context", "admin", 2, http.StatusForbidden},
			{"missing-user", "super_admin", 999, http.StatusForbidden},
		} {
			t.Run(route.method+"/"+tc.name, func(t *testing.T) {
				w := intelligentDeleteRequest(intelligentDeleteEngine(repo, tc.actor, tc.role), route.method, route.path, route.body)
				require.Equal(t, tc.status, w.Code, w.Body.String())
				require.EqualValues(t, 1, intelligentDeleteCount(t, db, `SELECT COUNT(*) FROM account_tests WHERE id=901`))
			})
		}
	}
	engine := intelligentDeleteEngine(repo, 1, "super_admin")
	require.Equal(t, http.StatusOK, intelligentDeleteRequest(engine, http.MethodGet, "/admin/records/901", "").Code)
	for _, change := range []string{
		`UPDATE users SET role='user' WHERE id=1`,
		`UPDATE users SET role='super_admin',status='disabled' WHERE id=1`,
		`UPDATE users SET status='active',deleted_at=NOW() WHERE id=1`,
	} {
		_, err := db.Exec(change)
		require.NoError(t, err)
		for _, route := range []struct{ method, path, body string }{
			{http.MethodDelete, "/admin/records/901", ""},
			{http.MethodPost, "/admin/records/delete-batch", `{"ids":[901]}`},
		} {
			w := intelligentDeleteRequest(engine, route.method, route.path, route.body)
			require.Equal(t, http.StatusForbidden, w.Code, "previously authenticated admin must be checked against the current database role/status: %s", w.Body.String())
		}
	}
	require.EqualValues(t, 1, intelligentDeleteCount(t, db, `SELECT COUNT(*) FROM account_tests WHERE id=901`))
	require.Zero(t, intelligentDeleteCount(t, db, `SELECT COUNT(*) FROM audit_logs`))
	_, err := db.Exec(`UPDATE users SET role='admin',status='active',deleted_at=NULL WHERE id=1`)
	require.NoError(t, err)
	w := intelligentDeleteRequest(engine, http.MethodDelete, "/admin/records/901", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.JSONEq(t, `{"code":0,"message":"success","data":{"deleted":1}}`, w.Body.String())
}

func TestIntelligentDeleteHTTPRejectsInvalidBatches(t *testing.T) {
	db := intelligentTestDB(t)
	repo := &intelligentTestRepository{db: db}
	intelligentDeleteFixture(t, db, 901, 10, "completed")
	engine := intelligentDeleteEngine(repo, 1, "admin")
	ids := make([]int64, 101)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	oversized, err := json.Marshal(map[string]any{"ids": ids})
	require.NoError(t, err)
	for _, body := range []string{`{}`, `{"ids":[]}`, `{"ids":[0]}`, `{"ids":[-1]}`, `{"ids":[901,901]}`, `{"ids":["901"]}`, `{"ids":[1.5]}`, `{"ids":`, string(oversized), `{"ids":[901],"padding":"` + strings.Repeat("x", 17000) + `"}`} {
		w := intelligentDeleteRequest(engine, http.MethodPost, "/admin/records/delete-batch", body)
		require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	}
	for _, path := range []string{"/admin/records/0", "/admin/records/-1", "/admin/records/abc"} {
		w := intelligentDeleteRequest(engine, http.MethodDelete, path, "")
		require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	}
	require.EqualValues(t, 1, intelligentDeleteCount(t, db, `SELECT COUNT(*) FROM account_tests`))
	require.Zero(t, intelligentDeleteCount(t, db, `SELECT COUNT(*) FROM audit_logs`))
}

func TestIntelligentDeleteRejectsMixedActiveBatchAtomically(t *testing.T) {
	for _, active := range []string{"queued", "running"} {
		t.Run(active, func(t *testing.T) {
			db := intelligentTestDB(t)
			repo := &intelligentTestRepository{db: db}
			intelligentDeleteFixture(t, db, 901, 10, "completed")
			intelligentDeleteFixture(t, db, 902, 11, active)
			before := intelligentDeleteTableSnapshot(t, db, "account_tests")
			w := intelligentDeleteRequest(intelligentDeleteEngine(repo, 1, "admin"), http.MethodPost, "/admin/records/delete-batch", `{"ids":[901,902,999]}`)
			require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
			require.Contains(t, w.Body.String(), "INTELLIGENT_TEST_DELETE_ACTIVE")
			require.Equal(t, before, intelligentDeleteTableSnapshot(t, db, "account_tests"), "settled rows in a mixed batch must survive unchanged")
			require.Zero(t, intelligentDeleteCount(t, db, `SELECT COUNT(*) FROM audit_logs`))
		})
	}
}

func TestIntelligentDeleteRemovesPublicAndAdminAccessWithoutRebilling(t *testing.T) {
	db := intelligentTestDB(t)
	repo := &intelligentTestRepository{db: db}
	svc := service.NewIntelligentTestService(repo, nil)
	ctx := context.Background()
	request := service.IntelligentTestEnqueue{AccountIDs: []int64{10, 11}, TestTypes: []string{"pelican"}, IdempotencyKey: uuid.NewString()}
	enqueued, err := svc.Enqueue(ctx, 1, request)
	require.NoError(t, err)
	require.Len(t, enqueued.Records, 2)
	first, second := enqueued.Records[0].ID, enqueued.Records[1].ID
	_, err = db.Exec(`UPDATE account_tests SET status='completed',finished_at=NOW(),result_image='<svg viewBox="0 0 10 10"><circle cx="5" cy="5" r="4"/></svg>';
UPDATE test_settings SET user_visible=true;
CREATE TABLE payment_orders(id BIGINT PRIMARY KEY,amount NUMERIC(18,6),status TEXT);
CREATE TABLE usage_logs(id BIGINT PRIMARY KEY,account_id BIGINT,cost NUMERIC(18,6));
INSERT INTO payment_orders VALUES(1,123.450000,'paid');
INSERT INTO usage_logs VALUES(1,10,0.125000)`)
	require.NoError(t, err)
	intelligentDeleteFixture(t, db, 901, 12, "failed")
	snapshots := map[string]string{}
	for _, table := range []string{"users", "accounts", "groups", "account_groups", "user_allowed_groups", "test_settings", "intelligent_test_schedules", "intelligent_test_requests", "payment_orders", "usage_logs"} {
		snapshots[table] = intelligentDeleteTableSnapshot(t, db, table)
	}
	var originalAudit string
	require.NoError(t, db.QueryRow(`SELECT to_jsonb(a)::text FROM audit_logs a WHERE action='admin.intelligent_tests.enqueue'`).Scan(&originalAudit))
	adminEngine := intelligentDeleteEngine(repo, 1, "super_admin")
	publicEngine := intelligentDeleteEngine(repo, 2, "user")
	for _, surface := range []struct {
		engine *gin.Engine
		path   string
	}{
		{adminEngine, fmt.Sprintf("/admin/records/%d", first)},
		{adminEngine, fmt.Sprintf("/admin/records/%d/image", first)},
		{publicEngine, fmt.Sprintf("/public/results/%d", first)},
		{publicEngine, fmt.Sprintf("/public/results/%d/image", first)},
	} {
		w := intelligentDeleteRequest(surface.engine, http.MethodGet, surface.path, "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	}
	w := intelligentDeleteRequest(adminEngine, http.MethodPost, "/admin/records/delete-batch", fmt.Sprintf(`{"ids":[%d,99999]}`, first))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.JSONEq(t, `{"code":0,"message":"success","data":{"deleted":1}}`, w.Body.String())
	for _, surface := range []struct {
		engine *gin.Engine
		path   string
	}{
		{adminEngine, fmt.Sprintf("/admin/records/%d", first)},
		{adminEngine, fmt.Sprintf("/admin/records/%d/image", first)},
		{publicEngine, fmt.Sprintf("/public/results/%d", first)},
		{publicEngine, fmt.Sprintf("/public/results/%d/image", first)},
	} {
		w := intelligentDeleteRequest(surface.engine, http.MethodGet, surface.path, "")
		require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
		require.NotContains(t, w.Body.String(), "<svg")
	}
	records, err := svc.Records(ctx, 1, service.IntelligentTestFilter{AccountID: 10})
	require.NoError(t, err)
	require.Zero(t, records.Total)
	public, err := svc.PublicRecords(ctx, 2, service.IntelligentTestFilter{AccountID: 10})
	require.NoError(t, err)
	require.Zero(t, public.Total)
	capabilities, total, err := svc.Capabilities(ctx, 2, service.IntelligentTestFilter{AccountID: 10})
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, capabilities)

	// Replaying a partially deleted request may return the retained sibling,
	// but must never produce replacement work for the deleted record.
	partial, err := svc.Enqueue(ctx, 1, request)
	require.NoError(t, err)
	require.True(t, partial.Reused)
	require.Zero(t, partial.CreatedCount)
	require.Len(t, partial.Records, 1)
	require.Equal(t, second, partial.Records[0].ID)
	require.EqualValues(t, 2, intelligentDeleteCount(t, db, `SELECT COUNT(*) FROM account_tests`))
	count, err := svc.DeleteRecords(ctx, 1, []int64{first})
	require.NoError(t, err)
	require.Zero(t, count)
	require.EqualValues(t, 1, intelligentDeleteCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE action='admin.intelligent_tests.delete'`))
	count, err = svc.DeleteRecords(ctx, 1, []int64{second})
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	for i := 0; i < 2; i++ {
		replayed, err := svc.Enqueue(ctx, 1, request)
		require.NoError(t, err)
		require.True(t, replayed.Reused)
		require.Zero(t, replayed.CreatedCount)
		require.Empty(t, replayed.Records)
	}
	altered := request
	altered.AccountIDs = []int64{10}
	_, err = svc.Enqueue(ctx, 1, altered)
	require.ErrorIs(t, err, service.ErrIntelligentTestConflict)
	require.Zero(t, intelligentDeleteCount(t, db, `SELECT COUNT(*) FROM account_tests WHERE status IN ('queued','running')`))
	require.EqualValues(t, 1, intelligentDeleteCount(t, db, `SELECT COUNT(*) FROM account_tests WHERE id=901 AND status='failed'`))
	for table, before := range snapshots {
		require.Equal(t, before, intelligentDeleteTableSnapshot(t, db, table), "%s must not change when deleting test history", table)
	}
	var retainedAudit string
	require.NoError(t, db.QueryRow(`SELECT to_jsonb(a)::text FROM audit_logs a WHERE action='admin.intelligent_tests.enqueue'`).Scan(&retainedAudit))
	require.Equal(t, originalAudit, retainedAudit)
	require.EqualValues(t, 3, intelligentDeleteCount(t, db, `SELECT COUNT(*) FROM audit_logs`))
	var deletedAudit struct {
		RecordIDs []int64 `json:"record_ids"`
		Count     int     `json:"count"`
	}
	var encoded []byte
	require.NoError(t, db.QueryRow(`SELECT extra FROM audit_logs WHERE action='admin.intelligent_tests.delete' ORDER BY id LIMIT 1`).Scan(&encoded))
	require.NoError(t, json.Unmarshal(encoded, &deletedAudit))
	require.Equal(t, []int64{first}, deletedAudit.RecordIDs)
	require.Equal(t, 1, deletedAudit.Count)
}

func TestIntelligentDeleteAuditFailureRollsBack(t *testing.T) {
	db := intelligentTestDB(t)
	repo := &intelligentTestRepository{db: db}
	intelligentDeleteFixture(t, db, 901, 10, "completed")
	intelligentDeleteFixture(t, db, 902, 11, "failed")
	before := intelligentDeleteTableSnapshot(t, db, "account_tests")
	_, err := db.Exec(`ALTER TABLE audit_logs ADD CONSTRAINT fail_delete_audit CHECK(action <> 'admin.intelligent_tests.delete')`)
	require.NoError(t, err)
	count, err := service.NewIntelligentTestService(repo, nil).DeleteRecords(context.Background(), 1, []int64{901, 902})
	require.Error(t, err)
	require.Zero(t, count)
	require.Equal(t, before, intelligentDeleteTableSnapshot(t, db, "account_tests"))
	require.Zero(t, intelligentDeleteCount(t, db, `SELECT COUNT(*) FROM audit_logs`))
}
