//go:build unit

package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/minimax"
	"github.com/stretchr/testify/require"
)

// This runner consumes an explicit sanitized export only. It has no repository,
// network transport, user balance, credential resolver, or billing writer.
type simpleBillingOfflineInventory struct {
	CapturedAt              string                        `json:"captured_at"`
	Accounts                []simpleBillingOfflineAccount `json:"accounts"`
	Groups                  []simpleBillingOfflineGroup   `json:"groups"`
	BillingManagementConfig json.RawMessage               `json:"billing_management_config"`
	PricingData             json.RawMessage               `json:"pricing_data"`
	ChannelModelPricing     json.RawMessage               `json:"channel_model_pricing"`
}

type simpleBillingOfflineAccount struct {
	ID               int64             `json:"id"`
	Type             string            `json:"type"`
	Platform         string            `json:"platform"`
	Status           string            `json:"status"`
	GroupIDs         []int64           `json:"group_ids"`
	ParentAccountID  *int64            `json:"parent_account_id"`
	ConfiguredModels json.RawMessage   `json:"configured_models"`
	ModelMapping     map[string]string `json:"model_mapping"`
	ImageSizes       []string          `json:"image_sizes"`
	BaseURL          string            `json:"base_url"`
	Region           string            `json:"region"`
	Credentials      map[string]string `json:"credentials"`
}

type simpleBillingOfflineGroup struct {
	ID                        int64    `json:"id"`
	Name                      string   `json:"name"`
	Platform                  string   `json:"platform"`
	Status                    string   `json:"status"`
	RateMultiplier            *float64 `json:"rate_multiplier"`
	AllowImageGeneration      bool     `json:"allow_image_generation"`
	AllowBatchImageGeneration bool     `json:"allow_batch_image_generation"`
	WebSearchPricePerCall     *float64 `json:"web_search_price_per_call"`
}

type simpleBillingOfflineRow struct {
	BillingCatalogEntry
	HasImageInput bool   `json:"has_image_input,omitempty"`
	Required      bool   `json:"required"`
	AccountStatus string `json:"account_status"`
}

type simpleBillingOfflineReport struct {
	CapturedAt            string                    `json:"captured_at"`
	GeneratedAt           string                    `json:"generated_at"`
	ConfigVersion         int64                     `json:"config_version"`
	APISaleMultiplier     float64                   `json:"api_sale_multiplier"`
	USDToCNYRate          float64                   `json:"usd_to_cny_rate"`
	RequiredReady         int                       `json:"required_ready"`
	RequiredUnknown       int                       `json:"required_unknown"`
	VisionReady           int                       `json:"vision_ready"`
	VisionUnknown         int                       `json:"vision_unknown"`
	Rows                  []simpleBillingOfflineRow `json:"rows"`
	Warnings              []string                  `json:"warnings"`
	IgnoredLegacyChannels int                       `json:"ignored_legacy_sale_channels"`
	InactiveRows          int                       `json:"inactive_rows"`
}

func sanitizedSimpleOfflineAccount(in simpleBillingOfflineAccount) (*Account, map[string]string, error) {
	if in.ID <= 0 || strings.TrimSpace(in.Type) == "" || strings.TrimSpace(in.Platform) == "" {
		return nil, nil, fmt.Errorf("inventory account requires a positive ID, type and platform")
	}
	credentials := map[string]any{}
	for key, value := range in.Credentials {
		if !slices.Contains([]string{"base_url", "aws_region", "location", "vertex_location"}, key) {
			return nil, nil, fmt.Errorf("account %d: credential field %q is not allowed in the sanitized inventory", in.ID, key)
		}
		credentials[key] = value
	}
	if in.BaseURL != "" {
		credentials["base_url"] = in.BaseURL
	}
	if value, ok := credentials["base_url"].(string); ok && value != "" {
		parsed, err := url.Parse(value)
		if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, nil, fmt.Errorf("account %d: base_url must contain only scheme, host and path", in.ID)
		}
	}
	if in.Region != "" {
		if in.Type == AccountTypeBedrock {
			credentials["aws_region"] = in.Region
		}
		if in.Type == AccountTypeServiceAccount {
			credentials["location"] = in.Region
		}
	}
	mapping := map[string]string{}
	if len(in.ConfiguredModels) > 0 && string(in.ConfiguredModels) != "null" {
		var models []string
		if err := json.Unmarshal(in.ConfiguredModels, &models); err == nil {
			for _, model := range models {
				mapping[model] = model
			}
		} else if err := json.Unmarshal(in.ConfiguredModels, &mapping); err != nil {
			return nil, nil, fmt.Errorf("account %d: configured_models must be a string array or alias-to-model object", in.ID)
		}
	}
	for alias, model := range in.ModelMapping {
		mapping[alias] = model
	}
	status := in.Status
	if status == "" {
		status = StatusActive
	}
	return &Account{ID: in.ID, Type: in.Type, Platform: in.Platform, Status: status, GroupIDs: in.GroupIDs, ParentAccountID: in.ParentAccountID, Credentials: credentials}, mapping, nil
}

func analyzeSimpleBillingOfflineInventory(in simpleBillingOfflineInventory, now time.Time) (*simpleBillingOfflineReport, error) {
	cfg, err := decodeBillingManagementConfig(string(in.BillingManagementConfig))
	if err != nil {
		return nil, err
	}
	// Readiness predicts the new policy even if production still uses legacy.
	cfg.SimpleBillingEnabled = true
	if err := ValidateBillingManagementConfig(cfg); err != nil {
		return nil, err
	}
	pricing := &PricingService{}
	if len(in.PricingData) > 0 && string(in.PricingData) != "null" && string(in.PricingData) != "{}" {
		// Reuse the production parser so absent token prices remain absent;
		// unmarshalling the runtime float struct would turn missing into zero.
		pricing.pricingData, err = pricing.parsePricingData(in.PricingData)
		if err != nil {
			return nil, err
		}
	}
	billing := NewBillingService(&config.Config{}, pricing)
	legacyChannelCount := 0
	var legacyArray []json.RawMessage
	var legacyObject map[string]json.RawMessage
	if len(in.ChannelModelPricing) > 0 && string(in.ChannelModelPricing) != "null" {
		if json.Unmarshal(in.ChannelModelPricing, &legacyArray) == nil {
			legacyChannelCount = len(legacyArray)
		} else if json.Unmarshal(in.ChannelModelPricing, &legacyObject) == nil {
			legacyChannelCount = len(legacyObject)
		} else {
			return nil, fmt.Errorf("channel_model_pricing must be an array or object")
		}
	}
	out := &simpleBillingOfflineReport{CapturedAt: in.CapturedAt, GeneratedAt: now.UTC().Format(time.RFC3339), ConfigVersion: cfg.Version, APISaleMultiplier: cfg.APISaleMultiplier, USDToCNYRate: FixedUSDToCNYRate, Rows: []simpleBillingOfflineRow{}, Warnings: []string{}, IgnoredLegacyChannels: legacyChannelCount}
	if legacyChannelCount > 0 {
		out.Warnings = append(out.Warnings, "旧渠道售价未被当作API采购成本；必须命中维护参考或绑定的人工价表")
	}
	groups := map[int64]*Group{}
	for _, group := range in.Groups {
		if group.ID <= 0 || group.RateMultiplier == nil || !finiteNonnegative(*group.RateMultiplier) {
			return nil, fmt.Errorf("group %d: explicit nonnegative rate_multiplier is required", group.ID)
		}
		status := group.Status
		if status == "" {
			status = StatusActive
		}
		groups[group.ID] = &Group{ID: group.ID, Name: group.Name, Platform: group.Platform, Status: status, RateMultiplier: *group.RateMultiplier, AllowImageGeneration: group.AllowImageGeneration, AllowBatchImageGeneration: group.AllowBatchImageGeneration, WebSearchPricePerCall: group.WebSearchPricePerCall}
	}
	for _, rawAccount := range in.Accounts {
		account, mapping, err := sanitizedSimpleOfflineAccount(rawAccount)
		if err != nil {
			return nil, err
		}
		if account.Status != StatusActive {
			out.Warnings = append(out.Warnings, fmt.Sprintf("账号%d当前状态%s，其覆盖结果仅用于恢复账号前检查，不计入活跃上线必需项", account.ID, account.Status))
		}
		if len(mapping) == 0 {
			out.Warnings = append(out.Warnings, fmt.Sprintf("账号%d没有显式配置模型，未将支持全部模型冒充已开放库存", account.ID))
		}
		for alias, model := range mapping {
			if alias == "" || strings.TrimSpace(model) == "" || strings.Contains(alias, "*") || strings.Contains(model, "*") {
				out.Warnings = append(out.Warnings, fmt.Sprintf("账号%d的通配声明未作为精确型号预检", account.ID))
				continue
			}
			modality := inferModelEndpointCapability(model, account.Platform).ModelType
			if !slices.Contains([]string{"chat", "image", "video"}, modality) {
				modality = "chat"
			}
			for _, profile := range cfg.CostProfiles {
				if !profile.Enabled || !slices.Contains(profile.AccountIDs, account.ID) {
					continue
				}
				for _, row := range profile.Prices {
					if row.Operation != "" {
						continue
					}
					for _, pattern := range row.Models {
						if matchesCostModel(pattern, model) {
							modality = row.Modality
						}
					}
				}
			}
			resolutions := []string{""}
			if modality == "video" {
				if spec, ok := minimax.VideoModelSpecFor(model); ok {
					resolutions = spec.Resolutions
				} else if strings.EqualFold(model, "grok-imagine-video") {
					resolutions = []string{"480P", "720P"}
				} else if strings.EqualFold(model, "grok-imagine-video-1.5") {
					resolutions = []string{"480P", "720P", "1080P"}
				}
			}
			if modality == "image" {
				resolutions = append([]string{}, rawAccount.ImageSizes...)
				if len(resolutions) == 0 {
					if strings.EqualFold(model, "doubao-seedream-5-0-pro-260628") {
						resolutions = []string{"1K", "1.5K", "2K"}
					} else {
						resolutions = []string{"2K"}
					}
				}
				for _, profile := range cfg.CostProfiles {
					if !profile.Enabled || !slices.Contains(profile.AccountIDs, account.ID) {
						continue
					}
					for _, row := range profile.Prices {
						if row.Modality != "image" || row.Resolution == "" {
							continue
						}
						for _, pattern := range row.Models {
							if matchesCostModel(pattern, model) && !slices.Contains(resolutions, row.Resolution) {
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
			for _, groupID := range groupIDs {
				for _, resolution := range resolutions {
					visionModes := []bool{false}
					if modality == "chat" {
						visionModes = append(visionModes, true)
					}
					for _, vision := range visionModes {
						entry := simpleBillingOfflineRow{BillingCatalogEntry: BillingCatalogEntry{Key: fmt.Sprintf("%d:%d:%s:%s:%t", account.ID, groupID, alias, resolution, vision), AccountIDs: []int64{account.ID}, GroupID: groupID, Platform: account.Platform, Model: alias, UpstreamModel: model, Modality: modality, Resolution: resolution, Route: procurementRoute(account), Currency: "CNY", ConfigVersion: cfg.Version, Status: "unknown", BasePrices: []BillingCatalogPrice{}}, HasImageInput: vision, Required: !vision && account.Status == StatusActive, AccountStatus: account.Status}
						entry.Provider, entry.Region, _ = simpleAccountProvider(account)
						group := groups[groupID]
						if group == nil || group.Status != StatusActive {
							entry.Reason = "上游账号未绑定有效分组"
						} else {
							q, resolveErr := billing.resolveSimpleBilling(cfg, SimpleBillingInput{Account: account, Group: group, Model: model, Modality: modality, Resolution: resolution, Scene: "single_image", HasImageInput: vision}, now)
							if resolveErr != nil {
								entry.Reason = resolveErr.Error()
							} else {
								entry.Status, entry.Route, entry.Source, entry.Multiplier = "ready", q.Route, q.Source, q.Multiplier
								entry.BasePrices = simpleCatalogPrices(q)
								entry.Reason = "离线匹配通过；未调用上游，未验证API余额或真实生成能力"
							}
						}
						out.Rows = append(out.Rows, entry)
						if account.Status != StatusActive {
							out.InactiveRows++
						} else if vision {
							if entry.Status == "ready" {
								out.VisionReady++
							} else {
								out.VisionUnknown++
							}
						} else if entry.Status == "ready" {
							out.RequiredReady++
						} else {
							out.RequiredUnknown++
						}
					}
				}
			}
		}
	}
	slices.SortFunc(out.Rows, func(a, b simpleBillingOfflineRow) int { return strings.Compare(a.Key, b.Key) })
	return out, nil
}

func TestSimpleBillingOfflineInventoryRunner(t *testing.T) {
	path := os.Getenv("STARX_BILLING_INVENTORY_PATH")
	if path == "" {
		t.Skip("set STARX_BILLING_INVENTORY_PATH to an explicitly sanitized inventory export")
	}
	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 2*1024*1024+1))
	require.NoError(t, err)
	require.LessOrEqual(t, len(raw), 2*1024*1024, "sanitized inventory exceeds 2 MiB")
	var inventory simpleBillingOfflineInventory
	require.NoError(t, json.Unmarshal(raw, &inventory))
	report, err := analyzeSimpleBillingOfflineInventory(inventory, time.Now())
	require.NoError(t, err)
	output, err := json.MarshalIndent(report, "", "  ")
	require.NoError(t, err)
	if reportPath := os.Getenv("STARX_BILLING_INVENTORY_REPORT"); reportPath != "" {
		require.NoError(t, os.WriteFile(reportPath, append(output, '\n'), 0600))
		t.Logf("Read-only pricing readiness: regular ready=%d unknown=%d; vision ready=%d unknown=%d; report=%s", report.RequiredReady, report.RequiredUnknown, report.VisionReady, report.VisionUnknown, reportPath)
	} else {
		t.Log(string(output))
	}
}

func TestSimpleBillingOfflineInventoryKeepsUnknownAndExplicitSpecifications(t *testing.T) {
	modelList := func(models ...string) json.RawMessage {
		raw, err := json.Marshal(models)
		require.NoError(t, err)
		return raw
	}
	input := simpleBillingOfflineInventory{Accounts: []simpleBillingOfflineAccount{
		{ID: 8, Type: AccountTypeAPIKey, Platform: PlatformOpenAI, GroupIDs: []int64{7}, BaseURL: "https://api.minimax.cn/v1", ConfiguredModels: modelList("MiniMax-M3", "MiniMax-H3")},
		{ID: 9, Type: AccountTypeAPIKey, Platform: PlatformOpenAI, GroupIDs: []int64{7}, BaseURL: "https://open.bigmodel.cn/api/paas/v4", ConfiguredModels: modelList("glm-5.3", "glm-5.3-flashx")},
		{ID: 10, Type: AccountTypeAPIKey, Platform: PlatformOpenAI, GroupIDs: []int64{7}, BaseURL: "https://unconfirmed.example/v1", ConfiguredModels: modelList("gpt-5.4")},
	}, Groups: []simpleBillingOfflineGroup{{ID: 7, Status: StatusActive, RateMultiplier: costNumber(2)}}}
	report, err := analyzeSimpleBillingOfflineInventory(input, time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Equal(t, 5, report.RequiredReady)
	require.Equal(t, 1, report.RequiredUnknown)
	require.Equal(t, 3, report.VisionUnknown)
	for _, row := range report.Rows {
		if row.Model == "MiniMax-H3" {
			require.Contains(t, []string{"768P", "2K"}, row.Resolution)
			require.Equal(t, "ready", row.Status)
		}
		if row.AccountIDs[0] == 10 {
			require.Equal(t, "unknown", row.Status)
			require.Empty(t, row.BasePrices)
		}
	}
}

func TestSimpleBillingOfflineInventoryRejectsCredentialsAndURLSecrets(t *testing.T) {
	for _, account := range []simpleBillingOfflineAccount{
		{ID: 8, Type: AccountTypeAPIKey, Platform: PlatformOpenAI, Credentials: map[string]string{"api_key": "not-a-real-secret"}},
		{ID: 8, Type: AccountTypeAPIKey, Platform: PlatformOpenAI, BaseURL: "https://userinfo:password@example.com/v1"},
		{ID: 8, Type: AccountTypeAPIKey, Platform: PlatformOpenAI, BaseURL: "https://example.com/v1?api_key=not-a-real-secret"},
	} {
		_, _, err := sanitizedSimpleOfflineAccount(account)
		require.Error(t, err)
		require.NotContains(t, err.Error(), "not-a-real-secret")
		require.NotContains(t, err.Error(), "password")
	}
}
