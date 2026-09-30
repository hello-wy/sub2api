package service

import (
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

const sampleTicketRules = `[{"amount_threshold":5,"ticket_count":1},{"amount_threshold":10,"ticket_count":1},{"amount_threshold":20,"ticket_count":2}]`

func TestTicketRebateCumulativeThresholds(t *testing.T) {
	rules, err := parseTicketRebateRules(sampleTicketRules)
	require.NoError(t, err)
	for _, test := range []struct {
		spend   string
		count   int
		matched int
	}{
		{"0", 0, 0}, {"4.99", 0, 0}, {"5", 1, 1},
		{"9.99", 1, 1}, {"10", 2, 2}, {"20", 4, 3},
	} {
		t.Run(test.spend, func(t *testing.T) {
			count, matched := CalculateTicketRebate(decimal.RequireFromString(test.spend), rules)
			require.Equal(t, test.count, count)
			require.Len(t, matched, test.matched)
		})
	}
}

func TestTicketRebateRulesValidateAndSort(t *testing.T) {
	normalized, err := NormalizeTicketRebateRules(`[{"amount_threshold":20,"ticket_count":2},{"amount_threshold":5,"ticket_count":1}]`)
	require.NoError(t, err)
	require.Equal(t, `[{"amount_threshold":5,"ticket_count":1},{"amount_threshold":20,"ticket_count":2}]`, normalized)
	for _, invalid := range []string{
		`[]`, `[{"amount_threshold":0,"ticket_count":1}]`,
		`[{"amount_threshold":5,"ticket_count":0}]`,
		`[{"amount_threshold":5,"ticket_count":1.5}]`,
		`[{"amount_threshold":5,"ticket_count":1},{"amount_threshold":5.00,"ticket_count":2}]`,
	} {
		_, err := NormalizeTicketRebateRules(invalid)
		require.Error(t, err, invalid)
	}
}

func TestTicketRebateMatchedRulesSnapshotSurvivesRuleChanges(t *testing.T) {
	rules, err := parseTicketRebateRules(sampleTicketRules)
	require.NoError(t, err)
	count, matched := CalculateTicketRebate(decimal.NewFromInt(10), rules)
	snapshot, err := json.Marshal(matched)
	require.NoError(t, err)
	rules[0].TicketCount = 99
	require.Equal(t, 2, count)
	require.JSONEq(t, `[{"amount_threshold":5,"ticket_count":1},{"amount_threshold":10,"ticket_count":1}]`, string(snapshot))
}
