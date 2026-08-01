package router

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

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

	// 前端 SPA 静态资源托管(web/ 目录)
	webDir := "./web"
	r.Static("/web", webDir)
	r.Static("/uploads", "./uploads")

	// 根路径 → 首页
	r.GET("/", func(c *gin.Context) {
		c.File(filepath.Join(webDir, "index.html"))
	})

	// 其余未匹配路径:若对应 web 下文件存在则返回,否则 404 页
	r.NoRoute(func(c *gin.Context) {
		// API 未实现的路由返回 JSON 格式错误(而非 HTML 页面)
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "接口不存在或未实现",
				"data":    nil,
			})
			return
		}
		p := filepath.Join(webDir, c.Request.URL.Path)
		if fileExists(p) {
			c.File(p)
			return
		}
		// 返回 404 页面(状态码为 404,而非 c.File 覆盖的 200)
		notFound, err := os.ReadFile(filepath.Join(webDir, "404.html"))
		if err != nil {
			c.String(http.StatusNotFound, "404 Not Found")
			return
		}
		c.Data(http.StatusNotFound, "text/html; charset=utf-8", notFound)
	})
}

// fileExists 判断文件是否存在
func fileExists(path string) bool {
	info, err := filepath.Glob(path)
	if err != nil || len(info) == 0 {
		return false
	}
	return true
}
