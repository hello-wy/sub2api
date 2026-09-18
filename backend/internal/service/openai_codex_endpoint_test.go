package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func codexGatewayTestAccount() *Account {
	return &Account{ID: 77, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "test-token"},
		Extra:       map[string]any{codexBaseURLExtraKey: "https://relay.example/backend-api/codex"}}
}

func TestCodexGatewayHTTPRouting(t *testing.T) {
	account := codexGatewayTestAccount()
	upstream := &httpUpstreamRecorder{}
	gateway := &OpenAIGatewayService{httpUpstream: upstream}
	for _, suffix := range []string{"/responses", "/responses/compact", "/models?client_version=1.0", "/images/generations", "/images/edits", "/alpha/search", "/realtime/calls?intent=quicksilver&architecture=avas"} {
		t.Run(suffix, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, "https://chatgpt.com/backend-api/codex"+suffix, strings.NewReader(`{"input":"hello"}`))
			require.NoError(t, err)
			req.Host = "chatgpt.com"
			req.Header.Set("Authorization", "Bearer test-token")
			req.Header.Set("ChatGPT-Account-ID", "test-account")
			_, err = gateway.doOpenAIUpstream(req, "socks5://proxy.example:1080", account)
			require.NoError(t, err)
			require.Equal(t, "https://relay.example/backend-api/codex"+suffix, upstream.lastReq.URL.String())
			require.Equal(t, "relay.example", upstream.lastReq.Host)
			require.Equal(t, "Bearer test-token", upstream.lastReq.Header.Get("Authorization"))
			require.Equal(t, "test-account", upstream.lastReq.Header.Get("ChatGPT-Account-ID"))
			require.Equal(t, "socks5://proxy.example:1080", upstream.lastProxyURL)
			require.JSONEq(t, `{"input":"hello"}`, string(upstream.lastBody))
		})
	}
	for _, original := range []string{"https://auth.openai.com/oauth/token", "https://chatgpt.com/backend-api/wham/usage", "https://chatgpt.com/backend-api/subscriptions", "https://chatgpt.com/backend-api/codex-other/responses", "https://other.example/backend-api/codex/responses"} {
		target, err := rewriteCodexEndpoint(account, nil, original)
		require.NoError(t, err)
		require.Equal(t, original, target)
	}
	account.Type = AccountTypeAPIKey
	target, err := rewriteCodexEndpoint(account, nil, chatgptCodexURL)
	require.NoError(t, err)
	require.Equal(t, chatgptCodexURL, target)
	account.Type = AccountTypeOAuth
	delete(account.Extra, codexBaseURLExtraKey)
	target, err = rewriteCodexEndpoint(account, nil, chatgptCodexURL)
	require.NoError(t, err)
	require.Equal(t, chatgptCodexURL, target)
}

func TestCodexGatewayRequestBuildersAndAdminTransports(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", nil)
	account := codexGatewayTestAccount()
	upstream := &httpUpstreamRecorder{}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	for _, passthrough := range []bool{false, true} {
		var req *http.Request
		var err error
		if passthrough {
			req, err = svc.buildUpstreamRequestOpenAIPassthrough(context.Background(), c, account, []byte(`{"model":"gpt-5"}`), "test-token")
		} else {
			req, err = svc.buildUpstreamRequest(context.Background(), c, account, []byte(`{"model":"gpt-5"}`), "test-token", false, "", false)
		}
		require.NoError(t, err)
		_, err = svc.doOpenAIUpstream(req, "", account)
		require.NoError(t, err)
		require.Equal(t, "https://relay.example/backend-api/codex/responses/compact", upstream.lastReq.URL.String())
	}
	tester := &AccountTestService{httpUpstream: upstream}
	req, err := tester.buildOpenAIOAuthUpstreamModelsRequest(context.Background(), account)
	require.NoError(t, err)
	_, err = tester.doUpstreamModelsRequest(req, "", account)
	require.NoError(t, err)
	require.Equal(t, "relay.example", upstream.lastReq.URL.Host)
	require.Equal(t, "/backend-api/codex/models", upstream.lastReq.URL.Path)
	req, _ = http.NewRequest(http.MethodPost, chatgptCodexURL, nil)
	_, err = tester.doOpenAIAccountTestUpstream(req, "", account, false)
	require.NoError(t, err)
	require.Equal(t, "relay.example", upstream.lastReq.URL.Host)
}

func TestCodexGatewayValidationFailsBeforeTransport(t *testing.T) {
	for _, raw := range []string{"http://relay.example", "https://user:secret@relay.example", "https://relay.example?token=secret", "https://relay.example#fragment", "https://relay.example/a/../b", "https://relay.example/%2e%2e/b", "https://relay.example/responses", "https://relay.example/responses/compact", "ftp://relay.example", "invalid"} {
		t.Run(raw, func(t *testing.T) {
			account := codexGatewayTestAccount()
			account.Extra[codexBaseURLExtraKey] = raw
			req, _ := http.NewRequest(http.MethodPost, chatgptCodexURL, nil)
			upstream := &httpUpstreamRecorder{}
			_, err := (&OpenAIGatewayService{httpUpstream: upstream}).doOpenAIUpstream(req, "", account)
			require.Error(t, err)
			require.Nil(t, upstream.lastReq)
		})
	}
	base, err := normalizeCodexBaseURL("  https://RELAY.example/backend-api/codex///  ", nil)
	require.NoError(t, err)
	require.Equal(t, "https://relay.example/backend-api/codex", base)
	base, err = normalizeCodexBaseURL("https://chatgpt.com/backend-api/codex/", nil)
	require.NoError(t, err)
	require.Empty(t, base)
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = true
	cfg.Security.URLAllowlist.UpstreamHosts = []string{"allowed.example"}
	_, err = normalizeCodexBaseURL("https://relay.example/backend-api/codex", cfg)
	require.Error(t, err)
}

func TestCodexGatewayShadowAndWebSocket(t *testing.T) {
	parent := codexGatewayTestAccount()
	shadow := &Account{ID: 78, Platform: PlatformOpenAI, Type: AccountTypeOAuth, ParentAccountID: &parent.ID}
	repo := newStubCredRepo(parent)
	svc := &OpenAIGatewayService{accountRepo: repo}
	got, err := svc.buildOpenAIResponsesWSURLForContext(context.Background(), shadow)
	require.NoError(t, err)
	require.Equal(t, "wss://relay.example/backend-api/codex/responses", got)
	req, _ := http.NewRequest(http.MethodPost, chatgptCodexURL, nil)
	require.NoError(t, applyCodexRequestEndpoint(req, repo, nil, shadow))
	require.Equal(t, "relay.example", req.URL.Host)
	got, err = resolveCodexEndpoint(context.Background(), repo, nil, shadow, "wss://chatgpt.com/backend-api/codex/call_123")
	require.NoError(t, err)
	require.Equal(t, "wss://relay.example/backend-api/codex/call_123", got)
	delete(parent.Extra, codexBaseURLExtraKey)
	got, err = svc.buildOpenAIResponsesWSURLForContext(context.Background(), shadow)
	require.NoError(t, err)
	require.Equal(t, "wss://chatgpt.com/backend-api/codex/responses", got)
}

func TestCodexGatewayModelsCacheSwitch(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer test-token" || r.URL.Query().Get("client_version") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[],"gateway":"` + r.URL.Path + `"}`))
	}))
	t.Cleanup(server.Close)
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	svc := &OpenAIGatewayService{cfg: cfg}
	account := codexGatewayTestAccount()
	for _, base := range []string{"/first/codex", "/second/codex"} {
		account.Extra[codexBaseURLExtraKey] = server.URL + base
		for i := 0; i < 2; i++ {
			manifest, err := svc.FetchCodexModelsManifest(context.Background(), account, "0.137.0", "")
			require.NoError(t, err)
			require.JSONEq(t, `{"models":[],"gateway":"`+base+`/models"}`, string(manifest.Body))
		}
	}
	require.Equal(t, int32(2), calls.Load(), "cache must be reused within a gateway and separated after a switch")
}

func TestCodexGatewayWebSocketPoolSwitch(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)
	pool.setClientDialerForTest(&openAIWSFakeDialer{})
	req := openAIWSAcquireRequest{Account: codexGatewayTestAccount()}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	previous := ""
	for _, endpoint := range []string{"wss://chatgpt.com/backend-api/codex/responses", "wss://relay.example/backend-api/codex/responses", "wss://another.example/codex/responses", "wss://chatgpt.com/backend-api/codex/responses"} {
		req.WSURL = endpoint
		lease, err := pool.Acquire(ctx, req)
		require.NoError(t, err)
		require.NotEqual(t, previous, lease.ConnID(), "changing gateways must establish a new connection")
		previous = lease.ConnID()
		lease.Release()
		lease, err = pool.Acquire(ctx, req)
		require.NoError(t, err)
		require.Equal(t, previous, lease.ConnID(), "unchanged gateways still reuse connections")
		lease.Release()
	}
}

type codexGatewaySettingsRepo struct {
	SettingRepository
	mu     sync.Mutex
	values map[string]string
}

func (r *codexGatewaySettingsRepo) Set(_ context.Context, key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.values[key] = value
	return nil
}

func (r *codexGatewaySettingsRepo) GetAll(context.Context) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	values := make(map[string]string, len(r.values))
	for k, v := range r.values {
		values[k] = v
	}
	return values, nil
}

type codexGatewayAccountRepo struct {
	AccountRepository
	account   *Account
	createErr error
}

func (r *codexGatewayAccountRepo) Create(_ context.Context, account *Account) error {
	if r.createErr != nil {
		return r.createErr
	}
	account.ID = 77
	r.account = account
	return nil
}

func (r *codexGatewayAccountRepo) Update(_ context.Context, account *Account) error {
	r.account = account
	return nil
}
func (r *codexGatewayAccountRepo) GetByID(context.Context, int64) (*Account, error) {
	return r.account, nil
}

func TestCodexGatewayHistorySurvivesAccountChanges(t *testing.T) {
	ctx := context.Background()
	settings := &codexGatewaySettingsRepo{values: map[string]string{"unrelated_secret": "never-return-this"}}
	repo := &codexGatewayAccountRepo{}
	svc := &adminServiceImpl{accountRepo: repo, settingService: &SettingService{settingRepo: settings}}
	input := &CreateAccountInput{Platform: PlatformOpenAI, Type: AccountTypeSetupToken, Name: "test", Concurrency: 1,
		Credentials: map[string]any{"access_token": "test"}, Extra: map[string]any{codexBaseURLExtraKey: " https://RELAY.example/backend-api/codex/ "}, SkipDefaultGroupBind: true}
	account, err := svc.CreateAccount(ctx, input)
	require.NoError(t, err)
	require.Equal(t, "https://relay.example/backend-api/codex", account.GetExtraString(codexBaseURLExtraKey))
	_, err = svc.UpdateAccount(ctx, account.ID, &UpdateAccountInput{Extra: map[string]any{codexBaseURLExtraKey: "https://second.example/codex"}})
	require.NoError(t, err)
	_, err = svc.UpdateAccount(ctx, account.ID, &UpdateAccountInput{Extra: map[string]any{}})
	require.NoError(t, err)
	require.Empty(t, repo.account.GetExtraString(codexBaseURLExtraKey))
	// A new service instance (another browser/server) reads the persisted history.
	svc = &adminServiceImpl{accountRepo: repo, settingService: &SettingService{settingRepo: settings}}
	history, err := svc.ListCodexGateways(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"https://relay.example/backend-api/codex", "https://second.example/codex"}, history)
	repo.createErr = errors.New("database unavailable")
	input.Extra = map[string]any{codexBaseURLExtraKey: "https://failed.example/codex"}
	_, err = svc.CreateAccount(ctx, input)
	require.Error(t, err)
	history, err = svc.ListCodexGateways(ctx)
	require.NoError(t, err)
	require.Len(t, history, 2)
}

func TestCodexGatewayHistoryConcurrentDeduplication(t *testing.T) {
	repo := &codexGatewaySettingsRepo{values: make(map[string]string)}
	svc := &adminServiceImpl{settingService: &SettingService{settingRepo: repo}}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			svc.rememberCodexGateway(context.Background(), codexGatewayTestAccount())
		}()
	}
	wg.Wait()
	gateways, err := svc.ListCodexGateways(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"https://relay.example/backend-api/codex"}, gateways)
}

func TestCodexGatewayRejectsUnsupportedAccounts(t *testing.T) {
	svc := &adminServiceImpl{}
	for _, account := range []*Account{
		{Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
		{Platform: PlatformAnthropic, Type: AccountTypeOAuth},
		{Platform: PlatformOpenAI, Type: AccountTypeOAuth, ParentAccountID: int64Ptr(77)},
	} {
		account.Extra = map[string]any{codexBaseURLExtraKey: "https://relay.example/codex"}
		require.Error(t, svc.normalizeAccountCodexGateway(account))
	}
}
