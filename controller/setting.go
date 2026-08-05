package controller

import (
	"blog-system/service"
	"blog-system/utils"

	"github.com/gin-gonic/gin"
)

type SettingController struct {
	service *service.SettingService
}

func NewSettingController(service *service.SettingService) *SettingController {
	return &SettingController{service: service}
}

// GetSiteSettings 站点信息
// GET /api/settings
func (c *SettingController) GetSiteSettings(ctx *gin.Context) {
	settings, err := c.service.GetSiteSettings()
	if err != nil {
		utils.Error(ctx, "获取站点信息失败")
		return
	}
	utils.Success(ctx, settings)
}
