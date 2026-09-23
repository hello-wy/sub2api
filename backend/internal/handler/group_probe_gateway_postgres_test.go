//go:build unit

package handler

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type probeGatewayGroups struct{ service.GroupRepository }

func (probeGatewayGroups) GetByID(_ context.Context, id int64) (*service.Group, error) {
	return &service.Group{ID: id, Status: service.StatusActive, Platform: service.PlatformOpenAI, Hydrated: true}, nil
}

type probeGatewayUsers struct{ service.UserRepository }

type probeGatewayMonitor struct {
	service.ChannelMonitorV2Repository
}

func (probeGatewayMonitor) GetConfig(context.Context) (*service.ChannelMonitorV2Config, error) {
	return &service.ChannelMonitorV2Config{Enabled: true}, nil
}

func (probeGatewayUsers) GetByID(context.Context, int64) (*service.User, error) {
	return &service.User{ID: 1, Status: service.StatusActive, Role: service.RoleAdmin}, nil
}

func TestGroupProbeGatewayPostgresRealRegisteredRouteAndStreamingFailures(t *testing.T) {
	dsn := os.Getenv("GROUP_STATUS_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("GROUP_STATUS_POSTGRES_TEST_DSN is not set")
	}
	admin, e := sql.Open("postgres", dsn)
	require.NoError(t, e)
	defer admin.Close()
	schema := "probe_http_" + uuid.NewString()[:8]
	_, e = admin.Exec("CREATE SCHEMA " + pq.QuoteIdentifier(schema))
	require.NoError(t, e)
	defer admin.Exec("DROP SCHEMA " + pq.QuoteIdentifier(schema) + " CASCADE")
	u, e := url.Parse(dsn)
	require.NoError(t, e)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, e := sql.Open("postgres", u.String())
	require.NoError(t, e)
	defer db.Close()
	_, e = db.Exec(`CREATE TABLE groups(id bigint PRIMARY KEY);CREATE TABLE users(id bigint PRIMARY KEY);INSERT INTO groups VALUES(1),(2),(3),(4);INSERT INTO users VALUES(1);CREATE TABLE channel_monitor_v2_metrics_1m(id bigint);CREATE TABLE channel_monitor_v2_metrics_rollup(id bigint);CREATE TABLE channel_monitor_v2_watermarks(id bigint,backfill_cursor timestamptz,usage_coverage_start timestamptz,error_coverage_start timestamptz);`)
	require.NoError(t, e)
	migration, e := os.ReadFile(filepath.Join("..", "..", "migrations", "252_group_service_status.sql"))
	require.NoError(t, e)
	_, e = db.Exec(string(migration))
	require.NoError(t, e)
	migration, e = os.ReadFile(filepath.Join("..", "..", "migrations", "254_group_status_request_identity.sql"))
	require.NoError(t, e)
	_, e = db.Exec(string(migration))
	require.NoError(t, e)
	s := service.NewGroupStatusService(service.NewChannelMonitorV2Service(probeGatewayMonitor{}), db, probeGatewayGroups{}, probeGatewayUsers{}, nil, nil, nil)
	defer s.Stop()
	gateway := gin.New()
	auth := middleware.NewAPIKeyAuthMiddleware(nil, nil, &config.Config{})
	gateway.POST("/v1/chat/completions", gin.HandlerFunc(auth), middleware.GroupModelAllowlist(), func(c *gin.Context) {
		key, ok := middleware.GetAPIKeyFromContext(c)
		require.True(t, ok)
		require.NotNil(t, service.GroupProbeKeyFromContext(c.Request.Context()))
		// A nil ordinary concurrency service is deliberate: admitted probes
		// must never access the administrator's Redis user slots or wait queue.
		helper := &ConcurrencyHelper{}
		streamStarted := false
		release, err := helper.AcquireUserSlotWithWait(c, key.UserID, 1, true, &streamStarted)
		require.NoError(t, err)
		defer release()
		body, e := io.ReadAll(c.Request.Body)
		require.NoError(t, e)
		require.Equal(t, int64(64), gjson.GetBytes(body, "max_completion_tokens").Int())
		require.False(t, gjson.GetBytes(body, "max_tokens").Exists())
		require.Equal(t, "low", gjson.GetBytes(body, "reasoning_effort").String())
		service.CaptureGroupProbeUsage(key, &service.UsageLog{InputTokens: 10, OutputTokens: 2, TotalCost: .003})
		c.Header("Content-Type", "text/event-stream")
		switch *key.GroupID {
		case 1:
			c.String(200, "data: {\"choices\":[{\"delta\":{\"content\":\"OK\"}}]}\n\ndata: [DONE]\n\n")
		case 2:
			c.String(200, "data: {\"choices\":[{\"delta\":{\"content\":\"OK\"}}]}\n\ndata: {\"error\":{\"message\":\"secret upstream detail\"}}\n\n")
		case 3:
			c.String(200, "data: {\"choices\":[{\"delta\":{\"content\":\"OK\"}}]}\n\n")
		case 4:
			_, _ = c.Writer.Write([]byte(strings.Repeat("x", 129<<10)))
		}
	})
	runner := NewGroupProbeGatewayRunner(gateway)
	s.SetProbeRunner(runner)
	denied := runner(context.Background(), service.GroupProbeExecution{})
	require.Equal(t, "internal_error", denied.ErrorCode)
	for _, tc := range []struct {
		id           int64
		status, code string
	}{{1, "success", ""}, {2, "failed", "stream_error"}, {3, "failed", "incomplete_response"}, {4, "failed", "output_limit"}} {
		_, e = s.SaveProbeConfig(context.Background(), service.GroupProbeConfig{GroupID: tc.id, Model: "gpt-5", ReasoningEffort: "low"}, 1)
		require.NoError(t, e)
		run, e := s.StartProbe(context.Background(), tc.id)
		require.NoError(t, e)
		require.Eventually(t, func() bool {
			var status string
			_ = db.QueryRow(`SELECT status FROM group_probe_runs WHERE id=$1`, run.ID).Scan(&status)
			return status != "running"
		}, 2*time.Second, 10*time.Millisecond)
		history, e := s.ProbeHistory(context.Background(), tc.id)
		require.NoError(t, e)
		require.Equal(t, tc.status, history[0].Status)
		require.Equal(t, tc.code, history[0].ErrorCode)
		require.Equal(t, int64(10), history[0].InputTokens)
	}
	// The completion observer sees a free request, in-band error after HTTP
	// 200, truncated stream and user-side billing rejection independently of
	// whether Ops logging is enabled. Only terminal success is green.
	normal := gin.New()
	normal.Use(middleware.ClientRequestID(), func(c *gin.Context) {
		group := int64(1)
		c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{UserID: 1, GroupID: &group, Group: &service.Group{ID: 1, Platform: service.PlatformOpenAI}})
		setOpsRequestContext(c, "gpt-5", true)
		c.Next()
	}, OpsErrorLoggerMiddleware(nil, s))
	normal.POST("/v1/chat/completions", func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream")
		switch c.GetHeader("X-Test-Case") {
		case "success":
			c.String(200, "data: {\"choices\":[{\"delta\":{\"content\":\"OK\"}}]}\n\ndata: [DONE]\n\n")
		case "error":
			c.String(200, "data: {\"error\":{\"type\":\"server_error\",\"message\":\"overload\"}}\n\ndata: [DONE]\n\n")
		case "incomplete":
			c.String(200, "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n")
		case "balance":
			c.JSON(402, gin.H{"error": gin.H{"type": "billing_error", "code": "INSUFFICIENT_BALANCE", "message": "insufficient balance"}})
		case "cancel", "cancel-error":
			ctx, cancel := context.WithCancel(c.Request.Context())
			c.Request = c.Request.WithContext(ctx)
			if c.GetHeader("X-Test-Case") == "cancel-error" {
				c.String(200, "data: {\"error\":{\"type\":\"server_error\",\"message\":\"upstream overloaded\"}}\n\n")
			} else {
				c.String(200, "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n")
			}
			cancel()
		case "deadline":
			ctx, cancel := context.WithDeadline(c.Request.Context(), time.Now().Add(-time.Second))
			defer cancel()
			c.Request = c.Request.WithContext(ctx)
			c.String(200, "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n")
		}
	})
	for _, kind := range []string{"success", "error", "incomplete", "balance", "cancel", "cancel-error", "deadline"} {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		req.Header.Set("X-Test-Case", kind)
		normal.ServeHTTP(httptest.NewRecorder(), req)
	}
	require.Eventually(t, func() bool {
		var n int
		_ = db.QueryRow(`SELECT COUNT(*) FROM channel_monitor_request_outcomes`).Scan(&n)
		return n == 7
	}, time.Second, 10*time.Millisecond)
	var successes, clientErrors int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FILTER(WHERE success),COUNT(*) FILTER(WHERE error_category='invalid_request') FROM channel_monitor_request_outcomes`).Scan(&successes, &clientErrors))
	require.Equal(t, 1, successes)
	require.Equal(t, 1, clientErrors)
	var cancelled, timedOut int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FILTER(WHERE error_category='client_cancelled'),COUNT(*) FILTER(WHERE error_category='timeout') FROM channel_monitor_request_outcomes`).Scan(&cancelled, &timedOut))
	require.Equal(t, 1, cancelled, "a real upstream failure remains a failure even if the client then disconnects")
	require.Equal(t, 1, timedOut)
	for _, path := range []string{"/v1/tts", "/v1/web_search", "/v1/x_search"} {
		normal.POST(path, func(c *gin.Context) {
			prefix := "grok_audio:"
			if c.Request.URL.Path == "/v1/web_search" {
				prefix = "web_search:"
			} else if c.Request.URL.Path == "/v1/x_search" {
				prefix = "x_search:"
			}
			bindGroupOutcomeBillingID(c, prefix+uuid.NewString())
			c.JSON(200, gin.H{"ok": true})
		})
		for range 2 {
			req := httptest.NewRequest(http.MethodPost, path, nil)
			req = req.WithContext(context.WithValue(req.Context(), ctxkey.ClientRequestID, "same-correlation"))
			normal.ServeHTTP(httptest.NewRecorder(), req)
		}
	}
	require.Eventually(t, func() bool {
		var n int
		_ = db.QueryRow(`SELECT COUNT(*) FROM channel_monitor_request_outcomes WHERE gateway_request_id='client:same-correlation' AND request_id<>gateway_request_id`).Scan(&n)
		return n == 6
	}, time.Second, 10*time.Millisecond)
	var xSearches int
	require.NoError(t, db.QueryRow(`SELECT COUNT(DISTINCT request_id) FROM channel_monitor_request_outcomes WHERE gateway_request_id='client:same-correlation' AND request_id LIKE 'x_search:%'`).Scan(&xSearches))
	require.Equal(t, 2, xSearches)
	// No API-key, user-balance, usage-log or ops table was needed or written by a
	// probe. The only persistent result is the bounded dedicated probe record.
}
