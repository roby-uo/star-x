//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/minimax"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func simpleTestBilling(t *testing.T, cfg BillingManagementConfig) *BillingService {
	t.Helper()
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	s := ProvideBillingService(&config.Config{}, nil, &memoryCostBook{raw: string(raw)})
	return s
}

func simpleTestConfig() BillingManagementConfig {
	return BillingManagementConfig{Version: 7, SimpleBillingEnabled: true, APISaleMultiplier: 1.4, USDToCNYRate: 6.2}
}
func simpleAPIAccount(id int64, base string) *Account {
	return &Account{ID: id, Type: AccountTypeAPIKey, Platform: PlatformOpenAI, Status: StatusActive, Credentials: map[string]any{"base_url": base}}
}

func TestSimpleBillingDefaultsAndDisabledCompatibility(t *testing.T) {
	cfg, err := decodeBillingManagementConfig("")
	require.NoError(t, err)
	require.True(t, cfg.SimpleBillingEnabled)
	require.Equal(t, 1.4, cfg.APISaleMultiplier)
	require.Equal(t, 6.2, cfg.USDToCNYRate)
	cfg, err = decodeBillingManagementConfig(`{"simple_billing_enabled":false,"usd_to_cny_rate":9,"api_sale_multiplier":1.6}`)
	require.NoError(t, err)
	require.False(t, cfg.SimpleBillingEnabled)
	require.Equal(t, 6.2, cfg.USDToCNYRate)
	s := simpleTestBilling(t, *cfg)
	q, err := s.PrepareSimpleBilling(context.Background(), SimpleBillingInput{})
	require.NoError(t, err)
	require.Nil(t, q)
}

func TestSimpleBillingNativeCNYAndDollarBasisUseOnlyAPIMultiplier(t *testing.T) {
	for _, currency := range []string{"CNY", "USD"} {
		t.Run(currency, func(t *testing.T) {
			cfg := simpleTestConfig()
			cfg.CostProfiles = []CostProfile{{ID: "contract", Name: "Confirmed supplier", Enabled: true, AccountIDs: []int64{8}, PurchaseMode: "payg", Currency: currency, DiscountMultiplier: .8, Prices: []CostPriceRow{{ID: "text", Models: []string{"custom-model"}, Modality: "chat", BillingUnit: "per_million_tokens", InputPrice: costNumber(2), OutputPrice: costNumber(8), CacheReadPrice: costNumber(.4), CacheWritePrice: costNumber(0)}}}}
			s := simpleTestBilling(t, cfg)
			group := &Group{ID: 7, RateMultiplier: 10, ImageRateIndependent: true, ImageRateMultiplier: 9, VideoRateIndependent: true, VideoRateMultiplier: 8, PeakRateEnabled: true, PeakRateMultiplier: 7}
			q, err := s.PrepareSimpleBilling(context.Background(), SimpleBillingInput{Account: simpleAPIAccount(8, "https://reseller.example/v1"), Group: group, Model: "custom-model", Modality: "chat"})
			require.NoError(t, err)
			cost, err := s.CalculateSimpleBilling(q, BillingPreviewInput{Model: q.Model, InputTokens: 1000, OutputTokens: 500, CacheReadTokens: 100, Requests: 1})
			require.NoError(t, err)
			base := (2.0*1000 + 8.0*500 + .4*100) / 1000000 * .8
			if currency == "CNY" {
				base /= 6.2
			}
			require.InDelta(t, base, cost.TotalCost, 1e-14)
			require.InDelta(t, base*1.4, cost.ActualCost, 1e-14)
			require.Equal(t, 1.4, q.Multiplier)
		})
	}
}

func TestSimpleBillingMiniMaxChinaSpecificationsAndFrozenVersion(t *testing.T) {
	cfg := simpleTestConfig()
	s := simpleTestBilling(t, cfg)
	group := &Group{ID: 7, RateMultiplier: 10, VideoRateIndependent: true, VideoRateMultiplier: 20}
	for _, tc := range []struct {
		model, resolution string
		price             float64
		duration          int
	}{{"MiniMax-H3", "768P", .5, 4}, {"MiniMax-H3", "2K", .8, 4}, {"MiniMax-H3-Max", "480P", .33, 5}, {"MiniMax-H3-Max", "768P", .5, 5}} {
		q, err := s.PrepareSimpleBilling(context.Background(), SimpleBillingInput{Account: simpleAPIAccount(8, "https://api.minimax.cn/v1"), Group: group, Model: tc.model, Modality: "video", Resolution: tc.resolution})
		require.NoError(t, err)
		cost, err := s.CalculateSimpleBilling(q, BillingPreviewInput{Model: q.Model, Requests: 1, DurationSeconds: float64(tc.duration)})
		require.NoError(t, err)
		require.Equal(t, "cn", q.Region)
		require.InDelta(t, tc.price*float64(tc.duration)*1.4, cost.ActualCost*6.2, 1e-12)
		// Updating the cached current policy must not change an accepted task.
		s.simpleMu.Lock()
		s.simpleConfig.APISaleMultiplier = 3
		s.simpleMu.Unlock()
		again, err := s.CalculateSimpleBilling(q, BillingPreviewInput{Model: q.Model, Requests: 1, DurationSeconds: float64(tc.duration)})
		require.NoError(t, err)
		require.Equal(t, cost.ActualCost, again.ActualCost)
		s.invalidateSimpleBillingConfig()
	}
	_, err := s.PrepareSimpleBilling(context.Background(), SimpleBillingInput{Account: simpleAPIAccount(8, "https://api.minimax.cn/v1"), Group: group, Model: "MiniMax-H3", Modality: "video", Resolution: "1080P"})
	require.Error(t, err)
}

func TestSimpleBillingAccountTextAndMediaUseGroupReferenceMultiplier(t *testing.T) {
	s := simpleTestBilling(t, simpleTestConfig())
	group := &Group{ID: 7, RateMultiplier: 2, ImageRateIndependent: true, ImageRateMultiplier: 9, VideoRateIndependent: true, VideoRateMultiplier: 8}
	account := &Account{ID: 8, Type: AccountTypeOAuth, Platform: PlatformOpenAI}
	q, err := s.PrepareSimpleBilling(context.Background(), SimpleBillingInput{Account: account, Group: group, Model: "gpt-5.4", Modality: "chat"})
	require.NoError(t, err)
	base, err := s.CalculateCost("gpt-5.4", UsageTokens{InputTokens: 1000, OutputTokens: 100}, 1)
	require.NoError(t, err)
	actual, err := s.CalculateSimpleBilling(q, BillingPreviewInput{Model: q.Model, InputTokens: 1000, OutputTokens: 100, Requests: 1})
	require.NoError(t, err)
	require.InDelta(t, base.TotalCost*2, actual.ActualCost, 1e-12)
	q, err = s.PrepareSimpleBilling(context.Background(), SimpleBillingInput{Account: account, Group: group, Model: "grok-imagine-video", Modality: "video", Resolution: "480P"})
	require.NoError(t, err)
	require.Equal(t, "per_second", q.BillingUnit)
	require.Equal(t, 2.0, q.Multiplier)
}

func TestSimpleBillingUnconfirmedAndAmbiguousPricesFailBeforeCall(t *testing.T) {
	cfg := simpleTestConfig()
	cfg.CostProfiles = []CostProfile{{ID: "manual", Name: "Manual", Enabled: true, AccountIDs: []int64{8}, PurchaseMode: "payg", Currency: "CNY", DiscountMultiplier: 1, Prices: []CostPriceRow{{ID: "image", Models: []string{"draw"}, Modality: "image", BillingUnit: "per_image", UnitPrice: costNumber(.2)}}}}
	group := &Group{ID: 7, RateMultiplier: 1}
	s := simpleTestBilling(t, cfg)
	_, err := s.PrepareSimpleBilling(context.Background(), SimpleBillingInput{Account: simpleAPIAccount(9, "https://reseller.example/v1"), Group: group, Model: "gpt-5.4", Modality: "chat"})
	require.Error(t, err)
	_, err = s.PrepareSimpleBilling(context.Background(), SimpleBillingInput{Account: simpleAPIAccount(9, "https://api.openai.com"), Group: group, Model: "gpt-unknown-suffix", Modality: "chat"})
	require.Error(t, err)
	cfg.CostProfiles[0].Prices = append(cfg.CostProfiles[0].Prices, cfg.CostProfiles[0].Prices[0])
	s = simpleTestBilling(t, cfg)
	_, err = s.PrepareSimpleBilling(context.Background(), SimpleBillingInput{Account: simpleAPIAccount(8, "https://reseller.example/v1"), Group: group, Model: "draw", Modality: "image"})
	require.Error(t, err)
	_, err = s.PrepareSimpleBilling(context.Background(), SimpleBillingInput{Account: simpleAPIAccount(9, "https://api.openai.com"), Group: group, Model: "unknown-image", Modality: "image", Resolution: "2K"})
	require.Error(t, err)
}

func TestSimpleBillingRealRecordUsageDeductsFrozenCostAndSavesPolicy(t *testing.T) {
	logs := &openAIRecordUsageLogRepoStub{inserted: true}
	bills := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, bills, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, &openAIUserGroupRateRepoStub{rate: costNumber(9)})
	svc.billingService = simpleTestBilling(t, simpleTestConfig())
	group := &Group{ID: 7, RateMultiplier: 5, ImageRateIndependent: true, ImageRateMultiplier: 9, PeakRateEnabled: true, PeakRateMultiplier: 10}
	account := simpleAPIAccount(8, "https://api.minimax.cn/v1")
	account.RateMultiplier = costNumber(10)
	q, err := svc.PrepareSimpleBilling(context.Background(), SimpleBillingInput{Account: account, Group: group, Model: "MiniMax-M3", Modality: "chat"})
	require.NoError(t, err)
	key := &APIKey{ID: 22, UserID: 11, GroupID: &group.ID, Group: group}
	err = svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{APIKey: key, User: &User{ID: 11}, Account: account, Result: &OpenAIForwardResult{RequestID: "simple-real", Model: "MiniMax-M3", SimpleBilling: q, Usage: OpenAIUsage{InputTokens: 1000, OutputTokens: 500}}})
	require.NoError(t, err)
	require.InDelta(t, (2.1*1000+8.4*500)/1000000*1.4/6.2, bills.lastCmd.BalanceCost, 1e-12)
	require.Equal(t, q, bills.lastCmd.PriceSnapshot)
	require.Equal(t, 1.4, logs.lastLog.RateMultiplier)
	require.Equal(t, 1.0, *logs.lastLog.AccountRateMultiplier)
	require.Equal(t, &group.ID, logs.lastLog.GroupID)
}

func TestSimpleBillingMixedTextAndImageChargesBothAtConfirmedRates(t *testing.T) {
	s := simpleTestBilling(t, simpleTestConfig())
	text := &SimpleBillingSnapshot{Multiplier: 1.4, BillingUnit: "per_million_tokens", ReferencePricing: &ModelPricing{InputPricePerToken: 1e-6, OutputPricePerToken: 2e-6}, ImageBilling: &SimpleBillingSnapshot{Multiplier: 1.4, BillingUnit: "per_million_tokens", ReferencePricing: &ModelPricing{ImageInputPricePerToken: 8e-6, ImageOutputPricePerToken: 30e-6, ImageOutputPriceExplicit: true}}}
	cost, err := s.CalculateSimpleBilling(text, BillingPreviewInput{InputTokens: 110, ImageInputTokens: 10, OutputTokens: 220, ImageOutputTokens: 200, ImageCount: 1, Requests: 1})
	require.NoError(t, err)
	require.InDelta(t, (100e-6+40e-6+80e-6+6000e-6)*1.4, cost.ActualCost, 1e-12)
	require.InDelta(t, 6000e-6, cost.ImageOutputCost, 1e-12)
}

func TestSimpleBillingLegacyVideoQuoteRemainsAccepted(t *testing.T) {
	s := &OpenAIGatewayService{billingService: simpleTestBilling(t, simpleTestConfig())}
	old := &OpenAIForwardResult{VideoCount: 1, VideoDurationSeconds: 4, VideoQuote: &VideoQuote{UnitPrice: .08, Multiplier: 1.4, Total: .448, Version: "legacy-version"}}
	cost, err := s.calculateOpenAIRecordUsageCost(context.Background(), old, nil, nil, 9, 9, 9, 9, UsageTokens{}, "", true)
	require.NoError(t, err)
	require.Equal(t, .448, cost.ActualCost)
	require.Nil(t, old.SimpleBilling)
}

func TestSimpleBillingChinaReferenceAndSeedreamProInputExtras(t *testing.T) {
	s := simpleTestBilling(t, simpleTestConfig())
	group := &Group{ID: 7, RateMultiplier: 8}
	q, err := s.PrepareSimpleBilling(context.Background(), SimpleBillingInput{Account: simpleAPIAccount(8, "https://api.deepseek.com"), Group: group, Model: "deepseek-flash", Modality: "chat"})
	require.NoError(t, err)
	cost, err := s.CalculateSimpleBilling(q, BillingPreviewInput{Model: q.Model, InputTokens: 1000000, OutputTokens: 1000000, Requests: 1})
	require.NoError(t, err)
	require.InDelta(t, 14, cost.ActualCost*6.2, 1e-12)
	for _, tc := range []struct {
		size  string
		price float64
	}{{"1.5K", .3}, {"2K", .6}, {"2048x1024", .3}, {"2048x2048", .6}} {
		q, err = s.PrepareSimpleBilling(context.Background(), SimpleBillingInput{Account: simpleAPIAccount(9, "https://ark.cn-beijing.volces.com/api/v3"), Group: group, Model: "doubao-seedream-5-0-pro-260628", Modality: "image", Resolution: tc.size, Scene: "single_image", InputImageCount: 3})
		require.NoError(t, err)
		cost, err = s.CalculateSimpleBilling(q, BillingPreviewInput{Model: q.Model, ImageCount: 1, Requests: 1})
		require.NoError(t, err)
		require.InDelta(t, (tc.price+.04)*1.4, cost.ActualCost*6.2, 1e-12)
	}
}

func TestSimpleBillingMiniMaxAcceptedQuoteHasPolicyAndUnit(t *testing.T) {
	s := &OpenAIGatewayService{billingService: simpleTestBilling(t, simpleTestConfig())}
	group := &Group{ID: 7, RateMultiplier: 9, AllowImageGeneration: true}
	q, err := s.QuoteMiniMaxVideoForAccount(context.Background(), &APIKey{GroupID: &group.ID, Group: group}, simpleAPIAccount(8, "https://api.minimax.cn/v1"), minimax.CreateVideoRequest{Model: "MiniMax-H3", Resolution: "768P", Duration: 4, Ratio: "16:9", Content: []minimax.VideoContent{{Type: "text", Text: "Price calculation only"}}})
	require.NoError(t, err)
	require.NotNil(t, q.SimpleBilling)
	require.InDelta(t, 2.8, q.Total*6.2, 1e-9)
	require.NotEmpty(t, q.Version)
	require.Equal(t, int64(7), q.SimpleBilling.Version)
}

type simpleCatalogAccounts struct {
	AccountRepository
	accounts []Account
}

func (r *simpleCatalogAccounts) ListAllWithFilters(context.Context, string, string, string, string, int64, string) ([]Account, error) {
	return r.accounts, nil
}
func (r *simpleCatalogAccounts) GetByID(_ context.Context, id int64) (*Account, error) {
	for i := range r.accounts {
		if r.accounts[i].ID == id {
			return &r.accounts[i], nil
		}
	}
	return nil, nil
}

type simpleCatalogGroups struct {
	GroupRepository
	groups []Group
}

func (r *simpleCatalogGroups) ListActive(context.Context) ([]Group, error) { return r.groups, nil }
func (r *simpleCatalogGroups) GetByID(_ context.Context, id int64) (*Group, error) {
	for i := range r.groups {
		if r.groups[i].ID == id {
			return &r.groups[i], nil
		}
	}
	return nil, nil
}

func TestSimpleBillingCatalogUsesRealPriceEngineAndKeepsUnknownNull(t *testing.T) {
	cfg := simpleTestConfig()
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	account := simpleAPIAccount(8, "https://api.minimax.cn/v1")
	account.GroupIDs = []int64{7}
	account.Credentials["model_mapping"] = map[string]any{"M3": "MiniMax-M3", "H3": "MiniMax-H3", "NotConfigured": "missing-model"}
	book := &memoryCostBook{raw: string(raw)}
	billing := ProvideBillingService(&config.Config{}, nil, book)
	s := NewBillingManagementService(book, &simpleCatalogAccounts{accounts: []Account{*account}}, &simpleCatalogGroups{groups: []Group{{ID: 7, Status: StatusActive, RateMultiplier: 5}}}, nil, nil, billing, nil)
	entries, err := s.Catalog(context.Background())
	require.NoError(t, err)
	require.Len(t, entries, 4)
	for _, entry := range entries {
		if entry.Model == "NotConfigured" {
			require.Equal(t, "unknown", entry.Status)
			require.Empty(t, entry.BasePrices)
			continue
		}
		require.Equal(t, "ready", entry.Status)
		require.Equal(t, 1.4, entry.Multiplier)
		if entry.Model == "H3" && entry.Resolution == "2K" {
			require.Len(t, entry.BasePrices, 1)
			require.Equal(t, .8, entry.BasePrices[0].AmountCNY)
		}
	}
}

func TestSimpleBillingStrictReferenceDoesNotFallThroughToFamilyGuess(t *testing.T) {
	pricing := &PricingService{pricingData: map[string]*LiteLLMModelPricing{"gpt-5": {InputCostPerToken: 99, OutputCostPerToken: 99}}}
	s := ProvideBillingService(&config.Config{}, pricing, &memoryCostBook{})
	q, err := s.PrepareSimpleBilling(context.Background(), SimpleBillingInput{Account: simpleAPIAccount(8, "https://api.openai.com"), Group: &Group{ID: 7, RateMultiplier: 1}, Model: "gpt-5.4", Modality: "chat"})
	require.NoError(t, err)
	require.InDelta(t, 2.5e-6, q.ReferencePricing.InputPricePerToken, 1e-15)
	_, err = s.PrepareSimpleBilling(context.Background(), SimpleBillingInput{Account: simpleAPIAccount(8, "https://api.openai.com"), Group: &Group{ID: 7, RateMultiplier: 1}, Model: "gpt-9-unknown", Modality: "chat"})
	require.Error(t, err)
}

func TestSimpleBillingChineseAccountSubscriptionUsesCNReference(t *testing.T) {
	s := simpleTestBilling(t, simpleTestConfig())
	account := simpleAPIAccount(8, "https://open.bigmodel.cn/api/coding/paas/v4")
	q, err := s.PrepareSimpleBilling(context.Background(), SimpleBillingInput{Account: account, Group: &Group{ID: 7, RateMultiplier: 2}, Model: "glm-5.3", Modality: "chat"})
	require.NoError(t, err)
	require.Equal(t, "account", q.Route)
	require.Equal(t, "CNY", q.Currency)
	cost, err := s.CalculateSimpleBilling(q, BillingPreviewInput{Model: q.Model, InputTokens: 1000000, OutputTokens: 1000000, Requests: 1})
	require.NoError(t, err)
	require.InDelta(t, 72, cost.ActualCost*6.2, 1e-10)
}

func TestSimpleBillingPreservesConfirmedCacheWriteTTL(t *testing.T) {
	s := simpleTestBilling(t, simpleTestConfig())
	q := &SimpleBillingSnapshot{Multiplier: 1.4, ReferencePricing: &ModelPricing{CacheCreationPricePerToken: 3e-6, CacheCreation5mPrice: 3e-6, CacheCreation1hPrice: 9e-6, SupportsCacheBreakdown: true}}
	cost, err := s.CalculateSimpleBilling(q, BillingPreviewInput{CacheWriteTokens: 10, CacheWrite5mTokens: 2, CacheWrite1hTokens: 8, Requests: 1})
	require.NoError(t, err)
	require.InDelta(t, 2*3e-6+8*9e-6, cost.CacheCreationCost, 1e-15)
	require.InDelta(t, (2*3e-6+8*9e-6)*1.4, cost.ActualCost, 1e-15)
}

func TestSimpleBillingSeedreamProBillsMixedActualPixelsAtFrozenPrices(t *testing.T) {
	s := simpleTestBilling(t, simpleTestConfig())
	q, err := s.PrepareSimpleBilling(context.Background(), SimpleBillingInput{Account: simpleAPIAccount(9, "https://ark.cn-beijing.volces.com/api/v3"), Group: &Group{ID: 7, RateMultiplier: 8}, Model: "doubao-seedream-5-0-pro-260628", Modality: "image", Resolution: "2K", Scene: "single_image", InputImageCount: 3})
	require.NoError(t, err)
	cost, err := s.CalculateSimpleBilling(q, BillingPreviewInput{Model: q.Model, ImageCount: 2, ImageOutputSizes: []string{"2048x1024", "2048x2048"}, Requests: 1})
	require.NoError(t, err)
	require.InDelta(t, (.3+.6+.04)*1.4, cost.ActualCost*6.2, 1e-12)
}

func TestSimpleBillingTokenGapOverlapAndPartialRangesAreRejectedAtPreparation(t *testing.T) {
	for _, kind := range []string{"partial", "gap", "overlap"} {
		t.Run(kind, func(t *testing.T) {
			cfg := simpleTestConfig()
			row := CostPriceRow{ID: "low", Models: []string{"custom-model"}, Modality: "chat", BillingUnit: "per_million_tokens", InputPrice: costNumber(2), OutputPrice: costNumber(8), CacheReadPrice: costNumber(0), CacheWritePrice: costNumber(0), MaxTokens: costTokenBound(512000)}
			rows := []CostPriceRow{row}
			if kind != "partial" {
				high := row
				high.ID = "high"
				high.MaxTokens = nil
				high.MinTokens = 512001
				if kind == "overlap" {
					high.MinTokens = 511999
				}
				rows = append(rows, high)
			}
			cfg.CostProfiles = []CostProfile{{ID: "supplier", Name: "Supplier", Enabled: true, AccountIDs: []int64{8}, PurchaseMode: "payg", Currency: "CNY", DiscountMultiplier: 1, Prices: rows}}
			s := simpleTestBilling(t, cfg)
			_, err := s.PrepareSimpleBilling(context.Background(), SimpleBillingInput{Account: simpleAPIAccount(8, "https://reseller.example/v1"), Group: &Group{ID: 7, RateMultiplier: 1}, Model: "custom-model", Modality: "chat"})
			require.Error(t, err)
		})
	}
}

func TestSimpleBillingKeepsAccountBudgetRateWithoutChangingSaleMultiplier(t *testing.T) {
	logs := &openAIRecordUsageLogRepoStub{inserted: true}
	bills := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, bills, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	svc.billingService = simpleTestBilling(t, simpleTestConfig())
	group := &Group{ID: 7, RateMultiplier: 2}
	account := simpleAPIAccount(8, "https://open.bigmodel.cn/api/coding/paas/v4")
	account.RateMultiplier = costNumber(.05)
	q, err := svc.PrepareSimpleBilling(context.Background(), SimpleBillingInput{Account: account, Group: group, Model: "glm-5.3", Modality: "chat"})
	require.NoError(t, err)
	err = svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{APIKey: &APIKey{ID: 22, UserID: 11, GroupID: &group.ID, Group: group}, User: &User{ID: 11}, Account: account, Result: &OpenAIForwardResult{RequestID: "account-budget", Model: "glm-5.3", SimpleBilling: q, Usage: OpenAIUsage{InputTokens: 3000, OutputTokens: 1000}}})
	require.NoError(t, err)
	require.Equal(t, .05, *logs.lastLog.AccountRateMultiplier)
	require.Equal(t, 2.0, logs.lastLog.RateMultiplier)
	require.InDelta(t, (8.0*3000+28.0*1000)/1000000*2/6.2, bills.lastCmd.BalanceCost, 1e-12)
}

func TestSimpleBillingShadowWithoutProcurementMetadataDoesNotGuessOfficial(t *testing.T) {
	s := simpleTestBilling(t, simpleTestConfig())
	account := simpleAPIAccount(8, "")
	parent := int64(3)
	account.ParentAccountID = &parent
	_, err := s.PrepareSimpleBilling(context.Background(), SimpleBillingInput{Account: account, Group: &Group{ID: 7, RateMultiplier: 1}, Model: "gpt-5.4", Modality: "chat"})
	require.Error(t, err)
}

func TestSimpleBillingAccountManualReferenceDoesNotRequireMonthlyCostOrApplyDiscount(t *testing.T) {
	cfg := simpleTestConfig()
	cfg.CostProfiles = []CostProfile{{ID: "reference", Name: "Account reference", Enabled: true, AccountIDs: []int64{8}, PurchaseMode: "subscription", Currency: "CNY", DiscountMultiplier: .1, Prices: []CostPriceRow{{ID: "text", Models: []string{"private-account-model"}, Modality: "chat", BillingUnit: "per_million_tokens", InputPrice: costNumber(10), OutputPrice: costNumber(20), CacheReadPrice: costNumber(0), CacheWritePrice: costNumber(0)}}}}
	require.NoError(t, ValidateBillingManagementConfig(&cfg))
	s := simpleTestBilling(t, cfg)
	q, err := s.PrepareSimpleBilling(context.Background(), SimpleBillingInput{Account: simpleAPIAccount(8, "https://reseller.example/v1"), Group: &Group{ID: 7, RateMultiplier: 2}, Model: "private-account-model", Modality: "chat"})
	require.NoError(t, err)
	require.Equal(t, "account", q.Route)
	require.Equal(t, "reference", q.Source)
	require.Equal(t, 1.0, q.DiscountMultiplier)
	cost, err := s.CalculateSimpleBilling(q, BillingPreviewInput{Model: q.Model, InputTokens: 1000000, Requests: 1})
	require.NoError(t, err)
	require.InDelta(t, 20, cost.ActualCost*6.2, 1e-12)
	cfg.CostProfiles[0].PeriodCost = costNumber(-1)
	require.Error(t, ValidateBillingManagementConfig(&cfg))
}

func TestSimpleBillingExplicitFreeImageInputIsNotChargedAtTextPrice(t *testing.T) {
	cfg := simpleTestConfig()
	cfg.CostProfiles = []CostProfile{{ID: "free-image-input", Name: "Free input", Enabled: true, AccountIDs: []int64{8}, PurchaseMode: "payg", Currency: "CNY", DiscountMultiplier: 1, Prices: []CostPriceRow{{ID: "image", Models: []string{"custom-image"}, Modality: "image", BillingUnit: "per_million_tokens", InputPrice: costNumber(10), OutputPrice: costNumber(20), ImageInputPrice: costNumber(0), ImageOutputPrice: costNumber(20), CacheReadPrice: costNumber(0), CacheWritePrice: costNumber(0)}}}}
	s := simpleTestBilling(t, cfg)
	q, err := s.PrepareSimpleBilling(context.Background(), SimpleBillingInput{Account: simpleAPIAccount(8, "https://reseller.example/v1"), Group: &Group{ID: 7, RateMultiplier: 2}, Model: "custom-image", Modality: "image", HasImageInput: true})
	require.NoError(t, err)
	cost, err := s.CalculateSimpleBilling(q, BillingPreviewInput{Model: q.Model, InputTokens: 100, ImageInputTokens: 100, Requests: 1})
	require.NoError(t, err)
	require.Zero(t, cost.TotalCost)
	require.Zero(t, cost.ActualCost)
	cfg.CostProfiles[0].Prices[0].ImageInputPrice = nil
	s = simpleTestBilling(t, cfg)
	_, err = s.PrepareSimpleBilling(context.Background(), SimpleBillingInput{Account: simpleAPIAccount(8, "https://reseller.example/v1"), Group: &Group{ID: 7, RateMultiplier: 2}, Model: "custom-image", Modality: "image", HasImageInput: true})
	require.Error(t, err)
}

func TestSimpleBillingCloudAndUpstreamProcurementAreAPIRoutes(t *testing.T) {
	group := &Group{ID: 7, RateMultiplier: 2}
	for _, kind := range []string{AccountTypeBedrock, AccountTypeServiceAccount, AccountTypeUpstream} {
		t.Run(kind, func(t *testing.T) {
			account := &Account{ID: 8, Type: kind, Platform: PlatformAnthropic, Credentials: map[string]any{}}
			require.Equal(t, "api", procurementRoute(account))
			_, _, official := simpleAccountProvider(account)
			require.False(t, official)
			s := simpleTestBilling(t, simpleTestConfig())
			_, err := s.PrepareSimpleBilling(context.Background(), SimpleBillingInput{Account: account, Group: group, Model: "claude-opus-4-6", Modality: "chat"})
			require.Error(t, err)
			cfg := simpleTestConfig()
			cfg.CostProfiles = []CostProfile{{ID: "contract", Name: "Confirmed cloud contract", Enabled: true, AccountIDs: []int64{8}, PurchaseMode: "payg", Currency: "USD", DiscountMultiplier: 1, Prices: []CostPriceRow{{ID: "cloud", Models: []string{"claude-opus-4-6"}, Modality: "chat", BillingUnit: "per_million_tokens", InputPrice: costNumber(5), OutputPrice: costNumber(25), CacheReadPrice: costNumber(.5), CacheWritePrice: costNumber(6.25)}}}}
			s = simpleTestBilling(t, cfg)
			q, err := s.PrepareSimpleBilling(context.Background(), SimpleBillingInput{Account: account, Group: group, Model: "claude-opus-4-6", Modality: "chat"})
			require.NoError(t, err)
			require.Equal(t, "api", q.Route)
			require.Equal(t, 1.4, q.Multiplier)
		})
	}
}

func TestSimpleBillingAlphaSearchFreezesPerRequestReferenceAndCountsActualCalls(t *testing.T) {
	s := simpleTestBilling(t, simpleTestConfig())
	group := &Group{ID: 7, RateMultiplier: 2, WebSearchPricePerCall: costNumber(.008), PeakRateEnabled: true, PeakRateMultiplier: 9}
	q, err := s.PrepareSimpleBillingSearch(context.Background(), SimpleBillingInput{Account: &Account{ID: 8, Type: AccountTypeOAuth, Platform: PlatformOpenAI}, Group: group, Model: "private-search-model"})
	require.NoError(t, err)
	require.Equal(t, "per_request", q.BillingUnit)
	require.Equal(t, "reference", q.Source)
	svc := &OpenAIGatewayService{billingService: s}
	cost, err := svc.calculateOpenAIRecordUsageCost(context.Background(), &OpenAIForwardResult{Model: q.Model, SimpleBilling: q, WebSearchCalls: 3}, &APIKey{Group: group}, []string{q.Model}, 9, 9, 9, 9, UsageTokens{}, "", false)
	require.NoError(t, err)
	require.InDelta(t, .008*3*2, cost.ActualCost, 1e-12)
	require.Equal(t, string(BillingModePerRequest), cost.BillingMode)
	group.WebSearchPricePerCall = costNumber(100)
	frozen, err := s.CalculateSimpleBilling(q, BillingPreviewInput{Model: q.Model, Requests: 3})
	require.NoError(t, err)
	require.Equal(t, cost.ActualCost, frozen.ActualCost)
}

func TestSimpleBillingAlphaSearchAPIRequiresPerRequestContractBeforeUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	group := &Group{ID: 7, RateMultiplier: 2}
	account := simpleAPIAccount(8, "https://api.openai.com")
	svc := &OpenAIGatewayService{cfg: &config.Config{}, billingService: simpleTestBilling(t, simpleTestConfig())}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/alpha/search", strings.NewReader(`{"model":"gpt-5.4"}`))
	c.Set("api_key", &APIKey{Group: group})
	// No access token or transport: reaching a paid call would fail differently.
	result, err := svc.ForwardAlphaSearch(context.Background(), c, account, []byte(`{"model":"gpt-5.4"}`))
	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)

	cfg := simpleTestConfig()
	cfg.CostProfiles = []CostProfile{{ID: "search-contract", Name: "Search API", Enabled: true, AccountIDs: []int64{8}, PurchaseMode: "payg", Currency: "CNY", DiscountMultiplier: .8, Prices: []CostPriceRow{{ID: "search", Models: []string{"gpt-5.4"}, Modality: "chat", Operation: "alpha_search", BillingUnit: "per_request", UnitPrice: costNumber(.1)}}}}
	account.Credentials["api_key"] = "test-only-placeholder"
	svc.billingService = simpleTestBilling(t, cfg)
	svc.httpUpstream = &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"output":"result"}`))}}
	recorder = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/alpha/search", strings.NewReader(`{"model":"gpt-5.4"}`))
	c.Set("api_key", &APIKey{Group: group})
	result, err = svc.ForwardAlphaSearch(context.Background(), c, account, []byte(`{"model":"gpt-5.4"}`))
	require.NoError(t, err)
	require.NotNil(t, result.SimpleBilling)
	require.Equal(t, 1, result.WebSearchCalls)
	cost, err := svc.calculateOpenAIRecordUsageCost(context.Background(), result, &APIKey{Group: group}, []string{result.Model}, 2, 2, 2, 2, UsageTokens{}, "", false)
	require.NoError(t, err)
	require.InDelta(t, .1*.8*1.4, cost.ActualCost*6.2, 1e-12)
	// The standalone search contract cannot be mistaken for normal chat cost.
	_, err = svc.PrepareSimpleBilling(context.Background(), SimpleBillingInput{Account: account, Group: group, Model: "gpt-5.4", Modality: "chat"})
	require.Error(t, err)
}

func TestSimpleBillingUnknownChineseSubscriptionAndVisionDoNotUseGlobalFallback(t *testing.T) {
	s := simpleTestBilling(t, simpleTestConfig())
	group := &Group{ID: 7, RateMultiplier: 2}
	account := simpleAPIAccount(8, "https://open.bigmodel.cn/api/coding/paas/v4")
	_, err := s.PrepareSimpleBilling(context.Background(), SimpleBillingInput{Account: account, Group: group, Model: "glm-5.3-future", Modality: "chat"})
	require.Error(t, err)
	_, err = s.PrepareSimpleBilling(context.Background(), SimpleBillingInput{Account: account, Group: group, Model: "glm-5.3", Modality: "chat", HasImageInput: true})
	require.Error(t, err)
}

func TestSimpleBillingPreviewAndCatalogKeepSearchSeparateFromText(t *testing.T) {
	cfg := simpleTestConfig()
	cfg.CostProfiles = []CostProfile{{ID: "supplier", Name: "Supplier", Enabled: true, AccountIDs: []int64{8}, PurchaseMode: "payg", Currency: "CNY", DiscountMultiplier: 1, Prices: []CostPriceRow{
		{ID: "search", Models: []string{"gpt-5.4"}, Modality: "chat", Operation: "alpha_search", BillingUnit: "per_request", UnitPrice: costNumber(.1)},
		{ID: "text", Models: []string{"gpt-5.4"}, Modality: "chat", BillingUnit: "per_million_tokens", InputPrice: costNumber(.2), OutputPrice: costNumber(.8), CacheReadPrice: costNumber(0), CacheWritePrice: costNumber(0)},
	}}}
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	book := &memoryCostBook{raw: string(raw)}
	account := simpleAPIAccount(8, "https://reseller.example/v1")
	account.GroupIDs = []int64{7}
	account.Credentials["model_mapping"] = map[string]any{"MyModel": "gpt-5.4"}
	billing := ProvideBillingService(&config.Config{}, nil, book)
	s := NewBillingManagementService(book, &simpleCatalogAccounts{accounts: []Account{*account}}, &simpleCatalogGroups{groups: []Group{{ID: 7, Status: StatusActive, RateMultiplier: 2}}}, nil, nil, billing, nil)
	preview, err := s.Preview(context.Background(), BillingPreviewInput{AccountID: 8, GroupID: 7, Model: "MyModel", Modality: "chat", Operation: "alpha_search", Requests: 5})
	require.NoError(t, err)
	require.Equal(t, "estimated", preview.Sale.Status)
	require.InDelta(t, .7, *preview.Sale.AmountCNY, 1e-12)
	preview, err = s.Preview(context.Background(), BillingPreviewInput{AccountID: 8, GroupID: 7, Model: "MyModel", Modality: "chat", InputTokens: 1000000})
	require.NoError(t, err)
	require.InDelta(t, .28, *preview.Sale.AmountCNY, 1e-12)
	entries, err := s.Catalog(context.Background())
	require.NoError(t, err)
	require.Len(t, entries, 2)
	for _, entry := range entries {
		require.Equal(t, "ready", entry.Status)
		if entry.Operation == "alpha_search" {
			require.Equal(t, "per_request", entry.BasePrices[0].Unit)
			require.Equal(t, .1, entry.BasePrices[0].AmountCNY)
		} else {
			require.Equal(t, "per_million_tokens", entry.BasePrices[0].Unit)
			require.Equal(t, .2, entry.BasePrices[0].AmountCNY)
		}
	}
}

func TestSimpleBillingBatchFreezesSoleAPIRateAndRejectsUnquotedReferenceSurcharge(t *testing.T) {
	for _, extraRefs := range []bool{false, true} {
		t.Run(map[bool]string{false: "frozen fixed price", true: "variable reference surcharge"}[extraRefs], func(t *testing.T) {
			svc, repo, _, provider, _ := newTestBatchImagePublicService(true)
			req := validBatchImageSubmitRequest()
			cfg := simpleTestConfig()
			row := CostPriceRow{ID: "batch", Models: []string{req.Model}, Modality: "image", BillingUnit: "per_image", UnitPrice: costNumber(.2)}
			if extraRefs {
				row.InputImageUnitPrice = costNumber(.02)
				req.Items[0].ReferenceImages = []BatchImageReferenceInput{{MimeType: "image/png", Data: []byte("one")}, {MimeType: "image/png", Data: []byte("two")}}
			}
			cfg.CostProfiles = []CostProfile{{ID: "batch-contract", Name: "Batch contract", Enabled: true, AccountIDs: []int64{202}, PurchaseMode: "payg", Currency: "CNY", DiscountMultiplier: 1, Prices: []CostPriceRow{row}}}
			svc.SimpleBilling = simpleTestBilling(t, cfg)
			groupID := int64(7)
			svc.GroupRepo = &publicBatchImageGroupRepo{groups: map[int64]*Group{7: {ID: 7, Platform: PlatformGemini, RateMultiplier: 10, AllowImageGeneration: true, AllowBatchImageGeneration: true, ImageRateIndependent: true, ImageRateMultiplier: 9, BatchImageDiscountMultiplier: .5, BatchImageHoldMultiplier: .6}}}
			svc.UserGroupRateRepo = &publicBatchImageUserGroupRateRepo{rates: map[int64]*float64{7: costNumber(.5)}}
			svc.AccountRepo.(*publicBatchImageAccountRepo).accounts[1].RateMultiplier = costNumber(10)
			got, err := svc.Submit(context.Background(), BatchImageOwner{UserID: 11, APIKeyID: 22, GroupID: &groupID}, req, "")
			if extraRefs {
				require.Error(t, err)
				require.Empty(t, provider.submits)
				require.Empty(t, repo.jobs)
				return
			}
			require.NoError(t, err)
			job := repo.jobs[got.ID]
			require.Equal(t, 1.4, job.GroupRateMultiplier)
			require.Equal(t, 1.0, job.AccountRateMultiplier)
			require.Equal(t, 1.0, job.BatchDiscountMultiplier)
			require.InDelta(t, .2*1.4/6.2, job.BillableUnitPrice, 1e-12)
			require.InDelta(t, .2*1.4/6.2*float64(len(req.Items)), *job.HoldAmount, 1e-12)
		})
	}
}

func TestSimpleBillingVerifiedChineseFlashAndKimiVisionReferences(t *testing.T) {
	s := simpleTestBilling(t, simpleTestConfig())
	for _, tc := range []struct {
		base, model          string
		input, output, cache float64
	}{
		{"https://open.bigmodel.cn/api/paas/v4/", "glm-5.3-flash", .8, 2.8, .23},
		{"https://open.bigmodel.cn/api/paas/v4/", "glm-5.3-flashx", 2, 7, .57},
		{"https://api.moonshot.cn/v1", "kimi-k2.6", 6.5, 27, 1.1},
		{"https://api.moonshot.cn/v1", "kimi-k2.7-code", 6.5, 27, 1.3},
		{"https://api.moonshot.cn/v1", "kimi-k2.7-code-highspeed", 13, 54, 2.6},
	} {
		t.Run(tc.model, func(t *testing.T) {
			q, err := s.PrepareSimpleBilling(context.Background(), SimpleBillingInput{Account: simpleAPIAccount(8, tc.base), Group: &Group{ID: 7, RateMultiplier: 10}, Model: tc.model, Modality: "chat", HasImageInput: true})
			require.NoError(t, err)
			require.Equal(t, "official_reference", q.Source)
			require.Equal(t, tc.input, *q.Rows[0].ImageInputPrice)
			cost, err := s.CalculateSimpleBilling(q, BillingPreviewInput{Model: q.Model, InputTokens: 1000, ImageInputTokens: 500, OutputTokens: 100, CacheReadTokens: 100, Requests: 1})
			require.NoError(t, err)
			require.InDelta(t, (1000*tc.input+100*tc.output+100*tc.cache)/1000000*1.4, cost.ActualCost*6.2, 1e-12)
			if q.Provider == "moonshot" {
				_, err = s.CalculateSimpleBilling(q, BillingPreviewInput{Model: q.Model, CacheWriteTokens: 1, Requests: 1})
				require.Error(t, err, "unexpected independent write bucket must not be silently priced free")
			}
		})
	}
	_, err := s.PrepareSimpleBilling(context.Background(), SimpleBillingInput{Account: simpleAPIAccount(8, "https://api.moonshot.cn/v1"), Group: &Group{ID: 7, RateMultiplier: 2}, Model: "kimi-k3", Modality: "chat"})
	require.Error(t, err)
}
