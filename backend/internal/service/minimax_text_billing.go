package service

import "strings"

// Scope MiniMax's tier pricing to a verified official account and final model.
// A similarly named model served by another provider keeps its existing prices.
func isMiniMaxM3Account(account *Account, upstreamModel string) bool {
	if !strings.EqualFold(strings.TrimSpace(upstreamModel), "MiniMax-M3") {
		return false
	}
	_, official := MiniMaxVideoBaseURL(account)
	return official
}

func miniMaxM3PriorityBilling(account *Account, result *OpenAIForwardResult, serviceTier string) bool {
	if result == nil || normalizeBillingServiceTier(serviceTier) != "priority" {
		return false
	}
	model := result.UpstreamModel
	if model == "" {
		model = result.Model
	}
	return isMiniMaxM3Account(account, model)
}

// MiniMax M3 priority is 1.5x standard at either context tier. Apply it after
// channel pricing so the configured base rates and group/user rates still apply.
func applyMiniMaxM3PriorityCost(cost *CostBreakdown) {
	if cost == nil || (cost.BillingMode != "" && cost.BillingMode != string(BillingModeToken)) {
		return
	}
	cost.InputCost *= 1.5
	cost.ImageInputCost *= 1.5
	cost.OutputCost *= 1.5
	cost.ImageOutputCost *= 1.5
	cost.CacheCreationCost *= 1.5
	cost.CacheReadCost *= 1.5
	cost.TotalCost *= 1.5
	cost.ActualCost *= 1.5
}
