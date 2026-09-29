package middleware

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// Tests and non-production callers retain the original constructor. The runtime
// provider always installs model access enforcement after authentication.
func NewAPIKeyAuthMiddlewareWithModelAccess(keys *service.APIKeyService, subscriptions *service.SubscriptionService, cfg *config.Config, tasks *service.MediaTaskService) APIKeyAuthMiddleware {
	return APIKeyAuthMiddleware(apiKeyAuthWithSubscription(keys, subscriptions, cfg, ModelAccessCheck(tasks)))
}

func ModelAccessCheck(tasks *service.MediaTaskService) func(*gin.Context, *service.APIKey) bool {
	return func(c *gin.Context, key *service.APIKey) bool {
		websocket := strings.EqualFold(c.GetHeader("Upgrade"), "websocket")
		if c.Request.Method != http.MethodPost && !websocket {
			return true
		}
		if id := c.GetHeader("Idempotency-Key"); id != "" && (c.Request.URL.Path == "/v2/video_generation" || c.Request.URL.Path == "/v1/videos/generations" || c.Request.URL.Path == "/videos/generations") {
			if task, err := tasks.ByIdempotency(c.Request.Context(), key.ID, id); err == nil && task.UserID == key.UserID {
				c.Set("video_task_retry", true)
				return true
			}
		}
		rules, err := tasks.KeyAccess(c.Request.Context(), key.ID)
		if err != nil {
			AbortWithError(c, http.StatusServiceUnavailable, "MODEL_ACCESS_UNAVAILABLE", "模型权限暂时无法读取")
			return false
		}
		if websocket {
			if rules.RestrictModels {
				AbortWithError(c, http.StatusForbidden, "MODEL_ACCESS_DENIED", "启用模型白名单的密钥请使用 HTTP 接口")
				return false
			}
			return true
		}
		if strings.HasSuffix(c.Request.URL.Path, "/cancel") {
			return true
		}
		if !rules.RestrictModels && rules.VideoMaxDuration == 0 && len(rules.VideoResolutions) == 0 {
			return true
		}
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			AbortWithError(c, http.StatusBadRequest, "INVALID_REQUEST", "请求内容读取失败")
			return false
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		model := gjson.GetBytes(body, "model").String()
		if strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data") {
			_, params, e := mime.ParseMediaType(c.GetHeader("Content-Type"))
			if e == nil {
				reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
				for {
					part, e := reader.NextPart()
					if e != nil {
						break
					}
					if part.FormName() == "model" {
						value, _ := io.ReadAll(io.LimitReader(part, 512))
						model = string(value)
						_ = part.Close()
						break
					}
					_ = part.Close()
				}
			}
		}
		// Native Gemini embeds the model in the path instead of the JSON body.
		if model == "" {
			if _, tail, ok := strings.Cut(c.Request.URL.Path, "/models/"); ok {
				model = strings.Split(tail, ":")[0]
			}
		}
		resolution := gjson.GetBytes(body, "resolution").String()
		duration := int(gjson.GetBytes(body, "duration").Int())
		if model == "MiniMax-H3" {
			if duration == 0 {
				duration = 5
			}
			if resolution == "" {
				resolution = "768P"
			}
		}
		if err = rules.Validate(model, resolution, duration); err != nil {
			AbortWithError(c, http.StatusForbidden, "MODEL_ACCESS_DENIED", err.Error())
			return false
		}
		return true
	}
}
