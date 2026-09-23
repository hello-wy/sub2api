//go:build unit

package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func openAICredentialGroupIntegrationRepo(t *testing.T) (*accountRepository, *dbent.Client, context.Context) {
	t.Helper()
	dsn := os.Getenv("OPENAI_GROUP_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("OPENAI_GROUP_POSTGRES_TEST_DSN is not set")
	}
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := "openai_group_" + uuid.NewString()[:8]
	_, err = admin.Exec("CREATE SCHEMA " + pq.QuoteIdentifier(schema))
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = admin.Exec("DROP SCHEMA " + pq.QuoteIdentifier(schema) + " CASCADE"); _ = admin.Close() })
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	q := parsed.Query()
	q.Set("search_path", schema)
	parsed.RawQuery = q.Encode()
	db, err := sql.Open("postgres", parsed.String())
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	require.NoError(t, client.Schema.Create(context.Background()))
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS scheduler_outbox (id BIGSERIAL PRIMARY KEY, event_type TEXT NOT NULL, account_id BIGINT, group_id BIGINT, payload JSONB, dedup_key TEXT, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS scheduler_outbox_dedup_key ON scheduler_outbox (dedup_key) WHERE dedup_key IS NOT NULL`)
	require.NoError(t, err)
	return newAccountRepositoryWithSQL(client, db, nil), client, context.Background()
}

func createGroupTestAccount(t *testing.T, ctx context.Context, client *dbent.Client, group, rt, at string) *service.Account {
	creds := map[string]any{"refresh_token": rt, "access_token": at, "chatgpt_account_id": "chat-1", "client_id": "client-1", service.OpenAIOAuthCredentialGroupKey: group, service.OpenAIOAuthSourceFingerprintKey: service.OpenAIOAuthSourceFingerprint(map[string]any{"refresh_token": rt})}
	e, err := client.Account.Create().SetName("group-test").SetPlatform(service.PlatformOpenAI).SetType(service.AccountTypeOAuth).SetCredentials(creds).Save(ctx)
	require.NoError(t, err)
	return accountEntityToService(e)
}

func TestOpenAICredentialGroupPostgresConcurrentRotationAndLateCreate(t *testing.T) {
	repo, client, ctx := openAICredentialGroupIntegrationRepo(t)
	group := "g-" + uuid.NewString()
	accounts := []*service.Account{
		createGroupTestAccount(t, ctx, client, group, "rt-0", "at-0"),
		createGroupTestAccount(t, ctx, client, group, "rt-0", "at-0"),
	}
	// Independent repository instances have no shared Go mutex; PostgreSQL
	// must coordinate the two different account records in this group.
	repos := []*accountRepository{repo, newAccountRepositoryWithSQL(client, repo.sql, nil)}
	var calls atomic.Int64
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			durable, err := repos[i].RefreshOpenAIOAuthCredentials(ctx, accounts[i], func(_ context.Context, fresh *service.Account) (map[string]any, error) {
				calls.Add(1)
				time.Sleep(50 * time.Millisecond)
				return map[string]any{"refresh_token": "rt-1", "access_token": "at-1", service.OpenAIOAuthCredentialGroupKey: fresh.GetCredential(service.OpenAIOAuthCredentialGroupKey), service.OpenAIOAuthSourceFingerprintKey: fresh.GetCredential(service.OpenAIOAuthSourceFingerprintKey)}, nil
			})
			if err == nil && durable.GetCredential("refresh_token") != "rt-1" {
				err = errors.New("replica did not observe durable rotation")
			}
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	for range 2 {
		require.NoError(t, <-results)
	}
	require.Equal(t, int64(1), calls.Load())
	for _, account := range accounts {
		row, err := repo.GetByID(ctx, account.ID)
		require.NoError(t, err)
		require.Equal(t, "rt-1", row.GetCredential("refresh_token"))
	}
	// A replica using the pre-rotation JSON must inherit the durable group token.
	late := &service.Account{Name: "late", Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Credentials: map[string]any{"refresh_token": "rt-0", "access_token": "at-0", service.OpenAIOAuthCredentialGroupKey: group}}
	require.NoError(t, repo.Create(ctx, late))
	current, err := repo.GetByID(ctx, late.ID)
	require.NoError(t, err)
	require.Equal(t, "rt-1", current.GetCredential("refresh_token"))
	// Reimporting the old source fingerprint cannot roll the group back.
	prepared, err := repo.RegisterOpenAIOAuthCredentialGroup(ctx, nil, map[string]any{"refresh_token": "rt-0", "access_token": "at-0"})
	require.NoError(t, err)
	require.Equal(t, "rt-1", credentialString(prepared, "refresh_token"))
}

func TestOpenAICredentialGroupPostgresDifferentGroupsRefreshConcurrently(t *testing.T) {
	repo, client, ctx := openAICredentialGroupIntegrationRepo(t)
	accounts := []*service.Account{
		createGroupTestAccount(t, ctx, client, "parallel-a", "rt-a", "at-a"),
		createGroupTestAccount(t, ctx, client, "parallel-b", "rt-b", "at-b"),
	}
	entered := make(chan int64, 2)
	release := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	results := make(chan error, 2)
	for _, account := range accounts {
		go func() {
			_, err := repo.RefreshOpenAIOAuthCredentials(ctx, account, func(ctx context.Context, fresh *service.Account) (map[string]any, error) {
				entered <- fresh.ID
				select {
				case <-release:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				fresh.Credentials["refresh_token"] = fresh.GetCredential("refresh_token") + "-new"
				return fresh.Credentials, nil
			})
			results <- err
		}()
	}
	seen := map[int64]bool{}
	for range 2 {
		select {
		case id := <-entered:
			seen[id] = true
		case <-time.After(3 * time.Second):
			t.Fatal("a different group's upstream refresh was blocked")
		}
	}
	require.Len(t, seen, 2)
	releaseOnce.Do(func() { close(release) })
	for range 2 {
		require.NoError(t, <-results)
	}
}

func TestOpenAICredentialGroupPostgresDelayedUpdateWaitsAndRejectsStaleToken(t *testing.T) {
	repo, client, ctx := openAICredentialGroupIntegrationRepo(t)
	account := createGroupTestAccount(t, ctx, client, "delayed-update", "rt-old", "at-old")
	entered := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	refreshed := make(chan error, 1)
	go func() {
		_, err := repo.RefreshOpenAIOAuthCredentials(ctx, account, func(_ context.Context, fresh *service.Account) (map[string]any, error) {
			close(entered)
			<-release
			fresh.Credentials["refresh_token"], fresh.Credentials["access_token"] = "rt-new", "at-new"
			return fresh.Credentials, nil
		})
		refreshed <- err
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("refresh never entered")
	}
	updated := make(chan error, 1)
	go func() { updated <- repo.UpdateCredentials(ctx, account.ID, account.Credentials) }()
	// A row-lock wait alone would permit a pre-read stale JSON map to be written
	// after refresh. Explicitly observe an advisory wait before releasing rotation.
	require.Eventually(t, func() bool {
		rows, err := client.QueryContext(ctx, "SELECT COUNT(*) FROM pg_locks WHERE locktype = 'advisory' AND NOT granted")
		if err != nil {
			return false
		}
		defer rows.Close()
		var waiting int
		return rows.Next() && rows.Scan(&waiting) == nil && waiting > 0
	}, 3*time.Second, 10*time.Millisecond)
	releaseOnce.Do(func() { close(release) })
	require.NoError(t, <-refreshed)
	require.ErrorContains(t, <-updated, "multi-IP reauthorization/import")
	current, err := repo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, "rt-new", current.GetCredential("refresh_token"))
	// A registered import snapshot is explicitly trusted as a snapshot; it
	// updates replica configuration while retaining the group's newer token.
	account.Name = "renamed-after-registration"
	require.NoError(t, repo.Update(service.WithOpenAIRegisteredCredentialSnapshot(ctx), account))
	require.Equal(t, "rt-new", account.GetCredential("refresh_token"))
	// Non-token per-replica settings continue to be editable after rotation.
	current.Credentials["model_mapping"] = map[string]any{"local": "upstream"}
	require.NoError(t, repo.UpdateCredentials(ctx, account.ID, current.Credentials))
}

type groupIntegrationOAuthClient struct{ calls atomic.Int64 }

func (c *groupIntegrationOAuthClient) ExchangeCode(context.Context, string, string, string, string, string) (*openai.TokenResponse, error) {
	return nil, errors.New("unexpected exchange")
}
func (c *groupIntegrationOAuthClient) RefreshToken(ctx context.Context, refreshToken, proxy string) (*openai.TokenResponse, error) {
	return c.RefreshTokenWithClientID(ctx, refreshToken, proxy, "")
}
func (c *groupIntegrationOAuthClient) RefreshTokenWithClientID(context.Context, string, string, string) (*openai.TokenResponse, error) {
	i := c.calls.Add(1)
	return &openai.TokenResponse{AccessToken: fmt.Sprintf("wrapper-at-%d", i), RefreshToken: fmt.Sprintf("wrapper-rt-%d", i), ExpiresIn: 3600}, nil
}

func TestOpenAICredentialGroupPostgresRefreshWrapperAndOuterPersistence(t *testing.T) {
	repo, client, ctx := ipChannelIntegrationRepo(t)
	account := createGroupTestAccount(t, ctx, client, "wrapper", "rt-old", "at-old")
	sibling := createGroupTestAccount(t, ctx, client, "wrapper", "rt-old", "at-old")
	account.Credentials["expires_at"] = time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	_, err := client.Account.UpdateOneID(account.ID).SetCredentials(account.Credentials).Save(ctx)
	require.NoError(t, err)
	provider := &groupIntegrationOAuthClient{}
	oauth := service.NewOpenAIOAuthService(nil, provider)
	defer oauth.Stop()
	oauth.SetAccountRepository(repo)
	api := service.NewOAuthRefreshAPI(repo, nil)
	refresher := service.NewOpenAITokenRefresher(oauth, repo)
	result, err := api.RefreshIfNeeded(ctx, account, refresher, time.Minute)
	require.NoError(t, err)
	require.True(t, result.Refreshed)
	require.Equal(t, "wrapper-rt-1", result.Account.GetCredential("refresh_token"))
	for _, id := range []int64{account.ID, sibling.ID} {
		current, err := repo.GetByID(ctx, id)
		require.NoError(t, err)
		require.Equal(t, "wrapper-rt-1", current.GetCredential("refresh_token"))
	}
	// Direct/manual refresh mutates only the caller's snapshot after commit.
	manual, err := repo.GetByID(ctx, sibling.ID)
	require.NoError(t, err)
	info, err := oauth.RefreshAccountToken(ctx, manual)
	require.NoError(t, err)
	require.Equal(t, "wrapper-rt-2", info.RefreshToken)
	require.Equal(t, "wrapper-rt-2", manual.GetCredential("refresh_token"))
	// The outer wrapper's old full-document write cannot roll back this newer refresh.
	require.Error(t, repo.UpdateCredentials(ctx, result.Account.ID, result.NewCredentials))
	current, err := repo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, "wrapper-rt-2", current.GetCredential("refresh_token"))
	require.Equal(t, int64(2), provider.calls.Load())
}

func TestOpenAICredentialGroupPostgresConcurrentFirstRegistrationsUseSameGroup(t *testing.T) {
	repo, _, ctx := openAICredentialGroupIntegrationRepo(t)
	creds := map[string]any{"refresh_token": "brand-new-source", "access_token": "brand-new-access"}
	results := make(chan map[string]any, 2)
	failures := make(chan error, 2)
	for range 2 {
		go func() {
			prepared, err := repo.RegisterOpenAIOAuthCredentialGroup(ctx, nil, creds)
			results <- prepared
			failures <- err
		}()
	}
	a, b := <-results, <-results
	for range 2 {
		require.NoError(t, <-failures)
	}
	require.NotEmpty(t, a[service.OpenAIOAuthCredentialGroupKey])
	require.Equal(t, a[service.OpenAIOAuthCredentialGroupKey], b[service.OpenAIOAuthCredentialGroupKey])
}

func TestOpenAICredentialGroupPostgresRegistrationFailureRollsBack(t *testing.T) {
	repo, client, ctx := openAICredentialGroupIntegrationRepo(t)
	a := createGroupTestAccount(t, ctx, client, "g-a", "rt-a", "at-a")
	_, err := repo.RegisterOpenAIOAuthCredentialGroup(ctx, []int64{a.ID}, map[string]any{"refresh_token": "rt-new", "access_token": "at-new", "chatgpt_account_id": "other"})
	require.Error(t, err)
	fresh, err := repo.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.Equal(t, "rt-a", fresh.GetCredential("refresh_token"))
}

func TestOpenAICredentialGroupPostgresHistoricalExportsNeverUndoReauthorization(t *testing.T) {
	repo, _, ctx := openAICredentialGroupIntegrationRepo(t)
	source := func(generation string) map[string]any {
		return map[string]any{"refresh_token": "history-rt-" + generation, "access_token": "history-at-" + generation, "chatgpt_account_id": "chat-history", "chatgpt_user_id": "user-history"}
	}
	r1 := source("1")
	prepared, err := repo.RegisterOpenAIOAuthCredentialGroup(ctx, nil, r1)
	require.NoError(t, err)
	first := &service.Account{Name: "history-first", Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Status: service.StatusActive, Credentials: prepared}
	require.NoError(t, repo.Create(ctx, first))
	second := &service.Account{Name: "history-second", Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Status: service.StatusActive, Credentials: prepared}
	require.NoError(t, repo.Create(ctx, second))
	group := first.GetCredential(service.OpenAIOAuthCredentialGroupKey)
	originalFingerprint := first.GetCredential(service.OpenAIOAuthSourceFingerprintKey)

	rotate := func(generation string) {
		current, err := repo.GetByID(ctx, first.ID)
		require.NoError(t, err)
		_, err = repo.RefreshOpenAIOAuthCredentials(ctx, current, func(_ context.Context, fresh *service.Account) (map[string]any, error) {
			// Deliberately mutate the callback snapshot; consumed-generation
			// history must already have been captured before invoking us.
			fresh.Credentials["refresh_token"], fresh.Credentials["access_token"] = "history-rt-"+generation, "history-at-"+generation
			return fresh.Credentials, nil
		})
		require.NoError(t, err)
	}
	rotate("2")
	r2 := source("2")
	currentExport, err := repo.RegisterOpenAIOAuthCredentialGroup(ctx, []int64{first.ID}, r2)
	require.NoError(t, err)
	require.Equal(t, originalFingerprint, currentExport[service.OpenAIOAuthSourceFingerprintKey], "reimporting current token must retain the group's original source")
	oldExport, err := repo.RegisterOpenAIOAuthCredentialGroup(ctx, nil, r1)
	require.NoError(t, err)
	require.Equal(t, "history-rt-2", oldExport["refresh_token"])
	require.Equal(t, group, oldExport[service.OpenAIOAuthCredentialGroupKey])

	// A genuinely new authorization is allowed to replace the group's token.
	r3 := source("3")
	newAuthorization, err := repo.RegisterOpenAIOAuthCredentialGroup(ctx, []int64{first.ID}, r3)
	require.NoError(t, err)
	require.Equal(t, "history-rt-3", newAuthorization["refresh_token"])
	for _, old := range []map[string]any{r1, r2} {
		current, err := repo.RegisterOpenAIOAuthCredentialGroup(ctx, nil, old)
		require.NoError(t, err)
		require.Equal(t, "history-rt-3", current["refresh_token"])
		require.Equal(t, group, current[service.OpenAIOAuthCredentialGroupKey])
	}
	rotate("4")
	for _, old := range []map[string]any{r1, r2, r3, {"access_token": "history-at-2"}} {
		current, err := repo.RegisterOpenAIOAuthCredentialGroup(ctx, nil, old)
		require.NoError(t, err)
		require.Equal(t, "history-rt-4", current["refresh_token"])
	}
	for _, id := range []int64{first.ID, second.ID} {
		current, err := repo.GetByID(ctx, id)
		require.NoError(t, err)
		require.Equal(t, "history-rt-4", current.GetCredential("refresh_token"))
	}
	// Request-supplied history is ignored; it cannot attach an unrelated token
	// to this group even if an administrator copied internal metadata.
	forged := source("unrelated")
	forged[service.OpenAIOAuthSourceFingerprintsKey] = []string{originalFingerprint}
	unrelated, err := repo.RegisterOpenAIOAuthCredentialGroup(ctx, nil, forged)
	require.NoError(t, err)
	require.NotEqual(t, group, unrelated[service.OpenAIOAuthCredentialGroupKey])
}
