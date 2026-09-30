//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestMiniMaxM3PriorityUsesChannelContextTierAndGroupRate(t *testing.T) {
	for _, tc := range []struct {
		name, baseURL, model, tier string
		input                      int
		wantTierRate               float64
	}{
		{"standard", "https://api.minimax.cn/v1", "MiniMax-M3", "", 512000, 1},
		{"priority-low-boundary", "https://api.minimax.cn/v1", "MiniMax-M3", "priority", 512000, 1.5},
		{"priority-high-boundary", "https://api.minimax.io/v1", "MiniMax-M3", "priority", 512001, 1.5},
		{"other-provider-unchanged", "https://relay.example/v1", "MiniMax-M3", "priority", 512000, 1},
		{"host-suffix-not-official", "https://api.minimax.cn.evil.example/v1", "MiniMax-M3", "priority", 512000, 1},
		{"other-model-unchanged", "https://api.minimax.cn/v1", "MiniMax-M2.7", "priority", 512000, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
			userRepo := &openAIRecordUsageUserRepoStub{}
			svc := newOpenAIRecordUsageServiceForTest(usageRepo, userRepo, &openAIRecordUsageSubRepoStub{}, nil)
			channel := &ChannelService{}
			channel.cache.Store(populateChannelCache([]Channel{{ID: 1, Status: StatusActive, GroupIDs: []int64{1}, ModelPricing: []ChannelModelPricing{{
				Platform: PlatformOpenAI, Models: []string{tc.model}, BillingMode: BillingModeToken,
				Intervals: []PricingInterval{
					{MinTokens: 0, MaxTokens: testPtrInt(512000), InputPrice: testPtrFloat64(3e-7), OutputPrice: testPtrFloat64(1.2e-6), CacheReadPrice: testPtrFloat64(6e-8), CacheWritePrice: testPtrFloat64(0)},
					{MinTokens: 512000, InputPrice: testPtrFloat64(6e-7), OutputPrice: testPtrFloat64(2.4e-6), CacheReadPrice: testPtrFloat64(1.2e-7), CacheWritePrice: testPtrFloat64(0)},
				},
			}}}}, map[int64]string{1: PlatformOpenAI}))
			svc.channelService = channel
			svc.resolver = NewModelPricingResolver(channel, svc.billingService)
			input := &OpenAIRecordUsageInput{
				Result:  &OpenAIForwardResult{RequestID: tc.name, Model: tc.model, UpstreamModel: tc.model, ServiceTier: &tc.tier, Usage: OpenAIUsage{InputTokens: tc.input, CacheReadInputTokens: 400000, OutputTokens: 1000}},
				APIKey:  &APIKey{ID: 1, GroupID: i64p(1), Group: &Group{ID: 1, Platform: PlatformOpenAI, RateMultiplier: 1.4}},
				User:    &User{ID: 1},
				Account: &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test-key", "base_url": tc.baseURL}},
			}
			require.NoError(t, svc.RecordUsage(context.Background(), input))
			priceScale := 1.0
			if tc.input > 512000 {
				priceScale = 2
			}
			base := (float64(tc.input-400000)*3e-7 + 400000*6e-8 + 1000*1.2e-6) * priceScale
			require.NotNil(t, usageRepo.lastLog)
			require.InDelta(t, base*tc.wantTierRate, usageRepo.lastLog.TotalCost, 1e-10)
			require.InDelta(t, base*tc.wantTierRate*1.4, usageRepo.lastLog.ActualCost, 1e-10)
			require.InDelta(t, usageRepo.lastLog.ActualCost, userRepo.lastAmount, 1e-10)
			require.Equal(t, 1.4, usageRepo.lastLog.RateMultiplier)
			require.Equal(t, tc.tier, *usageRepo.lastLog.ServiceTier)
		})
	}
}

func TestMiniMaxM3RawChatTierMatchesForwardedBody(t *testing.T) {
	for _, filter := range []bool{false, true} {
		settings := &OpenAIFastPolicySettings{}
		if filter {
			settings.Rules = []OpenAIFastPolicyRule{{ServiceTier: "priority", Scope: "all", Action: BetaPolicyActionFilter}}
		}
		svc := newOpenAIGatewayServiceWithSettings(t, settings)
		svc.cfg = rawChatCompletionsTestConfig()
		upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"id":"test-minimax","model":"MiniMax-M3","choices":[{"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":1}}`))}}
		svc.httpUpstream = upstream
		account := rawChatCompletionsTestAccount()
		account.Credentials["base_url"] = "https://api.minimax.cn/v1"
		body := []byte(`{"model":"MiniMax-M3","messages":[{"role":"user","content":"OK"}],"service_tier":"priority"}`)
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		result, err := svc.forwardAsRawChatCompletions(context.Background(), c, account, body, "")
		require.NoError(t, err)
		if filter {
			require.False(t, gjson.GetBytes(upstream.lastBody, "service_tier").Exists())
			require.Nil(t, result.ServiceTier)
		} else {
			require.Equal(t, "priority", gjson.GetBytes(upstream.lastBody, "service_tier").String())
			require.NotNil(t, result.ServiceTier)
			require.Equal(t, "priority", *result.ServiceTier)
		}
	}
}
