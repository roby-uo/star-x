package service

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// PrepareSimpleBillingSearch freezes a standalone successful search request's
// price. It never substitutes the language model's token pricing for a tool fee.
func (s *BillingService) PrepareSimpleBillingSearch(ctx context.Context, in SimpleBillingInput) (*SimpleBillingSnapshot, error) {
	in.Modality, in.Operation = "chat", "alpha_search"
	return s.PrepareSimpleBilling(ctx, in)
}

func (s *BillingService) resolveSimpleSearchBilling(q *SimpleBillingSnapshot, group *Group, profile *CostProfile) (*SimpleBillingSnapshot, error) {
	q.BillingUnit = "per_request"
	if profile != nil {
		if profile.Region != "" && !strings.EqualFold(profile.Region, q.Region) {
			return nil, simplePriceError("搜索价表地区与上游账号不一致")
		}
		q.Rows = matchingSimpleProfileRows(profile, q)
	}
	if q.Route == "account" {
		q.Source, q.Multiplier = "reference", group.RateMultiplier
		if len(q.Rows) > 0 {
			q.ProfileID, q.Currency = profile.ID, profile.Currency
			// Reference rows are independent of a monthly procurement discount.
			q.DiscountMultiplier = 1
		} else {
			unit := defaultWebSearchPricePerCall
			if group.WebSearchPricePerCall != nil {
				unit = *group.WebSearchPricePerCall
			}
			if !finiteNonnegative(unit) {
				return nil, simplePriceError("搜索参考单价无效")
			}
			q.UnitPriceUSD = &unit
		}
	} else {
		// API suppliers can charge hosted search, model tokens, or their own
		// search bundle. A per-request contract must be explicitly confirmed.
		if profile == nil || profile.PurchaseMode != "payg" || len(q.Rows) == 0 {
			return nil, simplePriceError("API搜索须配置已确认的按次采购基准，不能采用文本模型单价")
		}
		q.Source, q.ProfileID, q.Currency, q.DiscountMultiplier = "manual", profile.ID, profile.Currency, profile.DiscountMultiplier
	}
	if len(q.Rows) > 0 {
		if err := validateConfirmedSimpleRows(q, false); err != nil {
			return nil, err
		}
	}
	if !finiteNonnegative(q.Multiplier) {
		return nil, simplePriceError("搜索售价倍率无效")
	}
	if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		q.Multiplier = 0
	}
	q.RefreshHash()
	return q, nil
}
