package controller

import (
	"strconv"

	"blog-system/service"
	"blog-system/utils"

	"github.com/gin-gonic/gin"
)

// LinkController 友情链接管理（编辑+）
type LinkController struct {
	service *service.LinkService
}

func NewLinkController(service *service.LinkService) *LinkController {
	return &LinkController{service: service}
}

// linkRequest 友链请求体（前端 links.html 契约）
type linkRequest struct {
	Name        string `json:"name" binding:"required"`
	URL         string `json:"url" binding:"required"`
	Description string `json:"description"`
	Sort        int    `json:"sort"`
	Enabled     bool   `json:"enabled"`
}

// GetLinks 友链列表（编辑+）
// GET /api/admin/links
func (c *LinkController) GetLinks(ctx *gin.Context) {
	links, err := c.service.List()
	if err != nil {
		utils.Error(ctx, "获取友链失败")
		return
	}
	utils.Success(ctx, links)
}

// CreateLink 新增友链
// POST /api/admin/links
func (c *LinkController) CreateLink(ctx *gin.Context) {
	var req linkRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.Fail(ctx, "参数错误："+err.Error())
		return
	}
	link, err := c.service.Create(req.Name, req.URL, req.Description, req.Sort, req.Enabled)
	if err != nil {
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, link)
}

// UpdateLink 编辑友链
// PUT /api/admin/links/:id
func (c *LinkController) UpdateLink(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(ctx, "无效的链接ID")
		return
	}
	var req linkRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.Fail(ctx, "参数错误："+err.Error())
		return
	}
	if err := c.service.Update(uint(id), req.Name, req.URL, req.Description, req.Sort, req.Enabled); err != nil {
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, nil)
}

// DeleteLink 删除友链
// DELETE /api/admin/links/:id
func (c *LinkController) DeleteLink(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(ctx, "无效的链接ID")
		return
	}
	if err := c.service.Delete(uint(id)); err != nil {
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, nil)
}
