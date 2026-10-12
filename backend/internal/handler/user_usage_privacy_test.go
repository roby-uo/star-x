package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/minimax"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func assertNoFinanceFields(t *testing.T, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	for _, field := range []string{"cost", "actual_cost", "total_cost", "total_actual_cost", "today_cost", "today_actual_cost", "account_cost", "input_price", "pricing", "prices", "multiplier", "rate_multiplier", "unit_price", "currency", "estimated_cost", "hold_amount"} {
		require.NotContains(t, string(raw), `"`+field+`"`, "financial field must be omitted, not zeroed")
	}
}

func TestUserUsageResponseContractsExcludeFinancialData(t *testing.T) {
	stats := &usagestats.UsageStats{TotalTokens: 123, TotalCost: 2, TotalActualCost: 3,
		Endpoints: []usagestats.EndpointStat{{Endpoint: "/v1/chat/completions", Requests: 1, TotalTokens: 123, Cost: 2, ActualCost: 3}},
	}
	public := userUsageStatsFromUsageStats(stats)
	require.Equal(t, int64(123), public.TotalTokens)
	assertNoFinanceFields(t, public)
	// Redaction must not mutate shared service results used by administrator routes.
	require.Equal(t, float64(2), stats.TotalCost)
	assertNoFinanceFields(t, userTrendFromUsageTrend([]usagestats.TrendDataPoint{{TotalTokens: 123, Cost: 2, ActualCost: 3}}))
	assertNoFinanceFields(t, userDashboardFromUsageDashboard(&usagestats.UserDashboardStats{
		TotalTokens: 123, TotalCost: 2, TotalActualCost: 3, TodayCost: 1,
		ByPlatform: []usagestats.PlatformDashboardStats{{Platform: "openai", TotalTokens: 123, TotalActualCost: 3, TodayActualCost: 1}},
	}))
	assertNoFinanceFields(t, userDailyUsageFromUsage([]usagestats.APIKeyDailyUsagePoint{{TotalTokens: 123, Cost: 2, ActualCost: 3}}))
	assertNoFinanceFields(t, userAPIKeyStatsFromUsageStats(map[int64]*usagestats.BatchAPIKeyUsageStats{
		5: {APIKeyID: 5, TodayTokens: 12, TotalTokens: 123, TotalActualCost: 3, TodayActualCost: 1},
	}))
}

func TestUserVideoResponsesKeepVersionAndCapabilitiesWithoutPrices(t *testing.T) {
	quote := &service.VideoQuote{Version: "opaque-version", Currency: "USD", Total: .8, Multiplier: 1.4, UnitPrice: .08}
	public := userQuoteFromVideoQuote(quote)
	require.Equal(t, quote.Version, public.Version)
	assertNoFinanceFields(t, public)
	policy := &service.ModelServicePolicy{Model: "MiniMax-H3", Kind: "video", Resolutions: []string{"768P"}, MinDuration: 4, MaxDuration: 15, Prices: map[string]float64{"768P": .08}}
	publicPolicy := userPolicyFromModelPolicy(policy)
	require.Equal(t, policy.Resolutions, publicPolicy.Resolutions)
	require.Equal(t, policy.MinDuration, publicPolicy.MinDuration)
	assertNoFinanceFields(t, publicPolicy)
	assertNoFinanceFields(t, userMediaTaskFromService(&service.MediaTask{Quote: *quote, AccountID: 99, Specification: minimax.CreateVideoRequest{Model: "MiniMax-H3"}}))
}

func TestPublicBatchImageResponsesOmitFinancialSnapshots(t *testing.T) {
	actual := .2
	assertNoFinanceFields(t, &service.BatchImagePublicBatch{ID: "batch", EstimatedCost: .3, HoldAmount: .4, ActualCost: &actual})
}

type userUsagePrivacyRepo struct {
	service.UsageLogRepository
}

func (*userUsagePrivacyRepo) GetAPIKeyDashboardStats(context.Context, int64) (*usagestats.UserDashboardStats, error) {
	return &usagestats.UserDashboardStats{TodayTokens: 77, TotalTokens: 123, TodayCost: 1, TotalActualCost: 3}, nil
}

func TestPublicKeyUsageSummaryReturnsTokensWithoutFees(t *testing.T) {
	h := &GatewayHandler{usageService: service.NewUsageService(&userUsagePrivacyRepo{}, nil, nil, nil)}
	data := h.buildUsageData(context.Background(), 5)
	require.Equal(t, int64(77), data["today"].(gin.H)["total_tokens"])
	require.Equal(t, int64(123), data["total"].(gin.H)["total_tokens"])
	assertNoFinanceFields(t, data)
}

type ownedKeyPrivacyRepo struct {
	service.APIKeyRepository
}

func (*ownedKeyPrivacyRepo) VerifyOwnership(context.Context, int64, []int64) ([]int64, error) {
	return []int64{5}, nil
}

type batchKeyPrivacyRepo struct {
	service.UsageLogRepository
	ids []int64
}

func (r *batchKeyPrivacyRepo) GetBatchAPIKeyUsageStats(_ context.Context, ids []int64, _, _ time.Time) (map[int64]*usagestats.BatchAPIKeyUsageStats, error) {
	r.ids = ids
	return map[int64]*usagestats.BatchAPIKeyUsageStats{5: {APIKeyID: 5, TodayTokens: 77, TotalTokens: 123, TotalActualCost: 3, TodayActualCost: 1}}, nil
}

func TestUserBatchKeyUsageIsOwnedAndUsesMeasuredTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &batchKeyPrivacyRepo{}
	h := NewUsageHandler(service.NewUsageService(repo, nil, nil, nil), service.NewAPIKeyService(&ownedKeyPrivacyRepo{}, nil, nil, nil, nil, nil, nil), nil, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42}) })
	router.POST("/usage/dashboard/api-keys-usage", h.DashboardAPIKeysUsage)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/usage/dashboard/api-keys-usage", strings.NewReader(`{"api_key_ids":[5,999]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, []int64{5}, repo.ids)
	require.Contains(t, rec.Body.String(), `"today_tokens":77`)
	require.Contains(t, rec.Body.String(), `"total_tokens":123`)
	require.NotContains(t, rec.Body.String(), `"actual_cost"`)
	require.NotContains(t, rec.Body.String(), `"total_actual_cost"`)
}
