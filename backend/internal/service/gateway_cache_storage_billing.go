package service

import (
	"context"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// cacheStoragePrice 返回计费模型的显式缓存存储单价；计费模型未配置时依次回退到实际转发模型与请求模型。
func (s *GatewayService) cacheStoragePrice(result *ForwardResult, billingModel string) float64 {
	if s.billingService == nil {
		return 0
	}
	for _, model := range []string{billingModel, result.UpstreamModel, result.Model} {
		if strings.TrimSpace(model) == "" {
			continue
		}
		if price := s.billingService.CacheStoragePricePerTokenHour(model); price > 0 {
			return price
		}
	}
	return 0
}

// cacheStorageCost 返回 result 的显式缓存存储费（USD，倍率前）。
func (s *GatewayService) cacheStorageCost(result *ForwardResult, billingModel string) float64 {
	if result.CacheStorageTokenHours <= 0 {
		return 0
	}
	return result.CacheStorageTokenHours * s.cacheStoragePrice(result, billingModel)
}

// isCacheStorageOnlyResult 判断 result 是否只有显式缓存存储费（延长有效期）：不含 token 用量，不算一次模型请求。
func isCacheStorageOnlyResult(result *ForwardResult) bool {
	u := result.Usage
	return result.CacheStorageTokenHours > 0 && u.InputTokens == 0 && u.OutputTokens == 0 &&
		u.CacheCreationInputTokens == 0 && u.CacheReadInputTokens == 0 && u.ImageOutputTokens == 0 &&
		result.ImageCount == 0 && result.AudioUsage == nil && result.SearchCount == 0
}

// CacheStoragePriced 判断 RecordUsage 对 input.Result 采用的计费模型（含回退候选）是否配置了显式缓存存储单价。
func (s *GatewayService) CacheStoragePriced(ctx context.Context, input *RecordUsageInput) bool {
	billingModel := s.recordUsageBillingModel(ctx, input.APIKey, input.Result, input.ChannelUsageFields)
	return s.cacheStoragePrice(input.Result, billingModel) > 0
}

// QuoteUsageCost 按与 RecordUsage 相同的倍率、计费模型与定价计算 input.Result 的应付金额（倍率后 USD），不记录也不扣费。
func (s *GatewayService) QuoteUsageCost(ctx context.Context, input *RecordUsageInput) (float64, error) {
	user := input.User
	if user == nil {
		user = input.APIKey.User
	}
	multiplier, imageMultiplier, pricingAt := s.recordUsageMultipliers(ctx, input.APIKey, user, input.PricingAt)
	billingModel := s.recordUsageBillingModel(ctx, input.APIKey, input.Result, input.ChannelUsageFields)
	cost, err := s.calculateRecordUsageCost(ctx, input.Result, input.APIKey, billingModel, multiplier, imageMultiplier, pricingAt)
	if err != nil {
		return 0, err
	}
	if cost == nil {
		return 0, nil
	}
	return cost.ActualCost, nil
}

// addCacheStorageToAccountStatsCost 把显式缓存存储费按官方价叠加到账号统计成本。
// 账号统计定价只覆盖 token 部分；统计成本为空时成本统计回落 total_cost，其中已含存储费。
func addCacheStorageToAccountStatsCost(usageLog *UsageLog, storageCost float64) {
	if usageLog == nil || usageLog.AccountStatsCost == nil || storageCost <= 0 {
		return
	}
	cost := *usageLog.AccountStatsCost + storageCost
	usageLog.AccountStatsCost = &cost
}

type tokenSurchargeCostInput struct {
	result       *ForwardResult
	apiKey       *APIKey
	billingModel string
	multiplier   float64
	pricingAt    time.Time
}

func (s *GatewayService) calculateTokenSurchargeCost(ctx context.Context, input tokenSurchargeCostInput) (*CostBreakdown, error) {
	// Token 计费；SearchCount 为叠加 surcharge（不替代 token）。纯存储费不是一次模型请求，不计 token / 按次费用。
	var tokenCost *CostBreakdown
	var err error
	if isCacheStorageOnlyResult(input.result) {
		tokenCost = &CostBreakdown{}
	} else {
		tokenCost, err = s.calculateTokenCost(ctx, input.result, input.apiKey, input.billingModel, input.multiplier, input.pricingAt)
		if err != nil {
			return nil, err
		}
	}
	if input.result.SearchCount > 0 {
		price := groupSearchPricePer1kFromAPIKey(input.apiKey)
		if price != nil && *price == 0 {
			logger.LegacyPrintf("service.gateway", "[Billing] search_price_per_1k explicit 0; search free group_model=%s count=%d", input.billingModel, input.result.SearchCount)
		}
		searchCost := s.billingService.CalculateSearchCost(input.result.SearchCount, price, input.multiplier)
		if searchCost != nil && (searchCost.TotalCost > 0 || searchCost.ActualCost > 0) {
			if tokenCost == nil {
				return searchCost, nil
			}
			tokenCost.TotalCost += searchCost.TotalCost
			tokenCost.ActualCost += searchCost.ActualCost
		}
	}
	// 显式缓存存储费同为叠加 surcharge，与 token 费用使用同一计费模型与倍率。
	if storageCost := s.cacheStorageCost(input.result, input.billingModel); storageCost > 0 {
		if tokenCost == nil {
			tokenCost = &CostBreakdown{}
		}
		tokenCost.TotalCost += storageCost
		tokenCost.ActualCost += storageCost * input.multiplier
	}
	return tokenCost, nil
}
