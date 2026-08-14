package controller

import (
	"strconv"

	"blog-system/service"
	"blog-system/utils"

	"github.com/gin-gonic/gin"
)

// AuditController 操作审计查询（编辑+ 可看，管理员可清空）
type AuditController struct {
	service *service.AuditService
}

func NewAuditController(service *service.AuditService) *AuditController {
	return &AuditController{service: service}
}

// GetAuditLogs 分页查询审计日志（编辑+）
// GET /api/admin/audit-logs?page=1&pageSize=20&action=删除文章
func (c *AuditController) GetAuditLogs(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "20"))
	action := ctx.Query("action")

	list, total, err := c.service.List(page, pageSize, action)
	if err != nil {
		utils.Error(ctx, "获取审计日志失败")
		return
	}
	utils.Success(ctx, gin.H{"list": list, "total": total})
}

// ClearAuditLogs 清空审计日志（仅管理员）
// DELETE /api/admin/audit-logs
func (c *AuditController) ClearAuditLogs(ctx *gin.Context) {
	if err := c.service.Clear(); err != nil {
		utils.Error(ctx, "清空审计日志失败")
		return
	}
	utils.Success(ctx, nil)
}
