package handler

import (
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *AvailableChannelHandler) GetKeyModelAccess(c *gin.Context)  { h.keyModelAccess(c, false) }
func (h *AvailableChannelHandler) SaveKeyModelAccess(c *gin.Context) { h.keyModelAccess(c, true) }
func (h *AvailableChannelHandler) keyModelAccess(c *gin.Context, save bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	key, err := h.apiKeyService.GetByID(c.Request.Context(), id)
	if err != nil || key.UserID != subject.UserID {
		response.NotFound(c, "密钥不存在")
		return
	}
	if save {
		var rules service.APIKeyModelAccess
		if c.ShouldBindJSON(&rules) != nil {
			response.BadRequest(c, "权限格式无效")
			return
		}
		if err = h.mediaTasks.SaveKeyAccess(c.Request.Context(), id, rules); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Success(c, rules)
		return
	}
	rules, err := h.mediaTasks.KeyAccess(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, rules)
}

func (h *AvailableChannelHandler) CheckModelAccess(c *gin.Context) {
	key, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || h == nil || h.mediaTasks == nil {
		c.Next()
		return
	}
	if middleware.ModelAccessCheck(h.mediaTasks)(c, key) {
		c.Next()
	}
}

func (h *AvailableChannelHandler) ListMediaTasks(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	h.listMediaTasks(c, subject.UserID)
}
func (h *AvailableChannelHandler) ListAdminMediaTasks(c *gin.Context) { h.listMediaTasks(c, 0) }
func (h *AvailableChannelHandler) listMediaTasks(c *gin.Context, userID int64) {
	offset, _ := strconv.Atoi(c.Query("offset"))
	if offset < 0 {
		offset = 0
	}
	if offset > 100000 {
		offset = 100000
	}
	tasks, err := h.mediaTasks.List(c.Request.Context(), userID, offset)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, tasks)
}
func (h *AvailableChannelHandler) RefreshMediaTask(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	h.refreshMediaTask(c, subject.UserID)
}
func (h *AvailableChannelHandler) RefreshAdminMediaTask(c *gin.Context) { h.refreshMediaTask(c, 0) }
func (h *AvailableChannelHandler) refreshMediaTask(c *gin.Context, userID int64) {
	task, err := h.mediaTasks.Get(c.Request.Context(), c.Param("id"), userID)
	if err != nil {
		response.NotFound(c, "任务不存在")
		return
	}
	if err = h.mediaTasks.Refresh(c.Request.Context(), task); err != nil {
		response.Error(c, http.StatusBadGateway, "状态暂时无法刷新，请稍后重试")
		return
	}
	task, err = h.mediaTasks.Get(c.Request.Context(), task.ID, userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, task)
}

func (h *AvailableChannelHandler) ReleaseUncertainMediaTask(c *gin.Context) {
	var input struct {
		Confirmed bool `json:"confirmed_not_accepted"`
	}
	if c.ShouldBindJSON(&input) != nil || !input.Confirmed {
		response.BadRequest(c, "必须先在供应商后台确认任务未受理且未收费")
		return
	}
	task, err := h.mediaTasks.Get(c.Request.Context(), c.Param("id"), 0)
	if err != nil {
		response.NotFound(c, "任务不存在")
		return
	}
	if task.State != "uncertain" || task.UpstreamTaskID != "" {
		response.BadRequest(c, "仅可释放尚未确认受理的任务")
		return
	}
	if err = h.mediaTasks.Reject(c.Request.Context(), task.ID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"released": true})
}

func (h *AvailableChannelHandler) RefundMediaTask(c *gin.Context) {
	var input struct {
		Confirmed bool `json:"confirmed"`
	}
	if c.ShouldBindJSON(&input) != nil || !input.Confirmed {
		response.BadRequest(c, "请确认退还任务费用")
		return
	}
	if err := h.mediaTasks.Refund(c.Request.Context(), c.Param("id")); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"refunded": true})
}
