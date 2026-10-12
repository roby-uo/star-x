package service

import (
	"context"
	"regexp"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/minimax"
)

// ModelEndpointCapability describes generation purpose, not tool-calling or
// visual-input support. Unknown names remain callable for custom upstreams.
type ModelEndpointCapability struct {
	ModelType          string   `json:"model_type"`
	SupportedEndpoints []string `json:"supported_endpoints"`
}

var knownImageGenerationPattern = regexp.MustCompile(`(?i)^(?:(?:doubao-)?seedream-|gpt-image(?:-|$)|dall-e-|imagen[-.]|flux[-.])`)
var knownVideoGenerationPattern = regexp.MustCompile(`(?i)^(?:minimax-h3(?:-max)?$|(?:doubao-)?seedance[-.]|kling[-.]|sora(?:[-.]|$)|veo(?:[-.]|$))`)

func (p ModelEndpointCapability) IsGenerationModel() bool {
	return p.ModelType == "image" || p.ModelType == "video"
}

func inferModelEndpointCapability(model, platform string) ModelEndpointCapability {
	model = strings.TrimSpace(model)
	switch {
	case isOpenAIImageGenerationModel(model), isSeedreamImageGenerationModel(model):
		endpoints := []string{}
		if platform == PlatformOpenAI || (platform == PlatformGrok && isGrokImageGenerationModel(model)) {
			endpoints = []string{"/v1/images/generations"}
		}
		return ModelEndpointCapability{ModelType: "image", SupportedEndpoints: endpoints}
	case minimax.IsVideoModel(model):
		endpoints := []string{}
		if platform == PlatformOpenAI {
			endpoints = []string{"/v2/video_generation"}
		}
		return ModelEndpointCapability{ModelType: "video", SupportedEndpoints: endpoints}
	case strings.HasPrefix(strings.ToLower(model), "grok-imagine-video"):
		endpoints := []string{}
		if platform == PlatformGrok {
			endpoints = []string{"/v1/videos/generations"}
		}
		return ModelEndpointCapability{ModelType: "video", SupportedEndpoints: endpoints}
	case knownImageGenerationPattern.MatchString(model):
		return ModelEndpointCapability{ModelType: "image", SupportedEndpoints: []string{}}
	case knownVideoGenerationPattern.MatchString(model):
		return ModelEndpointCapability{ModelType: "video", SupportedEndpoints: []string{}}
	default:
		return ModelEndpointCapability{ModelType: "unknown", SupportedEndpoints: []string{}}
	}
}

func resolveModelEndpointCapability(ctx context.Context, channels *ChannelService, groupID *int64, platform, model string) (ModelEndpointCapability, error) {
	capability := inferModelEndpointCapability(model, platform)
	if channels == nil || groupID == nil {
		return capability, nil
	}
	policy, err := channels.ResolveModelServicePolicy(ctx, *groupID, model)
	if err != nil {
		return ModelEndpointCapability{}, err
	}
	mapping := channels.ResolveChannelMapping(ctx, *groupID, model)
	if mapping.Mapped {
		capability = inferModelEndpointCapability(mapping.MappedModel, platform)
	}
	if capability.ModelType == "unknown" {
		if aliasCapability, found := channels.cachedAccountAliasCapability(groupID, platform, mapping.MappedModel); found {
			capability = aliasCapability
		}
	}
	if policy != nil {
		switch policy.Kind {
		case "image":
			if capability.ModelType != "image" {
				capability = ModelEndpointCapability{ModelType: "image", SupportedEndpoints: []string{}}
			}
			return capability, nil
		case "video":
			if capability.ModelType != "video" {
				capability = ModelEndpointCapability{ModelType: "video", SupportedEndpoints: []string{}}
			}
			return capability, nil
		case "chat":
			if capability.IsGenerationModel() {
				return capability, nil
			}
			return ModelEndpointCapability{ModelType: "chat", SupportedEndpoints: textModelEndpoints(platform, model)}, nil
		}
	}
	return capability, nil
}

func textModelEndpoints(platform, model string) []string {
	if platform == PlatformGemini || (platform == PlatformAntigravity && !strings.Contains(strings.ToLower(model), "claude")) {
		return []string{"/v1beta/models/" + model + ":generateContent"}
	}
	if platform == PlatformOpenAI {
		return []string{"/v1/chat/completions", "/v1/responses"}
	}
	return []string{"/v1/chat/completions", "/v1/responses", "/v1/messages"}
}

func (s *GatewayService) ResolveModelEndpointCapability(ctx context.Context, groupID *int64, platform, model string) (ModelEndpointCapability, error) {
	var channels *ChannelService
	if s != nil {
		channels = s.channelService
	}
	return resolveModelEndpointCapability(ctx, channels, groupID, platform, model)
}

func (s *OpenAIGatewayService) ResolveModelEndpointCapability(ctx context.Context, groupID *int64, platform, model string) (ModelEndpointCapability, error) {
	var channels *ChannelService
	if s != nil {
		channels = s.channelService
	}
	return resolveModelEndpointCapability(ctx, channels, groupID, platform, model)
}

// AccountModelEndpointCapability uses the selected account's actual mapping.
// This guards cold caches and aliases with different targets across accounts.
func AccountModelEndpointCapability(account *Account, platform, model, messagesDispatchModel string) ModelEndpointCapability {
	return inferModelEndpointCapability(resolveOpenAIForwardModel(account, model, messagesDispatchModel), platform)
}
