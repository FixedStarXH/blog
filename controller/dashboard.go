package controller

import (
	"strconv"

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

// GetViewsTrend 浏览量趋势（编辑+）
// GET /api/admin/dashboard/views-trend?days=30
func (c *DashboardController) GetViewsTrend(ctx *gin.Context) {
	days, _ := strconv.Atoi(ctx.DefaultQuery("days", "30"))
	data, err := c.service.ViewTrend(days)
	if err != nil {
		utils.Error(ctx, "获取浏览量趋势失败")
		return
	}
	utils.Success(ctx, data)
}
