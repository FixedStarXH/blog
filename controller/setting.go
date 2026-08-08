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

// GetSiteSettings 站点信息（老接口，Apifox 契约保留）
// GET /api/settings
func (c *SettingController) GetSiteSettings(ctx *gin.Context) {
	settings, err := c.service.GetSiteSettings()
	if err != nil {
		utils.Error(ctx, "获取站点信息失败")
		return
	}
	utils.Success(ctx, settings)
}

// GetDailyQuote 每日一言（老接口，Apifox 契约保留）
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

// ------------------------------------------------------------
// 前台站点信息（前端 home.js / common.js / about.html 契约）
// ------------------------------------------------------------

// GetSiteInfo 前台站点信息（含统计数字）
// GET /api/site
func (c *SettingController) GetSiteInfo(ctx *gin.Context) {
	info, err := c.service.GetSiteInfo()
	if err != nil {
		utils.Error(ctx, "获取站点信息失败")
		return
	}
	utils.Success(ctx, info)
}

// GetRandomQuote 前台每日一言（结构化 {content, author}）
// GET /api/quote/random
func (c *SettingController) GetRandomQuote(ctx *gin.Context) {
	quote, err := c.service.GetRandomQuote()
	if err != nil {
		utils.Error(ctx, "获取每日一言失败")
		return
	}
	utils.Success(ctx, quote)
}

// ------------------------------------------------------------
// 后台设置（admin/settings.html 契约）
// ------------------------------------------------------------

// updateSettingsRequest 站点设置保存体（与前端表单字段一一对应）
// 各字段加 max 限制，防止超长文本写入 settings.value（varchar(255)）报错
type updateSettingsRequest struct {
	SiteTitle       string              `json:"site_title" binding:"max=255"`
	SiteSubtitle    string              `json:"site_subtitle" binding:"max=255"`
	SiteDescription string              `json:"site_description" binding:"max=255"`
	SiteLogo        string              `json:"site_logo" binding:"max=255"`
	SiteBeian       string              `json:"site_beian" binding:"max=255"`
	SocialGithub    string              `json:"social_github" binding:"max=255"`
	SocialEmail     string              `json:"social_email" binding:"max=255"`
	Quotes          []service.QuoteItem `json:"quotes"` // 名言池（结构化和前端编辑框来回转换）
}

// GetAdminSettings 后台站点设置全量（编辑+）
// GET /api/admin/settings
func (c *SettingController) GetAdminSettings(ctx *gin.Context) {
	settings, err := c.service.GetAdminSettings()
	if err != nil {
		utils.Error(ctx, "获取站点设置失败")
		return
	}
	utils.Success(ctx, settings)
}

// UpdateSettings 更新站点设置（编辑+）
// PUT /api/admin/settings  Body: {"site_title":"...","quotes":[{"content":"...","author":"..."}]}
func (c *SettingController) UpdateSettings(ctx *gin.Context) {
	var req updateSettingsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.Fail(ctx, "参数错误："+err.Error())
		return
	}
	input := &service.SiteSettingsInput{
		SiteTitle:       req.SiteTitle,
		SiteSubtitle:    req.SiteSubtitle,
		SiteDescription: req.SiteDescription,
		SiteLogo:        req.SiteLogo,
		SiteBeian:       req.SiteBeian,
		SocialGithub:    req.SocialGithub,
		SocialEmail:     req.SocialEmail,
		Quotes:          req.Quotes,
	}
	if err := c.service.UpdateSettings(input); err != nil {
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, nil)
}
