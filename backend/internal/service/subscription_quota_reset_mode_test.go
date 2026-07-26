package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeSubscriptionQuotaResetMode(t *testing.T) {
	tests := []struct {
		name             string
		subscriptionType string
		mode             string
		want             string
		wantErr          bool
	}{
		{
			name:             "subscription group defaults to rolling",
			subscriptionType: SubscriptionTypeSubscription,
			want:             SubscriptionQuotaResetModeRolling,
		},
		{
			name:             "standard group always uses rolling",
			subscriptionType: SubscriptionTypeStandard,
			mode:             SubscriptionQuotaResetModeUntilSubscriptionExpires,
			want:             SubscriptionQuotaResetModeRolling,
		},
		{
			name:             "subscription lifetime quota mode is accepted",
			subscriptionType: SubscriptionTypeSubscription,
			mode:             SubscriptionQuotaResetModeUntilSubscriptionExpires,
			want:             SubscriptionQuotaResetModeUntilSubscriptionExpires,
		},
		{
			name:             "invalid mode is rejected",
			subscriptionType: SubscriptionTypeSubscription,
			mode:             "invalid",
			wantErr:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeSubscriptionQuotaResetMode(tt.subscriptionType, tt.mode)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
