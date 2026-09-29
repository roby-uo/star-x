//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMiniMaxVideoBaseURLAndBindingScope(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"api_key": "test-key", "base_url": "https://api.minimax.cn/v1",
	}}
	base, ok := MiniMaxVideoBaseURL(account)
	require.True(t, ok)
	require.Equal(t, "https://api.minimax.cn", base)
	account.Credentials["base_url"] = "https://api.minimax.cn.evil.example/v1"
	_, ok = MiniMaxVideoBaseURL(account)
	require.False(t, ok)
	require.NotEqual(t, miniMaxVideoBindingKey("task-1", 11, 22), miniMaxVideoBindingKey("task-1", 12, 22))
	require.NotEqual(t, miniMaxVideoBindingKey("task-1", 11, 22), miniMaxVideoBindingKey("task-1", 11, 23))
}

func TestMiniMaxH3OutputSecondPricing(t *testing.T) {
	billing := newTestBillingService()
	low := billing.CalculateVideoCost("MiniMax-H3", "768P", 1, 5, nil, 2)
	high := billing.CalculateVideoCost("MiniMax-H3", "2K", 1, 5, nil, 2)
	require.InDelta(t, 0.4, low.TotalCost, 1e-10)
	require.InDelta(t, 0.8, low.ActualCost, 1e-10)
	require.InDelta(t, 0.65, high.TotalCost, 1e-10)
	require.InDelta(t, 1.3, high.ActualCost, 1e-10)
	require.Equal(t, VideoBillingResolution2K, NormalizeVideoBillingResolutionOrDefault("2K"))
}
