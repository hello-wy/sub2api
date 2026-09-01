package admin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOptionalLimitField_NullMeansUnlimited(t *testing.T) {
	var input struct {
		Limit optionalLimitField `json:"limit"`
	}

	require.NoError(t, json.Unmarshal([]byte(`{"limit":null}`), &input))
	require.True(t, input.Limit.IsSet())
	require.Nil(t, input.Limit.ToServiceInput())
}

func TestOptionalLimitField_OmittedRemainsUnset(t *testing.T) {
	var input struct {
		Limit optionalLimitField `json:"limit"`
	}

	require.NoError(t, json.Unmarshal([]byte(`{}`), &input))
	require.False(t, input.Limit.IsSet())
	require.Nil(t, input.Limit.ToServiceInput())
}

func TestOptionalLimitField_NumberPreservesZero(t *testing.T) {
	var input struct {
		Limit optionalLimitField `json:"limit"`
	}

	require.NoError(t, json.Unmarshal([]byte(`{"limit":0}`), &input))
	require.NotNil(t, input.Limit.ToServiceInput())
	require.Equal(t, 0.0, *input.Limit.ToServiceInput())
}

func TestOptionalLimitField_RejectsNonFiniteStrings(t *testing.T) {
	for _, value := range []string{"NaN", "Inf", "-Inf"} {
		t.Run(value, func(t *testing.T) {
			var input struct {
				Limit optionalLimitField `json:"limit"`
			}

			err := json.Unmarshal([]byte(`{"limit":"`+value+`"}`), &input)
			require.ErrorContains(t, err, "must be finite")
		})
	}
}

func TestUpdateGroupRequestLimitFieldsTriState(t *testing.T) {
	t.Run("omitted means unchanged", func(t *testing.T) {
		var req UpdateGroupRequest
		require.NoError(t, json.Unmarshal([]byte(`{}`), &req))
		require.False(t, req.DailyLimitUSD.IsSet())
		require.False(t, req.WeeklyLimitUSD.IsSet())
		require.False(t, req.MonthlyLimitUSD.IsSet())
	})

	t.Run("null means unlimited", func(t *testing.T) {
		var req UpdateGroupRequest
		require.NoError(t, json.Unmarshal([]byte(`{"daily_limit_usd":null}`), &req))
		require.True(t, req.DailyLimitUSD.IsSet())
		require.Nil(t, req.DailyLimitUSD.ToServiceInput())
	})

	t.Run("number is preserved", func(t *testing.T) {
		var req UpdateGroupRequest
		require.NoError(t, json.Unmarshal([]byte(`{"weekly_limit_usd":0,"monthly_limit_usd":42.5}`), &req))
		require.Equal(t, 0.0, *req.WeeklyLimitUSD.ToServiceInput())
		require.Equal(t, 42.5, *req.MonthlyLimitUSD.ToServiceInput())
	})
}
