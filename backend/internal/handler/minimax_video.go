package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/minimax"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// MiniMaxVideoGeneration accepts the native H3 request body. The OpenAI-style
// /v1/videos/generations alias also accepts a text-only prompt shortcut.
func (h *OpenAIGatewayHandler) MiniMaxVideoGeneration(c *gin.Context) {
	apiKey, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil || apiKey.Group == nil || apiKey.Group.Platform != service.PlatformOpenAI {
		h.errorResponse(c, http.StatusForbidden, "permission_error", "An OpenAI-compatible group is required")
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "User context not found")
		return
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil || len(body) == 0 {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Invalid video request")
		return
	}
	var input minimax.CreateVideoRequest
	if err := json.Unmarshal(body, &input); err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Invalid video request")
		return
	}
	if len(input.Content) == 0 {
		var shortcut struct {
			Prompt string `json:"prompt"`
		}
		_ = json.Unmarshal(body, &shortcut)
		if strings.TrimSpace(shortcut.Prompt) != "" {
			input.Content = []minimax.VideoContent{{Type: "text", Text: shortcut.Prompt}}
			if input.Resolution == "" {
				input.Resolution = "768P"
			}
			if input.Duration == 0 {
				input.Duration = 5
			}
			if input.Ratio == "" {
				input.Ratio = "16:9"
			}
		}
	}
	if err := input.Validate(); err != nil || !miniMaxPublicContentAllowed(input.Content) {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "MiniMax-H3 requires a text prompt and supports up to two first/last-frame images at 768P or 2K")
		return
	}
	if !service.GroupAllowsImageGeneration(apiKey.Group) {
		h.errorResponse(c, http.StatusForbidden, "permission_error", service.ImageGenerationPermissionMessage())
		return
	}
	reqLog := requestLogger(c, "handler.openai_gateway.minimax_video", zap.Int64("user_id", subject.UserID), zap.Int64("api_key_id", apiKey.ID))
	decision := h.checkSecurityAudit(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIImages, input.Model, body)
	if decision != nil && !decision.AllowNextStage {
		h.openAISecurityAuditError(c, decision)
		return
	}
	subscription, _ := middleware.GetSubscriptionFromContext(c)
	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(c.Request.Context(), apiKey)); err != nil {
		status, code, message, _ := billingErrorDetails(err)
		h.errorResponse(c, status, code, message)
		return
	}
	setOpsRequestContext(c, input.Model, false)
	setOpsEndpointContext(c, "/v2/video_generation", int16(service.RequestTypeSync))
	ctx := c.Request.Context()
	failed := make(map[int64]struct{})
	var account *service.Account
	var baseURL string
	var selection *service.AccountSelectionResult
	for {
		var err error
		selection, _, err = h.gatewayService.SelectAccountWithSchedulerForCapability(ctx, apiKey.GroupID, "", "", input.Model, failed, service.OpenAIUpstreamTransportHTTPSSE, "", false, false, false, service.PlatformOpenAI)
		if err != nil || selection == nil || selection.Account == nil {
			h.errorResponse(c, http.StatusServiceUnavailable, "no_available_account", "No MiniMax-H3 account is available in this group")
			return
		}
		account = selection.Account
		if baseURL, ok = service.MiniMaxVideoBaseURL(account); ok {
			break
		}
		if selection.ReleaseFunc != nil {
			selection.ReleaseFunc()
		}
		failed[account.ID] = struct{}{}
	}
	if selection.ReleaseFunc != nil {
		defer selection.ReleaseFunc()
	}
	client, err := minimax.NewVideoClient(baseURL, &http.Client{Timeout: 45 * time.Second})
	if err != nil {
		h.errorResponse(c, http.StatusBadGateway, "upstream_error", "MiniMax account URL is invalid")
		return
	}
	start := time.Now()
	created, err := client.Create(ctx, account.GetCredential("api_key"), input)
	if err != nil {
		h.errorResponse(c, http.StatusBadGateway, "upstream_error", "MiniMax video task creation failed")
		return
	}
	if err := h.gatewayService.BindMiniMaxVideoTaskAccount(ctx, apiKey.GroupID, created.TaskID, subject.UserID, apiKey.ID, account.ID); err != nil {
		logger.L().Error("minimax_video.bind_task_failed", zap.String("task_id", created.TaskID), zap.Error(err))
		c.JSON(http.StatusServiceUnavailable, gin.H{"task_id": created.TaskID, "error": "Video task created but status lookup is temporarily unavailable"})
		return
	}
	setOpsSelectedAccount(c, account.ID, account.Platform)
	result := &service.OpenAIForwardResult{
		RequestID: created.TaskID, ResponseID: created.TaskID, Model: input.Model,
		UpstreamModel: input.Model, UpstreamEndpoint: "/v2/video_generation",
		VideoCount: 1, VideoResolution: input.Resolution, VideoDurationSeconds: input.Duration,
		Duration: time.Since(start),
	}
	userAgent, clientIP := c.GetHeader("User-Agent"), ip.GetClientIP(c)
	quotaPlatform := service.QuotaPlatform(ctx, apiKey)
	h.submitOpenAIUsageRecordTask(ctx, result, func(billCtx context.Context) {
		if err := h.gatewayService.RecordUsage(billCtx, &service.OpenAIRecordUsageInput{
			Result: result, APIKey: apiKey, User: apiKey.User, Account: account,
			Subscription: subscription, InboundEndpoint: "/v2/video_generation",
			UpstreamEndpoint: "/v2/video_generation", UserAgent: userAgent,
			IPAddress: clientIP, RequestPayloadHash: service.HashUsageRequestPayload([]byte(created.TaskID)),
			APIKeyService: h.apiKeyService, QuotaPlatform: quotaPlatform,
		}); err != nil {
			logger.L().Error("minimax_video.record_usage_failed", zap.String("task_id", created.TaskID), zap.Error(err))
		}
	})
	c.JSON(http.StatusOK, created)
}

func miniMaxPublicContentAllowed(items []minimax.VideoContent) bool {
	images := 0
	roles := make(map[string]bool)
	for _, item := range items {
		switch item.Type {
		case "text":
		case "image_url":
			images++
			role := item.Role
			if role == "" {
				role = "first_frame"
			}
			if (role != "first_frame" && role != "last_frame") || roles[role] {
				return false
			}
			roles[role] = true
		default:
			return false
		}
	}
	return images <= 2
}

func (h *OpenAIGatewayHandler) MiniMaxVideoStatus(c *gin.Context) {
	apiKey, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "User context not found")
		return
	}
	taskID := c.Param("task_id")
	if taskID == "" {
		taskID = c.Param("request_id")
	}
	account, err := h.gatewayService.ResolveMiniMaxVideoTaskAccount(c.Request.Context(), apiKey.GroupID, taskID, subject.UserID, apiKey.ID)
	if err != nil {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Video task not found")
		return
	}
	baseURL, ok := service.MiniMaxVideoBaseURL(account)
	if !ok {
		h.errorResponse(c, http.StatusBadGateway, "upstream_error", "MiniMax account unavailable")
		return
	}
	client, err := minimax.NewVideoClient(baseURL, &http.Client{Timeout: 30 * time.Second})
	if err != nil {
		h.errorResponse(c, http.StatusBadGateway, "upstream_error", "MiniMax account URL is invalid")
		return
	}
	result, err := client.Query(c.Request.Context(), account.GetCredential("api_key"), taskID)
	if err != nil {
		h.errorResponse(c, http.StatusBadGateway, "upstream_error", "MiniMax task query failed")
		return
	}
	c.JSON(http.StatusOK, result)
}
