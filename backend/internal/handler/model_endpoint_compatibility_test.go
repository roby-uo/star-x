package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestTextHandlersRejectGenerationModelsBeforeAccountSelectionOrBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, model := range []string{"MiniMax-H3", "MiniMax-H3-Max", "doubao-seedream-5-0-260128", "gpt-image-2"} {
		for _, endpoint := range []string{"/v1/chat/completions", "/v1/responses", "/v1/messages"} {
			for _, openAI := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/openai=%t", model, endpoint, openAI), func(t *testing.T) {
					body := fmt.Sprintf(`{"model":%q,"max_tokens":128,"messages":[{"role":"user","content":"hello"}],"input":"hello"}`, model)
					c, rec := newOpenAICompatibleStreamValidationContext(endpoint, body, false)
					key, ok := middleware2.GetAPIKeyFromContext(c)
					require.True(t, ok)
					key.Group.Platform = service.PlatformOpenAI
					key.Group.AllowMessagesDispatch = true
					// These services have no account repository or billing cache.
					// Reaching either operation would panic instead of this 400.
					gateway := &GatewayHandler{gatewayService: &service.GatewayService{}}
					openAIHandler := newOpenAIHandlerForPreviousResponseIDValidation(t, nil)
					switch endpoint {
					case "/v1/chat/completions":
						if openAI {
							openAIHandler.ChatCompletions(c)
						} else {
							gateway.ChatCompletions(c)
						}
					case "/v1/responses":
						if openAI {
							openAIHandler.Responses(c)
						} else {
							gateway.Responses(c)
						}
					case "/v1/messages":
						if openAI {
							openAIHandler.Messages(c)
						} else {
							gateway.Messages(c)
						}
					}
					require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
					require.Equal(t, "model_endpoint_mismatch", gjson.GetBytes(rec.Body.Bytes(), "error.code").String())
					require.Equal(t, "model", gjson.GetBytes(rec.Body.Bytes(), "error.param").String())
					require.Contains(t, rec.Body.String(), "Agent")
					require.NotEmpty(t, gjson.GetBytes(rec.Body.Bytes(), "error.supported_endpoints").Array())
				})
			}
		}
	}
}

func TestTextEndpointGuardPreservesReasoningModelsWithImageToolsAndInput(t *testing.T) {
	for _, body := range []string{
		`{"model":"gpt-5.6","tools":[{"type":"image_generation"}]}`,
		`{"model":"qwen3-vl","messages":[{"content":[{"type":"image_url","image_url":{"url":"https://example.test/image.png"}}]}]}`,
		`{"model":"my-video-understanding-model"}`,
	} {
		c, rec := newOpenAICompatibleStreamValidationContext("/v1/responses", body, false)
		require.False(t, rejectGenerationModelOnTextEndpoint(c, &service.GatewayService{}, nil, gjson.Get(body, "model").String(), false))
		require.Empty(t, rec.Body.String())
	}
}

func TestGatewayModelsAgentDiscoveryFiltersOnlyKnownGenerationModels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	groupID := int64(77)
	handler := newGatewayModelsHandlerForTest(&gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
		groupID: {{ID: 1, Platform: service.PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-5.6": "gpt-5.6", "qwen3-vl": "qwen3-vl", "custom": "custom", "MiniMax-H3": "MiniMax-H3", "doubao-seedream-5-0-260128": "doubao-seedream-5-0-260128"}}}},
	}})
	for _, purpose := range []string{"", "agent"} {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/v1/models?purpose="+purpose, nil)
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{GroupID: &groupID, Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI}})
		handler.Models(c)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var response struct {
			Data []struct {
				ID        string   `json:"id"`
				ModelType string   `json:"model_type"`
				Endpoints []string `json:"supported_endpoints"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		ids := make([]string, 0, len(response.Data))
		for _, model := range response.Data {
			ids = append(ids, model.ID)
			require.NotEmpty(t, model.ModelType)
		}
		require.Contains(t, ids, "gpt-5.6")
		require.Contains(t, ids, "qwen3-vl")
		require.Contains(t, ids, "custom")
		if purpose == "agent" {
			require.Len(t, ids, 3)
			require.NotContains(t, ids, "MiniMax-H3")
		} else {
			require.Len(t, ids, 5)
		}
	}
}

func TestCodexManifestFiltersGenerationAndPreservesOtherModelMetadata(t *testing.T) {
	handler, upstream, groupID := newCodexModelsFailoverTestHandlerWithAccountCount(http.StatusOK, 1, 3)
	upstream.firstBody = `{"models":[{"slug":"gpt-5.6","input_modalities":["text","image"],"tools":["image_generation"]},{"slug":"MiniMax-H3"},{"slug":"doubao-seedream-5-0-260128"},{"slug":"custom","extra":true}],"version":"upstream-v1"}`
	rec := performCodexModelsRequest(t, handler, groupID)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "upstream-v1", gjson.GetBytes(rec.Body.Bytes(), "version").String())
	require.Len(t, gjson.GetBytes(rec.Body.Bytes(), "models").Array(), 2)
	require.Equal(t, "image_generation", gjson.GetBytes(rec.Body.Bytes(), "models.0.tools.0").String())
	require.True(t, gjson.GetBytes(rec.Body.Bytes(), "models.1.extra").Bool())
	require.Contains(t, rec.Header().Get("ETag"), "starx-agent-")

	conditional := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(conditional)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models?client_version=0.144.0", nil)
	c.Request.Header.Set("If-None-Match", rec.Header().Get("ETag"))
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{GroupID: &groupID, Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI}})
	handler.CodexModels(c)
	c.Writer.WriteHeaderNow()
	require.Equal(t, http.StatusNotModified, conditional.Code, conditional.Body.String())
	require.Empty(t, conditional.Body.String())
}

func TestResponsesWebSocketRejectsGenerationModelBeforeUserSlotAndBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var userAcquisitions atomic.Int32
	cache := &concurrencyCacheMock{acquireUserSlotFn: func(context.Context, int64, int, string) (bool, error) { userAcquisitions.Add(1); return true, nil }}
	handler := newOpenAIHandlerForPreviousResponseIDValidation(t, cache)
	server := newOpenAIWSHandlerTestServer(t, handler, middleware2.AuthSubject{UserID: 1, Concurrency: 1})
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	connection, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/openai/v1/responses", nil)
	require.NoError(t, err)
	defer func() { _ = connection.CloseNow() }()
	require.NoError(t, connection.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"MiniMax-H3","input":"hello"}`)))
	_, data, err := connection.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, "model_endpoint_mismatch", gjson.GetBytes(data, "error.code").String())
	_, _, err = connection.Read(ctx)
	require.Equal(t, coderws.StatusPolicyViolation, coderws.CloseStatus(err))
	require.Zero(t, userAcquisitions.Load())
}

type modelEndpointAliasUsageRepository struct {
	service.UsageLogRepository
	writes atomic.Int32
}

func (r *modelEndpointAliasUsageRepository) Create(context.Context, *service.UsageLog) (bool, error) {
	r.writes.Add(1)
	return true, nil
}

func newModelEndpointAliasHandler(t *testing.T, upstream service.HTTPUpstream) (*OpenAIGatewayHandler, *modelEndpointAliasUsageRepository) {
	t.Helper()
	cfg := &config.Config{RunMode: config.RunModeSimple}
	account := service.Account{ID: 1, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://upstream.example/v1", "model_mapping": map[string]any{"my-image": "gpt-image-2", "my-video": "MiniMax-H3", "my-text": "gpt-5.6"}},
		Extra:       map[string]any{"openai_passthrough": true},
	}
	usage := &modelEndpointAliasUsageRepository{}
	billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billing.Stop)
	gateway := service.NewOpenAIGatewayService(&openAIWSFailoverHandlerAccountRepoStub{accounts: []service.Account{account}}, usage,
		nil, nil, nil, nil, nil, cfg, nil, nil, service.NewBillingService(cfg, nil), nil, billing, upstream, &service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil)
	return NewOpenAIGatewayHandler(gateway, service.NewConcurrencyService(nil), billing, service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, cfg), nil, nil, nil, nil, cfg), usage
}

func TestAccountMediaAliasesAreRejectedWithoutUpstreamOrUsageWrites(t *testing.T) {
	for _, alias := range []string{"my-image", "my-video"} {
		for _, endpoint := range []string{"/v1/chat/completions", "/v1/responses", "/v1/messages"} {
			t.Run(alias+endpoint, func(t *testing.T) {
				upstream := &openAIHTTPPassthroughFailoverUpstream{}
				handler, usage := newModelEndpointAliasHandler(t, upstream)
				body := fmt.Sprintf(`{"model":%q,"max_tokens":128,"messages":[{"role":"user","content":"hello"}],"input":"hello"}`, alias)
				c, rec := newOpenAICompatibleStreamValidationContext(endpoint, body, false)
				key, ok := middleware2.GetAPIKeyFromContext(c)
				require.True(t, ok)
				key.Group.Platform = service.PlatformOpenAI
				key.Group.AllowMessagesDispatch = true
				c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 13, Concurrency: 0})
				switch endpoint {
				case "/v1/chat/completions":
					handler.ChatCompletions(c)
				case "/v1/responses":
					handler.Responses(c)
				case "/v1/messages":
					handler.Messages(c)
				}
				require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
				require.Equal(t, "model_endpoint_mismatch", gjson.GetBytes(rec.Body.Bytes(), "error.code").String())
				require.Empty(t, upstream.calls(), "no upstream request may be sent for a generation-model alias")
				require.Zero(t, usage.writes.Load(), "rejected aliases produce no model usage")
			})
		}
	}
}

func TestCodexManifestFiltersSelectedAccountMediaAliases(t *testing.T) {
	upstream := &codexModelsFailoverHTTPUpstream{firstBody: `{"models":[{"slug":"my-image"},{"slug":"my-video"},{"slug":"my-text","tools":["image_generation"]}]}`}
	handler, _ := newModelEndpointAliasHandler(t, upstream)
	rec := performCodexModelsRequest(t, handler, 7)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	models := gjson.GetBytes(rec.Body.Bytes(), "models").Array()
	require.Len(t, models, 1)
	require.Equal(t, "my-text", models[0].Get("slug").String())
	require.Equal(t, "image_generation", models[0].Get("tools.0").String())
}

func TestSelectedAccountMediaAliasGuardReleasesAcquiredSlot(t *testing.T) {
	c, rec := newOpenAICompatibleStreamValidationContext("/v1/responses", `{"model":"my-image"}`, false)
	key, ok := middleware2.GetAPIKeyFromContext(c)
	require.True(t, ok)
	key.Group.Platform = service.PlatformOpenAI
	var releases int
	selection := &service.AccountSelectionResult{Account: &service.Account{Credentials: map[string]any{"model_mapping": map[string]any{"my-image": "gpt-image-2"}}}, Acquired: true, ReleaseFunc: func() { releases++ }}
	require.True(t, rejectSelectedAccountGenerationModel(c, key, selection, "my-image", "my-image", "", false))
	require.Equal(t, 1, releases)
	require.Nil(t, selection.ReleaseFunc)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestModelEndpointMismatchAfterHeartbeatUsesTerminalStreamEvent(t *testing.T) {
	for _, endpoint := range []string{"/v1/responses", "/v1/chat/completions", "/v1/messages"} {
		c, rec := newOpenAICompatibleStreamValidationContext(endpoint, `{"model":"my-video"}`, false)
		setOpsRequestContext(c, "my-video", true)
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		_, err := c.Writer.Write([]byte(": heartbeat\n\n"))
		require.NoError(t, err)
		c.Writer.Flush()
		writeModelEndpointError(c, http.StatusBadRequest, "model_endpoint_mismatch", "use /v2/video_generation", []string{"/v2/video_generation"}, endpoint == "/v1/messages")
		if endpoint == "/v1/responses" {
			require.Contains(t, rec.Body.String(), "event: response.failed")
			require.Contains(t, rec.Body.String(), `"code":"model_endpoint_mismatch"`)
		} else {
			require.Contains(t, rec.Body.String(), "event: error")
		}
		require.Equal(t, http.StatusOK, rec.Code, "headers were already committed by the heartbeat")
		streamError, ok := service.GetOpsStreamError(c)
		require.True(t, ok)
		require.Equal(t, http.StatusBadRequest, streamError.IntendedStatus)
		require.False(t, streamError.CountTowardsSLA)
	}
}
