package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
)

type modelEndpointResolver interface {
	ResolveModelEndpointCapability(context.Context, *int64, string, string) (service.ModelEndpointCapability, error)
}

type discoveryModelResolver func(string) (service.ModelEndpointCapability, error)

func modelCapabilityForKey(ctx context.Context, resolver modelEndpointResolver, key *service.APIKey, model string) (service.ModelEndpointCapability, error) {
	var groupID *int64
	platform := ""
	if key != nil {
		groupID = key.GroupID
		if key.Group != nil {
			platform = key.Group.Platform
			if groupID == nil {
				groupID = &key.Group.ID
			}
		}
	}
	return resolver.ResolveModelEndpointCapability(ctx, groupID, platform, model)
}

// Only the selected primary model is checked. LLM image input and generation
// tools remain valid; they are capabilities of a reasoning model request.
func rejectGenerationModelOnTextEndpoint(c *gin.Context, resolver modelEndpointResolver, key *service.APIKey, model string, anthropic bool) bool {
	capability, err := modelCapabilityForKey(c.Request.Context(), resolver, key, model)
	if err != nil {
		writeModelEndpointError(c, http.StatusServiceUnavailable, "model_capability_unavailable", "模型接入能力暂时无法读取，请稍后重试", nil, anthropic)
		return true
	}
	if !capability.IsGenerationModel() {
		return false
	}
	message := modelEndpointMismatchMessage(model, capability)
	service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalPolicyDenied)
	writeModelEndpointError(c, http.StatusBadRequest, "model_endpoint_mismatch", message, capability.SupportedEndpoints, anthropic)
	return true
}

func rejectSelectedAccountGenerationModel(c *gin.Context, key *service.APIKey, selection *service.AccountSelectionResult, requestedModel, mappedModel, messagesDispatchModel string, anthropic bool) bool {
	capability := selectedAccountModelCapability(key, selection.Account, mappedModel, messagesDispatchModel)
	if !capability.IsGenerationModel() {
		return false
	}
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
		selection.ReleaseFunc = nil
	}
	service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalPolicyDenied)
	writeModelEndpointError(c, http.StatusBadRequest, "model_endpoint_mismatch", modelEndpointMismatchMessage(requestedModel, capability), capability.SupportedEndpoints, anthropic)
	return true
}

func selectedAccountModelCapability(key *service.APIKey, account *service.Account, model, messagesDispatchModel string) service.ModelEndpointCapability {
	platform := ""
	if key != nil && key.Group != nil {
		platform = key.Group.Platform
	} else if account != nil {
		platform = account.Platform
	}
	return service.AccountModelEndpointCapability(account, platform, model, messagesDispatchModel)
}

func modelEndpointMismatchMessage(model string, capability service.ModelEndpointCapability) string {
	kind := "图像生成"
	if capability.ModelType == "video" {
		kind = "视频生成"
	}
	message := fmt.Sprintf("%s 是%s模型，不能作为 Agent 的主推理模型调用对话接口。", model, kind)
	if len(capability.SupportedEndpoints) > 0 {
		return message + "请使用 " + capability.SupportedEndpoints[0] + "，或在模型目录查看接入示例。"
	}
	return message + "请在模型目录确认专用生成接口的适配状态。"
}

func writeModelEndpointMismatchWSError(ctx context.Context, connection *coderws.Conn, model string, capability service.ModelEndpointCapability) {
	payload, _ := json.Marshal(gin.H{"type": "error", "error": gin.H{"type": "invalid_request_error", "code": "model_endpoint_mismatch", "message": modelEndpointMismatchMessage(model, capability), "param": "model", "supported_endpoints": capability.SupportedEndpoints}})
	_ = connection.Write(ctx, coderws.MessageText, payload)
}

func writeModelEndpointError(c *gin.Context, status int, code, message string, endpoints []string, anthropic bool) {
	// Cold account aliases can be discovered after a concurrency wait has
	// flushed a heartbeat. Preserve the client protocol's terminal event then.
	service.StopOpenAICompactSSEKeepaliveCommitted(c)
	errorBody := gin.H{"type": "invalid_request_error", "code": code, "message": message, "param": "model"}
	if status >= 500 {
		errorBody["type"] = "api_error"
	}
	if endpoints != nil {
		errorBody["supported_endpoints"] = endpoints
	}
	body := gin.H{"error": errorBody}
	if anthropic {
		body["type"] = "error"
	}
	if c.Writer.Written() && strings.Contains(c.Writer.Header().Get("Content-Type"), "text/event-stream") {
		service.MarkOpsStreamError(c, "invalid_request_error", message, status)
		if inboundIsResponses(c) {
			writeResponsesFailedSSE(c, code, message)
			return
		}
		data, _ := json.Marshal(body)
		_, _ = fmt.Fprintf(c.Writer, "event: error\ndata: %s\n\n", data)
		c.Writer.Flush()
		return
	}
	c.JSON(status, body)
}

// Ordinary discovery preserves every model. Explicit Agent discovery excludes
// known generation models while keeping unknown custom upstream model IDs.
func writeGatewayModelsPayload(c *gin.Context, payload gin.H, resolvers ...discoveryModelResolver) {
	if len(resolvers) == 0 {
		c.JSON(http.StatusOK, payload)
		return
	}
	data, err := json.Marshal(payload["data"])
	var models []map[string]any
	if err == nil {
		err = json.Unmarshal(data, &models)
	}
	if err != nil {
		writeModelEndpointError(c, http.StatusInternalServerError, "model_discovery_unavailable", "模型列表暂时无法读取", nil, false)
		return
	}
	visible := make([]map[string]any, 0, len(models))
	for _, model := range models {
		id, _ := model["id"].(string)
		capability, resolveErr := resolvers[0](id)
		if resolveErr != nil {
			writeModelEndpointError(c, http.StatusServiceUnavailable, "model_capability_unavailable", "模型接入能力暂时无法读取", nil, false)
			return
		}
		if strings.EqualFold(c.Query("purpose"), "agent") && capability.IsGenerationModel() {
			continue
		}
		model["model_type"] = capability.ModelType
		model["supported_endpoints"] = capability.SupportedEndpoints
		visible = append(visible, model)
	}
	payload["data"] = visible
	c.JSON(http.StatusOK, payload)
}

// Preserve every entry field (including hosted tools and vision metadata) and
// the rest of the upstream envelope. Only known generation model slugs vanish.
func filterCodexAgentModels(body []byte, resolver discoveryModelResolver) ([]byte, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	var models []json.RawMessage
	if err := json.Unmarshal(envelope["models"], &models); err != nil {
		return nil, err
	}
	visible := make([]json.RawMessage, 0, len(models))
	changed := false
	for _, model := range models {
		var entry struct {
			Slug string `json:"slug"`
			ID   string `json:"id"`
		}
		if err := json.Unmarshal(model, &entry); err != nil {
			visible = append(visible, model)
			continue
		}
		id := entry.Slug
		if id == "" {
			id = entry.ID
		}
		capability, err := resolver(id)
		if err != nil {
			return nil, err
		}
		if capability.IsGenerationModel() {
			changed = true
			continue
		}
		visible = append(visible, model)
	}
	if !changed {
		return body, nil
	}
	data, err := json.Marshal(visible)
	if err != nil {
		return nil, err
	}
	envelope["models"] = data
	return json.Marshal(envelope)
}
