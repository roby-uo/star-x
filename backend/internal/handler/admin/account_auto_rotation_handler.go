package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type updateAccountAutoRotationRequest struct {
	Enabled bool `json:"enabled"`
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

// UpdateAutoRotation enables or disables the five-hour account rotation.
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
	status, err := h.autoRotation.SetEnabled(c.Request.Context(), req.Enabled)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}
