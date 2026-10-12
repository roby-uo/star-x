//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func startPassthroughBillingServer(t *testing.T, ctx context.Context, svc *OpenAIGatewayService, account *Account, results chan<- *OpenAIForwardResult) (*httptest.Server, <-chan error) {
	t.Helper()
	serverErr := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			serverErr <- err
			return
		}
		defer conn.CloseNow()
		_, payload, err := conn.Read(ctx)
		if err != nil {
			serverErr <- err
			return
		}
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = r.Clone(ctx)
		c.Set("api_key", &APIKey{Group: &Group{ID: 7, RateMultiplier: 5}})
		hooks := &OpenAIWSIngressHooks{AfterTurn: func(_ int, result *OpenAIForwardResult, _ error) {
			if result != nil && results != nil {
				results <- result
			}
		}}
		serverErr <- svc.ProxyResponsesWebSocketFromClient(ctx, c, conn, account, "sk-test", payload, hooks)
	}))
	return server, serverErr
}

func dialPassthroughBillingClient(t *testing.T, server *httptest.Server, payload string) *coderws.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	require.NoError(t, conn.Write(ctx, coderws.MessageText, []byte(payload)))
	return conn
}

func passthroughBillingService(t *testing.T, upstream *stagedPassthroughConn, config BillingManagementConfig) *OpenAIGatewayService {
	t.Helper()
	cfg := passthroughLifecycleConfig()
	cfg.Gateway.OpenAIFirstOutputTimeoutSeconds = 30
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 30
	cfg.Gateway.OpenAIWS.IngressInterTurnIdleTimeoutSeconds = 30
	svc := newPassthroughLifecycleService(cfg, upstream)
	svc.billingService = simpleTestBilling(t, config)
	return svc
}

func passthroughConfirmedCostConfig() BillingManagementConfig {
	cfg := simpleTestConfig()
	cfg.CostProfiles = []CostProfile{{ID: "known", Name: "Confirmed reseller", Enabled: true, AccountIDs: []int64{901}, PurchaseMode: "payg", Currency: "USD", DiscountMultiplier: 1,
		Prices: []CostPriceRow{{ID: "standard", Models: []string{"gpt-5.1"}, Modality: "chat", BillingUnit: "per_million_tokens", InputPrice: costNumber(1), OutputPrice: costNumber(2), CacheReadPrice: costNumber(.1), CacheWritePrice: costNumber(0)},
			{ID: "priority", Models: []string{"gpt-5.1"}, Modality: "chat", ServiceTier: "priority", BillingUnit: "per_million_tokens", InputPrice: costNumber(3), OutputPrice: costNumber(6), CacheReadPrice: costNumber(.3), CacheWritePrice: costNumber(0)}}}}
	return cfg
}

func TestPassthroughSimpleBillingRejectsUnknownFirstTurnBeforeDial(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	upstream := newStagedPassthroughConn()
	svc := passthroughBillingService(t, upstream, simpleTestConfig())
	// A nil dialer makes any attempt to reach the upstream visibly fail.
	svc.openaiWSPassthroughDialer = nil
	account := passthroughLifecycleAccount()
	account.Credentials["base_url"] = "https://private-supplier.example/v1"
	server, serverErr := startPassthroughBillingServer(t, ctx, svc, account, nil)
	defer server.Close()
	conn := dialPassthroughBillingClient(t, server, `{"type":"response.create","model":"gpt-5.1"}`)
	defer conn.CloseNow()
	event, err := readPassthroughLifecycleFrame(t, conn, 3*time.Second)
	require.NoError(t, err)
	require.Equal(t, "pricing_unconfirmed", gjson.GetBytes(event, "error.code").String())
	require.NotContains(t, string(event), "private-supplier")
	select {
	case err := <-serverErr:
		var closeErr *OpenAIWSClientCloseError
		require.ErrorAs(t, err, &closeErr)
		require.NotContains(t, err.Error(), "private-supplier")
	case <-time.After(3 * time.Second):
		t.Fatal("pricing rejection did not finish")
	}
	require.Empty(t, upstream.writes)
}

func TestPassthroughSimpleBillingRejectsUnknownFollowingTurnWithoutUpstreamWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	upstream := newStagedPassthroughConn()
	svc := passthroughBillingService(t, upstream, passthroughConfirmedCostConfig())
	account := passthroughLifecycleAccount()
	account.Credentials["base_url"] = "https://private-supplier.example/v1"
	results := make(chan *OpenAIForwardResult, 2)
	server, serverErr := startPassthroughBillingServer(t, ctx, svc, account, results)
	defer server.Close()
	conn := dialPassthroughBillingClient(t, server, `{"type":"response.create","model":"gpt-5.1"}`)
	defer conn.CloseNow()
	requirePassthroughUpstreamWrite(t, upstream, 3*time.Second)
	upstream.Send(`{"type":"response.completed","response":{"id":"resp_known","usage":{"input_tokens":1,"output_tokens":1}}}`)
	_, err := readPassthroughLifecycleFrame(t, conn, 3*time.Second)
	require.NoError(t, err)
	require.NotNil(t, (<-results).SimpleBilling)
	writeCtx, cancelWrite := context.WithTimeout(ctx, 3*time.Second)
	err = conn.Write(writeCtx, coderws.MessageText, []byte(`{"type":"response.create","model":"unpriced-model"}`))
	cancelWrite()
	require.NoError(t, err)
	event, err := readPassthroughLifecycleFrame(t, conn, 3*time.Second)
	require.NoError(t, err)
	require.Equal(t, "pricing_unconfirmed", gjson.GetBytes(event, "error.code").String())
	require.Empty(t, upstream.writes, "unpriced second turn must not reach upstream")
	_, err = readPassthroughLifecycleFrame(t, conn, 3*time.Second)
	var closeErr coderws.CloseError
	require.ErrorAs(t, err, &closeErr)
	require.Equal(t, coderws.StatusPolicyViolation, closeErr.Code)
	select {
	case err := <-serverErr:
		require.Error(t, err)
		require.False(t, errors.Is(err, context.DeadlineExceeded))
	case <-time.After(3 * time.Second):
		t.Fatal("second-turn pricing rejection did not finish")
	}
}

func TestPassthroughSimpleBillingFreezesMappedTurnPriceAndMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	upstream := newStagedPassthroughConn()
	cfg := passthroughConfirmedCostConfig()
	cfg.CostProfiles[0].Prices[1].ImageInputPrice = costNumber(4)
	svc := passthroughBillingService(t, upstream, cfg)
	account := passthroughLifecycleAccount()
	account.Credentials["base_url"] = "https://private-supplier.example/v1"
	// Chain mappings catch accidentally applying GetMappedModel twice.
	account.Credentials["model_mapping"] = map[string]any{"alias": "gpt-5.1", "gpt-5.1": "unpriced-double-mapping"}
	results := make(chan *OpenAIForwardResult, 4)
	server, _ := startPassthroughBillingServer(t, ctx, svc, account, results)
	defer server.Close()
	conn := dialPassthroughBillingClient(t, server, `{"type":"response.create","model":"alias","service_tier":"fast","reasoning":{"effort":"high"},"input":[{"type":"message","role":"user","content":[{"type":"input_image","image_url":"https://example.org/image.png"}]}]}`)
	defer conn.CloseNow()
	firstWrite := requirePassthroughUpstreamWrite(t, upstream, 3*time.Second)
	require.Equal(t, "gpt-5.1", gjson.GetBytes(firstWrite, "model").String())
	require.Equal(t, "priority", gjson.GetBytes(firstWrite, "service_tier").String())
	upstream.Send(`{"type":"response.created","response":{"id":"resp_one"}}`)
	_, err := readPassthroughLifecycleFrame(t, conn, 3*time.Second)
	require.NoError(t, err)
	// Change the configured future rate while turn one is still in flight.
	cfg.Version, cfg.APISaleMultiplier = 8, 2
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	repo := svc.billingService.simpleRepo.(*memoryCostBook)
	repo.mu.Lock()
	repo.raw = string(raw)
	repo.mu.Unlock()
	svc.billingService.invalidateSimpleBillingConfig()
	upstream.Send(`{"type":"response.completed","response":{"id":"resp_one","usage":{"input_tokens":1000,"output_tokens":500,"input_tokens_details":{"image_tokens":250}}}}`)
	_, err = readPassthroughLifecycleFrame(t, conn, 3*time.Second)
	require.NoError(t, err)
	first := <-results
	require.Equal(t, "alias", first.Model)
	require.Equal(t, "gpt-5.1", first.UpstreamModel)
	require.Equal(t, int64(7), first.SimpleBilling.Version)
	require.Equal(t, 1.4, first.SimpleBilling.Multiplier)
	require.Equal(t, "priority", *first.ServiceTier)
	require.Equal(t, 250, first.Usage.ImageInputTokens)
	writeCtx, cancelWrite := context.WithTimeout(ctx, 3*time.Second)
	require.NoError(t, conn.Write(writeCtx, coderws.MessageText, []byte(`{"type":"session.update","session":{"model":"alias"}}`)))
	require.NoError(t, conn.Write(writeCtx, coderws.MessageText, []byte(`{"type":"response.create","reasoning":{"effort":"low"}}`)))
	cancelWrite()
	sessionWrite := requirePassthroughUpstreamWrite(t, upstream, 3*time.Second)
	require.Equal(t, "gpt-5.1", gjson.GetBytes(sessionWrite, "session.model").String())
	secondWrite := requirePassthroughUpstreamWrite(t, upstream, 3*time.Second)
	require.Equal(t, "gpt-5.1", gjson.GetBytes(secondWrite, "model").String())
	// A delayed duplicate must not bind the second turn or charge again.
	upstream.Send(`{"type":"response.completed","response":{"id":"resp_one","usage":{"input_tokens":1000,"output_tokens":500}}}`)
	_, err = readPassthroughLifecycleFrame(t, conn, 3*time.Second)
	require.NoError(t, err)
	require.Empty(t, results)
	upstream.Send(`{"type":"response.completed","response":{"id":"resp_two","usage":{"input_tokens":1000,"output_tokens":500}}}`)
	_, err = readPassthroughLifecycleFrame(t, conn, 3*time.Second)
	require.NoError(t, err)
	second := <-results
	require.Equal(t, "resp_two", second.RequestID)
	require.Equal(t, "alias", second.Model)
	require.Equal(t, int64(8), second.SimpleBilling.Version)
	require.Equal(t, 2.0, second.SimpleBilling.Multiplier)
	require.Nil(t, second.ServiceTier, "default turn must not inherit priority")
	require.Zero(t, second.Usage.ImageInputTokens, "next text-only turn must not inherit vision usage")
	require.Equal(t, "low", *second.ReasoningEffort)
	firstCost, err := svc.billingService.CalculateSimpleBilling(first.SimpleBilling, BillingPreviewInput{InputTokens: first.Usage.InputTokens, ImageInputTokens: first.Usage.ImageInputTokens, OutputTokens: first.Usage.OutputTokens, Requests: 1})
	require.NoError(t, err)
	secondCost, err := svc.billingService.CalculateSimpleBilling(second.SimpleBilling, BillingPreviewInput{InputTokens: 1000, OutputTokens: 500, Requests: 1})
	require.NoError(t, err)
	require.InDelta(t, .00875, firstCost.ActualCost, 1e-12)
	require.InDelta(t, .004, secondCost.ActualCost, 1e-12)
	require.NotSame(t, first.SimpleBilling, second.SimpleBilling)
}

func TestPassthroughTurnMetadataAssociatesResponseIDsWithoutConsumingDuplicates(t *testing.T) {
	q := &openAIWSPassthroughTurnMetadataQueue{}
	first := &openAIWSPassthroughTurnMetadata{requestModel: "first", simpleBilling: &SimpleBillingSnapshot{Version: 1}}
	second := &openAIWSPassthroughTurnMetadata{requestModel: "second", simpleBilling: &SimpleBillingSnapshot{Version: 2}}
	q.enqueue(first)
	require.True(t, q.observe([]byte(`{"type":"response.created","response":{"id":"first-id"}}`)))
	q.enqueue(second)
	require.True(t, q.observe([]byte(`{"type":"response.created","response":{"id":"second-id"}}`)))
	// Response completion order cannot switch the metadata snapshots.
	got, ok := q.complete("second-id")
	require.True(t, ok)
	require.Same(t, second, got)
	got, ok = q.complete("first-id")
	require.True(t, ok)
	require.Same(t, first, got)
	third := &openAIWSPassthroughTurnMetadata{requestModel: "third"}
	q.enqueue(third)
	require.False(t, q.observe([]byte(`{"type":"response.completed","response":{"id":"first-id"}}`)))
	_, ok = q.complete("first-id")
	require.False(t, ok)
	require.True(t, q.observe([]byte(`{"type":"response.completed","id":"third-id"}`)))
	got, ok = q.complete("third-id")
	require.True(t, ok)
	require.Same(t, third, got)
}

func TestPassthroughSimpleBillingPreflightsAndCountsImageToolsPerTurn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	c.Set("api_key", &APIKey{Group: &Group{ID: 7, RateMultiplier: 5}})
	account := passthroughLifecycleAccount()
	account.Credentials["base_url"] = "https://private-supplier.example/v1"
	cfg := passthroughConfirmedCostConfig()
	payload := []byte(`{"type":"response.create","model":"gpt-5.1","tools":[{"type":"image_generation","model":"draw","size":"1024x1024"}]}`)
	svc := passthroughBillingService(t, newStagedPassthroughConn(), cfg)
	_, _, err := svc.prepareOpenAIWSPassthroughTurn(context.Background(), c, account, "gpt-5.1", "gpt-5.1", payload)
	require.Error(t, err, "known text pricing cannot authorize an unpriced image tool")
	cfg.CostProfiles[0].Prices = append(cfg.CostProfiles[0].Prices, CostPriceRow{ID: "draw", Models: []string{"draw"}, Modality: "image", BillingUnit: "per_image", UnitPrice: costNumber(.7)})
	svc = passthroughBillingService(t, newStagedPassthroughConn(), cfg)
	_, meta, err := svc.prepareOpenAIWSPassthroughTurn(context.Background(), c, account, "gpt-5.1", "gpt-5.1", payload)
	require.NoError(t, err)
	require.NotNil(t, meta.simpleBilling.ImageBilling)
	q := &openAIWSPassthroughTurnMetadataQueue{}
	q.enqueue(meta)
	q.observe([]byte(`{"type":"response.created","response":{"id":"image-turn"}}`))
	// Some standard output-item events omit response_id and terminal output.
	q.observe([]byte(`{"type":"response.output_item.done","item":{"id":"image-one","type":"image_generation_call","status":"completed","result":"base64-image"}}`))
	q.observe([]byte(`{"type":"response.completed","response":{"id":"image-turn","usage":{"input_tokens":1000,"output_tokens":500}}}`))
	meta, ok := q.complete("image-turn")
	require.True(t, ok)
	result := &OpenAIForwardResult{}
	meta.applyImages(result)
	require.Equal(t, 1, result.ImageCount)
	require.Equal(t, "draw", result.BillingModel)
	cost, err := svc.billingService.CalculateSimpleBilling(meta.simpleBilling, BillingPreviewInput{InputTokens: 1000, OutputTokens: 500, ImageCount: result.ImageCount, Requests: 1})
	require.NoError(t, err)
	require.InDelta(t, (.002+.7)*1.4, cost.ActualCost, 1e-12)
	q.enqueue(&openAIWSPassthroughTurnMetadata{imageCounter: newOpenAIImageOutputCounter()})
	require.Equal(t, 0, q.fallback().imageCounter.Count(), "a new turn must not inherit generated images")
}
