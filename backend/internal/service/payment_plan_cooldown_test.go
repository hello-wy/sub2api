package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAddPlanCooldownHoursDoesNotOverflowTimeDuration(t *testing.T) {
	const cooldownHours = 3_000_000
	start := time.Date(2026, time.September, 23, 0, 0, 0, 0, time.UTC)

	availableAt := addPlanCooldownHours(start, cooldownHours)

	require.Equal(t, int64(cooldownHours)*secondsPerHour, availableAt.Unix()-start.Unix())
}
