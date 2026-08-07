package controller

import (
	"strconv"

	"blog-system/service"
	"blog-system/utils"

	"github.com/gin-gonic/gin"
)

// ImageController 图片图库管理（上传走 UploadController，这里只管列表/删除）
type ImageController struct {
	service *service.ImageService
}

func NewImageController(service *service.ImageService) *ImageController {
	return &ImageController{service: service}
}

// GetImages 图片库分页列表（编辑+）
// GET /api/admin/images?page=1&pageSize=24
func (c *ImageController) GetImages(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "24"))

	images, total, err := c.service.List(page, pageSize)
	if err != nil {
		utils.Error(ctx, "获取图片列表失败")
		return
	}
	utils.Success(ctx, gin.H{"list": images, "total": total, "page": page, "pageSize": pageSize})
}

// DeleteImage 删除图片（编辑+）：删记录 + 删磁盘文件
// DELETE /api/admin/images/:id
func (c *ImageController) DeleteImage(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(ctx, "无效的图片ID")
		return
	}
	if err := c.service.Delete(uint(id)); err != nil {
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, nil)
}
