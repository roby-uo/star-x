//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestModelEndpointCapabilityKeepsUnknownAndVisionReasoningModels(t *testing.T) {
	for _, tc := range []struct {
		model, kind, endpoint string
	}{
		{"doubao-seedream-5-0-260128", "image", "/v1/images/generations"},
		{"gpt-image-2", "image", "/v1/images/generations"},
		{"MiniMax-H3", "video", "/v2/video_generation"},
		{"MiniMax-H3-Max", "video", "/v2/video_generation"},
		{"grok-imagine-video", "video", "/v1/videos/generations"},
		{"gpt-5.6", "unknown", ""},
		{"qwen3-vl", "unknown", ""},
		{"gemini-2.5-pro", "unknown", ""},
		{"my-video-understanding-model", "unknown", ""},
	} {
		t.Run(tc.model, func(t *testing.T) {
			platform := PlatformOpenAI
			if tc.model == "grok-imagine-video" {
				platform = PlatformGrok
			}
			capability, err := (&GatewayService{}).ResolveModelEndpointCapability(context.Background(), nil, platform, tc.model)
			require.NoError(t, err)
			require.Equal(t, tc.kind, capability.ModelType)
			if tc.endpoint == "" {
				require.Empty(t, capability.SupportedEndpoints)
				require.False(t, capability.IsGenerationModel())
			} else {
				require.Contains(t, capability.SupportedEndpoints, tc.endpoint)
			}
		})
	}
}

func TestModelEndpointCapabilityUsesPolicyAndChannelAliases(t *testing.T) {
	groupID := int64(10)
	channel := Channel{
		ID: 1, Status: StatusActive, GroupIDs: []int64{groupID},
		ModelMapping: map[string]map[string]string{PlatformOpenAI: {"video-alias": "MiniMax-H3-Max", "image-alias": "doubao-seedream-5-0-260128"}},
		FeaturesConfig: map[string]any{"model_services": []ModelServicePolicy{
			{Platform: PlatformOpenAI, Model: "image-alias", Kind: "image", State: "published"},
			{Platform: PlatformOpenAI, Model: "video-alias", Kind: "video", State: "published"},
			{Platform: PlatformOpenAI, Model: "custom-image", Kind: "image", State: "published"},
			{Platform: PlatformOpenAI, Model: "custom-reasoner", Kind: "chat", State: "published"},
		}},
	}
	channels := &ChannelService{}
	channels.cache.Store(populateChannelCache([]Channel{channel}, map[int64]string{groupID: PlatformOpenAI}))
	service := &GatewayService{channelService: channels}
	for model, kind := range map[string]string{"video-alias": "video", "image-alias": "image", "custom-image": "image", "custom-reasoner": "chat"} {
		capability, err := service.ResolveModelEndpointCapability(context.Background(), &groupID, PlatformOpenAI, model)
		require.NoError(t, err)
		require.Equal(t, kind, capability.ModelType)
		if model == "custom-image" {
			require.Empty(t, capability.SupportedEndpoints)
		} else {
			require.NotEmpty(t, capability.SupportedEndpoints)
		}
	}
}

func TestModelEndpointCapabilityDoesNotAdvertiseUnimplementedMediaAdapters(t *testing.T) {
	for _, tc := range []struct{ model, platform, kind string }{
		{"sora-2", PlatformOpenAI, "video"},
		{"doubao-seedance-2", PlatformOpenAI, "video"},
		{"kling-v2", PlatformOpenAI, "video"},
		{"veo-3.0", PlatformGemini, "video"},
		{"imagen-4", PlatformGemini, "image"},
		{"flux-1", PlatformOpenAI, "image"},
		{"dall-e-3", PlatformOpenAI, "image"},
		{"gpt-image-2", PlatformAnthropic, "image"},
		{"MiniMax-H3", PlatformAnthropic, "video"},
	} {
		capability, err := (&GatewayService{}).ResolveModelEndpointCapability(context.Background(), nil, tc.platform, tc.model)
		require.NoError(t, err)
		require.Equal(t, tc.kind, capability.ModelType)
		require.Empty(t, capability.SupportedEndpoints)
	}
}
