package admin

import (
	"encoding/json"
	"io"
	"net/http"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type BillingManagementHandler struct {
	service *service.BillingManagementService
}

func NewBillingManagementHandler(s *service.BillingManagementService) *BillingManagementHandler {
	return &BillingManagementHandler{service: s}
}

func (h *BillingManagementHandler) GetConfig(c *gin.Context) {
	cfg, err := h.service.GetConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

func (h *BillingManagementHandler) Catalog(c *gin.Context) {
	items, err := h.service.Catalog(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

// Unknown properties are rejected to prevent credentials or unrelated wallet
// settings from accidentally being persisted in the administrator cost book.
func decodeBillingManagementBody(c *gin.Context, target any) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2*1024*1024)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return infraerrors.BadRequest("INVALID_BILLING_BODY", "费率配置或算费用量格式无效")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return infraerrors.BadRequest("INVALID_BILLING_BODY", "请求只能包含一个JSON对象")
	}
	return nil
}

func (h *BillingManagementHandler) UpdateConfig(c *gin.Context) {
	req := service.DefaultBillingManagementConfig()
	if err := decodeBillingManagementBody(c, &req); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	cfg, err := h.service.UpdateConfig(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

func (h *BillingManagementHandler) Preview(c *gin.Context) {
	var req service.BillingPreviewInput
	if err := decodeBillingManagementBody(c, &req); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := h.service.Preview(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
