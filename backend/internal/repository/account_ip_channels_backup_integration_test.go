//go:build unit

package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAccountIPBackupPostgresHTTPRoundTrip(t *testing.T) {
	for _, setting := range []struct{ retire, enabled bool }{{false, true}, {true, true}, {false, false}, {true, false}} {
		retire, enabled := setting.retire, setting.enabled
		t.Run(fmt.Sprintf("retired_root_%v_enabled_%v", retire, enabled), func(t *testing.T) {
			r, c, ctx := ipChannelIntegrationRepo(t)
			accounts := createIPChannelFixture(t, r, c, ctx, "backup", 3)
			root, err := r.JoinAccountIPChannels(ctx, []int64{accounts[0].ID, accounts[1].ID, accounts[2].ID})
			require.NoError(t, err)
			off := false
			zero := 0
			priority := 77
			weight := 9
			require.NoError(t, r.PatchAccountIPChannel(ctx, root, accounts[1].ID, service.AccountIPChannelPatch{Schedulable: &off, Concurrency: &zero, Priority: &priority, LoadFactor: &weight}))
			if retire {
				require.NoError(t, r.PatchAccountIPChannel(ctx, root, root, service.AccountIPChannelPatch{Schedulable: &off}))
				_, err = r.sql.ExecContext(ctx, `UPDATE account_ip_channels SET disabled_at=NOW()-interval '3 minutes' WHERE account_id=$1`, root)
				require.NoError(t, err)
				require.NoError(t, r.RemoveAccountIPChannel(ctx, root, root))
			}
			_, err = r.SetLogicalAccountSchedulable(ctx, root, enabled)
			require.NoError(t, err)
			service.PrepareNewAccountProtection(accounts[2])
			_, err = c.Account.UpdateOneID(accounts[2].ID).SetExtra(accounts[2].Extra).SetConcurrency(accounts[2].Concurrency).Save(ctx)
			require.NoError(t, err)
			proxyRepo := &proxyRepository{client: c, sql: r.sql}
			adminSvc := service.NewAdminService(&config.Config{}, nil, nil, r, proxyRepo, nil, nil, nil, nil, nil, nil, nil, nil, c, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			h := admin.NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.GET("/export", h.ExportData)
			router.POST("/import", h.ImportData)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/export?ids=%d", accounts[1].ID), nil))
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			var exported struct {
				Data admin.DataPayload `json:"data"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &exported))
			require.Len(t, exported.Data.Accounts, 1)
			backup := exported.Data.Accounts[0]
			require.Equal(t, 2, exported.Data.Version)
			require.Equal(t, retire, backup.IPChannels.RootRetired)
			require.Equal(t, enabled, backup.IPChannels.LogicalEnabled)
			count := 3
			if retire {
				count = 2
			}
			require.Len(t, backup.IPChannels.Items, count)
			require.Len(t, exported.Data.Proxies, count)
			idsBefore, err := r.AccountIPChannelHistoryIDs(ctx, []int64{root})
			require.NoError(t, err)
			oldRoot, err := r.GetByID(ctx, root)
			require.NoError(t, err)
			_, err = r.RefreshOpenAIOAuthCredentials(ctx, oldRoot, func(_ context.Context, a *service.Account) (map[string]any, error) {
				credentials := copyJSONMap(a.Credentials)
				credentials["access_token"] = "rotated-after-export"
				credentials["refresh_token"] = "rotated-refresh"
				return credentials, nil
			})
			require.NoError(t, err)
			body, err := json.Marshal(admin.DataImportRequest{Data: exported.Data})
			require.NoError(t, err)
			rec = httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/import", bytes.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(rec, request)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			var imported struct {
				Data admin.DataImportResult `json:"data"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &imported))
			require.Equal(t, 1, imported.Data.AccountCreated, rec.Body.String())
			require.Zero(t, imported.Data.AccountFailed)
			rows, page, err := r.ListLogicalAccounts(ctx, pagination.PaginationParams{Page: 1, PageSize: 20}, "", "", "", "", 0, "")
			require.NoError(t, err)
			require.EqualValues(t, 2, page.Total)
			var restoredID int64
			for _, a := range rows {
				if a.ID != root {
					restoredID = a.ID
				}
			}
			restored, err := r.GetAccountIPChannels(ctx, []int64{restoredID})
			require.NoError(t, err)
			require.Len(t, restored[restoredID], count)
			require.Equal(t, enabled, restored[restoredID][0].LogicalEnabled)
			for _, ch := range restored[restoredID] {
				require.True(t, ch.Account.AntiDegradationEnabled(), "every restored IP channel receives new-account protection")
				require.Equal(t, "legacy", ch.Account.ProtectionMode())
				require.NoError(t, service.ValidateAccountProtectionConfiguration(ch.Account))
				require.Equal(t, "rotated-after-export", ch.Account.GetCredential("access_token"), "old backups must retain the current durable OAuth generation")
				if !enabled {
					require.False(t, ch.Account.Schedulable)
				}
				if *ch.Account.ProxyID == *accounts[2].ProxyID {
					require.True(t, ch.Account.AntiDegradationEnabled())
					require.Equal(t, accounts[2].ProtectionMode(), ch.Account.ProtectionMode())
				}
				if *ch.Account.ProxyID == *accounts[1].ProxyID {
					require.False(t, ch.Enabled)
					require.False(t, ch.Account.Schedulable)
					require.Equal(t, service.AntiDegradeConcurrencyCap, ch.Account.Concurrency)
					require.Equal(t, priority, ch.Account.Priority)
					require.Equal(t, &weight, ch.Account.LoadFactor)
				}
			}
			if retire {
				for _, ch := range restored[restoredID] {
					require.NotEqual(t, restoredID, ch.Account.ID)
				}
				a, err := r.GetByID(ctx, restoredID)
				require.NoError(t, err)
				require.False(t, a.Schedulable)
			}
			_, err = r.SetLogicalAccountSchedulable(ctx, restoredID, false)
			require.NoError(t, err)
			_, err = r.SetLogicalAccountSchedulable(ctx, restoredID, true)
			require.NoError(t, err)
			restored, err = r.GetAccountIPChannels(ctx, []int64{restoredID})
			require.NoError(t, err)
			for _, ch := range restored[restoredID] {
				if *ch.Account.ProxyID == *accounts[1].ProxyID {
					require.False(t, ch.Account.Schedulable)
				}
			}
			idsAfter, err := r.AccountIPChannelHistoryIDs(ctx, []int64{root})
			require.NoError(t, err)
			require.Equal(t, idsBefore, idsAfter)
			rec = httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/export?ids=%d&include_proxies=false", root), nil))
			require.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

type ipBackupRefreshClient struct {
	service.OpenAIOAuthClient
	proxy string
	calls int
}

func (c *ipBackupRefreshClient) RefreshTokenWithClientID(_ context.Context, _, proxy, _ string) (*openai.TokenResponse, error) {
	c.proxy = proxy
	c.calls++
	return &openai.TokenResponse{AccessToken: "new-access", RefreshToken: "new-refresh", ExpiresIn: 3600}, nil
}

type recoveryTempCache struct {
	service.TempUnschedCache
	deleted []int64
}

func (c *recoveryTempCache) DeleteTempUnsched(_ context.Context, id int64) error {
	c.deleted = append(c.deleted, id)
	return nil
}
func TestAccountIPBackupPostgresRetiredRootRefreshAndRecovery(t *testing.T) {
	r, c, ctx := ipChannelIntegrationRepo(t)
	accounts := createIPChannelFixture(t, r, c, ctx, "refresh", 3)
	root, err := r.JoinAccountIPChannels(ctx, []int64{accounts[0].ID, accounts[1].ID, accounts[2].ID})
	require.NoError(t, err)
	off := false
	require.NoError(t, r.PatchAccountIPChannel(ctx, root, root, service.AccountIPChannelPatch{Schedulable: &off}))
	_, err = r.sql.ExecContext(ctx, `UPDATE account_ip_channels SET disabled_at=NOW()-interval '3 minutes' WHERE account_id=$1`, root)
	require.NoError(t, err)
	require.NoError(t, r.RemoveAccountIPChannel(ctx, root, root))
	require.NoError(t, r.PatchAccountIPChannel(ctx, root, accounts[1].ID, service.AccountIPChannelPatch{Schedulable: &off}))
	client := &ipBackupRefreshClient{}
	oauth := service.NewOpenAIOAuthService(&proxyRepository{client: c, sql: r.sql}, client)
	oauth.SetAccountRepository(r)
	account, err := r.GetByID(ctx, root)
	require.NoError(t, err)
	_, err = oauth.RefreshAccountToken(ctx, account)
	require.NoError(t, err)
	proxy, err := (&proxyRepository{client: c, sql: r.sql}).GetByID(ctx, *accounts[2].ProxyID)
	require.NoError(t, err)
	require.Equal(t, proxy.URL(), client.proxy)
	require.Equal(t, 1, client.calls)
	for _, a := range accounts {
		fresh, err := r.GetByID(ctx, a.ID)
		require.NoError(t, err)
		require.Equal(t, "new-access", fresh.GetCredential("access_token"))
	}
	for _, a := range accounts[1:] {
		require.NoError(t, r.SetError(ctx, a.ID, "repair me"))
		require.NoError(t, r.SetTempUnschedulable(ctx, a.ID, time.Now().Add(time.Hour), "cooldown"))
	}
	cache := &recoveryTempCache{}
	rate := service.NewRateLimitService(r, nil, &config.Config{}, nil, cache)
	result, err := rate.RecoverLogicalAccountState(ctx, root, service.AccountRecoveryOptions{})
	require.NoError(t, err)
	require.True(t, result.ClearedError)
	require.True(t, result.ClearedRateLimit)
	require.ElementsMatch(t, []int64{accounts[1].ID, accounts[2].ID}, cache.deleted)
	paused, err := r.GetByID(ctx, accounts[1].ID)
	require.NoError(t, err)
	require.False(t, paused.Schedulable)
	require.Equal(t, service.StatusActive, paused.Status)
	active, err := r.GetByID(ctx, accounts[2].ID)
	require.NoError(t, err)
	require.True(t, active.Schedulable)
	retired, err := r.GetByID(ctx, root)
	require.NoError(t, err)
	require.False(t, retired.Schedulable)
	adminSvc := service.NewAdminService(&config.Config{}, nil, nil, r, &proxyRepository{client: c, sql: r.sql}, nil, nil, nil, nil, nil, nil, nil, nil, c, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := admin.NewAccountHandler(adminSvc, nil, oauth, nil, nil, nil, rate, nil, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.POST("/accounts/:id/ip-channels/:channelId/recover-state", handler.RecoverIPChannelState)
	require.NoError(t, r.SetError(ctx, accounts[1].ID, "channel error"))
	request := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/accounts/%d/ip-channels/%d/recover-state", root, accounts[1].ID), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, request)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	paused, err = r.GetByID(ctx, accounts[1].ID)
	require.NoError(t, err)
	require.False(t, paused.Schedulable)
	require.Equal(t, service.StatusActive, paused.Status)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, fmt.Sprintf("/accounts/%d/ip-channels/%d/recover-state", root, root), nil))
	require.Equal(t, http.StatusNotFound, rec.Code, "retired root cannot be recovered as a routing channel")
	_, err = r.SetLogicalAccountSchedulable(ctx, root, false)
	require.NoError(t, err)
	_, err = oauth.RefreshAccountToken(ctx, retired)
	require.Error(t, err)
	require.Equal(t, 1, client.calls)
}
func TestAccountIPBackupPostgresRollback(t *testing.T) {
	r, c, ctx := ipChannelIntegrationRepo(t)
	accounts := createIPChannelFixture(t, r, c, ctx, "rollback", 2)
	before, err := c.Account.Query().Count(ctx)
	require.NoError(t, err)
	root := *accounts[0]
	root.ID = 0
	channels := []service.AccountIPChannelBackup{{ProxyID: *accounts[0].ProxyID, IsRoot: true, Enabled: true, Status: service.StatusActive}, {ProxyID: *accounts[0].ProxyID, Enabled: true, Status: service.StatusActive}}
	require.Error(t, r.RestoreAccountIPChannels(ctx, &root, channels, true, false))
	after, err := c.Account.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, before, after)
}
