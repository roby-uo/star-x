package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSeedreamArkAccountAndMixedSizeBilling(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"base_url": "https://ark.cn-beijing.volces.com/api/v3",
	}}
	require.True(t, IsVolcengineArkAccount(account))
	require.False(t, IsVolcengineArkAccount(&Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"base_url": "https://ark.cn-beijing.volces.com.evil.example/api/v3",
	}}))

	price1K, price2K := 0.02, 0.04
	svc := &OpenAIGatewayService{billingService: &BillingService{}}
	result := &OpenAIForwardResult{ImageCount: 2, ImageSize: ImageBillingSize2K, ImageSizeBreakdown: map[string]int{ImageBillingSize1K: 1, ImageBillingSize2K: 1}}
	apiKey := &APIKey{Group: &Group{ImagePrice1K: &price1K, ImagePrice2K: &price2K}}
	cost := svc.calculateOpenAIImageCost(context.Background(), "doubao-seedream-4-5-251128", apiKey, result, 1)
	require.InDelta(t, 0.06, cost.ActualCost, 1e-9)
}
