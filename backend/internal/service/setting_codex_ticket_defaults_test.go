package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewAccountCodexTicketDefaultsSettingsRoundTrip(t *testing.T) {
	ctx := context.Background()
	repo := &codexTicketSettingRepo{values: map[string]string{SettingKeyOpenAICodexTicketEnabled: "false", SettingKeyOpenAICodexTicketHarvestProxyURL: ""}}
	svc := NewSettingService(repo, nil)
	got, err := svc.GetNewAccountCodexTicketDefaults(ctx)
	require.NoError(t, err)
	require.Equal(t, &NewAccountCodexTicketDefaults{TicketPlan: "pro", Model: "gpt-6-astra"}, got)
	for _, plan := range []string{"pro", "team"} {
		got, err = svc.UpdateNewAccountCodexTicketDefaults(ctx, NewAccountCodexTicketDefaults{Enabled: true, TicketPlan: plan, Model: "untrusted-model"})
		require.NoError(t, err)
		require.True(t, got.Enabled)
		require.Equal(t, plan, got.TicketPlan)
		require.Equal(t, "gpt-6-astra", got.Model)
		another := NewSettingService(repo, nil)
		read, readErr := another.GetNewAccountCodexTicketDefaults(ctx)
		require.NoError(t, readErr)
		require.Equal(t, got, read)
	}
	require.Equal(t, "false", repo.values[SettingKeyOpenAICodexTicketEnabled], "saving defaults must not enable the upstream harvester")
	require.Empty(t, repo.values[SettingKeyOpenAICodexTicketHarvestProxyURL])
	require.Len(t, repo.values, 3, "only one independent policy key is written")
}

func TestNewAccountCodexTicketDefaultsFailClosedAndDoNotWriteInvalidPlans(t *testing.T) {
	ctx := context.Background()
	for _, plan := range []string{"", "312", "enterprise"} {
		repo := &codexTicketSettingRepo{}
		_, err := NewSettingService(repo, nil).UpdateNewAccountCodexTicketDefaults(ctx, NewAccountCodexTicketDefaults{Enabled: true, TicketPlan: plan})
		require.Error(t, err)
		require.Zero(t, repo.writes)
	}
	for _, raw := range []string{"", "null", "{}", `{"enabled":true}`, `{"enabled":true,"ticket_plan":"312"}`, `{"enabled":"true","ticket_plan":"pro"}`} {
		repo := &codexTicketSettingRepo{values: map[string]string{SettingKeyOpenAINewAccountCodexTicketDefaults: raw}}
		_, err := NewSettingService(repo, nil).GetNewAccountCodexTicketDefaults(ctx)
		require.Error(t, err)
	}
	secretErr := errors.New("private-db-password")
	for _, repo := range []*codexTicketSettingRepo{{readErr: secretErr}, {writeErr: secretErr}} {
		svc := NewSettingService(repo, nil)
		var err error
		if repo.readErr != nil {
			_, err = svc.GetNewAccountCodexTicketDefaults(ctx)
		} else {
			_, err = svc.UpdateNewAccountCodexTicketDefaults(ctx, NewAccountCodexTicketDefaults{TicketPlan: "pro"})
		}
		require.Error(t, err)
		require.NotContains(t, err.Error(), "private-db-password")
	}
}
