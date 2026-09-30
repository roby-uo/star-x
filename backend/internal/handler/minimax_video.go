package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/minimax"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// MiniMaxVideoQuote is authenticated with the same API key as generation, so
// user-specific rates and group permissions cannot drift from the displayed price.
func (h *OpenAIGatewayHandler) MiniMaxVideoQuote(c *gin.Context) {
	key, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || key == nil || key.Group == nil || key.Group.Platform != service.PlatformOpenAI {
		h.errorResponse(c, http.StatusForbidden, "permission_error", "需要有效的 OpenAI 兼容分组密钥")
		return
	}
	var input minimax.CreateVideoRequest
	if c.ShouldBindJSON(&input) != nil || input.Validate() != nil || !miniMaxPublicContentAllowed(input.Content) {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "视频规格或输入无效")
		return
	}
	quote, err := h.gatewayService.QuoteMiniMaxVideo(c.Request.Context(), key, input)
	if err != nil {
		h.errorResponse(c, http.StatusForbidden, "permission_error", err.Error())
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, quote)
}

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
	if err := input.Validate(); err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	if !miniMaxPublicContentAllowed(input.Content) {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "MiniMax video generation supports a text prompt and up to two first/last-frame images")
		return
	}
	if h.mediaTasks != nil && c.GetHeader("Idempotency-Key") != "" {
		task, lookupErr := h.mediaTasks.ByIdempotency(c.Request.Context(), apiKey.ID, c.GetHeader("Idempotency-Key"))
		if lookupErr == nil {
			payload, _ := json.Marshal(input)
			if task.RequestHash != service.HashUsageRequestPayload(payload) {
				h.errorResponse(c, http.StatusConflict, "idempotency_conflict", "幂等键已用于不同请求")
				return
			}
			c.JSON(http.StatusOK, gin.H{"id": task.ID, "task_id": task.UpstreamTaskID, "state": task.State, "quote": task.Quote})
			return
		}
	}
	quote, err := h.gatewayService.QuoteMiniMaxVideo(c.Request.Context(), apiKey, input)
	if err != nil {
		h.errorResponse(c, http.StatusForbidden, "permission_error", err.Error())
		return
	}
	if version := c.GetHeader("X-Video-Quote"); version != "" && version != quote.Version {
		h.errorResponse(c, http.StatusConflict, "quote_changed", "价格或开放规格已变化，请重新获取报价")
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
			h.errorResponse(c, http.StatusServiceUnavailable, "no_available_account", "No account for the requested MiniMax video model is available in this group")
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
	if h.mediaTasks == nil {
		h.errorResponse(c, http.StatusServiceUnavailable, "storage_unavailable", "视频任务存储尚未就绪")
		return
	}
	idempotency := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if len(idempotency) > 128 {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Idempotency-Key 太长")
		return
	}
	if idempotency == "" {
		idempotency = uuid.NewString()
	}
	task, fresh, err := h.mediaTasks.Begin(ctx, apiKey, account.ID, input, quote, idempotency, subscription)
	if err != nil {
		h.errorResponse(c, http.StatusConflict, "video_task_error", err.Error())
		return
	}
	if !fresh {
		c.JSON(http.StatusOK, gin.H{"id": task.ID, "task_id": task.UpstreamTaskID, "state": task.State, "quote": task.Quote})
		return
	}
	created, err := client.Create(ctx, account.GetCredential("api_key"), input)
	// Give persistence its own full timeout after the upstream request completes.
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 60*time.Second)
	defer cancel()
	if err != nil {
		var upstream *minimax.HTTPError
		if errors.As(err, &upstream) && upstream.Status >= 400 && upstream.Status < 500 && upstream.Status != 408 {
			_ = h.mediaTasks.Reject(persistCtx, task.ID)
		} else {
			_ = h.mediaTasks.Uncertain(persistCtx, task.ID)
		}
		c.JSON(http.StatusBadGateway, gin.H{"id": task.ID, "error": gin.H{"message": "视频提交未完成，请在任务记录中核对状态，勿重复提交"}})
		return
	}
	if err = h.mediaTasks.Accepted(persistCtx, task.ID, created.TaskID); err != nil {
		logger.L().Error("minimax_video.persist_task_failed", zap.String("id", task.ID), zap.String("task_id", created.TaskID), zap.Error(err))
		c.JSON(http.StatusServiceUnavailable, gin.H{"id": task.ID, "task_id": created.TaskID, "error": gin.H{"message": "上游已受理，任务记录待核对；请保留任务编号，不要重新提交"}})
		return
	}
	task.UpstreamTaskID = created.TaskID
	if err = h.mediaTasks.Settle(persistCtx, task); err != nil {
		logger.L().Error("minimax_video.settlement_pending", zap.String("id", task.ID), zap.Error(err))
	}
	setOpsSelectedAccount(c, account.ID, account.Platform)
	c.JSON(http.StatusOK, gin.H{"id": task.ID, "task_id": created.TaskID, "quote": quote, "state": "queued"})
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
	if h.mediaTasks != nil {
		task, err := h.mediaTasks.ByUpstream(c.Request.Context(), taskID, subject.UserID, apiKey.ID)
		if err == nil {
			if err = h.mediaTasks.Refresh(c.Request.Context(), task); err != nil {
				h.errorResponse(c, http.StatusBadGateway, "upstream_error", "视频状态刷新失败，请稍后重试")
				return
			}
			task, err = h.mediaTasks.Get(c.Request.Context(), task.ID, subject.UserID)
			if err != nil {
				h.errorResponse(c, http.StatusServiceUnavailable, "storage_error", "任务读取失败")
				return
			}
			if task.Result != nil {
				c.JSON(http.StatusOK, task.Result)
			} else {
				c.JSON(http.StatusOK, gin.H{"task": gin.H{"id": task.UpstreamTaskID, "status": task.State}})
			}
			return
		}
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
