package controller

import (
	"blog-system/service"
	"blog-system/utils"

	"github.com/gin-gonic/gin"
)

type CategoryController struct {
	service *service.CategoryService
}

func NewCategoryController(service *service.CategoryService) *CategoryController {
	return &CategoryController{service: service}
}

func (c *CategoryController) GetCategoryList(ctx *gin.Context) {
	categories, err := c.service.GetAllCategories()
	if err != nil {
		utils.Error(ctx, "获取·列表失败")
		return
	}
	utils.Success(ctx, categories)
}
