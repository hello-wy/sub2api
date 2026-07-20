package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
)

func validateBillableModelPricing(ctx context.Context, billing *BillingService, resolver *ModelPricingResolver, apiKey *APIKey, model string) error {
	model = strings.TrimSpace(model)
	if model == "" {
		return fmt.Errorf("%w: model is empty", ErrModelPricingUnavailable)
	}
	if billing == nil {
		return fmt.Errorf("%w: billing service is unavailable", ErrModelPricingUnavailable)
	}

	var groupID *int64
	if apiKey != nil && apiKey.GroupID != nil {
		groupID = apiKey.GroupID
	} else if apiKey != nil && apiKey.Group != nil && apiKey.Group.ID > 0 {
		gid := apiKey.Group.ID
		groupID = &gid
	}
	if resolver == nil {
		pricing, err := billing.GetModelPricing(model)
		if err != nil || !hasPositiveTokenPricing(pricing) {
			return fmt.Errorf("%w for model: %s", ErrModelPricingUnavailable, model)
		}
		return nil
	}

	resolved := resolver.Resolve(ctx, PricingInput{Model: model, GroupID: groupID})
	if resolved == nil {
		return fmt.Errorf("%w for model: %s", ErrModelPricingUnavailable, model)
	}
	switch resolved.Mode {
	case BillingModePerRequest, BillingModeImage:
		if resolved.DefaultPerRequestPrice > 0 {
			return nil
		}
		for i := range resolved.RequestTiers {
			if resolved.RequestTiers[i].PerRequestPrice != nil && *resolved.RequestTiers[i].PerRequestPrice > 0 {
				return nil
			}
		}
	default:
		if hasPositiveTokenPricing(resolved.BasePricing) {
			return nil
		}
		for i := range resolved.Intervals {
			iv := resolved.Intervals[i]
			if positivePrice(iv.InputPrice) || positivePrice(iv.OutputPrice) ||
				positivePrice(iv.CacheWritePrice) || positivePrice(iv.CacheReadPrice) {
				return nil
			}
		}
	}
	return fmt.Errorf("%w for model: %s", ErrModelPricingUnavailable, model)
}

func hasPositiveTokenPricing(pricing *ModelPricing) bool {
	return pricing != nil && (pricing.InputPricePerToken > 0 || pricing.OutputPricePerToken > 0 ||
		pricing.CacheCreationPricePerToken > 0 || pricing.CacheReadPricePerToken > 0 ||
		pricing.ImageInputPricePerToken > 0 || pricing.ImageOutputPricePerToken > 0)
}

func positivePrice(price *float64) bool {
	return price != nil && *price > 0
}

func validateBillableModelCandidates(
	ctx context.Context,
	billing *BillingService,
	resolver *ModelPricingResolver,
	apiKey *APIKey,
	models ...string,
) error {
	seen := make(map[string]struct{}, len(models))
	checked := make([]string, 0, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if _, exists := seen[model]; exists {
			continue
		}
		seen[model] = struct{}{}
		checked = append(checked, model)
		if err := validateBillableModelPricing(ctx, billing, resolver, apiKey, model); err == nil {
			return nil
		}
	}
	if len(checked) == 0 {
		return fmt.Errorf("%w: model is empty", ErrModelPricingUnavailable)
	}
	return fmt.Errorf("%w for models: %s", ErrModelPricingUnavailable, strings.Join(checked, ", "))
}

func appendChannelMappedPricingCandidates(
	ctx context.Context,
	channelService *ChannelService,
	apiKey *APIKey,
	models []string,
) []string {
	if channelService == nil || apiKey == nil {
		return models
	}
	var groupID int64
	if apiKey.GroupID != nil {
		groupID = *apiKey.GroupID
	} else if apiKey.Group != nil {
		groupID = apiKey.Group.ID
	}
	if groupID <= 0 {
		return models
	}
	candidates := append([]string(nil), models...)
	for _, model := range models {
		mapping := channelService.ResolveChannelMapping(ctx, groupID, model)
		if mapped := strings.TrimSpace(mapping.MappedModel); mapped != "" {
			candidates = append(candidates, mapped)
		}
	}
	return candidates
}

func (s *GatewayService) validatePricingBeforeForward(ctx context.Context, c *gin.Context, models ...string) error {
	if s == nil || s.billingService == nil {
		return nil
	}
	if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		return nil
	}
	apiKey := getAPIKeyFromContext(c)
	if apiKey == nil {
		return nil
	}
	models = appendChannelMappedPricingCandidates(ctx, s.channelService, apiKey, models)
	return validateBillableModelCandidates(ctx, s.billingService, s.resolver, apiKey, models...)
}

func (s *OpenAIGatewayService) validatePricingBeforeForward(ctx context.Context, c *gin.Context, models ...string) error {
	if s == nil || s.billingService == nil {
		return nil
	}
	if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		return nil
	}
	apiKey := getAPIKeyFromContext(c)
	if apiKey == nil {
		return nil
	}
	models = appendChannelMappedPricingCandidates(ctx, s.channelService, apiKey, models)
	return validateBillableModelCandidates(ctx, s.billingService, s.resolver, apiKey, models...)
}
