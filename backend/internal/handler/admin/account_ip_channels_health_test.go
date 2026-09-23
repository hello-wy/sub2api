package admin

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type displayIPChannelAdmin struct {
	*stubAdminService
	service.AccountIPChannelAdmin
	members []service.AccountIPChannel
}

func (s *displayIPChannelAdmin) GetAccountIPChannels(context.Context, []int64) (map[int64][]service.AccountIPChannel, error) {
	return map[int64][]service.AccountIPChannel{1: s.members}, nil
}

type displayTicketSummaries struct {
	*codexTicketControlStub
	statuses map[int64]*service.CodexAccountTicketStatus
}

func (s *displayTicketSummaries) CodexAccountTicketStatusFromAccount(_ context.Context, a *service.Account) *service.CodexAccountTicketStatus {
	return s.statuses[a.ID]
}

func TestIPChannelAggregateReflectsRequiredTicketReadiness(t *testing.T) {
	proxyID := int64(10)
	first := &service.Account{ID: 1, Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Status: service.StatusActive, Schedulable: true, ProxyID: &proxyID, Proxy: &service.Proxy{ID: proxyID, Status: service.StatusActive}}
	second := *first
	second.ID = 2
	manager := &displayIPChannelAdmin{stubAdminService: newStubAdminService(), members: []service.AccountIPChannel{
		{Account: first, LogicalAccountID: 1, Enabled: true, LogicalEnabled: true},
		{Account: &second, LogicalAccountID: 1, Enabled: true, LogicalEnabled: true},
	}}
	expires := time.Now().Add(time.Hour)
	pending := &service.CodexAccountTicketStatus{Enabled: true, RequireVerified: true, GlobalEnabled: true, State: "waiting"}
	ready := &service.CodexAccountTicketStatus{Enabled: true, RequireVerified: true, GlobalEnabled: true, State: "ready", TicketUsable: true, ExpiresAt: &expires}
	summaries := &displayTicketSummaries{codexTicketControlStub: &codexTicketControlStub{}, statuses: map[int64]*service.CodexAccountTicketStatus{}}
	handler := &AccountHandler{adminService: manager, codexAccountTickets: summaries}
	for _, tc := range []struct {
		name, want    string
		first, second *service.CodexAccountTicketStatus
	}{
		{"all waiting", "unavailable", pending, pending},
		{"one ready", "partial", ready, pending},
		{"both ready", "normal", ready, ready},
		{"manual target-only ticket", "normal", &service.CodexAccountTicketStatus{Enabled: true, State: "waiting"}, &service.CodexAccountTicketStatus{Enabled: true, State: "waiting"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			summaries.statuses[1], summaries.statuses[2] = tc.first, tc.second
			items := []AccountWithConcurrency{{Account: dto.AccountFromService(first)}}
			require.NoError(t, handler.enrichIPChannels(context.Background(), items))
			require.Equal(t, tc.want, items[0].ChannelStatus)
			require.True(t, items[0].Schedulable, "the operator's logical switch is independent of health")
			require.Zero(t, summaries.calls, "display must reuse loaded summaries without per-account reads")
		})
	}
	blocked := *ready
	blocked.AuthenticationBlocked = true
	require.False(t, channelBusinessHealthy(first, &blocked))
	blocked = *ready
	blocked.GlobalEnabled = false
	require.False(t, channelBusinessHealthy(first, &blocked))
	blocked = *ready
	expired := time.Now().Add(-time.Second)
	blocked.ExpiresAt = &expired
	require.False(t, channelBusinessHealthy(first, &blocked))
	first.Status, first.Schedulable = service.StatusError, false
	require.False(t, channelBusinessHealthy(first, ready), "a proof never overrides current authentication failure")
}
