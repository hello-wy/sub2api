//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type openAIRiskControlAutoProberSpy struct {
	scheduled []struct {
		accountID int64
		trigger   string
	}
	observed []struct {
		accountID int64
		status    int
	}
}

func (s *openAIRiskControlAutoProberSpy) ScheduleOpenAIRiskControlProbe(accountID int64, trigger string) bool {
	s.scheduled = append(s.scheduled, struct {
		accountID int64
		trigger   string
	}{accountID: accountID, trigger: trigger})
	return true
}

func (s *openAIRiskControlAutoProberSpy) ObserveOpenAIRiskControlError(account *Account, status int) bool {
	s.observed = append(s.observed, struct {
		accountID int64
		status    int
	}{accountID: account.ID, status: status})
	return false
}

func TestCreateAccountSchedulesInitialOpenAIRiskControlProbe(t *testing.T) {
	repo := &upstreamBillingProbeAccountRepo{}
	prober := &openAIRiskControlAutoProberSpy{}
	service := &adminServiceImpl{accountRepo: repo, openaiRiskControl: prober}

	created, err := service.CreateAccount(context.Background(), &CreateAccountInput{
		Name:                 "oauth-import",
		Platform:             PlatformOpenAI,
		Type:                 AccountTypeOAuth,
		Credentials:          map[string]any{},
		SkipDefaultGroupBind: true,
	})

	require.NoError(t, err)
	require.Len(t, prober.scheduled, 1)
	require.Equal(t, created.ID, prober.scheduled[0].accountID)
	require.Equal(t, openAIRiskControlTriggerAccountCreated, prober.scheduled[0].trigger)
}

func TestCreateAccountDoesNotScheduleRiskControlForIneligibleAccount(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		typeName string
	}{
		{name: "OpenAI API key", platform: PlatformOpenAI, typeName: AccountTypeAPIKey},
		{name: "Gemini OAuth", platform: PlatformGemini, typeName: AccountTypeOAuth},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &upstreamBillingProbeAccountRepo{}
			prober := &openAIRiskControlAutoProberSpy{}
			service := &adminServiceImpl{accountRepo: repo, openaiRiskControl: prober}
			_, err := service.CreateAccount(context.Background(), &CreateAccountInput{
				Name:                 tt.name,
				Platform:             tt.platform,
				Type:                 tt.typeName,
				Credentials:          map[string]any{},
				SkipDefaultGroupBind: true,
			})
			require.NoError(t, err)
			require.Empty(t, prober.scheduled)
		})
	}
}

func TestRateLimitServiceReportsErrorsBeforeEarlyReturn(t *testing.T) {
	prober := &openAIRiskControlAutoProberSpy{}
	repo := &oauth429RateLimitRepo{}
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	service.SetOpenAIRiskControlAutoProber(prober)
	account := &Account{
		ID:       88,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
	}

	require.False(t, service.HandleUpstreamError(context.Background(), account, http.StatusTooManyRequests, http.Header{}, nil))
	require.Equal(t, []struct {
		accountID int64
		status    int
	}{{accountID: account.ID, status: http.StatusTooManyRequests}}, prober.observed)
}
