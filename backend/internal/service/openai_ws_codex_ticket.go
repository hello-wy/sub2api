package service

import (
	"context"
	coderws "github.com/coder/websocket"
)

// Ticket-managed accounts use one HTTP request per turn so expiration, refresh,
// and opt-out are evaluated by applyOpenAICodexTicket before each upstream write.
// Account opt-in deliberately controls this independently of the global switch:
// turning the global switch back on must not retain a native WS handshake.
func (s *OpenAIGatewayService) shouldBridgeOpenAICodexTicketAccount(ctx context.Context, account *Account) bool {
	if s == nil || !isOpenAICodexTicketAccount(account) {
		return false
	}
	enabled, settingsErr := s.openAICodexTicketRuntimeEnabled(ctx)
	if !codexAccountTicketConfigOf(account).Enabled && !enabled && settingsErr == nil {
		return false
	}
	live, err := s.codexTicketLiveAccount(ctx, account)
	if err != nil {
		// A known managed account stays on the safe transport during a transient
		// repository error; normal accounts retain their existing transport.
		return enabled || settingsErr != nil || codexAccountTicketConfigOf(account).Enabled
	}
	return isOpenAICodexTicketAccount(live) && codexAccountTicketConfigOf(live).Enabled
}

func (s *OpenAIGatewayService) checkOpenAICodexTicketNativeTurn(ctx context.Context, account *Account) error {
	if s.shouldBridgeOpenAICodexTicketAccount(ctx, account) {
		return NewOpenAIWSClientCloseError(coderws.StatusTryAgainLater, "STATE ticket settings changed; reconnect to use the account's current ticket", nil)
	}
	return nil
}
