package controller

import (
	"blog-system/service"
	"blog-system/utils"
	"strconv"

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

// 新增/修改标签：name 必填，最长 50 字
type createTagRequest struct {
	Name string `json:"name" binding:"required,max=50"`
}
type updateTagRequest struct {
	Name string `json:"name" binding:"required,max=50"`
}

// CreateTag 新增标签
// POST /api/admin/tags
func (c *TagController) CreateTag(ctx *gin.Context) {
	var req createTagRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.Fail(ctx, "参数错误："+err.Error())
		return
	}
	if err := c.service.CreateTag(req.Name); err != nil {
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, nil)
}

// UpdateTag 修改标签
// PUT /api/admin/tags/:id
func (c *TagController) UpdateTag(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(ctx, "无效的标签ID")
		return
	}
	var req updateTagRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.Fail(ctx, "参数错误："+err.Error())
		return
	}
	if err := c.service.UpdateTag(uint(id), req.Name); err != nil {
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, nil)
}

// DeleteTag 删除标签
// DELETE /api/admin/tags/:id
func (c *TagController) DeleteTag(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(ctx, "无效的标签ID")
		return
	}
	if err := c.service.DeleteTag(uint(id)); err != nil {
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, nil)
}
