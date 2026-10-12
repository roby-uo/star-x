package service

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/minimax"
)

type BillingCatalogPrice struct {
	Label       string  `json:"label"`
	Unit        string  `json:"unit"`
	AmountCNY   float64 `json:"amount_cny"`
	Resolution  string  `json:"resolution,omitempty"`
	Quality     string  `json:"quality,omitempty"`
	ServiceTier string  `json:"service_tier,omitempty"`
	MinTokens   int     `json:"min_tokens,omitempty"`
	MaxTokens   *int    `json:"max_tokens,omitempty"`
}

type BillingCatalogEntry struct {
	Key           string                `json:"key"`
	Platform      string                `json:"platform"`
	Model         string                `json:"model"`
	UpstreamModel string                `json:"upstream_model"`
	Modality      string                `json:"modality"`
	AccountIDs    []int64               `json:"account_ids"`
	GroupID       int64                 `json:"group_id"`
	Route         string                `json:"route"`
	Source        string                `json:"source"`
	Provider      string                `json:"provider"`
	Region        string                `json:"region"`
	Status        string                `json:"status"`
	Currency      string                `json:"currency"`
	ConfigVersion int64                 `json:"config_version"`
	Multiplier    float64               `json:"multiplier"`
	Resolution    string                `json:"resolution,omitempty"`
	BasePrices    []BillingCatalogPrice `json:"base_prices"`
	Reason        string                `json:"reason"`
	Operation     string                `json:"operation,omitempty"`
}

// Catalog is an administrator-only readiness view, not a paid model request.
// Only explicitly configured model IDs are enumerated; supporting every model
// does not establish either availability or a procurement price.
func (s *BillingManagementService) Catalog(ctx context.Context) ([]BillingCatalogEntry, error) {
	cfg, err := s.GetConfig(ctx)
	if err != nil {
		return nil, err
	}
	accounts, err := s.accounts.ListAllWithFilters(ctx, "", "", StatusActive, "", 0, "")
	if err != nil {
		return nil, err
	}
	groups, err := s.groups.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	groupByID := map[int64]*Group{}
	for i := range groups {
		groupByID[groups[i].ID] = &groups[i]
	}
	out := make([]BillingCatalogEntry, 0)
	for i := range accounts {
		account := &accounts[i]
		mapping := account.GetModelMapping()
		for model, upstream := range mapping {
			if strings.Contains(model, "*") || strings.Contains(upstream, "*") || strings.TrimSpace(upstream) == "" {
				continue
			}
			modality := inferModelEndpointCapability(upstream, account.Platform).ModelType
			if modality == "unknown" || modality == "" {
				modality = "chat"
				for _, profile := range cfg.CostProfiles {
					if !slices.Contains(profile.AccountIDs, account.ID) {
						continue
					}
					for _, row := range profile.Prices {
						if row.Operation != "" {
							continue
						}
						for _, pattern := range row.Models {
							if matchesCostModel(pattern, upstream) {
								modality = row.Modality
							}
						}
					}
				}
			}
			resolutions := []string{""}
			if spec, ok := minimax.VideoModelSpecFor(upstream); ok {
				resolutions = spec.Resolutions
			}
			if strings.EqualFold(upstream, "grok-imagine-video") || strings.EqualFold(upstream, "grok-imagine-video-1.5") {
				resolutions = []string{"480P", "720P", "1080P"}
			}
			if modality == "image" {
				if strings.EqualFold(upstream, "doubao-seedream-5-0-pro-260628") {
					resolutions = []string{"1K", "1.5K", "2K"}
				} else {
					resolutions = []string{"2K"}
				}
				for _, profile := range cfg.CostProfiles {
					if !slices.Contains(profile.AccountIDs, account.ID) {
						continue
					}
					for _, row := range profile.Prices {
						for _, pattern := range row.Models {
							if matchesCostModel(pattern, upstream) && row.Resolution != "" && !slices.Contains(resolutions, row.Resolution) {
								resolutions = append(resolutions, row.Resolution)
							}
						}
					}
				}
			}
			groupIDs := account.GroupIDs
			if len(groupIDs) == 0 {
				groupIDs = []int64{0}
			}
			operations := []string{""}
			if modality == "chat" {
				for _, profile := range cfg.CostProfiles {
					if !profile.Enabled || !slices.Contains(profile.AccountIDs, account.ID) {
						continue
					}
					for _, row := range profile.Prices {
						if row.Operation != "alpha_search" {
							continue
						}
						for _, pattern := range row.Models {
							if matchesCostModel(pattern, upstream) && !slices.Contains(operations, "alpha_search") {
								operations = append(operations, "alpha_search")
							}
						}
					}
				}
			}
			for _, groupID := range groupIDs {
				for _, resolution := range resolutions {
					for _, operation := range operations {
						entry := BillingCatalogEntry{Key: fmt.Sprintf("%d:%d:%s:%s:%s", account.ID, groupID, model, resolution, operation), Platform: account.Platform, Model: model, UpstreamModel: upstream, Modality: modality, AccountIDs: []int64{account.ID}, GroupID: groupID, Route: procurementRoute(account), Status: "unknown", Currency: "CNY", ConfigVersion: cfg.Version, Resolution: resolution, BasePrices: []BillingCatalogPrice{}, Operation: operation}
						entry.Provider, entry.Region, _ = simpleAccountProvider(account)
						group := groupByID[groupID]
						if group == nil {
							entry.Reason = "上游账号尚未绑定有效分组"
							out = append(out, entry)
							continue
						}
						q, resolveErr := s.billing.resolveSimpleBilling(cfg, SimpleBillingInput{Account: account, Group: group, Model: upstream, Modality: modality, Resolution: resolution, Scene: "single_image", Operation: operation}, s.now())
						if resolveErr != nil {
							entry.Reason = resolveErr.Error()
							out = append(out, entry)
							continue
						}
						entry.Route, entry.Source, entry.Multiplier = q.Route, q.Source, q.Multiplier
						entry.BasePrices = simpleCatalogPrices(q)
						entry.Status = "ready"
						entry.Reason = "基准价已匹配；销售按唯一倍率，人民币统一展示"
						if q.Route == "account" || q.Source == "official_reference" {
							entry.Reason = "使用维护的参考基准，不等于供应商已确认实付成本"
						}
						out = append(out, entry)
					}
				}
			}
		}
	}
	slices.SortFunc(out, func(a, b BillingCatalogEntry) int { return strings.Compare(a.Key, b.Key) })
	return out, nil
}

func simpleCatalogPrices(q *SimpleBillingSnapshot) []BillingCatalogPrice {
	out := []BillingCatalogPrice{}
	add := func(label, unit string, value float64, row CostPriceRow) {
		out = append(out, BillingCatalogPrice{Label: label, Unit: unit, AmountCNY: value, Resolution: row.Resolution, Quality: row.Quality, ServiceTier: row.ServiceTier, MinTokens: row.MinTokens, MaxTokens: row.MaxTokens})
	}
	if q.ReferencePricing != nil {
		p := q.ReferencePricing
		for _, item := range []struct {
			label string
			price float64
		}{{"输入", p.InputPricePerToken}, {"输出", p.OutputPricePerToken}, {"缓存读取", p.CacheReadPricePerToken}, {"缓存写入", p.CacheCreationPricePerToken}} {
			add(item.label, "per_million_tokens", item.price*1000000*FixedUSDToCNYRate, CostPriceRow{})
		}
		// A missing optional image tariff is not a confirmed free component.
		if p.ImageInputPricePerToken > 0 || p.ImageInputPriceExplicit {
			add("图像输入token", "per_million_tokens", p.ImageInputPricePerToken*1000000*FixedUSDToCNYRate, CostPriceRow{})
		}
		if p.ImageOutputPricePerToken > 0 || p.ImageOutputPriceExplicit {
			add("图像输出token", "per_million_tokens", p.ImageOutputPricePerToken*1000000*FixedUSDToCNYRate, CostPriceRow{})
		}
		return out
	}
	if q.UnitPriceUSD != nil {
		add("基准单价", q.BillingUnit, *q.UnitPriceUSD*FixedUSDToCNYRate, CostPriceRow{Resolution: q.Resolution})
		return out
	}
	scale := q.DiscountMultiplier
	if q.Currency == "USD" {
		scale *= FixedUSDToCNYRate
	}
	for _, row := range q.Rows {
		for _, item := range []struct {
			label string
			price *float64
		}{{"输入", row.InputPrice}, {"输出", row.OutputPrice}, {"缓存读取", row.CacheReadPrice}, {"缓存写入", row.CacheWritePrice}, {"图像输入token", row.ImageInputPrice}, {"图像输出token", row.ImageOutputPrice}, {"基准单价", row.UnitPrice}, {"首张之后的参考输入图", row.InputImageUnitPrice}} {
			if item.price != nil {
				add(item.label, row.BillingUnit, *item.price*scale, row)
			}
		}
	}
	return out
}
