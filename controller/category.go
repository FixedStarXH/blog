package controller

import (
	"blog-system/service"
	"blog-system/utils"
	"strconv"

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
		utils.Error(ctx, "获取分类列表失败")
		return
	}
	utils.Success(ctx, categories)
}

// 新增分类：name 必填，最长 50 字；description/sort 可空
type createCategoryRequest struct {
	Name        string `json:"name" binding:"required,max=50"`
	Description string `json:"description"`
	Sort        int    `json:"sort"`
}

// 修改分类：name 可空（只改描述/排序时不传 name）
type updateCategoryRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Sort        int    `json:"sort"`
}

// CreateCategory 新增分类
// POST /api/admin/categories
func (c *CategoryController) CreateCategory(ctx *gin.Context) {
	var req createCategoryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.Fail(ctx, "参数错误："+err.Error())
		return
	}
	if err := c.service.CreateCategory(req.Name, req.Description, req.Sort); err != nil {
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, nil)
}

// UpdateCategory 修改分类
// PUT /api/admin/categories/:id
func (c *CategoryController) UpdateCategory(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(ctx, "无效的分类ID")
		return
	}
	var req updateCategoryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.Fail(ctx, "参数错误："+err.Error())
		return
	}
	if err := c.service.UpdateCategory(uint(id), req.Name, req.Description, req.Sort); err != nil {
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, nil)
}

// DeleteCategory 删除分类（有文章引用会被拒）
// DELETE /api/admin/categories/:id
func (c *CategoryController) DeleteCategory(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(ctx, "无效的分类ID")
		return
	}
	if err := c.service.DeleteCategory(uint(id)); err != nil {
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, nil)
}
