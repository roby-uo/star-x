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
	for _, model := range []string{"MiniMax-H3", "MiniMax-H3-Max"} {
		cost, err := svc.calculateOpenAIRecordUsageCost(context.Background(), result, nil, []string{model}, 100, 100, 100, 100, UsageTokens{}, "", false)
		require.NoError(t, err)
		require.InDelta(t, .56, cost.ActualCost, 1e-9)
		require.InDelta(t, .4, cost.TotalCost, 1e-9)
	}
}

func testVideoMaxPolicy() ModelServicePolicy {
	p := testVideoPolicy()
	p.Model = "MiniMax-H3-Max"
	p.Resolutions = []string{"480P", "768P"}
	p.MinDuration = 5
	p.Prices = map[string]float64{"480P": .05, "768P": .08}
	p.Groups["1"] = ModelServiceRule{Enabled: true, Resolutions: p.Resolutions, MaxDuration: 15, Modes: p.Modes}
	return p
}

func TestMiniMaxH3MaxPolicyAndQuoteAreModelSpecific(t *testing.T) {
	p := testVideoMaxPolicy()
	h3 := testVideoPolicy()
	h3.Prices["768P"] = 3
	cfg := map[string]any{"model_services": []ModelServicePolicy{p, h3}}
	require.NoError(t, ValidateModelServicePolicies(cfg))
	channel := &ChannelService{}
	channel.cache.Store(populateChannelCache([]Channel{{ID: 1, Status: StatusActive, GroupIDs: []int64{1, 2}, FeaturesConfig: cfg}}, map[int64]string{1: PlatformOpenAI, 2: PlatformOpenAI}))
	svc := &OpenAIGatewayService{channelService: channel}
	gid := int64(1)
	key := &APIKey{UserID: 1, GroupID: &gid, Group: &Group{ID: gid, Platform: PlatformOpenAI, RateMultiplier: 1.4}}
	input := minimax.CreateVideoRequest{Model: p.Model, Resolution: "480P", Duration: 5, Ratio: "16:9", Content: []minimax.VideoContent{{Type: "text", Text: "sunrise"}}}
	quote, err := svc.QuoteMiniMaxVideo(context.Background(), key, input)
	require.NoError(t, err)
	require.InDelta(t, .05, quote.UnitPrice, 1e-10)
	require.InDelta(t, .35, quote.Total, 1e-10)
	require.Equal(t, "USD", quote.Currency)
	input.Resolution = "768P"
	nextQuote, err := svc.QuoteMiniMaxVideo(context.Background(), key, input)
	require.NoError(t, err)
	require.InDelta(t, .56, nextQuote.Total, 1e-10)
	require.NotEqual(t, quote.Version, nextQuote.Version)
	key.Group.VideoRateIndependent, key.Group.VideoRateMultiplier = true, .5
	nextQuote, err = svc.QuoteMiniMaxVideo(context.Background(), key, input)
	require.NoError(t, err)
	require.InDelta(t, .2, nextQuote.Total, 1e-10)
	input.Resolution = "2K"
	_, err = svc.QuoteMiniMaxVideo(context.Background(), key, input)
	require.Error(t, err, "Max cannot inherit H3's 2K capability")
	input.Resolution, input.Duration = "768P", 4
	_, err = svc.QuoteMiniMaxVideo(context.Background(), key, input)
	require.Error(t, err, "Max cannot inherit H3's four-second duration")
	input.Duration = 5
	gid = 2
	_, err = svc.QuoteMiniMaxVideo(context.Background(), key, input)
	require.Error(t, err, "group with no Max rule must not inherit H3 access")
	require.Error(t, h3.ValidateVideo(input), "a different model's policy must not validate Max")
}

func TestMiniMaxH3MaxPolicyRejectsUnsupportedSpecifications(t *testing.T) {
	p := testVideoMaxPolicy()
	p.MinDuration = 4
	require.Error(t, ValidateModelServicePolicies(map[string]any{"model_services": []ModelServicePolicy{p}}))
	p = testVideoMaxPolicy()
	p.Resolutions = append(p.Resolutions, "2K")
	p.Prices["2K"] = .13
	require.Error(t, ValidateModelServicePolicies(map[string]any{"model_services": []ModelServicePolicy{p}}))
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
