package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/minimax"
	"github.com/tidwall/gjson"
)

// ModelServicePolicy lives in the channel configuration. Group overrides narrow
// capability without changing the existing group/user multiplier precedence.
type ModelServicePolicy struct {
	Platform    string                      `json:"platform"`
	Model       string                      `json:"model"`
	ImageSizes  []string                    `json:"image_sizes,omitempty"`
	MaxImages   int                         `json:"max_images,omitempty"`
	Kind        string                      `json:"kind"`
	State       string                      `json:"state"`
	Resolutions []string                    `json:"resolutions,omitempty"`
	MinDuration int                         `json:"min_duration,omitempty"`
	MaxDuration int                         `json:"max_duration,omitempty"`
	Modes       []string                    `json:"modes,omitempty"`
	Prices      map[string]float64          `json:"prices,omitempty"`
	Groups      map[string]ModelServiceRule `json:"groups,omitempty"`
}
type ModelServiceRule struct {
	Enabled     bool     `json:"enabled"`
	Resolutions []string `json:"resolutions,omitempty"`
	MaxDuration int      `json:"max_duration,omitempty"`
	Modes       []string `json:"modes,omitempty"`
}

func ParseModelServicePolicies(config map[string]any) ([]ModelServicePolicy, error) {
	raw, exists := config["model_services"]
	if !exists {
		return nil, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var policies []ModelServicePolicy
	if err = json.Unmarshal(data, &policies); err != nil {
		return nil, fmt.Errorf("invalid model services")
	}
	return policies, nil
}

func ValidateModelServicePolicies(config map[string]any) error {
	policies, err := ParseModelServicePolicies(config)
	if err != nil {
		return infraerrors.BadRequest("INVALID_MODEL_SERVICE", err.Error())
	}
	seen := map[string]bool{}
	invalid := func(message string) error { return infraerrors.BadRequest("INVALID_MODEL_SERVICE", message) }
	for _, p := range policies {
		key := p.Platform + ":" + strings.ToLower(p.Model)
		if p.Model == "" || p.Platform == "" || seen[key] {
			return invalid("模型服务必须指定平台及唯一模型名称")
		}
		seen[key] = true
		if !slices.Contains([]string{"chat", "image", "video"}, p.Kind) || !slices.Contains([]string{"draft", "published", "paused"}, p.State) {
			return invalid("模型类型或发布状态无效")
		}
		if p.MaxImages < 0 || p.MaxImages > 15 {
			return invalid("单次图片数量限制应在 0–15 范围内；0 表示沿用上游")
		}
		if p.Kind != "video" {
			continue
		}
		if p.Model != "MiniMax-H3" || p.Platform != PlatformOpenAI {
			return invalid("规格定价目前仅支持已适配的 MiniMax-H3；其他视频模型沿用原有配置")
		}
		if p.MinDuration < 4 || p.MaxDuration > 15 || p.MaxDuration < p.MinDuration {
			return invalid("H3 时长须在 4–15 秒内")
		}
		if len(p.Resolutions) == 0 || len(p.Modes) == 0 {
			return invalid("请选择分辨率及生成方式")
		}
		for _, resolution := range p.Resolutions {
			price, ok := p.Prices[resolution]
			if !slices.Contains([]string{"768P", "2K"}, resolution) || !ok || price < 0 || math.IsNaN(price) || math.IsInf(price, 0) {
				return invalid("每个开放分辨率都必须配置有效的每秒价格")
			}
		}
		for _, mode := range p.Modes {
			if !slices.Contains([]string{"text", "first_frame", "first_last_frame"}, mode) {
				return invalid("不支持的输入方式")
			}
		}
		for id, rule := range p.Groups {
			gid, err := strconv.ParseInt(id, 10, 64)
			if err != nil || gid <= 0 {
				return invalid("分组 ID 无效")
			}
			if !rule.Enabled {
				continue
			}
			if rule.MaxDuration < p.MinDuration || rule.MaxDuration > p.MaxDuration || len(rule.Resolutions) == 0 || len(rule.Modes) == 0 {
				return invalid("分组必须选择有效规格和时长")
			}
			for _, r := range rule.Resolutions {
				if !slices.Contains(p.Resolutions, r) {
					return invalid("分组分辨率不能超过服务范围")
				}
			}
			for _, m := range rule.Modes {
				if !slices.Contains(p.Modes, m) {
					return invalid("分组生成方式不能超过服务范围")
				}
			}
		}
	}
	return nil
}

// ResolveModelServicePolicy deliberately reads disabled channels too: pausing
// a configured service must never fall back to unrestricted legacy behavior.
func (s *ChannelService) ResolveModelServicePolicy(ctx context.Context, groupID int64, model string) (*ModelServicePolicy, error) {
	if s == nil {
		return nil, nil
	}
	cache, err := s.loadCache(ctx)
	if err != nil {
		return nil, err
	}
	ch := cache.channelByGroupID[groupID]
	if ch == nil {
		return nil, nil
	}
	policies, err := ParseModelServicePolicies(ch.FeaturesConfig)
	if err != nil {
		return nil, err
	}
	for _, policy := range policies {
		if !strings.EqualFold(policy.Model, model) || policy.Platform != cache.groupPlatform[groupID] {
			continue
		}
		p := policy
		if !ch.IsActive() {
			p.State = "paused"
		}
		if rule, exists := p.Groups[strconv.FormatInt(groupID, 10)]; exists {
			if !rule.Enabled {
				p.State = "paused"
			}
			if p.Kind == "video" {
				p.Resolutions = rule.Resolutions
				p.MaxDuration = rule.MaxDuration
				p.Modes = rule.Modes
			}
		}
		if p.Groups != nil {
			if _, exists := p.Groups[strconv.FormatInt(groupID, 10)]; !exists {
				p.State = "paused"
			}
		}
		p.Groups = nil
		return &p, nil
	}
	return nil, nil
}

func (s *OpenAIGatewayService) MiniMaxServicePolicy(ctx context.Context, key *APIKey) (*ModelServicePolicy, error) {
	if key == nil || key.GroupID == nil || key.Group == nil {
		return nil, fmt.Errorf("需要绑定分组")
	}
	return s.channelService.ResolveModelServicePolicy(ctx, *key.GroupID, "MiniMax-H3")
}

func (p *ModelServicePolicy) ValidateVideo(input minimax.CreateVideoRequest) error {
	if p == nil {
		return nil
	}
	if p.State != "published" {
		return fmt.Errorf("该分组尚未开放此视频服务")
	}
	if !slices.Contains(p.Resolutions, input.Resolution) || input.Duration < p.MinDuration || input.Duration > p.MaxDuration {
		return fmt.Errorf("该分组未开放所选分辨率或时长")
	}
	mode := "text"
	first, last := false, false
	for _, item := range input.Content {
		if item.Type == "image_url" {
			if item.Role == "last_frame" {
				last = true
			} else {
				first = true
			}
		}
	}
	if last && !first {
		return fmt.Errorf("尾帧必须与首帧同时提供")
	}
	if first {
		mode = "first_frame"
	}
	if last {
		mode = "first_last_frame"
	}
	if !slices.Contains(p.Modes, mode) {
		return fmt.Errorf("该分组未开放所选生成方式")
	}
	return nil
}

type VideoQuote struct {
	Currency   string  `json:"currency"`
	UnitPrice  float64 `json:"unit_price"`
	Multiplier float64 `json:"multiplier"`
	Total      float64 `json:"total"`
	Version    string  `json:"version"`
}

func (s *OpenAIGatewayService) QuoteMiniMaxVideo(ctx context.Context, key *APIKey, input minimax.CreateVideoRequest) (*VideoQuote, error) {
	policy, err := s.MiniMaxServicePolicy(ctx, key)
	if err != nil {
		return nil, err
	}
	if err = policy.ValidateVideo(input); err != nil {
		return nil, err
	}
	if policy == nil && !GroupAllowsImageGeneration(key.Group) {
		return nil, fmt.Errorf("该分组尚未开放视频生成")
	}
	price := 0.08
	if input.Resolution == "2K" {
		price = 0.13
	}
	if policy != nil {
		var ok bool
		price, ok = policy.Prices[input.Resolution]
		if !ok {
			return nil, fmt.Errorf("视频规格尚未定价")
		}
	}
	multiplier := s.ResolveUserGroupRateMultiplier(ctx, key.UserID, *key.GroupID, key.Group.RateMultiplier)
	multiplier = resolveVideoRateMultiplier(key, multiplier)
	if s.cfg != nil && s.cfg.RunMode == "simple" {
		multiplier = 0
	}
	q := &VideoQuote{Currency: "USD", UnitPrice: price, Multiplier: multiplier, Total: price * float64(input.Duration) * multiplier}
	data, _ := json.Marshal([]any{input.Model, input.Resolution, input.Duration, price, multiplier, policy})
	q.Total = math.Round(q.Total*1e10) / 1e10
	q.Version = HashUsageRequestPayload(data)
	return q, nil
}

func (s *OpenAIGatewayService) ValidateImageServiceRequest(ctx context.Context, key *APIKey, req *OpenAIImagesRequest) error {
	if key == nil || key.GroupID == nil || s.channelService == nil {
		return nil
	}
	policy, err := s.channelService.ResolveModelServicePolicy(ctx, *key.GroupID, req.Model)
	if err != nil {
		return err
	}
	if policy == nil {
		return nil
	}
	if policy.State != "published" {
		return fmt.Errorf("图片模型服务尚未向此分组开放")
	}
	count := req.N
	if gjson.GetBytes(req.Body, "sequential_image_generation").String() == "auto" {
		maximum := int(gjson.GetBytes(req.Body, "sequential_image_generation_options.max_images").Int())
		if maximum <= 0 {
			maximum = 15
		}
		if maximum > count {
			count = maximum
		}
	}
	if policy.MaxImages > 0 && count > policy.MaxImages {
		return fmt.Errorf("图片数量超出服务限制")
	}
	if len(policy.ImageSizes) > 0 && !slices.Contains(policy.ImageSizes, req.Size) {
		return fmt.Errorf("请选择已开放的图片尺寸")
	}
	return nil
}
