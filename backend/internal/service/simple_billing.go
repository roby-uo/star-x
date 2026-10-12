package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

// SimpleBillingInput identifies the actual upstream procurement route. The
// customer's API key is not a procurement type: an OAuth upstream is an account
// route even when the customer accesses it using an API key.
type SimpleBillingInput struct {
	Account         *Account
	Group           *Group
	Model           string
	Modality        string
	Resolution      string
	Quality         string
	ServiceTier     string
	InputImageCount int
	Scene           string
	HasImageInput   bool
	Operation       string
}

// SimpleBillingSnapshot is captured before an upstream call. It contains only
// prices and identifiers, never credentials or prompts, and is immutable after
// preparation. Dollar amounts retain the existing ledger/API semantics.
type SimpleBillingSnapshot struct {
	Version                  int64                  `json:"version"`
	AccountID                int64                  `json:"account_id"`
	Model                    string                 `json:"model"`
	Modality                 string                 `json:"modality"`
	Provider                 string                 `json:"provider"`
	Region                   string                 `json:"region"`
	Source                   string                 `json:"source"`
	Route                    string                 `json:"route"`
	Multiplier               float64                `json:"multiplier"`
	Currency                 string                 `json:"currency"`
	Resolution               string                 `json:"resolution,omitempty"`
	Quality                  string                 `json:"quality,omitempty"`
	ServiceTier              string                 `json:"service_tier,omitempty"`
	ProfileID                string                 `json:"profile_id,omitempty"`
	DiscountMultiplier       float64                `json:"discount_multiplier"`
	Rows                     []CostPriceRow         `json:"rows,omitempty"`
	ReferencePricing         *ModelPricing          `json:"reference_pricing,omitempty"`
	UnitPriceUSD             *float64               `json:"unit_price_usd,omitempty"`
	BillingUnit              string                 `json:"billing_unit"`
	Hash                     string                 `json:"hash"`
	InputImageCount          int                    `json:"input_image_count,omitempty"`
	Scene                    string                 `json:"image_scene,omitempty"`
	ImageBilling             *SimpleBillingSnapshot `json:"image_billing,omitempty"`
	LowPixelUnitPrice        *float64               `json:"low_pixel_unit_price,omitempty"`
	HighPixelUnitPrice       *float64               `json:"high_pixel_unit_price,omitempty"`
	PixelPriceThreshold      int64                  `json:"pixel_price_threshold,omitempty"`
	Operation                string                 `json:"operation,omitempty"`
	RejectSeparateCacheWrite bool                   `json:"reject_separate_cache_write,omitempty"`
}

func decodeBillingManagementConfig(raw string) (*BillingManagementConfig, error) {
	defaults := DefaultBillingManagementConfig()
	cfg := &defaults
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), cfg); err != nil {
			return nil, fmt.Errorf("invalid persisted billing management config: %w", err)
		}
	}
	if cfg.APISaleMultiplier == 0 {
		cfg.APISaleMultiplier = 1.4
	}
	cfg.USDToCNYRate = FixedUSDToCNYRate
	if cfg.CostProfiles == nil {
		cfg.CostProfiles = []CostProfile{}
	}
	return cfg, nil
}

func DefaultBillingManagementConfig() BillingManagementConfig {
	return BillingManagementConfig{SimpleBillingEnabled: true, APISaleMultiplier: 1.4, USDToCNYRate: FixedUSDToCNYRate, CostProfiles: []CostProfile{}}
}

// ProvideBillingService attaches the independent price book in production.
// NewBillingService remains usable by legacy unit tests without a database.
func ProvideBillingService(cfg *config.Config, pricing *PricingService, repo BillingManagementRepository) *BillingService {
	s := NewBillingService(cfg, pricing)
	s.simpleRepo = repo
	return s
}

func (s *BillingService) invalidateSimpleBillingConfig() {
	s.simpleMu.Lock()
	defer s.simpleMu.Unlock()
	s.simpleConfig, s.simpleConfigAt = nil, time.Time{}
}

func (s *BillingService) simpleBillingConfig(ctx context.Context) (*BillingManagementConfig, error) {
	if s == nil || s.simpleRepo == nil {
		return nil, nil
	}
	s.simpleMu.Lock()
	defer s.simpleMu.Unlock()
	if s.simpleConfig != nil && time.Since(s.simpleConfigAt) < 15*time.Second {
		return s.simpleConfig, nil
	}
	raw, err := s.simpleRepo.Get(ctx)
	if err != nil {
		return nil, err
	}
	cfg, err := decodeBillingManagementConfig(raw)
	if err != nil {
		return nil, err
	}
	if err := ValidateBillingManagementConfig(cfg); err != nil {
		return nil, err
	}
	s.simpleConfig, s.simpleConfigAt = cfg, time.Now()
	return cfg, nil
}

func (s *BillingService) PrepareSimpleBilling(ctx context.Context, in SimpleBillingInput) (*SimpleBillingSnapshot, error) {
	cfg, err := s.simpleBillingConfig(ctx)
	if err != nil {
		return nil, err
	}
	if cfg == nil || !cfg.SimpleBillingEnabled {
		return nil, nil
	}
	return s.resolveSimpleBilling(cfg, in, time.Now())
}

func (s *OpenAIGatewayService) PrepareSimpleBilling(ctx context.Context, in SimpleBillingInput) (*SimpleBillingSnapshot, error) {
	if s == nil || s.billingService == nil {
		return nil, nil
	}
	return s.billingService.PrepareSimpleBilling(ctx, in)
}

func (s *GatewayService) PrepareSimpleBilling(ctx context.Context, in SimpleBillingInput) (*SimpleBillingSnapshot, error) {
	if s == nil || s.billingService == nil {
		return nil, nil
	}
	return s.billingService.PrepareSimpleBilling(ctx, in)
}

func simplePriceError(reason string) error {
	return infraerrors.BadRequest("PRICING_UNCONFIRMED", "此模型或规格的计费基准尚未确认，请联系管理员配置价格："+reason)
}

func procurementRoute(account *Account) string {
	if account != nil {
		if u, err := url.Parse(account.GetCredential("base_url")); err == nil && (strings.Contains(u.Path, "/coding/") || strings.Contains(u.Path, "/coding-plan/")) {
			return "account"
		}
	}
	if account != nil {
		switch account.Type {
		case AccountTypeAPIKey, AccountTypeServiceAccount, AccountTypeBedrock, AccountTypeUpstream:
			return "api"
		}
	}
	return "account"
}

func simpleAccountProvider(account *Account) (provider, region string, official bool) {
	if account == nil {
		return "", "", false
	}
	// Cloud credentials do not buy the direct provider's API. Region-specific
	// Bedrock/Vertex contracts require an explicitly bound procurement table.
	if account.Type == AccountTypeBedrock {
		region := strings.TrimSpace(account.GetCredential("aws_region"))
		if region == "" {
			region = "unknown"
		}
		return "bedrock", region, false
	}
	if account.Type == AccountTypeServiceAccount {
		region := strings.TrimSpace(account.GetCredential("location"))
		if region == "" {
			region = strings.TrimSpace(account.GetCredential("vertex_location"))
		}
		if region == "" {
			region = "unknown"
		}
		return "vertex", region, false
	}
	base := strings.TrimSpace(account.GetCredential("base_url"))
	if base == "" {
		if account.IsShadow() {
			return "shadow", "custom", false
		}
		if account.Type == AccountTypeUpstream {
			return "upstream", "custom", false
		}
		switch account.Platform {
		case PlatformOpenAI:
			return "openai", "global", true
		case PlatformAnthropic:
			return "anthropic", "global", true
		case PlatformGemini:
			return "gemini", "global", true
		case PlatformGrok:
			return "xai", "global", true
		case PlatformAntigravity:
			return "google", "global", false
		}
	}
	u, err := url.Parse(base)
	if err != nil {
		return "", "", false
	}
	host := strings.ToLower(u.Hostname())
	if u.Scheme != "https" || u.User != nil || (u.Port() != "" && u.Port() != "443") {
		return host, "custom", false
	}
	switch host {
	case "api.minimax.cn":
		return "minimax", "cn", true
	case "api.minimax.io":
		return "minimax", "global", true
	case "api.openai.com":
		return "openai", "global", true
	case "api.anthropic.com":
		return "anthropic", "global", true
	case "generativelanguage.googleapis.com":
		return "gemini", "global", true
	case "api.x.ai":
		return "xai", "global", true
	case "api.deepseek.com":
		return "deepseek", "cn", true
	case "open.bigmodel.cn":
		return "zhipu", "cn", true
	case "api.z.ai":
		return "zhipu", "global", true
	case "ark.cn-beijing.volces.com", "ark.cn-shanghai.volces.com":
		return "volcengine", "cn", true
	case "api.moonshot.cn":
		return "moonshot", "cn", true
	case "api.moonshot.ai":
		return "moonshot", "global", true
	}
	return host, "custom", false
}

func (s *BillingService) resolveSimpleBilling(cfg *BillingManagementConfig, in SimpleBillingInput, now time.Time) (*SimpleBillingSnapshot, error) {
	if in.Account == nil || in.Group == nil || strings.TrimSpace(in.Model) == "" {
		return nil, simplePriceError("缺少上游账号、分组或型号")
	}
	// Callers pass the final upstream model; applying account mappings again
	// could incorrectly turn an A -> B alias into B -> C.
	model := strings.TrimSpace(in.Model)
	provider, region, official := simpleAccountProvider(in.Account)
	q := &SimpleBillingSnapshot{Version: cfg.Version, AccountID: in.Account.ID, Model: model, Modality: in.Modality, Provider: provider, Region: region, Route: procurementRoute(in.Account), Multiplier: cfg.APISaleMultiplier, Currency: "USD", Resolution: in.Resolution, Quality: in.Quality, ServiceTier: normalizeBillingServiceTier(in.ServiceTier), DiscountMultiplier: 1, Operation: in.Operation}
	q.InputImageCount, q.Scene = in.InputImageCount, in.Scene
	if q.InputImageCount < 0 {
		return nil, simplePriceError("输入图片数量无效")
	}
	if q.ServiceTier == "auto" || q.ServiceTier == "default" {
		q.ServiceTier = ""
	}
	if !slices.Contains([]string{"chat", "image", "video"}, q.Modality) {
		return nil, simplePriceError("不支持的计费类型")
	}
	var profile *CostProfile
	for i := range cfg.CostProfiles {
		p := &cfg.CostProfiles[i]
		if !p.Enabled || !slices.Contains(p.AccountIDs, in.Account.ID) || (p.ValidFrom != nil && now.Before(*p.ValidFrom)) || (p.ValidUntil != nil && !now.Before(*p.ValidUntil)) {
			continue
		}
		if profile != nil {
			return nil, simplePriceError("同一上游账号绑定了多个有效采购方案")
		}
		profile = p
	}
	if profile != nil && profile.PurchaseMode == "subscription" {
		q.Route = "account"
	}
	if q.Operation == "alpha_search" {
		return s.resolveSimpleSearchBilling(q, in.Group, profile)
	}
	if q.Route == "account" {
		q.Multiplier, q.Source, q.BillingUnit = in.Group.RateMultiplier, "reference", "per_million_tokens"
		if profile != nil {
			q.Rows = matchingSimpleProfileRows(profile, q)
			if len(q.Rows) > 0 {
				if profile.Region != "" && !strings.EqualFold(profile.Region, region) {
					return nil, simplePriceError("参考价表地区与上游账号不一致")
				}
				q.ProfileID, q.Currency, q.BillingUnit = profile.ID, profile.Currency, q.Rows[0].BillingUnit
				// A monthly purchase discount is not a per-request reference price.
				q.DiscountMultiplier = 1
				if err := validateConfirmedSimpleRows(q, in.HasImageInput); err != nil {
					return nil, err
				}
			}
		}
		if len(q.Rows) == 0 && region == "cn" && official {
			if provider == "minimax" {
				q.Rows = miniMaxChinaSimplePrices(model, q.Modality, q.Resolution, q.ServiceTier)
			} else {
				q.Rows = chinaSimplePrices(provider, model, q.Modality, q.Resolution, q.ServiceTier, q.Scene)
			}
			if len(q.Rows) > 0 {
				q.Currency = "CNY"
				q.BillingUnit = q.Rows[0].BillingUnit
			} else {
				return nil, simplePriceError("中国站的账户参考价尚未维护，请确认人民币基准，不能挪用国际价格")
			}
		}
		if len(q.Rows) > 0 {
			// Subscription procurement uses the region's published reference,
			// never a claimed per-request amortization of the monthly payment.
		} else if q.Modality == "video" || q.Modality == "image" {
			unit, billingUnit, err := s.strictMediaPrice(model, q.Modality, q.Resolution)
			if err == nil {
				q.UnitPriceUSD, q.BillingUnit = &unit, billingUnit
			} else if q.Modality == "image" {
				pricing, priceErr := s.strictImageTokenPricing(model)
				if priceErr != nil {
					return nil, err
				}
				q.ReferencePricing = pricing
			} else {
				return nil, err
			}
		} else {
			pricing, err := s.strictReferencePricing(model)
			if err != nil {
				return nil, err
			}
			q.ReferencePricing = pricing
		}
	} else if profile != nil {
		if profile.PurchaseMode != "payg" {
			return nil, simplePriceError("额度包或混合采购须先确认按量基准价，不能猜测单次成本")
		}
		if profile.Region != "" && !strings.EqualFold(profile.Region, region) {
			return nil, simplePriceError("采购价表地区与上游账号地区不一致")
		}
		q.Source, q.ProfileID, q.Currency, q.DiscountMultiplier = "manual", profile.ID, profile.Currency, profile.DiscountMultiplier
		q.Rows = matchingSimpleProfileRows(profile, q)
		if len(q.Rows) == 0 {
			return nil, simplePriceError("采购价表没有此模型或规格")
		}
		q.BillingUnit = q.Rows[0].BillingUnit
		if err := validateConfirmedSimpleRows(q, in.HasImageInput); err != nil {
			return nil, err
		}
	} else if official {
		// The maintained price book is a published reference, not the supplier's
		// invoice or automatic discovery of this account's private contract.
		q.Source = "official_reference"
		if region == "cn" {
			q.Currency = "CNY"
			q.Source = "official_reference"
			if provider == "minimax" {
				q.Rows = miniMaxChinaSimplePrices(model, q.Modality, q.Resolution, q.ServiceTier)
			} else {
				q.Rows = chinaSimplePrices(provider, model, q.Modality, q.Resolution, q.ServiceTier, q.Scene)
			}
			if len(q.Rows) == 0 {
				return nil, simplePriceError("中国站尚无此型号、服务档位或规格的已维护人民币价表")
			}
			q.BillingUnit = q.Rows[0].BillingUnit
		} else if q.Modality == "chat" {
			pricing, err := s.strictReferencePricing(model)
			if err != nil {
				return nil, err
			}
			q.ReferencePricing, q.BillingUnit = pricing, "per_million_tokens"
		} else {
			unit, billingUnit, err := s.strictMediaPrice(model, q.Modality, q.Resolution)
			if err != nil {
				if q.Modality != "image" {
					return nil, err
				}
				pricing, priceErr := s.strictImageTokenPricing(model)
				if priceErr != nil {
					return nil, err
				}
				q.ReferencePricing, q.BillingUnit = pricing, "per_million_tokens"
			} else {
				q.UnitPriceUSD, q.BillingUnit = &unit, billingUnit
			}
		}
	} else {
		return nil, simplePriceError("此供应商的实付价无法自动识别，请关联采购价表")
	}
	if !finiteNonnegative(q.Multiplier) {
		return nil, simplePriceError("销售倍率无效")
	}
	if len(q.Rows) > 0 {
		if err := validateConfirmedSimpleRows(q, in.HasImageInput); err != nil {
			return nil, err
		}
	}
	// The published K2 schedule charges uncached input and cache hits, without
	// a separate cache-creation bucket. An unexpected new bucket needs review;
	// it cannot be silently interpreted as confirmed free usage.
	if q.Provider == "moonshot" && q.Region == "cn" && q.ProfileID == "" && slices.Contains([]string{"kimi-k2.6", "kimi-k2.7-code", "kimi-k2.7-code-highspeed"}, strings.ToLower(q.Model)) {
		q.RejectSeparateCacheWrite = true
	}
	if q.Provider == "volcengine" && strings.EqualFold(q.Model, "doubao-seedream-5-0-pro-260628") && len(q.Rows) > 0 && q.ProfileID == "" {
		low, high := .30, .60
		if q.Scene == "layered" {
			low /= 2
			high /= 2
		}
		q.LowPixelUnitPrice, q.HighPixelUnitPrice, q.PixelPriceThreshold = &low, &high, 2610000
	}
	if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		q.Multiplier = 0
	}
	q.RefreshHash()
	return q, nil
}

func matchingSimpleProfileRows(profile *CostProfile, q *SimpleBillingSnapshot) []CostPriceRow {
	var rows []CostPriceRow
	for _, row := range profile.Prices {
		if row.Operation != q.Operation {
			continue
		}
		if q.Operation == "alpha_search" && row.BillingUnit != "per_request" {
			continue
		}
		if row.Modality != q.Modality || row.ServiceTier != q.ServiceTier || row.Resolution != "" && !strings.EqualFold(row.Resolution, q.Resolution) || row.Quality != "" && !strings.EqualFold(row.Quality, q.Quality) {
			continue
		}
		for _, pattern := range row.Models {
			if matchesCostModel(pattern, q.Model) {
				rows = append(rows, row)
				break
			}
		}
	}
	return mostSpecificSimpleRows(rows, q.Model)
}

func validateConfirmedSimpleRows(q *SimpleBillingSnapshot, hasImageInput bool) error {
	for _, row := range q.Rows {
		if row.BillingUnit != q.BillingUnit {
			return simplePriceError("同规格同时配置了不同计费单位")
		}
		if row.BillingUnit == "per_million_tokens" {
			if row.InputPrice == nil || row.OutputPrice == nil || row.CacheReadPrice == nil || row.CacheWritePrice == nil {
				return simplePriceError("输入、输出和缓存项目须填写价格；免费项目请明确填0")
			}
			if (q.Modality == "image" || hasImageInput) && row.ImageInputPrice == nil {
				return simplePriceError("图片输入token单价尚未确认")
			}
			if q.Modality == "image" && row.ImageOutputPrice == nil {
				return simplePriceError("图片输出token单价尚未确认")
			}
		} else if row.UnitPrice == nil {
			return simplePriceError("此规格单价为空")
		}
	}
	return validateSimpleRowCoverage(q.Rows, q.Model)
}

func mostSpecificSimpleRows(rows []CostPriceRow, model string) []CostPriceRow {
	best := -1
	var selected []CostPriceRow
	for _, row := range rows {
		score := -1
		for _, pattern := range row.Models {
			if matchesCostModel(pattern, model) {
				candidate := len(pattern)
				if !strings.HasSuffix(pattern, "*") {
					candidate = 1000
				}
				if candidate > score {
					score = candidate
				}
			}
		}
		if row.Resolution != "" {
			score += 2000
		}
		if row.Quality != "" {
			score += 4000
		}
		if score < best {
			continue
		}
		if score > best {
			selected = nil
			best = score
		}
		selected = append(selected, row)
	}
	return selected
}

func validateSimpleRowCoverage(rows []CostPriceRow, model string) error {
	if len(rows) == 0 {
		return simplePriceError("缺少价表")
	}
	if rows[0].BillingUnit != "per_million_tokens" {
		if len(rows) != 1 {
			return simplePriceError("同规格单价重复")
		}
		return nil
	}
	checks := []int{0}
	for _, row := range rows {
		if row.MinTokens > 0 {
			checks = append(checks, row.MinTokens, row.MinTokens+1)
		}
		if row.MaxTokens != nil {
			checks = append(checks, *row.MaxTokens, *row.MaxTokens+1)
		}
	}
	for _, n := range checks {
		if _, err := simpleBillingRow(rows, BillingPreviewInput{Model: model, InputTokens: n}); err != nil {
			return err
		}
	}
	return nil
}

// RefreshHash must be called when a forwarding adapter attaches a separately
// confirmed image tool price to a text snapshot.
func (q *SimpleBillingSnapshot) RefreshHash() {
	if q == nil {
		return
	}
	copy := *q
	copy.Hash = ""
	raw, err := json.Marshal(&copy)
	if err == nil {
		q.Hash = HashUsageRequestPayload(raw)
	}
}

// Exact known entries only. Family/unknown-model price guessing belongs to the
// legacy path, and must never be presented as a confirmed procurement cost.
func (s *BillingService) strictReferencePricing(model string) (*ModelPricing, error) {
	name := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(model), "models/"))
	if s.pricingService != nil {
		s.pricingService.mu.RLock()
		p := s.pricingService.pricingData[name]
		var exact *LiteLLMModelPricing
		if p != nil && !p.TokenPricingAbsent {
			copy := *p
			exact = &copy
		}
		s.pricingService.mu.RUnlock()
		if exact != nil {
			price5m, price1h := exact.CacheCreationInputTokenCost, exact.CacheCreationInputTokenCostAbove1hr
			pricing := &ModelPricing{InputPricePerToken: exact.InputCostPerToken, InputPricePerTokenPriority: exact.InputCostPerTokenPriority, OutputPricePerToken: exact.OutputCostPerToken, OutputPricePerTokenPriority: exact.OutputCostPerTokenPriority, CacheCreationPricePerToken: exact.CacheCreationInputTokenCost, CacheCreationPricePerTokenPriority: exact.CacheCreationInputTokenCostPriority, CacheReadPricePerToken: exact.CacheReadInputTokenCost, CacheReadPricePerTokenPriority: exact.CacheReadInputTokenCostPriority, CacheCreation5mPrice: price5m, CacheCreation1hPrice: price1h, SupportsCacheBreakdown: price1h > 0 && price1h > price5m, LongContextInputThreshold: exact.LongContextInputTokenThreshold, LongContextInputMultiplier: exact.LongContextInputCostMultiplier, LongContextOutputMultiplier: exact.LongContextOutputCostMultiplier, ImageInputPricePerToken: exact.InputCostPerImageToken, ImageOutputPricePerToken: exact.OutputCostPerImageToken}
			return s.applyModelSpecificPricingPolicy(name, pricing), nil
		}
	}
	if pricing := s.fallbackPrices[name]; pricing != nil {
		copy := *pricing
		return s.applyModelSpecificPricingPolicy(name, &copy), nil
	}
	return nil, simplePriceError("型号未精确命中已维护的参考价表")
}

func (s *BillingService) strictMediaPrice(model, modality, resolution string) (float64, string, error) {
	if modality == "video" {
		name := strings.ToLower(strings.TrimSpace(model))
		grokExact := name == "grok-imagine-video" || name == "grok-imagine-video-1.5"
		grokSize := slices.Contains([]string{"480P", "720P", "1080P"}, strings.ToUpper(resolution))
		if price, ok := getDefaultGrokImagineVideoPrice(model, resolution); ok && grokExact && grokSize {
			return price, "per_second", nil
		}
		// MiniMax global prices are maintained per exact supported resolution.
		if spec, ok := miniMaxSimpleGlobalVideoPrice(model, resolution); ok {
			return spec, "per_second", nil
		}
	} else if modality == "image" {
		if price, ok := getDefaultGrokImagineImagePrice(model, resolution); ok && slices.Contains([]string{"1K", "2K", "4K"}, strings.ToUpper(resolution)) {
			return price, "per_image", nil
		}
		if s.pricingService != nil {
			s.pricingService.mu.RLock()
			p := s.pricingService.pricingData[strings.ToLower(model)]
			var price float64
			if p != nil {
				price = p.OutputCostPerImage
			}
			s.pricingService.mu.RUnlock()
			if price > 0 && (resolution == "" || strings.EqualFold(resolution, "default")) {
				return price, "per_image", nil
			}
		}
	}
	return 0, "", simplePriceError("媒体规格未精确命中价表；不能沿用通用图片默认价")
}

func (s *BillingService) strictImageTokenPricing(model string) (*ModelPricing, error) {
	if s.pricingService == nil {
		return nil, simplePriceError("图片token价格未确认")
	}
	s.pricingService.mu.RLock()
	p := s.pricingService.pricingData[strings.ToLower(model)]
	known := p != nil && !p.TokenPricingAbsent && (p.OutputCostPerImageToken > 0 || (p.Mode == "image_generation" && p.OutputCostPerToken > 0))
	s.pricingService.mu.RUnlock()
	if !known {
		return nil, simplePriceError("图片token价格未确认")
	}
	return s.strictReferencePricing(model)
}

func (s *BillingService) CalculateSimpleBilling(q *SimpleBillingSnapshot, in BillingPreviewInput) (*CostBreakdown, error) {
	if q == nil {
		return nil, simplePriceError("缺少请求前的计费快照")
	}
	if q.RejectSeparateCacheWrite && in.CacheWriteTokens > 0 {
		return nil, simplePriceError("此官方参考采用未命中输入和缓存命中计费；独立缓存写入用量须重新确认供应商规则")
	}
	if q.ImageBilling != nil {
		text := *q
		text.ImageBilling = nil
		textIn := in
		textIn.InputTokens = max(0, in.InputTokens-in.ImageInputTokens)
		textIn.OutputTokens = max(0, in.OutputTokens-in.ImageOutputTokens)
		textIn.ImageInputTokens, textIn.ImageOutputTokens, textIn.ImageCount = 0, 0, 0
		main, err := s.CalculateSimpleBilling(&text, textIn)
		if err != nil {
			return nil, err
		}
		mediaIn := BillingPreviewInput{Model: q.ImageBilling.Model, InputTokens: in.ImageInputTokens, ImageInputTokens: in.ImageInputTokens, OutputTokens: in.ImageOutputTokens, ImageOutputTokens: in.ImageOutputTokens, ImageCount: in.ImageCount, ImageOutputSizes: in.ImageOutputSizes, Requests: 1}
		media, err := s.CalculateSimpleBilling(q.ImageBilling, mediaIn)
		if err != nil {
			return nil, err
		}
		main.ImageInputCost += media.InputCost + media.ImageInputCost
		main.ImageOutputCost += media.OutputCost + media.ImageOutputCost
		main.TotalCost += media.TotalCost
		main.ActualCost += media.ActualCost
		return main, nil
	}
	tokens := UsageTokens{InputTokens: in.InputTokens, OutputTokens: in.OutputTokens, CacheReadTokens: in.CacheReadTokens, CacheCreationTokens: in.CacheWriteTokens, CacheCreation5mTokens: in.CacheWrite5mTokens, CacheCreation1hTokens: in.CacheWrite1hTokens, ImageInputTokens: in.ImageInputTokens, ImageOutputTokens: in.ImageOutputTokens}
	if q.ReferencePricing != nil {
		cost := s.computeTokenBreakdown(q.ReferencePricing, tokens, q.Multiplier, q.ServiceTier, true)
		cost.BillingMode = string(BillingModeToken)
		return cost, nil
	}
	if q.UnitPriceUSD != nil {
		count := float64(in.Requests)
		if q.BillingUnit == "per_image" {
			count = float64(in.ImageCount)
		}
		if q.BillingUnit == "per_second" {
			count *= in.DurationSeconds
		}
		if count < 0 {
			return nil, simplePriceError("用量无效")
		}
		total := *q.UnitPriceUSD * count
		return &CostBreakdown{TotalCost: total, ActualCost: total * q.Multiplier, BillingMode: simpleBillingMode(q.BillingUnit)}, nil
	}
	row, err := simpleBillingRow(q.Rows, in)
	if err != nil {
		return nil, err
	}
	scale := q.DiscountMultiplier
	if q.Currency == "CNY" {
		scale /= FixedUSDToCNYRate
	}
	if row.BillingUnit == "per_million_tokens" {
		convert := func(p *float64) float64 {
			if p == nil {
				return 0
			}
			return *p * scale / 1000000
		}
		if in.ImageInputTokens > 0 && row.ImageInputPrice == nil || in.ImageOutputTokens > 0 && row.ImageOutputPrice == nil {
			return nil, simplePriceError("图片token价未确认")
		}
		pricing := &ModelPricing{InputPricePerToken: convert(row.InputPrice), OutputPricePerToken: convert(row.OutputPrice), CacheReadPricePerToken: convert(row.CacheReadPrice), CacheCreationPricePerToken: convert(row.CacheWritePrice), CacheCreationPriceExplicit: true, ImageInputPricePerToken: convert(row.ImageInputPrice), ImageInputPriceExplicit: row.ImageInputPrice != nil, ImageOutputPricePerToken: convert(row.ImageOutputPrice), ImageOutputPriceExplicit: row.ImageOutputPrice != nil}
		cost := s.computeTokenBreakdown(pricing, tokens, q.Multiplier, "", false)
		cost.BillingMode = string(BillingModeToken)
		return cost, nil
	}
	if row.UnitPrice == nil {
		return nil, simplePriceError("单价为空")
	}
	count := float64(in.Requests)
	if row.BillingUnit == "per_image" {
		count = float64(in.ImageCount)
	}
	if row.BillingUnit == "per_second" {
		count *= in.DurationSeconds
	}
	amount := decimal.NewFromFloat(*row.UnitPrice).Mul(decimal.NewFromFloat(scale)).Mul(decimal.NewFromFloat(count)).InexactFloat64()
	if row.BillingUnit == "per_image" && q.LowPixelUnitPrice != nil && q.HighPixelUnitPrice != nil {
		unitTotal := decimal.NewFromFloat(*row.UnitPrice).Mul(decimal.NewFromInt(int64(in.ImageCount)))
		for i := 0; i < len(in.ImageOutputSizes) && i < in.ImageCount; i++ {
			unit := *row.UnitPrice
			if width, height, ok := parseImageBillingDimensions(in.ImageOutputSizes[i]); ok && width > 0 && height > 0 && width < 1000000000 && height < 1000000000 {
				if int64(width)*int64(height) <= q.PixelPriceThreshold {
					unit = *q.LowPixelUnitPrice
				} else {
					unit = *q.HighPixelUnitPrice
				}
			}
			unitTotal = unitTotal.Add(decimal.NewFromFloat(unit).Sub(decimal.NewFromFloat(*row.UnitPrice)))
		}
		amount = unitTotal.Mul(decimal.NewFromFloat(scale)).InexactFloat64()
	}
	if row.InputImageUnitPrice != nil && q.InputImageCount > 1 {
		amount += *row.InputImageUnitPrice * scale * float64(q.InputImageCount-1)
	}
	return &CostBreakdown{TotalCost: amount, ActualCost: amount * q.Multiplier, BillingMode: simpleBillingMode(row.BillingUnit)}, nil
}

func simpleBillingMode(unit string) string {
	switch unit {
	case "per_image":
		return string(BillingModeImage)
	case "per_second":
		return string(BillingModeVideo)
	case "per_request":
		return string(BillingModePerRequest)
	default:
		return string(BillingModeToken)
	}
}

func simpleBillingRow(rows []CostPriceRow, in BillingPreviewInput) (*CostPriceRow, error) {
	context := in.InputTokens + in.CacheReadTokens + in.CacheWriteTokens
	var selected *CostPriceRow
	best := -1
	for i := range rows {
		row := &rows[i]
		if row.MinTokens > 0 && context <= row.MinTokens || row.MaxTokens != nil && context > *row.MaxTokens {
			continue
		}
		score := 0
		for _, pattern := range row.Models {
			if matchesCostModel(pattern, in.Model) {
				if !strings.HasSuffix(pattern, "*") {
					score = 1000
				} else if len(pattern) > score {
					score = len(pattern)
				}
			}
		}
		if row.Resolution != "" {
			score += 2000
		}
		if row.Quality != "" {
			score += 4000
		}
		if score < best {
			continue
		}
		if score == best {
			selected = nil
			continue
		}
		best, selected = score, row
	}
	if selected == nil {
		return nil, simplePriceError("上下文价表缺失或有重复区间")
	}
	return selected, nil
}

func (s *BillingManagementService) simplePreview(ctx context.Context, cfg *BillingManagementConfig, in BillingPreviewInput) (*BillingPreviewResult, error) {
	out := &BillingPreviewResult{Warnings: []string{"固定汇率1美元=6.2人民币；账本和金额API继续保存美元额度", "报价采用维护价表，不代表供应商已确认账单；预览不会调用上游或扣费"}, Cost: PurchaseCostEstimate{Status: "unknown", ConfigVersion: cfg.Version}, Sale: SaleChargeEstimate{Status: "unknown", Currency: "USD"}}
	out.Comparison.Reason = "扣费额度不等于历史充值实收；历史余额换算保留已有权益"
	if in.AccountID <= 0 || in.GroupID <= 0 || s.accounts == nil || s.groups == nil || s.billing == nil {
		out.Sale.Reason = "请选择实际上游账号和分组，才能确认采购类型及销售倍率"
		return out, nil
	}
	account, err := s.accounts.GetByID(ctx, in.AccountID)
	if err != nil || account == nil {
		out.Sale.Reason = "上游账号不存在"
		return out, nil
	}
	group, err := s.groups.GetByID(ctx, in.GroupID)
	if err != nil || group == nil || group.Status != StatusActive {
		out.Sale.Reason = "分组不存在或未启用"
		return out, nil
	}
	if len(account.GroupIDs) > 0 && !slices.Contains(account.GroupIDs, group.ID) {
		out.Sale.Reason = "此账号不属于所选分组"
		return out, nil
	}
	q, err := s.billing.resolveSimpleBilling(cfg, SimpleBillingInput{Account: account, Group: group, Model: account.GetMappedModel(in.Model), Modality: in.Modality, Resolution: in.Resolution, Quality: in.Quality, ServiceTier: in.ServiceTier, InputImageCount: in.InputImageCount, Scene: in.ImageScene, HasImageInput: in.ImageInputTokens > 0, Operation: in.Operation}, s.now())
	if err != nil {
		out.Cost.Reason, out.Sale.Reason = err.Error(), err.Error()
		return out, nil
	}
	in.Model = q.Model
	cost, err := s.billing.CalculateSimpleBilling(q, in)
	if err != nil {
		out.Cost.Reason, out.Sale.Reason = err.Error(), err.Error()
		return out, nil
	}
	amount, cny := cost.ActualCost, cost.ActualCost*FixedUSDToCNYRate
	out.Sale = SaleChargeEstimate{Status: "estimated", Currency: "USD", Amount: &amount, AmountCNY: &cny, Multiplier: &q.Multiplier, Source: q.Source, Reason: "人民币基准价乘唯一销售倍率；已排除用户专属、高峰及媒体独立倍率"}
	base, baseCNY := cost.TotalCost, cost.TotalCost*FixedUSDToCNYRate
	out.Cost = PurchaseCostEstimate{Status: "estimated", ProfileID: q.ProfileID, Currency: "USD", Amount: &base, AmountCNY: &baseCNY, ConfigVersion: q.Version, Reason: "绑定上游地区和规格的维护基准价，尚未供应商对账"}
	if q.Route == "account" {
		out.Cost.Status, out.Cost.Reason = "reference", "授权或包月账号使用参考价销售，不能据此推算实际单次采购成本"
	}
	return out, nil
}
