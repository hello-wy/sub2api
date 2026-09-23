package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCodexTicketMaterialNeverAppearsInAccountDTOs(t *testing.T) {
	a := &service.Account{ID: 99, Extra: map[string]any{
		"codex_ticket_config":              map[string]any{"proxy_url": "socks5://user:private-password@example.invalid:1080"},
		"codex_turn_ticket:gpt-6-astra":    map[string]any{"state": "private-state"},
		"codex_turn_ticket:v2:gpt-6-astra": map[string]any{"state": "private-v2-state", "credential_fingerprint": "private-binding"},
		"codex_harvest_proxy_url":          "private-legacy-proxy",
		"codex_ticket_watchdog":            map[string]any{"last_reason": "state_312"},
		"ordinary":                         "visible",
	}}
	for _, dto := range []*Account{AccountFromService(a), AccountFromServiceShallow(a)} {
		raw, err := json.Marshal(dto)
		require.NoError(t, err)
		require.NotContains(t, string(raw), "private-")
		require.NotContains(t, string(raw), "codex_turn_ticket")
		require.Equal(t, "visible", dto.Extra["ordinary"])
	}
	require.Contains(t, a.Extra, "codex_turn_ticket:gpt-6-astra", "redaction must not mutate the runtime account")
	require.Contains(t, a.Extra, "codex_turn_ticket:v2:gpt-6-astra", "redaction must preserve both runtime versions")
}
