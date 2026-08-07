package controller

import (
	"blog-system/service"
	"blog-system/utils"

	"github.com/gin-gonic/gin"
)

// DashboardController 仪表盘统计（编辑+）
type DashboardController struct {
	service *service.DashboardService
}

func NewDashboardController(service *service.DashboardService) *DashboardController {
	return &DashboardController{service: service}
}

// GetDashboard 仪表盘整包数据（编辑+）
// GET /api/admin/dashboard
func (c *DashboardController) GetDashboard(ctx *gin.Context) {
	data, err := c.service.GetDashboard()
	if err != nil {
		utils.Error(ctx, "获取统计数据失败")
		return
	}
	utils.Success(ctx, data)
}
