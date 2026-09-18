package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	coderws "github.com/coder/websocket"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const codexDiagnosticComplete = "data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n"

type codexDiagnosticDialer struct {
	target  string
	headers http.Header
	conn    *codexDiagnosticConn
	err     error
	cancel  context.CancelFunc
}

func (d *codexDiagnosticDialer) Dial(ctx context.Context, target string, headers http.Header, proxy string) (openAIWSClientConn, int, http.Header, error) {
	d.target, d.headers = target, headers
	if d.cancel != nil {
		d.cancel()
		return nil, 0, nil, ctx.Err()
	}
	if d.err != nil {
		return nil, 403, nil, d.err
	}
	return d.conn, 0, nil, nil
}

type codexDiagnosticConn struct {
	payload  any
	messages []string
	closed   bool
}

func (c *codexDiagnosticConn) WriteJSON(_ context.Context, value any) error {
	c.payload = value
	return nil
}
func (c *codexDiagnosticConn) ReadMessage(_ context.Context) ([]byte, error) {
	if len(c.messages) == 0 {
		return nil, io.EOF
	}
	message := c.messages[0]
	c.messages = c.messages[1:]
	return []byte(message), nil
}
func (c *codexDiagnosticConn) Ping(context.Context) error { return nil }
func (c *codexDiagnosticConn) Close() error               { c.closed = true; return nil }

func codexDiagnosticResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
func runCodexDiagnostics(t *testing.T, svc *AccountTestService, account *Account, ctx context.Context) (map[string]codexCapabilityResult, string, error) {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("POST", "/admin/accounts/77/test", nil).WithContext(ctx)
	err := svc.TestAccountConnection(c, account.ID, "gpt-5.4", "", AccountTestModeGateway)
	results := map[string]codexCapabilityResult{}
	terminalCount := 0
	for _, line := range strings.Split(rec.Body.String(), "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var event struct {
			Type    string                `json:"type"`
			Data    codexCapabilityResult `json:"data"`
			Success bool                  `json:"success"`
		}
		require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event))
		if event.Type == "capability_result" {
			results[event.Data.Capability] = event.Data
		}
		if event.Type == "test_complete" {
			terminalCount++
			require.Equal(t, err == nil, event.Success)
		}
	}
	require.Equal(t, 1, terminalCount)
	require.Len(t, results, 3)
	return results, rec.Body.String(), err
}
func codexDiagnosticService(accounts ...Account) (*AccountTestService, *httpUpstreamRecorder, *codexDiagnosticDialer) {
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		codexDiagnosticResponse(200, codexDiagnosticComplete),
		codexDiagnosticResponse(200, compactProbeSSESuccessBody),
	}}
	dialer := &codexDiagnosticDialer{conn: &codexDiagnosticConn{messages: []string{`{"type":"response.completed","response":{"status":"completed"}}`}}}
	repo := &snapshotUpdateAccountRepo{stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: accounts}}
	return &AccountTestService{accountRepo: repo, httpUpstream: upstream, codexTestWSDialer: dialer}, upstream, dialer
}

func TestCodexDiagnosticsSourcesAndActualTargets(t *testing.T) {
	for _, source := range []string{"account", "parent", "official", "parent_official", "setup_token"} {
		t.Run(source, func(t *testing.T) {
			account := codexGatewayTestAccount()
			if source == "official" || source == "parent_official" {
				account.Extra = nil
			}
			if source == "setup_token" {
				account.Type = AccountTypeSetupToken
			}
			accounts := []Account{*account}
			if strings.HasPrefix(source, "parent") {
				parent := *account
				account = &Account{ID: 78, ParentAccountID: &parent.ID, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
					Extra: map[string]any{codexBaseURLExtraKey: "https://ignored.example"}}
				accounts = []Account{*account, parent}
			}
			svc, upstream, dialer := codexDiagnosticService(accounts...)
			results, output, err := runCodexDiagnostics(t, svc, account, context.Background())
			require.NoError(t, err)
			expectedSource := source
			if source == "parent_official" {
				expectedSource = "official"
			}
			if source == "setup_token" {
				expectedSource = "account"
			}
			expectedTarget := "https://relay.example/backend-api/codex/responses"
			if expectedSource == "official" {
				expectedTarget = chatgptCodexAPIURL
			}
			for _, result := range results {
				require.Equal(t, "passed", result.Status)
				require.Equal(t, expectedSource, result.Source)
			}
			require.Len(t, upstream.requests, 2)
			require.Equal(t, expectedTarget, results["http"].TargetURL)
			require.Equal(t, upstream.requests[0].URL.String(), results["http"].TargetURL)
			require.Equal(t, upstream.requests[1].URL.String(), results["compact"].TargetURL)
			require.Equal(t, dialer.target, results["websocket"].TargetURL)
			require.Equal(t, "wss"+strings.TrimPrefix(expectedTarget, "https"), dialer.target)
			require.Equal(t, 101, results["websocket"].HTTPStatus)
			require.Equal(t, "Bearer test-token", dialer.headers.Get("Authorization"))
			payload, ok := dialer.conn.payload.(map[string]any)
			require.True(t, ok)
			require.Equal(t, "response.create", payload["type"])
			require.True(t, dialer.conn.closed)
			require.NotContains(t, output, "test-token")
		})
	}
}

func TestCodexDiagnosticsFailuresAreIndependent(t *testing.T) {
	for _, scenario := range []string{"handshake", "ws_eof", "ws_failed", "compact_missing", "compact_truncated", "http_error", "http_incomplete", "http_failed_status"} {
		t.Run(scenario, func(t *testing.T) {
			account := codexGatewayTestAccount()
			svc, upstream, dialer := codexDiagnosticService(*account)
			failed := "websocket"
			switch scenario {
			case "handshake":
				dialer.err = errors.New("secret-token echoed in upstream body")
			case "ws_eof":
				dialer.conn.messages = nil
			case "ws_failed":
				dialer.conn.messages = []string{`{"type":"response.done","response":{"status":"failed"}}`}
			case "compact_missing":
				failed = "compact"
				upstream.responses[1] = codexDiagnosticResponse(200, codexDiagnosticComplete)
			case "compact_truncated":
				failed = "compact"
				upstream.responses[1] = codexDiagnosticResponse(200, "data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"compaction\",\"encrypted_content\":\"blob\"}}\n\n")
			case "http_error":
				failed = "http"
				upstream.responses[0] = codexDiagnosticResponse(502, "secret-token echoed in upstream body")
			case "http_incomplete":
				failed = "http"
				upstream.responses[0] = codexDiagnosticResponse(200, "data: {\"type\":\"response.created\"}\n\n")
			case "http_failed_status":
				failed = "http"
				upstream.responses[0] = codexDiagnosticResponse(200, "data: {\"type\":\"response.done\",\"response\":{\"status\":\"failed\"}}\n\n")
			}
			results, output, err := runCodexDiagnostics(t, svc, account, context.Background())
			require.Error(t, err)
			require.Len(t, upstream.requests, 2)
			for key, result := range results {
				if key == failed {
					require.Equal(t, "failed", result.Status)
					require.NotEmpty(t, result.Code)
				} else {
					require.Equal(t, "passed", result.Status)
				}
			}
			require.NotContains(t, output, "secret-token")
		})
	}
}

func TestCodexDiagnosticsCancellationRetainsCompletedResult(t *testing.T) {
	account := codexGatewayTestAccount()
	svc, upstream, dialer := codexDiagnosticService(*account)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dialer.cancel = cancel
	results, _, err := runCodexDiagnostics(t, svc, account, ctx)
	require.Error(t, err)
	require.Equal(t, "passed", results["http"].Status)
	require.Equal(t, "cancelled", results["websocket"].Status)
	require.Equal(t, "cancelled", results["compact"].Status)
	require.Empty(t, results["compact"].TargetURL)
	require.Len(t, upstream.requests, 1)
}

func TestCodexDiagnosticsUnobservablePluginAndInvalidConfiguration(t *testing.T) {
	for _, plugin := range []bool{true, false} {
		account := codexGatewayTestAccount()
		if !plugin {
			account.Extra[codexBaseURLExtraKey] = "https://user:secret@relay.example"
		}
		svc, upstream, dialer := codexDiagnosticService(*account)
		if plugin {
			svc.pluginManager = &PluginManager{}
			svc.pluginManager.route.Store(&pluginRoute{rolloutPercent: 100})
		}
		results, output, err := runCodexDiagnostics(t, svc, account, context.Background())
		require.Error(t, err)
		require.Empty(t, upstream.requests)
		require.Empty(t, dialer.target)
		for _, result := range results {
			require.Empty(t, result.TargetURL)
			if plugin {
				require.Equal(t, "skipped", result.Status)
				require.Equal(t, "plugin_transport", result.Code)
			} else {
				require.Equal(t, "configuration_error", result.Code)
			}
		}
		require.NotContains(t, output, "secret")
	}
}

func TestCodexDiagnosticRedirectURLIsObservedAndRedacted(t *testing.T) {
	account := codexGatewayTestAccount()
	svc, upstream, _ := codexDiagnosticService(*account)
	finalRequest, err := http.NewRequest("POST", "https://user:secret@final.example/codex/responses?token=secret#secret", nil)
	require.NoError(t, err)
	upstream.responses[0].Request = finalRequest
	results, output, err := runCodexDiagnostics(t, svc, account, context.Background())
	require.NoError(t, err)
	require.Equal(t, "https://final.example/codex/responses", results["http"].TargetURL)
	require.NotContains(t, output, "secret")
}

func TestCodexDiagnosticsWebSocketRealHandshakeRedirectAndResponse(t *testing.T) {
	// Local upstream only; this test never sends account credentials externally.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/backend-api/codex/responses" {
			http.Redirect(w, r, "/actual/responses?token=redacted", http.StatusTemporaryRedirect)
			return
		}
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		_, message, err := conn.Read(ctx)
		if err != nil {
			return
		}
		var payload map[string]any
		if json.Unmarshal(message, &payload) != nil || payload["type"] != "response.create" {
			return
		}
		_ = conn.Write(ctx, coderws.MessageText, []byte(`{"type":"response.completed","response":{"status":"completed"}}`))
		_, _, _ = conn.Read(ctx) // allow the client to finish before closing
	}))
	defer server.Close()
	account := codexGatewayTestAccount()
	account.Extra[codexBaseURLExtraKey] = server.URL + "/backend-api/codex"
	svc, _, _ := codexDiagnosticService(*account)
	svc.codexTestWSDialer = nil
	svc.cfg = &config.Config{}
	svc.cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	results, output, err := runCodexDiagnostics(t, svc, account, context.Background())
	require.NoError(t, err)
	require.Equal(t, "ws"+strings.TrimPrefix(server.URL, "http")+"/actual/responses", results["websocket"].TargetURL)
	require.Equal(t, 101, results["websocket"].HTTPStatus)
	require.NotContains(t, output, "token=redacted")
}
