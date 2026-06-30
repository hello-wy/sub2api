package service

import (
	"context"
	"encoding/json"
	"strconv"
)

func (s *WelfareService) loadRewardSettings(ctx context.Context) (int, []float64) {
	limitRaw, _ := s.settingRepo.GetValue(ctx, SettingKeyWelfareLeaderboardRankLimit)
	ratiosRaw, _ := s.settingRepo.GetValue(ctx, SettingKeyWelfareLeaderboardRewardRatios)
	return parseWelfareRewardSettings(limitRaw, ratiosRaw)
}

func parseWelfareRewardSettings(limitRaw string, ratiosRaw string) (int, []float64) {
	limit := welfareDefaultRankLimit
	if parsed, err := strconv.Atoi(limitRaw); err == nil && parsed >= 1 {
		limit = parsed
	}
	ratios := append([]float64(nil), welfareDefaultRewardRatios...)
	if parsed := parseWelfareRatios(ratiosRaw); len(parsed) > 0 {
		ratios = parsed
	}
	return limit, resizeWelfareRatios(limit, ratios)
}

func parseWelfareRatios(raw string) []float64 {
	var parsed []float64
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil
	}
	out := make([]float64, 0, len(parsed))
	for _, ratio := range parsed {
		if ratio >= 0 {
			out = append(out, ratio)
		}
	}
	return out
}

func resizeWelfareRatios(limit int, ratios []float64) []float64 {
	if len(ratios) >= limit {
		return append([]float64(nil), ratios[:limit]...)
	}
	out := append([]float64(nil), ratios...)
	for len(out) < limit {
		out = append(out, out[len(out)-1])
	}
	return out
}
