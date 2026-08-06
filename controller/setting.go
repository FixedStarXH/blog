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

// GetDailyQuote 每日一言
// GET /api/quote
func (c *SettingController) GetDailyQuote(ctx *gin.Context) {
	quote, err := c.service.GetDailyQuote()
	if err != nil {
		utils.Error(ctx, "获取每日一言失败")
		return
	}
	// gin.H 包一层，前端取 data.quote
	utils.Success(ctx, gin.H{"quote": quote})
}

// UpdateSettings 更新站点设置（编辑+）
// PUT /api/admin/settings   Body: {"site_title":"新标题","daily_quotes":"第1条\n第2条"}
func (c *SettingController) UpdateSettings(ctx *gin.Context) {
	var kv map[string]string
	if err := ctx.ShouldBindJSON(&kv); err != nil {
		utils.Fail(ctx, "参数错误："+err.Error())
		return
	}
	if err := c.service.UpdateSettings(kv); err != nil {
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, nil)
}
