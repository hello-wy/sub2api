package service

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/require"
)

func TestConsumptionRevenueRange(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, timezone.Location())
	start, end, err := ConsumptionRevenueRange("", "", now)
	require.NoError(t, err)
	require.Equal(t, "2026-11-01", start.Format("2006-01-02"))
	require.Equal(t, "2026-11-17", end.Format("2006-01-02"))
	start, end, err = ConsumptionRevenueRange("2026-11-16", "2026-11-16", now)
	require.NoError(t, err)
	require.True(t, end.Equal(start.AddDate(0, 0, 1)))
	for _, dates := range [][2]string{{"bad", "2026-11-16"}, {"2026-11-16", "bad"}, {"2026-11-16", "2026-11-15"}, {"2026-11-16", "2026-11-17"}, {"2024-01-01", "2026-11-16"}} {
		_, _, err = ConsumptionRevenueRange(dates[0], dates[1], now)
		require.Error(t, err)
	}
}
