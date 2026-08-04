package controller

import (
	"blog-system/service"
	"blog-system/utils"

	"github.com/gin-gonic/gin"
)

// TagController 标签控制器（参数 + 响应）
type TagController struct {
	service *service.TagService
}

func NewTagController(service *service.TagService) *TagController {
	return &TagController{service: service}
}

// GetTagList 标签列表
// GET /api/tags（公开接口）
func (c *TagController) GetTagList(ctx *gin.Context) {
	tags, err := c.service.GetAllTags()
	if err != nil {
		utils.Error(ctx, "获取标签列表失败")
		return
	}
	utils.Success(ctx, tags)
}
