//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/minimax"
	"github.com/stretchr/testify/require"
)

func testVideoPolicy() ModelServicePolicy {
	return ModelServicePolicy{Platform: PlatformOpenAI, Model: "MiniMax-H3", Kind: "video", State: "published", Resolutions: []string{"768P", "2K"}, MinDuration: 4, MaxDuration: 15, Modes: []string{"text", "first_frame", "first_last_frame"}, Prices: map[string]float64{"768P": 0.08, "2K": 0.13}, Groups: map[string]ModelServiceRule{"1": {Enabled: true, Resolutions: []string{"768P"}, MaxDuration: 5, Modes: []string{"text"}}}}
}
func TestModelServiceGroupRulesCannotBeBypassed(t *testing.T) {
	policy := testVideoPolicy()
	cfg := map[string]any{"model_services": []ModelServicePolicy{policy}}
	require.NoError(t, ValidateModelServicePolicies(cfg))
	ch := Channel{ID: 1, Status: StatusActive, GroupIDs: []int64{1, 2}, FeaturesConfig: cfg}
	svc := &ChannelService{}
	svc.cache.Store(populateChannelCache([]Channel{ch}, map[int64]string{1: PlatformOpenAI, 2: PlatformOpenAI}))
	p, err := svc.ResolveModelServicePolicy(context.Background(), 1, "MiniMax-H3")
	require.NoError(t, err)
	input := minimax.CreateVideoRequest{Model: "MiniMax-H3", Resolution: "768P", Duration: 5, Ratio: "16:9", Content: []minimax.VideoContent{{Type: "text", Text: "sunrise"}}}
	require.NoError(t, p.ValidateVideo(input))
	input.Resolution = "2K"
	require.Error(t, p.ValidateVideo(input))
	input.Resolution = "768P"
	input.Duration = 10
	require.Error(t, p.ValidateVideo(input))
	input.Duration = 5
	input.Content = append(input.Content, minimax.VideoContent{Type: "image_url", Role: "first_frame"})
	require.Error(t, p.ValidateVideo(input))
	hidden, err := svc.ResolveModelServicePolicy(context.Background(), 2, "MiniMax-H3")
	require.NoError(t, err)
	require.Equal(t, "paused", hidden.State)
	ch.Status = "disabled"
	svc.cache.Store(populateChannelCache([]Channel{ch}, map[int64]string{1: PlatformOpenAI}))
	require.True(t, svc.IsModelRestricted(context.Background(), 1, "MiniMax-H3"))
}
func TestModelServiceRejectsIncompletePricesAndOverbroadGroups(t *testing.T) {
	p := testVideoPolicy()
	delete(p.Prices, "2K")
	require.Error(t, ValidateModelServicePolicies(map[string]any{"model_services": []ModelServicePolicy{p}}))
	p = testVideoPolicy()
	p.Groups["1"] = ModelServiceRule{Enabled: true, Resolutions: []string{"4K"}, MaxDuration: 5, Modes: []string{"text"}}
	require.Error(t, ValidateModelServicePolicies(map[string]any{"model_services": []ModelServicePolicy{p}}))
	p = testVideoPolicy()
	p.Model = "MiniMax-H3-Max"
	require.Error(t, ValidateModelServicePolicies(map[string]any{"model_services": []ModelServicePolicy{p}}))
}
func TestVideoSettlementUsesOriginalQuoteAfterPriceChange(t *testing.T) {
	svc := &OpenAIGatewayService{}
	result := &OpenAIForwardResult{VideoCount: 1, VideoDurationSeconds: 5, VideoResolution: "768P", VideoQuote: &VideoQuote{UnitPrice: .08, Multiplier: 1.4, Total: .56}}
	cost, err := svc.calculateOpenAIRecordUsageCost(context.Background(), result, nil, []string{"MiniMax-H3"}, 100, 100, 100, 100, UsageTokens{}, "", false)
	require.NoError(t, err)
	require.InDelta(t, .56, cost.ActualCost, 1e-9)
	require.InDelta(t, .4, cost.TotalCost, 1e-9)
}
func TestKeyModelAccessOnlyNarrows(t *testing.T) {
	rules := APIKeyModelAccess{RestrictModels: true, Models: []string{"MiniMax-H3"}, VideoResolutions: []string{"768P"}, VideoMaxDuration: 5}
	require.NoError(t, rules.Validate("MiniMax-H3", "768P", 5))
	require.Error(t, rules.Validate("MiniMax-H3", "2K", 5))
	require.Error(t, rules.Validate("MiniMax-H3", "768P", 6))
	require.Error(t, rules.Validate("another-model", "", 0))
	rules.Models = nil
	require.Error(t, rules.Validate("MiniMax-H3", "768P", 5))
}
