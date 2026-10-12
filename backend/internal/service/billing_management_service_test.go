package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func costNumber(v float64) *float64 { return &v }
func costTokenBound(v int) *int     { return &v }

func exampleCostBook() BillingManagementConfig {
	return BillingManagementConfig{Version: 3, USDToCNYRate: 7, CostProfiles: []CostProfile{{
		ID: "official", Name: "Official API", Provider: "MiniMax", Enabled: true, AccountIDs: []int64{8}, PurchaseMode: "payg", Currency: "CNY", DiscountMultiplier: 1,
		Prices: []CostPriceRow{{ID: "h3", Models: []string{"MiniMax-H3"}, Modality: "video", BillingUnit: "per_second", Resolution: "768P", UnitPrice: costNumber(.5)}, {ID: "m3", Models: []string{"MiniMax-M3"}, Modality: "chat", BillingUnit: "per_million_tokens", InputPrice: costNumber(2), OutputPrice: costNumber(8), CacheReadPrice: costNumber(.4)}},
	}}}
}

func TestPurchaseCostEstimateCurrencyUnitsAndUnknown(t *testing.T) {
	now := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	t.Run("native CNY video is two yuan not a wallet dollar", func(t *testing.T) {
		book := exampleCostBook()
		cost := estimatePurchaseCost(&book, BillingPreviewInput{AccountID: 8, Model: "MiniMax-H3", Modality: "video", Resolution: "768P", DurationSeconds: 4, Requests: 1}, now)
		require.Equal(t, "estimated", cost.Status)
		require.Equal(t, "CNY", cost.Currency)
		require.InDelta(t, 2, *cost.Amount, 1e-12)
		require.Equal(t, *cost.Amount, *cost.AmountCNY)
		require.Nil(t, cost.ExchangeRate)
	})
	t.Run("text prices are per million and caches are separate", func(t *testing.T) {
		book := exampleCostBook()
		cost := estimatePurchaseCost(&book, BillingPreviewInput{AccountID: 8, Model: "MiniMax-M3", Modality: "chat", InputTokens: 500, OutputTokens: 100, CacheReadTokens: 1000}, now)
		require.Equal(t, "estimated", cost.Status)
		require.InDelta(t, .0022, *cost.Amount, 1e-12)
	})
	t.Run("unknown cache write cannot silently cost zero", func(t *testing.T) {
		book := exampleCostBook()
		cost := estimatePurchaseCost(&book, BillingPreviewInput{AccountID: 8, Model: "MiniMax-M3", Modality: "chat", CacheWriteTokens: 1}, now)
		require.Equal(t, "unknown", cost.Status)
		require.Nil(t, cost.Amount)
		require.Nil(t, cost.AmountCNY)
	})
	t.Run("explicit free differs from missing price", func(t *testing.T) {
		book := exampleCostBook()
		book.CostProfiles[0].Prices[1].CacheWritePrice = costNumber(0)
		cost := estimatePurchaseCost(&book, BillingPreviewInput{AccountID: 8, Model: "MiniMax-M3", Modality: "chat", CacheWriteTokens: 10}, now)
		require.Equal(t, "estimated", cost.Status)
		require.Equal(t, 0.0, *cost.Amount)
	})
	t.Run("USD conversion independent of wallet recharge", func(t *testing.T) {
		book := exampleCostBook()
		book.CostProfiles[0].Currency = "USD"
		cost := estimatePurchaseCost(&book, BillingPreviewInput{AccountID: 8, Model: "MiniMax-H3", Modality: "video", Resolution: "768P", DurationSeconds: 4, Requests: 1}, now)
		require.Equal(t, 2.0, *cost.Amount)
		require.Equal(t, 14.0, *cost.AmountCNY)
		require.Equal(t, 7.0, *cost.ExchangeRate)
		book.USDToCNYRate = 0
		cost = estimatePurchaseCost(&book, BillingPreviewInput{AccountID: 8, Model: "MiniMax-H3", Modality: "video", Resolution: "768P", DurationSeconds: 4, Requests: 1}, now)
		require.Equal(t, "estimated", cost.Status)
		require.Nil(t, cost.AmountCNY)
	})
	t.Run("future exchange rule and missing usage cannot invent zero costs", func(t *testing.T) {
		book := exampleCostBook()
		book.CostProfiles[0].Currency = "USD"
		future := now.Add(time.Hour)
		book.ExchangeRateEffectiveAt = &future
		cost := estimatePurchaseCost(&book, BillingPreviewInput{AccountID: 8, Model: "MiniMax-H3", Modality: "video", Resolution: "768P", DurationSeconds: 4, Requests: 1}, now)
		require.Equal(t, "estimated", cost.Status)
		require.Equal(t, 2.0, *cost.Amount)
		require.Nil(t, cost.AmountCNY)
		cost = estimatePurchaseCost(&book, BillingPreviewInput{AccountID: 8, Model: "MiniMax-M3", Modality: "chat"}, now)
		require.Equal(t, "unknown", cost.Status)
		require.Nil(t, cost.Amount)
	})
	t.Run("unbound templates do not match arbitrary accounts", func(t *testing.T) {
		book := exampleCostBook()
		book.CostProfiles[0].AccountIDs = nil
		input := BillingPreviewInput{AccountID: 8, Model: "MiniMax-H3", Modality: "video", Resolution: "768P", DurationSeconds: 4, Requests: 1}
		require.Equal(t, "unknown", estimatePurchaseCost(&book, input, now).Status)
		input.ProfileID = "official"
		require.Equal(t, "estimated", estimatePurchaseCost(&book, input, now).Status)
	})
	t.Run("expired disabled and future profiles are unknown", func(t *testing.T) {
		for _, state := range []string{"expired", "disabled", "future"} {
			book := exampleCostBook()
			past, future := now.Add(-time.Hour), now.Add(time.Hour)
			switch state {
			case "expired":
				book.CostProfiles[0].ValidUntil = &past
			case "future":
				book.CostProfiles[0].ValidFrom = &future
			case "disabled":
				book.CostProfiles[0].Enabled = false
			}
			require.Equal(t, "unknown", estimatePurchaseCost(&book, BillingPreviewInput{AccountID: 8, Model: "MiniMax-H3", Modality: "video"}, now).Status, state)
		}
	})
}

func TestPurchaseCostEstimateModesAndConflicts(t *testing.T) {
	now := time.Now()
	input := BillingPreviewInput{AccountID: 8, Model: "MiniMax-H3", Modality: "video", Resolution: "768P", DurationSeconds: 4, Requests: 1}
	book := exampleCostBook()
	book.CostProfiles[0].PurchaseMode = "subscription"
	book.CostProfiles[0].PeriodCost = costNumber(100)
	cost := estimatePurchaseCost(&book, input, now)
	require.Equal(t, "period_only", cost.Status)
	require.Equal(t, 100.0, *cost.PeriodCost)
	require.Nil(t, cost.Amount)
	book.CostProfiles[0].PurchaseMode = "credit_pack"
	book.CostProfiles[0].CreditAmount = costNumber(200)
	cost = estimatePurchaseCost(&book, input, now)
	require.Equal(t, 1.0, *cost.Amount)
	require.Contains(t, cost.Reason, "分摊估算")
	book.CostProfiles[0].PurchaseMode = "hybrid"
	cost = estimatePurchaseCost(&book, input, now)
	require.Equal(t, 2.0, *cost.Amount)
	require.Equal(t, 100.0, *cost.PeriodCost)
	book.CostProfiles[0].Prices = append(book.CostProfiles[0].Prices, book.CostProfiles[0].Prices[0])
	require.Equal(t, "unknown", estimatePurchaseCost(&book, input, now).Status)
	book = exampleCostBook()
	duplicate := book.CostProfiles[0]
	duplicate.ID = "duplicate"
	book.CostProfiles = append(book.CostProfiles, duplicate)
	require.Equal(t, "unknown", estimatePurchaseCost(&book, input, now).Status)
	input.ProfileID = "official"
	require.Equal(t, "estimated", estimatePurchaseCost(&book, input, now).Status)
}

func TestPurchaseCostContextBoundaryPriorityAndImage(t *testing.T) {
	book := exampleCostBook()
	book.CostProfiles[0].Prices = []CostPriceRow{
		{ID: "low", Models: []string{"MiniMax-M3"}, Modality: "chat", BillingUnit: "per_million_tokens", MaxTokens: costTokenBound(512000), InputPrice: costNumber(2)},
		{ID: "high", Models: []string{"MiniMax-M3"}, Modality: "chat", BillingUnit: "per_million_tokens", MinTokens: 512000, InputPrice: costNumber(4)},
		{ID: "image", Models: []string{"seedream-*"}, Modality: "image", BillingUnit: "per_image", Resolution: "2K", UnitPrice: costNumber(.1)},
	}
	for _, tokens := range []int{512000, 512001} {
		cost := estimatePurchaseCost(&book, BillingPreviewInput{AccountID: 8, Model: "MiniMax-M3", Modality: "chat", InputTokens: tokens}, time.Now())
		require.Equal(t, "estimated", cost.Status)
		rate := 2.0
		if tokens > 512000 {
			rate = 4
		}
		require.InDelta(t, rate*float64(tokens)/1e6, *cost.Amount, 1e-12)
	}
	priority := estimatePurchaseCost(&book, BillingPreviewInput{AccountID: 8, Model: "MiniMax-M3", Modality: "chat", ServiceTier: "priority", InputTokens: 100}, time.Now())
	require.Equal(t, "unknown", priority.Status)
	image := estimatePurchaseCost(&book, BillingPreviewInput{AccountID: 8, Model: "seedream-4", Modality: "image", Resolution: "2K", ImageCount: 3}, time.Now())
	require.InDelta(t, .3, *image.Amount, 1e-12)
}

type memoryCostBook struct {
	mu            sync.Mutex
	raw           string
	forceConflict bool
}

func (r *memoryCostBook) Get(context.Context) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.raw, nil
}
func (r *memoryCostBook) CompareAndSwap(_ context.Context, expected, next string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.forceConflict || r.raw != expected {
		return false, nil
	}
	r.raw = next
	return true, nil
}

type costAccountRepo struct {
	AccountRepository
	found bool
}

func (r *costAccountRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	if !r.found {
		return nil, errors.New("missing")
	}
	return &Account{ID: id}, nil
}

type costGroupRepo struct {
	GroupRepository
	group *Group
}

func (r *costGroupRepo) GetByID(context.Context, int64) (*Group, error) { return r.group, nil }

func TestBillingManagementVersionAndReferenceValidation(t *testing.T) {
	repo := &memoryCostBook{}
	svc := NewBillingManagementService(repo, &costAccountRepo{found: true}, nil, nil, nil, nil, nil)
	cfg, err := svc.GetConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(0), cfg.Version)
	require.Empty(t, cfg.CostProfiles)
	first := exampleCostBook()
	first.Version = 0
	saved, err := svc.UpdateConfig(context.Background(), first)
	require.NoError(t, err)
	require.Equal(t, int64(1), saved.Version)
	_, err = svc.UpdateConfig(context.Background(), first)
	require.Error(t, err)
	first.Version = 1
	repo.forceConflict = true
	_, err = svc.UpdateConfig(context.Background(), first)
	require.Error(t, err)
	repo.forceConflict = false
	svc.accounts = &costAccountRepo{}
	_, err = svc.UpdateConfig(context.Background(), first)
	require.Error(t, err)
	var persisted BillingManagementConfig
	require.NoError(t, json.Unmarshal([]byte(repo.raw), &persisted))
	require.Equal(t, int64(1), persisted.Version)
}

func TestBillingManagementRejectsInvalidMoneyAndSpecifications(t *testing.T) {
	for name, mutate := range map[string]func(*BillingManagementConfig){
		"nan":                         func(c *BillingManagementConfig) { c.USDToCNYRate = math.NaN() },
		"negative":                    func(c *BillingManagementConfig) { c.CostProfiles[0].Prices[0].UnitPrice = costNumber(-1) },
		"missing subscription amount": func(c *BillingManagementConfig) { c.CostProfiles[0].PurchaseMode = "subscription" },
		"bad currency":                func(c *BillingManagementConfig) { c.CostProfiles[0].Currency = "EUR" },
		"bad wildcard":                func(c *BillingManagementConfig) { c.CostProfiles[0].Prices[0].Models = []string{"Mini*H3"} },
		"bad unit":                    func(c *BillingManagementConfig) { c.CostProfiles[0].Prices[0].BillingUnit = "per_image" },
		"duplicate account":           func(c *BillingManagementConfig) { c.CostProfiles[0].AccountIDs = []int64{8, 8} },
	} {
		t.Run(name, func(t *testing.T) {
			cfg := exampleCostBook()
			mutate(&cfg)
			require.Error(t, ValidateBillingManagementConfig(&cfg))
		})
	}
}

func TestBillingPreviewSaleUsesVideoIndependentMultiplierAndNoCashRevenue(t *testing.T) {
	book := exampleCostBook()
	raw, _ := json.Marshal(book)
	billing := NewBillingService(nil, nil)
	group := &Group{ID: 7, Status: StatusActive, RateMultiplier: 1.4, AllowImageGeneration: true, VideoRateIndependent: true, VideoRateMultiplier: 1}
	svc := NewBillingManagementService(&memoryCostBook{raw: string(raw)}, nil, &costGroupRepo{group: group}, nil, NewModelPricingResolver(nil, billing), billing, nil)
	result, err := svc.Preview(context.Background(), BillingPreviewInput{AccountID: 8, GroupID: 7, Model: "MiniMax-H3", Modality: "video", Resolution: "768P", DurationSeconds: 4})
	require.NoError(t, err)
	require.Equal(t, "estimated", result.Cost.Status)
	require.Equal(t, 2.0, *result.Cost.AmountCNY)
	require.Equal(t, "estimated", result.Sale.Status)
	require.InDelta(t, .32, *result.Sale.Amount, 1e-12)
	require.Equal(t, 1.0, *result.Sale.Multiplier)
	require.Nil(t, result.Comparison.IncomeCNY)
	require.Nil(t, result.Comparison.MarginCNY)
	group.VideoRateIndependent = false
	result, err = svc.Preview(context.Background(), BillingPreviewInput{GroupID: 7, Model: "MiniMax-H3", Modality: "video", Resolution: "768P", DurationSeconds: 4})
	require.NoError(t, err)
	require.InDelta(t, .448, *result.Sale.Amount, 1e-12)
	require.Equal(t, "unknown", result.Cost.Status)
	result, err = svc.Preview(context.Background(), BillingPreviewInput{GroupID: 7, Model: "MiniMax-H3-Max", Modality: "video", Resolution: "2K", DurationSeconds: 4})
	require.NoError(t, err)
	require.Equal(t, "unknown", result.Sale.Status)
	require.Nil(t, result.Sale.Amount)
}

func TestBillingPreviewImageTokenSubsetsNeverDoubleCountOrUseImageMultiplier(t *testing.T) {
	book := exampleCostBook()
	book.CostProfiles[0].Prices = []CostPriceRow{{ID: "image-token", Models: []string{"gpt-image-test"}, Modality: "image", BillingUnit: "per_million_tokens", InputPrice: costNumber(2), OutputPrice: costNumber(8), ImageInputPrice: costNumber(3), ImageOutputPrice: costNumber(10)}}
	raw, _ := json.Marshal(book)
	group := &Group{ID: 7, Status: StatusActive, Platform: PlatformOpenAI, RateMultiplier: 2, ImageRateIndependent: true, ImageRateMultiplier: 10}
	channels := &ChannelService{}
	pricing := ChannelModelPricing{Platform: PlatformOpenAI, Models: []string{"gpt-image-test"}, BillingMode: BillingModeToken, InputPrice: costNumber(.000002), OutputPrice: costNumber(.000008), ImageInputPrice: costNumber(.000003), ImageOutputPrice: costNumber(.000010)}
	channels.cache.Store(populateChannelCache([]Channel{{ID: 1, Status: StatusActive, GroupIDs: []int64{7}, ModelPricing: []ChannelModelPricing{pricing}}}, map[int64]string{7: PlatformOpenAI}))
	billing := NewBillingService(nil, nil)
	svc := NewBillingManagementService(&memoryCostBook{raw: string(raw)}, nil, &costGroupRepo{group: group}, channels, NewModelPricingResolver(channels, billing), billing, nil)
	input := BillingPreviewInput{AccountID: 8, GroupID: 7, Model: "gpt-image-test", Modality: "image", InputTokens: 100, ImageInputTokens: 40, OutputTokens: 100, ImageOutputTokens: 80, ImageCount: 1}
	out, err := svc.Preview(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, "estimated", out.Cost.Status)
	require.InDelta(t, .0012, *out.Cost.Amount, 1e-12)
	require.InDelta(t, .0024, *out.Sale.Amount, 1e-12)
	require.Equal(t, 2.0, *out.Sale.Multiplier)
	input.ImageInputTokens = 101
	_, err = svc.Preview(context.Background(), input)
	require.Error(t, err)
	input.ImageInputTokens = 40
	book.CostProfiles[0].Prices[0].ImageOutputPrice = nil
	out.Cost = estimatePurchaseCost(&book, input, time.Now())
	require.Equal(t, "unknown", out.Cost.Status)
	require.Contains(t, out.Cost.Reason, "图片输出")
	require.Nil(t, out.Cost.Amount)
}
