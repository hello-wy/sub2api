package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCodexTicketAuditSecretsAreRedacted(t *testing.T) {
	raw := []byte(`{"enabled":true,"harvest_proxy_url":"socks5://user:private-password@proxy.invalid:1080","proxy_url":"private-legacy","extra":{"codex_turn_ticket:gpt-6-astra":{"state":"private-state"},"codex_turn_ticket:v2:gpt-6-astra":{"state":"private-v2-state","credential_fingerprint":"private-binding"},"codex_ticket_config":{"proxy_url":"private-nested"}}}`)
	redacted := RedactAuditBody(raw, "application/json")
	require.NotContains(t, redacted, "private-")
	require.Contains(t, redacted, `"enabled":true`)
}
