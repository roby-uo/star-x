package handler

import (
	"net/http"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

const keyBillingInfoSchemaVersion = 2

// The compatibility endpoint advertises visibility without disclosing a customer's
// pricing policy. Billing and multiplier resolution continue on the server.
type keyBillingInfoResponse struct {
	Object            string    `json:"object"`
	SchemaVersion     int       `json:"schema_version"`
	BillingScope      string    `json:"billing_scope"`
	PricingVisibility string    `json:"pricing_visibility"`
	ObservedAt        time.Time `json:"observed_at"`
}

// KeyBillingInfo keeps the existing authenticated route for integrations.
// GET /v1/sub2api/billing
func (h *GatewayHandler) KeyBillingInfo(c *gin.Context) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	if h.cfg != nil && h.cfg.RunMode == config.RunModeSimple {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Billing information is not supported in simple mode")
		return
	}
	if apiKey.GroupID == nil {
		h.errorResponse(c, http.StatusForbidden, "permission_error", "API key is not assigned to a group")
		return
	}
	if apiKey.Group == nil {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "Billing information is unavailable")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, keyBillingInfoResponse{
		Object: "sub2api.key_billing", SchemaVersion: keyBillingInfoSchemaVersion,
		BillingScope: "token", PricingVisibility: "admin_only", ObservedAt: timezone.Now().UTC(),
	})
}
