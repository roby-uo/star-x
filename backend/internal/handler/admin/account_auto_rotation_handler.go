package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type updateAccountAutoRotationRequest struct {
	Enabled         *bool  `json:"enabled"`
	IntervalSeconds *int64 `json:"interval_seconds" binding:"omitempty,oneof=1800 3600 5400 7200 9000 10800 12600 14400"`
}

// GetAutoRotation returns the persisted rotation state and performs a safe
// reconciliation before responding.
func (h *AccountHandler) GetAutoRotation(c *gin.Context) {
	if h.autoRotation == nil {
		response.Error(c, 503, "Automatic account rotation is unavailable")
		return
	}
	status, err := h.autoRotation.Status(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}

// UpdateAutoRotation enables/disables rotation or changes its interval.
func (h *AccountHandler) UpdateAutoRotation(c *gin.Context) {
	if h.autoRotation == nil {
		response.Error(c, 503, "Automatic account rotation is unavailable")
		return
	}
	var req updateAccountAutoRotationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.Enabled == nil && req.IntervalSeconds == nil {
		response.BadRequest(c, "At least one automatic rotation setting is required")
		return
	}
	status, err := h.autoRotation.Update(c.Request.Context(), req.Enabled, req.IntervalSeconds)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}
