package controller

import (
	"blog-system/service"
	"blog-system/utils"
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ArticleController struct {
	service *service.ArticleService
}

func NewArticleController(service *service.ArticleService) *ArticleController {
	return &ArticleController{service: service}
}

func (c *ArticleController) GetArticleList(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "10"))

	articles, total, err := c.service.GetPublishedArticles(page, pageSize)
	if err != nil {
		utils.Error(ctx, "获取文章列表失败")
		return
	}

	utils.Success(ctx, gin.H{
		"list":     articles,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (c *ArticleController) GetArticleDetail(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		utils.Fail(ctx, "无效的文章ID")
		return
	}

	article, err := c.service.GetArticleByID(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.Fail(ctx, "文章不存在")
			return
		}
		utils.Error(ctx, "获取文章失败")
		return
	}
	utils.Success(ctx, article)

}
