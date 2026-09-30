package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/shopspring/decimal"
)

const DefaultTicketRebateRules = `[{"amount_threshold":5,"ticket_count":1}]`

type TicketRebateRule struct {
	AmountThreshold json.Number `json:"amount_threshold"`
	TicketCount     int         `json:"ticket_count"`
}

func parseTicketRebateRules(raw string) ([]TicketRebateRule, error) {
	if strings.TrimSpace(raw) == "" {
		raw = DefaultTicketRebateRules
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	var rules []TicketRebateRule
	if err := decoder.Decode(&rules); err != nil {
		return nil, fmt.Errorf("ticket_rebate_rules must be a JSON rule array: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, errors.New("ticket_rebate_rules must contain one JSON array")
	}
	if len(rules) == 0 {
		return nil, errors.New("ticket_rebate_rules must contain at least one rule")
	}
	seen := make(map[string]bool, len(rules))
	for _, rule := range rules {
		threshold, err := decimal.NewFromString(rule.AmountThreshold.String())
		if err != nil || !threshold.GreaterThan(decimal.Zero) {
			return nil, errors.New("amount_threshold must be greater than zero")
		}
		if rule.TicketCount <= 0 {
			return nil, errors.New("ticket_count must be a positive integer")
		}
		key := threshold.String()
		if seen[key] {
			return nil, errors.New("duplicate amount_threshold")
		}
		seen[key] = true
	}
	sort.Slice(rules, func(i, j int) bool {
		a, _ := decimal.NewFromString(rules[i].AmountThreshold.String())
		b, _ := decimal.NewFromString(rules[j].AmountThreshold.String())
		return a.LessThan(b)
	})
	return rules, nil
}

func NormalizeTicketRebateRules(raw string) (string, error) {
	rules, err := parseTicketRebateRules(raw)
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(rules)
	return string(encoded), err
}

func CalculateTicketRebate(consumption decimal.Decimal, rules []TicketRebateRule) (int, []TicketRebateRule) {
	matched := make([]TicketRebateRule, 0)
	count := 0
	for _, rule := range rules {
		threshold, _ := decimal.NewFromString(rule.AmountThreshold.String())
		if consumption.LessThan(threshold) {
			continue
		}
		matched = append(matched, rule)
		count += rule.TicketCount
	}
	return count, matched
}
