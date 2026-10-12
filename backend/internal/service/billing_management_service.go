package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/minimax"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/shopspring/decimal"
)

// BillingManagementRepository stores an independent administrator cost book.
// CompareAndSwap must be atomic across application instances.
type BillingManagementRepository interface {
	Get(context.Context) (string, error)
	CompareAndSwap(context.Context, string, string) (bool, error)
}

type BillingManagementConfig struct {
	Version                 int64         `json:"version"`
	USDToCNYRate            float64       `json:"usd_to_cny_rate"`
	ExchangeRateSource      string        `json:"exchange_rate_source"`
	ExchangeRateEffectiveAt *time.Time    `json:"exchange_rate_effective_at"`
	CostProfiles            []CostProfile `json:"cost_profiles"`
}

type CostProfile struct {
	ID                 string         `json:"id"`
	Name               string         `json:"name"`
	Provider           string         `json:"provider"`
	Enabled            bool           `json:"enabled"`
	AccountIDs         []int64        `json:"account_ids"`
	PurchaseMode       string         `json:"purchase_mode"`
	Currency           string         `json:"currency"`
	DiscountMultiplier float64        `json:"discount_multiplier"`
	PeriodCost         *float64       `json:"period_cost"`
	CreditAmount       *float64       `json:"credit_amount"`
	ValidFrom          *time.Time     `json:"valid_from"`
	ValidUntil         *time.Time     `json:"valid_until"`
	Source             string         `json:"source"`
	Notes              string         `json:"notes"`
	Prices             []CostPriceRow `json:"prices"`
}

// Text prices are in the profile's original currency per million tokens.
// A nil price means unknown; an explicit zero is a confirmed free item.
type CostPriceRow struct {
	ID               string   `json:"id"`
	Models           []string `json:"models"`
	Modality         string   `json:"modality"`
	BillingUnit      string   `json:"billing_unit"`
	Resolution       string   `json:"resolution"`
	Quality          string   `json:"quality"`
	ServiceTier      string   `json:"service_tier"`
	MinTokens        int      `json:"min_tokens"`
	MaxTokens        *int     `json:"max_tokens"`
	InputPrice       *float64 `json:"input_price"`
	OutputPrice      *float64 `json:"output_price"`
	ImageInputPrice  *float64 `json:"image_input_price"`
	ImageOutputPrice *float64 `json:"image_output_price"`
	CacheReadPrice   *float64 `json:"cache_read_price"`
	CacheWritePrice  *float64 `json:"cache_write_price"`
	UnitPrice        *float64 `json:"unit_price"`
}

type BillingPreviewInput struct {
	ProfileID         string  `json:"profile_id"`
	AccountID         int64   `json:"account_id"`
	GroupID           int64   `json:"group_id"`
	Model             string  `json:"model"`
	Modality          string  `json:"modality"`
	Resolution        string  `json:"resolution"`
	Quality           string  `json:"quality"`
	ServiceTier       string  `json:"service_tier"`
	InputTokens       int     `json:"input_tokens"`
	OutputTokens      int     `json:"output_tokens"`
	ImageInputTokens  int     `json:"image_input_tokens"`
	ImageOutputTokens int     `json:"image_output_tokens"`
	CacheReadTokens   int     `json:"cache_read_tokens"`
	CacheWriteTokens  int     `json:"cache_write_tokens"`
	ImageCount        int     `json:"image_count"`
	DurationSeconds   float64 `json:"duration_seconds"`
	Requests          int     `json:"requests"`
}

type PurchaseCostEstimate struct {
	Status        string   `json:"status"`
	ProfileID     string   `json:"profile_id"`
	Currency      string   `json:"currency"`
	Amount        *float64 `json:"amount"`
	AmountCNY     *float64 `json:"amount_cny"`
	PeriodCost    *float64 `json:"period_cost"`
	ExchangeRate  *float64 `json:"exchange_rate"`
	ConfigVersion int64    `json:"config_version"`
	Reason        string   `json:"reason"`
}

type SaleChargeEstimate struct {
	Status     string   `json:"status"`
	Currency   string   `json:"currency"`
	Amount     *float64 `json:"amount"`
	Multiplier *float64 `json:"multiplier"`
	Source     string   `json:"source"`
	Reason     string   `json:"reason"`
}

type BillingPreviewResult struct {
	Cost       PurchaseCostEstimate `json:"cost"`
	Sale       SaleChargeEstimate   `json:"sale"`
	Comparison struct {
		IncomeCNY *float64 `json:"income_cny"`
		MarginCNY *float64 `json:"margin_cny"`
		Reason    string   `json:"reason"`
	} `json:"comparison"`
	Warnings []string `json:"warnings"`
}

type BillingManagementService struct {
	repo     BillingManagementRepository
	accounts AccountRepository
	groups   GroupRepository
	channels *ChannelService
	resolver *ModelPricingResolver
	billing  *BillingService
	cfg      *config.Config
	now      func() time.Time
}

func NewBillingManagementService(repo BillingManagementRepository, accounts AccountRepository, groups GroupRepository, channels *ChannelService, resolver *ModelPricingResolver, billing *BillingService, cfg *config.Config) *BillingManagementService {
	return &BillingManagementService{repo: repo, accounts: accounts, groups: groups, channels: channels, resolver: resolver, billing: billing, cfg: cfg, now: timezone.Now}
}

func (s *BillingManagementService) load(ctx context.Context) (*BillingManagementConfig, string, error) {
	raw, err := s.repo.Get(ctx)
	if err != nil {
		return nil, "", err
	}
	cfg := &BillingManagementConfig{CostProfiles: []CostProfile{}}
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), cfg); err != nil {
			return nil, "", fmt.Errorf("invalid persisted billing management config: %w", err)
		}
	}
	if cfg.CostProfiles == nil {
		cfg.CostProfiles = []CostProfile{}
	}
	return cfg, raw, nil
}

func (s *BillingManagementService) GetConfig(ctx context.Context) (*BillingManagementConfig, error) {
	cfg, _, err := s.load(ctx)
	return cfg, err
}

func (s *BillingManagementService) UpdateConfig(ctx context.Context, next BillingManagementConfig) (*BillingManagementConfig, error) {
	if err := ValidateBillingManagementConfig(&next); err != nil {
		return nil, err
	}
	current, raw, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	if current.Version != next.Version {
		return nil, infraerrors.Conflict("BILLING_CONFIG_CHANGED", "费率配置已被修改，请刷新后重新保存")
	}
	seenAccounts := map[int64]bool{}
	for _, profile := range next.CostProfiles {
		for _, id := range profile.AccountIDs {
			if seenAccounts[id] {
				continue
			}
			seenAccounts[id] = true
			if s.accounts == nil {
				return nil, fmt.Errorf("account repository unavailable")
			}
			if account, err := s.accounts.GetByID(ctx, id); err != nil || account == nil {
				return nil, infraerrors.BadRequest("COST_ACCOUNT_NOT_FOUND", "采购方案关联的上游账号不存在")
			}
		}
	}
	next.Version++
	encoded, err := json.Marshal(next)
	if err != nil {
		return nil, err
	}
	ok, err := s.repo.CompareAndSwap(ctx, raw, string(encoded))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, infraerrors.Conflict("BILLING_CONFIG_CHANGED", "费率配置已被修改，请刷新后重新保存")
	}
	return &next, nil
}

func finiteNonnegative(n float64) bool { return n >= 0 && !math.IsNaN(n) && !math.IsInf(n, 0) }

func ValidateBillingManagementConfig(cfg *BillingManagementConfig) error {
	bad := func(message string) error { return infraerrors.BadRequest("INVALID_BILLING_CONFIG", message) }
	if cfg.Version < 0 || cfg.Version == math.MaxInt64 || !finiteNonnegative(cfg.USDToCNYRate) || cfg.USDToCNYRate > 1000000 {
		return bad("配置版本或成本汇率无效；汇率 0 表示未配置")
	}
	if len(cfg.ExchangeRateSource) > 500 || len(cfg.CostProfiles) > 200 {
		return bad("汇率来源最多500字，采购方案最多200个")
	}
	profileIDs := map[string]bool{}
	for i := range cfg.CostProfiles {
		p := &cfg.CostProfiles[i]
		p.Name = strings.TrimSpace(p.Name)
		p.ID = strings.TrimSpace(p.ID)
		if p.ID == "" || len(p.ID) > 100 || profileIDs[p.ID] || p.Name == "" || len(p.Name) > 100 || len(p.Provider) > 100 {
			return bad("采购方案须有唯一编号和名称，名称及供应商最多100字")
		}
		profileIDs[p.ID] = true
		if !slices.Contains([]string{"payg", "subscription", "credit_pack", "hybrid"}, p.PurchaseMode) || !slices.Contains([]string{"CNY", "USD"}, p.Currency) {
			return bad("采购方式或原币无效")
		}
		if !finiteNonnegative(p.DiscountMultiplier) || p.DiscountMultiplier == 0 || p.DiscountMultiplier > 1000000 {
			return bad("采购折扣倍率必须大于0；免费项目请显式填写单价0")
		}
		if p.PeriodCost != nil && !finiteNonnegative(*p.PeriodCost) || p.CreditAmount != nil && (!finiteNonnegative(*p.CreditAmount) || *p.CreditAmount == 0) {
			return bad("采购实付须为非负数，额度包的标称额度须大于0")
		}
		if p.PurchaseMode != "payg" && p.PeriodCost == nil {
			return bad("包月、额度包和混合采购须填写本期实付")
		}
		if p.PurchaseMode == "credit_pack" && p.CreditAmount == nil {
			return bad("额度包须填写供应商原币标称额度")
		}
		if p.ValidFrom != nil && p.ValidUntil != nil && !p.ValidUntil.After(*p.ValidFrom) {
			return bad("采购有效期的结束时间须晚于开始时间")
		}
		if len(p.Source) > 500 || len(p.Notes) > 2000 || len(p.AccountIDs) > 1000 || len(p.Prices) > 500 {
			return bad("采购方案备注或配置数量超限")
		}
		if p.AccountIDs == nil {
			p.AccountIDs = []int64{}
		}
		if p.Prices == nil {
			p.Prices = []CostPriceRow{}
		}
		accountIDs, rowIDs := map[int64]bool{}, map[string]bool{}
		for _, id := range p.AccountIDs {
			if id <= 0 || accountIDs[id] {
				return bad("采购方案账号编号须为唯一正数")
			}
			accountIDs[id] = true
		}
		for j := range p.Prices {
			r := &p.Prices[j]
			if r.ID == "" || len(r.ID) > 100 || rowIDs[r.ID] || len(r.Models) == 0 || len(r.Models) > 100 {
				return bad("成本价表行须有唯一编号，且至少指定一个模型")
			}
			rowIDs[r.ID] = true
			for k, model := range r.Models {
				model = strings.TrimSpace(model)
				if model == "" || len(model) > 200 || strings.Contains(strings.TrimSuffix(model, "*"), "*") {
					return bad("模型名无效；通配符只能放在末尾")
				}
				r.Models[k] = model
			}
			if !slices.Contains([]string{"chat", "image", "video"}, r.Modality) || !slices.Contains([]string{"per_million_tokens", "per_image", "per_second", "per_request"}, r.BillingUnit) {
				return bad("模型类型或计费单位无效")
			}
			if (r.BillingUnit == "per_image" && r.Modality != "image") || (r.BillingUnit == "per_second" && r.Modality != "video") {
				return bad("按张仅用于图像，按秒仅用于视频")
			}
			if len(r.Resolution) > 30 || len(r.Quality) > 50 || !slices.Contains([]string{"", "priority", "flex"}, r.ServiceTier) || r.MinTokens < 0 || (r.MaxTokens != nil && *r.MaxTokens <= r.MinTokens) {
				return bad("规格或上下文区间无效；上限包含在区间内")
			}
			for _, price := range []*float64{r.InputPrice, r.OutputPrice, r.ImageInputPrice, r.ImageOutputPrice, r.CacheReadPrice, r.CacheWritePrice, r.UnitPrice} {
				if price != nil && (!finiteNonnegative(*price) || *price > 1e12) {
					return bad("单价须为有效非负数，空单价表示待核对")
				}
			}
		}
	}
	if cfg.CostProfiles == nil {
		cfg.CostProfiles = []CostProfile{}
	}
	return nil
}

func (s *BillingManagementService) Preview(ctx context.Context, in BillingPreviewInput) (*BillingPreviewResult, error) {
	if strings.TrimSpace(in.Model) == "" || len(in.Model) > 200 || !slices.Contains([]string{"chat", "image", "video"}, in.Modality) || in.AccountID < 0 || in.GroupID < 0 {
		return nil, infraerrors.BadRequest("INVALID_BILLING_PREVIEW", "请选择有效模型、类型及账号/分组")
	}
	if len(in.Resolution) > 30 || len(in.Quality) > 50 || len(in.ProfileID) > 100 || !slices.Contains([]string{"", "auto", "default", "priority", "flex"}, in.ServiceTier) {
		return nil, infraerrors.BadRequest("INVALID_BILLING_PREVIEW", "模型规格或服务档位无效")
	}
	for _, qty := range []int{in.InputTokens, in.OutputTokens, in.ImageInputTokens, in.ImageOutputTokens, in.CacheReadTokens, in.CacheWriteTokens, in.ImageCount, in.Requests} {
		if qty < 0 || qty > 1000000000 {
			return nil, infraerrors.BadRequest("INVALID_BILLING_PREVIEW", "用量须在0至十亿之间")
		}
	}
	if in.ImageInputTokens > in.InputTokens || in.ImageOutputTokens > in.OutputTokens {
		return nil, infraerrors.BadRequest("INVALID_BILLING_PREVIEW", "图片输入/输出tokens为对应输入/输出总量的子集，不能超过总量")
	}
	if !finiteNonnegative(in.DurationSeconds) || in.DurationSeconds > 86400 {
		return nil, infraerrors.BadRequest("INVALID_BILLING_PREVIEW", "视频时长无效")
	}
	if in.Requests == 0 {
		in.Requests = 1
	}
	cfg, err := s.GetConfig(ctx)
	if err != nil {
		return nil, err
	}
	out := &BillingPreviewResult{Warnings: []string{}, Cost: estimatePurchaseCost(cfg, in, s.now()), Sale: SaleChargeEstimate{Status: "unknown", Currency: "USD"}}
	out.Comparison.Reason = "未绑定具体充值订单和赠送比例，扣费额度不等于现金收入；成本汇率不用于推算充值实收"
	out.Warnings = append(out.Warnings, "采购成本为价表估算，需与供应商账单核对；预览不会调用上游或扣费")
	out.Sale = s.estimateSale(ctx, in, &out.Warnings)
	return out, nil
}

func matchesCostModel(pattern, model string) bool {
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(strings.ToLower(model), strings.ToLower(strings.TrimSuffix(pattern, "*")))
	}
	return strings.EqualFold(pattern, model)
}

func estimatePurchaseCost(cfg *BillingManagementConfig, in BillingPreviewInput, now time.Time) PurchaseCostEstimate {
	out := PurchaseCostEstimate{Status: "unknown", ConfigVersion: cfg.Version, Reason: "没有匹配的上游采购方案；未知成本不会记为0"}
	var selected *CostProfile
	for i := range cfg.CostProfiles {
		p := &cfg.CostProfiles[i]
		if !p.Enabled || (p.ValidFrom != nil && now.Before(*p.ValidFrom)) || (p.ValidUntil != nil && !now.Before(*p.ValidUntil)) {
			continue
		}
		if in.ProfileID != "" {
			if p.ID != in.ProfileID {
				continue
			}
			if in.AccountID > 0 && len(p.AccountIDs) > 0 && !slices.Contains(p.AccountIDs, in.AccountID) {
				out.Reason = "所选采购方案未关联此账号"
				return out
			}
		} else if in.AccountID <= 0 || !slices.Contains(p.AccountIDs, in.AccountID) {
			continue
		}
		if selected != nil {
			out.Reason = "多个有效采购方案匹配此账号，请指定采购方案"
			return out
		}
		selected = p
	}
	if selected == nil {
		return out
	}
	out.ProfileID, out.Currency, out.PeriodCost = selected.ID, selected.Currency, selected.PeriodCost
	if selected.PurchaseMode == "subscription" {
		out.Status, out.Reason = "period_only", "包月采购仅记录本期实付，缺少分摊依据时不估算单次成本"
		return out
	}
	var price *CostPriceRow
	best := -1
	contextTokens := in.InputTokens + in.CacheReadTokens + in.CacheWriteTokens
	for i := range selected.Prices {
		r := &selected.Prices[i]
		tier := in.ServiceTier
		if tier == "auto" || tier == "default" {
			tier = ""
		}
		if r.ServiceTier != tier {
			continue
		}
		if r.Modality != in.Modality || (r.MinTokens > 0 && contextTokens <= r.MinTokens) || (r.MaxTokens != nil && contextTokens > *r.MaxTokens) {
			continue
		}
		if r.Resolution != "" && !strings.EqualFold(r.Resolution, in.Resolution) || r.Quality != "" && !strings.EqualFold(r.Quality, in.Quality) {
			continue
		}
		score := -1
		for _, pattern := range r.Models {
			if matchesCostModel(pattern, in.Model) {
				candidate := 0
				if !strings.HasSuffix(pattern, "*") {
					candidate = 1000
				} else {
					candidate = len(pattern)
				}
				if r.Resolution != "" {
					candidate += 2000
				}
				if r.Quality != "" {
					candidate += 4000
				}
				if candidate > score {
					score = candidate
				}
			}
		}
		if score < 0 || score < best {
			continue
		}
		if score == best {
			price = nil
			continue
		}
		best, price = score, r
	}
	if price == nil {
		out.Reason = "成本单价缺失或匹配区间重复，请补齐模型规格价表"
		return out
	}
	amount := decimal.Zero
	if price.BillingUnit == "per_million_tokens" {
		if in.InputTokens+in.OutputTokens+in.CacheReadTokens+in.CacheWriteTokens == 0 {
			out.Reason = "缺少可计量的token用量，不能据此确认免费调用"
			return out
		}
		for _, item := range []struct {
			qty   int
			price *float64
			label string
		}{{in.InputTokens - in.ImageInputTokens, price.InputPrice, "文本输入"}, {in.OutputTokens - in.ImageOutputTokens, price.OutputPrice, "文本输出"}, {in.ImageInputTokens, price.ImageInputPrice, "图片输入"}, {in.ImageOutputTokens, price.ImageOutputPrice, "图片输出"}, {in.CacheReadTokens, price.CacheReadPrice, "缓存读取"}, {in.CacheWriteTokens, price.CacheWritePrice, "缓存写入"}} {
			if item.qty == 0 {
				continue
			}
			if item.price == nil {
				out.Reason = item.label + "项目缺少成本单价，不能假设免费或沿用其他项目价格"
				return out
			}
			amount = amount.Add(decimal.NewFromFloat(*item.price).Mul(decimal.NewFromInt(int64(item.qty))).Div(decimal.NewFromInt(1000000)))
		}
	} else {
		if price.UnitPrice == nil {
			out.Reason = "此规格的成本单价待核对"
			return out
		}
		qty := float64(in.Requests)
		switch price.BillingUnit {
		case "per_image":
			qty = float64(in.ImageCount)
		case "per_second":
			qty = in.DurationSeconds * float64(in.Requests)
		}
		if qty <= 0 {
			out.Reason = "缺少此计费单位的有效用量，无法估算采购费用"
			return out
		}
		amount = decimal.NewFromFloat(*price.UnitPrice).Mul(decimal.NewFromFloat(qty))
	}
	amount = amount.Mul(decimal.NewFromFloat(selected.DiscountMultiplier))
	out.Reason = "按原币采购单价和协议折扣估算，尚未供应商对账"
	if selected.PurchaseMode == "credit_pack" {
		if selected.PeriodCost == nil || selected.CreditAmount == nil || *selected.CreditAmount <= 0 {
			out.Reason = "额度包缺少实付或标称额度"
			return out
		}
		amount = amount.Mul(decimal.NewFromFloat(*selected.PeriodCost)).Div(decimal.NewFromFloat(*selected.CreditAmount))
		out.Reason = "按额度包实付/原币标称额度分摊估算；额度未用完、过期或超额费用另计"
	} else if selected.PurchaseMode == "hybrid" {
		out.Reason = "仅为按量部分估算；本期固定实付单列，尚未分摊"
	}
	value := amount.Round(12).InexactFloat64()
	out.Amount, out.Status = &value, "estimated"
	if out.Currency == "CNY" {
		out.AmountCNY = &value
	} else if cfg.USDToCNYRate > 0 {
		rate := cfg.USDToCNYRate
		if cfg.ExchangeRateEffectiveAt != nil && now.Before(*cfg.ExchangeRateEffectiveAt) {
			out.Reason += "；成本换算汇率尚未生效"
			return out
		}
		converted := amount.Mul(decimal.NewFromFloat(rate)).Round(12).InexactFloat64()
		out.AmountCNY, out.ExchangeRate = &converted, &rate
	} else {
		out.Reason += "；美元成本未折算，尚未配置成本汇率"
	}
	return out
}

func (s *BillingManagementService) estimateSale(ctx context.Context, in BillingPreviewInput, warnings *[]string) SaleChargeEstimate {
	out := SaleChargeEstimate{Status: "unknown", Currency: "USD", Reason: "请选择分组以读取实际销售规则"}
	if in.GroupID <= 0 || s.groups == nil || s.billing == nil || s.resolver == nil {
		return out
	}
	group, err := s.groups.GetByID(ctx, in.GroupID)
	if err != nil || group == nil {
		out.Reason = "所选分组不存在或读取失败"
		return out
	}
	if group.Status != StatusActive {
		out.Reason = "所选分组未启用"
		return out
	}
	*warnings = append(*warnings, "销售预览使用分组费率；用户专属倍率、订阅权益、批量折扣及退款未纳入")
	multiplier := group.RateMultiplier
	key := &APIKey{GroupID: &group.ID, Group: group}
	if in.Modality == "image" {
		multiplier = resolveImageRateMultiplier(key, multiplier)
	}
	if in.Modality == "video" {
		multiplier = resolveVideoRateMultiplier(key, multiplier)
	}
	if in.Modality == "chat" {
		multiplier *= group.PeakMultiplierAt(s.now())
	}
	if s.cfg != nil && s.cfg.RunMode == "simple" {
		multiplier = 0
	}
	out.Multiplier = &multiplier
	policy, err := s.channels.ResolveModelServicePolicy(ctx, group.ID, in.Model)
	if err != nil {
		out.Reason = "模型服务配置读取失败"
		return out
	}
	if policy != nil && policy.State != "published" {
		out.Reason = "此模型尚未向所选分组开放"
		return out
	}
	if policy != nil && policy.Kind != in.Modality {
		out.Reason = "所选模型类型与模型服务不一致"
		return out
	}
	if in.Modality == "video" {
		if spec, ok := minimax.VideoModelSpecFor(in.Model); ok {
			if policy == nil && !GroupAllowsImageGeneration(group) {
				out.Reason = "此分组尚未开放视频生成"
				return out
			}
			if !slices.Contains(spec.Resolutions, in.Resolution) || in.DurationSeconds < float64(spec.MinDuration) || in.DurationSeconds > float64(spec.MaxDuration) || math.Trunc(in.DurationSeconds) != in.DurationSeconds {
				out.Reason = "视频分辨率或时长不符合此型号规格"
				return out
			}
			price := spec.PricesUSD[in.Resolution]
			out.Source = "model_default"
			if policy != nil {
				if !slices.Contains(policy.Resolutions, in.Resolution) || in.DurationSeconds < float64(policy.MinDuration) || in.DurationSeconds > float64(policy.MaxDuration) {
					out.Reason = "此分组未开放所选视频规格"
					return out
				}
				configured, exists := policy.Prices[in.Resolution]
				if !exists {
					out.Reason = "视频规格销售单价尚未配置"
					return out
				}
				price, out.Source = configured, "model_service"
			}
			amount := price * in.DurationSeconds * float64(in.Requests) * multiplier
			out.Amount, out.Status, out.Reason = &amount, "estimated", "按现有视频每秒售价和独立分组倍率计算站内美元扣费"
			return out
		}
	}
	resolved := s.resolver.Resolve(ctx, PricingInput{Model: in.Model, GroupID: &group.ID})
	out.Source = resolved.Source
	tokens := UsageTokens{InputTokens: in.InputTokens, OutputTokens: in.OutputTokens, ImageInputTokens: in.ImageInputTokens, ImageOutputTokens: in.ImageOutputTokens, CacheReadTokens: in.CacheReadTokens, CacheCreationTokens: in.CacheWriteTokens}
	requestCount := in.Requests
	imageTokenBilling := in.Modality == "image" && resolved.Source == PricingSourceChannel && resolved.Mode == BillingModeToken
	if imageTokenBilling && in.InputTokens+in.OutputTokens+in.CacheReadTokens+in.CacheWriteTokens == 0 {
		out.Reason = "此图像模型按token计费，请提供实际输入、输出用量；张数不能代替token"
		return out
	}
	if in.Modality == "image" && in.Quality != "" {
		*warnings = append(*warnings, "图像质量需由独立型号或售价规格明确区分；当前销售预览依据型号与分辨率")
	}
	if in.Modality == "image" && !imageTokenBilling {
		// Reuse the gateway calculation so group overrides, channel tiers and
		// historical fallback image prices keep exactly the same precedence.
		calculator := &OpenAIGatewayService{billingService: s.billing, resolver: s.resolver, channelService: s.channels}
		cost := calculator.calculateOpenAIImageCost(ctx, in.Model, key, &OpenAIForwardResult{ImageCount: in.ImageCount, ImageSize: in.Resolution}, multiplier)
		if apiKeyHasConfiguredImagePrice(key, NormalizeImageBillingTierOrDefault(in.Resolution)) {
			out.Source = "group_image_price"
		}
		if resolved.Source != PricingSourceChannel && out.Source != "group_image_price" {
			out.Source = "historical_image_default"
			*warnings = append(*warnings, "图片销售沿用现有默认价，请在模型售价中核对每个型号的正式价格")
		}
		out.Amount, out.Status, out.Reason = &cost.ActualCost, "estimated", "按现有图片售价和独立分组倍率计算站内美元扣费"
		return out
	}
	if imageTokenBilling {
		multiplier = group.RateMultiplier * group.PeakMultiplierAt(s.now())
		if s.cfg != nil && s.cfg.RunMode == "simple" {
			multiplier = 0
		}
		out.Multiplier = &multiplier
	}
	if in.Modality == "video" {
		if resolved.Source == PricingSourceChannel && resolved.Mode == BillingModeToken && in.InputTokens+in.OutputTokens+in.CacheReadTokens+in.CacheWriteTokens == 0 {
			out.Reason = "此视频配置按token计费，请提供实际用量"
			return out
		}
		if !strings.HasPrefix(strings.ToLower(in.Model), "grok-imagine-video") {
			out.Reason = "此视频型号的销售计费需要专用适配，当前预览未确认完整规则"
			return out
		}
		if math.Trunc(in.DurationSeconds) != in.DurationSeconds {
			out.Reason = "视频时长须为整秒"
			return out
		}
		if resolved.Source != PricingSourceChannel || resolved.Mode != BillingModeToken {
			calculator := &OpenAIGatewayService{billingService: s.billing, resolver: s.resolver, channelService: s.channels}
			cost := calculator.calculateOpenAIVideoCost(ctx, in.Model, key, &OpenAIForwardResult{VideoCount: in.Requests, VideoDurationSeconds: int(in.DurationSeconds), VideoResolution: in.Resolution}, multiplier)
			if apiKeyHasConfiguredVideoPrice(key, NormalizeVideoBillingResolutionOrDefault(in.Resolution)) {
				out.Source = "group_video_price"
			}
			out.Amount, out.Status, out.Reason = &cost.ActualCost, "estimated", "按现有视频价格和独立分组倍率计算站内美元扣费"
			return out
		}
		multiplier = group.RateMultiplier * group.PeakMultiplierAt(s.now())
		if s.cfg != nil && s.cfg.RunMode == "simple" {
			multiplier = 0
		}
		out.Multiplier = &multiplier
	}
	longContext := true
	var account *Account
	if in.AccountID > 0 && s.accounts != nil {
		var accountErr error
		account, accountErr = s.accounts.GetByID(ctx, in.AccountID)
		if accountErr != nil || account == nil {
			out.Reason = "所选上游账号不存在"
			return out
		}
		longContext = account.IsOpenAILongContextBillingEnabled()
	}
	tier := in.ServiceTier
	miniMaxPriority := isMiniMaxM3Account(account, in.Model) && tier == "priority"
	if miniMaxPriority {
		tier = ""
	}
	cost, err := s.billing.CalculateCostUnified(CostInput{Ctx: ctx, Model: in.Model, GroupID: &group.ID, Tokens: tokens, RequestCount: requestCount, SizeTier: in.Resolution, RateMultiplier: multiplier, ServiceTier: tier, Resolver: s.resolver, Resolved: resolved, LongContextBillingEnabled: &longContext})
	if err != nil {
		out.Reason = "此模型缺少可计算的销售单价"
		return out
	}
	if miniMaxPriority {
		applyMiniMaxM3PriorityCost(cost)
	}
	out.Amount, out.Status, out.Reason = &cost.ActualCost, "estimated", "按现有模型售价和分组规则计算站内美元扣费"
	return out
}
