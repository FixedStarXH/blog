package router

import (
	"blog-system/controller"
	"blog-system/dao"
	"blog-system/model"
	"blog-system/service"

	"github.com/gin-gonic/gin"
)

func Init(r *gin.Engine) {
	articleDAO := dao.NewArticleDAO()
	categoryDAO := dao.NewCategoryDAO()

	articleSvc := service.NewArticleService(articleDAO, model.DB)
	categorySvc := service.NewCategoryService(categoryDAO, model.DB)

	articleCtl := controller.NewArticleController(articleSvc)
	categoryCtl := controller.NewCategoryController(categorySvc)

	api := r.Group("/api")
	{
		api.GET("/articles", articleCtl.GetArticleList)
		api.GET("/categories", categoryCtl.GetCategoryList)
		api.GET("/articles/:id", articleCtl.GetArticleDetail)
	}
}
