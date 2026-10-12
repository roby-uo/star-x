//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func finalBodyCostConfig(accountID int64, model string) BillingManagementConfig {
	cfg := simpleTestConfig()
	cfg.CostProfiles = []CostProfile{{ID: "final", Name: "Confirmed standard tier", Enabled: true, AccountIDs: []int64{accountID}, PurchaseMode: "payg", Currency: "USD", DiscountMultiplier: 1,
		Prices: []CostPriceRow{{ID: "standard", Models: []string{model}, Modality: "chat", BillingUnit: "per_million_tokens", InputPrice: costNumber(1), OutputPrice: costNumber(2), CacheReadPrice: costNumber(.1), CacheWritePrice: costNumber(0)}}}}
	return cfg
}

func TestSimpleBillingHTTPPreflightUsesFilteredFinalTier(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name, path, body string
		raw, stream      bool
		forward          func(*OpenAIGatewayService, *gin.Context, *Account, []byte) (*OpenAIForwardResult, error)
	}{
		{"responses", "/v1/responses", `{"model":"gpt-5.1","input":"Hi","service_tier":"priority"}`, false, false, func(s *OpenAIGatewayService, c *gin.Context, a *Account, b []byte) (*OpenAIForwardResult, error) {
			return s.Forward(context.Background(), c, a, b)
		}},
		{"chat to responses", "/v1/chat/completions", `{"model":"gpt-5.1","messages":[{"role":"user","content":"Hi"}],"service_tier":"fast"}`, false, true, func(s *OpenAIGatewayService, c *gin.Context, a *Account, b []byte) (*OpenAIForwardResult, error) {
			return s.ForwardAsChatCompletions(context.Background(), c, a, b, "", "")
		}},
		{"messages to responses", "/v1/messages", `{"model":"gpt-5.1","max_tokens":10,"messages":[{"role":"user","content":"Hi"}]}`, false, true, func(s *OpenAIGatewayService, c *gin.Context, a *Account, b []byte) (*OpenAIForwardResult, error) {
			return s.ForwardAsAnthropic(context.Background(), c, a, b, "", "")
		}},
		{"raw chat", "/v1/chat/completions", `{"model":"gpt-5.1","messages":[{"role":"user","content":"Hi"}],"service_tier":"priority"}`, true, false, func(s *OpenAIGatewayService, c *gin.Context, a *Account, b []byte) (*OpenAIForwardResult, error) {
			return s.ForwardAsChatCompletions(context.Background(), c, a, b, "", "")
		}},
		{"responses to raw chat", "/v1/responses", `{"model":"gpt-5.1","input":"Hi","service_tier":"priority"}`, true, false, func(s *OpenAIGatewayService, c *gin.Context, a *Account, b []byte) (*OpenAIForwardResult, error) {
			return s.Forward(context.Background(), c, a, b)
		}},
		{"messages to raw chat", "/v1/messages", `{"model":"gpt-5.1","max_tokens":10,"messages":[{"role":"user","content":"Hi"}]}`, true, false, func(s *OpenAIGatewayService, c *gin.Context, a *Account, b []byte) (*OpenAIForwardResult, error) {
			return s.ForwardAsAnthropic(context.Background(), c, a, b, "", "")
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			responseBody := `{"id":"resp_final","model":"gpt-5.1","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Hi"}]}],"usage":{"input_tokens":10,"output_tokens":5}}`
			if test.raw {
				responseBody = `{"id":"cc_final","model":"gpt-5.1","choices":[{"index":0,"message":{"role":"assistant","content":"Hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`
			} else if test.stream {
				responseBody = "data: {\"type\":\"response.completed\",\"response\":" + responseBody + "}\n\ndata: [DONE]\n\n"
			}
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(responseBody))}}
			svc := newOpenAIImageGenerationControlTestService(upstream)
			svc.settingService = newOpenAIGatewayServiceWithSettings(t, openAIFastFilterPriorityPolicy()).settingService
			account := newOpenAIImageGenerationControlTestAccount()
			account.Credentials["base_url"] = "https://reseller.example/v1"
			if test.raw {
				account.Extra = map[string]any{openai_compat.ExtraKeyResponsesMode: string(openai_compat.ResponsesSupportModeForceChatCompletions)}
			}
			svc.billingService = simpleTestBilling(t, finalBodyCostConfig(account.ID, "gpt-5.1"))
			c, recorder := newOpenAIImageGenerationControlTestContext(true, "unit-test-agent/1.0")
			c.Request.URL.Path = test.path
			if strings.HasSuffix(test.path, "/messages") {
				c.Request.Header.Set("anthropic-beta", claude.BetaFastMode)
			}
			result, err := test.forward(svc, c, account, []byte(test.body))
			require.NoError(t, err, recorder.Body.String())
			require.NotNil(t, result)
			require.NotNil(t, result.SimpleBilling)
			require.Empty(t, result.SimpleBilling.ServiceTier, "snapshot must price what the upstream actually received")
			require.Nil(t, result.ServiceTier)
			require.False(t, gjson.GetBytes(upstream.lastBody, "service_tier").Exists())
			require.Len(t, upstream.requests, 1)
		})
	}
}

func TestSimpleBillingPreflightsInjectedImageToolAndIgnoresRemovedTool(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, strip := range []bool{false, true} {
		t.Run(map[bool]string{false: "injected tool needs price", true: "removed tool needs no price"}[strip], func(t *testing.T) {
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"id":"text","usage":{"input_tokens":1,"output_tokens":1}}`))}}
			svc := newOpenAIImageGenerationControlTestService(upstream)
			svc.cfg.Gateway.CodexImageGenerationBridgeEnabled = !strip
			account := newOpenAIImageGenerationControlTestAccount()
			account.Credentials["base_url"] = "https://reseller.example/v1"
			svc.billingService = simpleTestBilling(t, finalBodyCostConfig(account.ID, "gpt-5.4"))
			body := []byte(`{"model":"gpt-5.4","input":"draw"}`)
			if strip {
				account.Extra = map[string]any{featureKeyCodexImageGenerationExplicitToolPolicy: codexImageGenerationExplicitToolPolicyStrip}
				body = []byte(`{"model":"gpt-5.4","input":"draw","tools":[{"type":"image_generation","model":"unpriced-image"}]}`)
			}
			c, recorder := newOpenAIImageGenerationControlTestContext(true, "codex_cli_rs/0.144.1")
			result, err := svc.Forward(context.Background(), c, account, body)
			if strip {
				require.NoError(t, err)
				require.NotNil(t, result.SimpleBilling)
				require.Nil(t, result.SimpleBilling.ImageBilling)
				require.False(t, gjson.GetBytes(upstream.lastBody, `tools.#(type=="image_generation")`).Exists())
			} else {
				require.Error(t, err)
				require.Nil(t, result)
				require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
				require.Empty(t, upstream.requests, "unknown auto-injected image tool must block before generation")
			}
		})
	}
}

func TestSimpleBillingAnthropicPricesFinalBetaHeaderAndWireImages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request.Header.Set("anthropic-beta", claude.BetaFastMode)
	c.Set("api_key", &APIKey{Group: &Group{ID: 7, RateMultiplier: 4}})
	account := &Account{ID: 40, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://reseller.example"}}
	svc := &GatewayService{billingService: simpleTestBilling(t, finalBodyCostConfig(account.ID, "claude-sonnet-4-6")), cfg: &config.Config{}}
	body := []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"Hi"}]}`)
	q, err := svc.prepareSimpleBillingAnthropicForward(context.Background(), c, account, "wrong-original-alias", body, http.Header{})
	require.NoError(t, err)
	require.Empty(t, q.ServiceTier, "filtered final beta must not recover priority from client headers")
	require.Equal(t, "claude-sonnet-4-6", q.Model)
	_, err = svc.prepareSimpleBillingAnthropicForward(context.Background(), c, account, q.Model, body, http.Header{"Anthropic-Beta": []string{claude.BetaFastMode}})
	require.Error(t, err, "actual priority header requires a confirmed priority price")
	_, err = svc.prepareSimpleBillingAnthropicForward(context.Background(), c, account, q.Model, []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":[{"type":"image","source":{"type":"base64","media_type":"image/png","data":"example"}}]}]}`), http.Header{})
	require.Error(t, err, "actual image input requires an explicitly confirmed input-image token price")
}

func TestSimpleBillingImageRouteUsesActualChannelThenAccountMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"data":[{"b64_json":"example-image"}]}`))}}
	svc := newOpenAIImageGenerationControlTestService(upstream)
	account := newOpenAIImageGenerationControlTestAccount()
	account.Credentials["base_url"] = "https://reseller.example/v1"
	account.Credentials["model_mapping"] = map[string]any{"gpt-image-1": "gpt-image-2", "gpt-image-1.5": "gpt-image-1-mini", "gpt-image-1-mini": "gpt-image-unconfirmed"}
	cfg := simpleTestConfig()
	cfg.CostProfiles = []CostProfile{{ID: "images", Name: "Confirmed images", Enabled: true, AccountIDs: []int64{account.ID}, PurchaseMode: "payg", Currency: "USD", DiscountMultiplier: 1,
		Prices: []CostPriceRow{{ID: "mini", Models: []string{"gpt-image-1-mini"}, Modality: "image", BillingUnit: "per_image", UnitPrice: costNumber(.25)}}}}
	svc.billingService = simpleTestBilling(t, cfg)
	c, _ := newOpenAIImageGenerationControlTestContext(true, "unit-test-agent/1.0")
	c.Request.URL.Path = "/v1/images/generations"
	result, err := svc.ForwardImages(context.Background(), c, account, []byte(`{"model":"gpt-image-1","prompt":"Draw a square","size":"1024x1024"}`), &OpenAIImagesRequest{Model: "gpt-image-1", ContentType: "application/json", Endpoint: openAIImagesGenerationsEndpoint, Size: "1024x1024", SizeTier: "1K", N: 1}, "gpt-image-1.5")
	require.NoError(t, err)
	require.NotNil(t, result.SimpleBilling)
	require.Equal(t, "gpt-image-1-mini", result.SimpleBilling.Model)
	require.Equal(t, "gpt-image-1-mini", result.UpstreamModel)
	require.Equal(t, "gpt-image-1-mini", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, 1, result.ImageCount)
	cost, err := svc.billingService.CalculateSimpleBilling(result.SimpleBilling, BillingPreviewInput{ImageCount: result.ImageCount, Requests: 1})
	require.NoError(t, err)
	require.InDelta(t, .35, cost.ActualCost, 1e-12)
}
