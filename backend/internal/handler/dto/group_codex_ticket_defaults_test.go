package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupCodexTicketDefaultsAdminDTO(t *testing.T) {
	g := &service.Group{ID: 1, Platform: service.PlatformOpenAI, CodexTicketDefaults: service.GroupCodexTicketDefaults{Enabled: true, TicketPlan: "team", Model: "gpt-5.6-sol"}}
	adminJSON, err := json.Marshal(GroupFromServiceAdmin(g))
	require.NoError(t, err)
	var payload struct {
		Defaults service.GroupCodexTicketDefaults `json:"codex_ticket_defaults"`
	}
	require.NoError(t, json.Unmarshal(adminJSON, &payload))
	require.Equal(t, g.CodexTicketDefaults, payload.Defaults)
	userJSON, err := json.Marshal(GroupFromService(g))
	require.NoError(t, err)
	require.NotContains(t, string(userJSON), "codex_ticket_defaults")
}
