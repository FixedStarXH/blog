package router

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"blog-system/controller"
	"blog-system/dao"
	"blog-system/middleware"
	"blog-system/model"
	"blog-system/service"

	"github.com/gin-gonic/gin"
)

func Init(r *gin.Engine) {
	articleDAO := dao.NewArticleDAO()
	categoryDAO := dao.NewCategoryDAO()
	userDAO := dao.NewUserDAO()
	tagDAO := dao.NewTagDAO()
	commentDAO := dao.NewCommentDAO()
	settingDAO := dao.NewSettingDAO()

	articleSvc := service.NewArticleService(articleDAO, model.DB)
	categorySvc := service.NewCategoryService(categoryDAO, model.DB)
	authSvc := service.NewAuthService(userDAO, model.DB)
	tagSvc := service.NewTagService(tagDAO, model.DB)
	commentSvc := service.NewCommentService(commentDAO, model.DB)
	settingSvc := service.NewSettingService(settingDAO, model.DB)

	articleCtl := controller.NewArticleController(articleSvc)
	userSvc := service.NewUserService(userDAO, model.DB)
	userCtl := controller.NewUserController(userSvc)
	categoryCtl := controller.NewCategoryController(categorySvc)
	authCtl := controller.NewAuthController(authSvc)
	tagCtl := controller.NewTagController(tagSvc)
	commentCtl := controller.NewCommentController(commentSvc)
	settingCtl := controller.NewSettingController(settingSvc)
	uploadCtl := controller.NewUploadController()

	api := r.Group("/api")
	{
		api.GET("/articles", articleCtl.GetArticleList)
		api.GET("/categories", categoryCtl.GetCategoryList)
		api.GET("/tags", tagCtl.GetTagList)
		api.GET("/articles/hot", articleCtl.GetHotArticles)
		api.GET("/articles/:id", articleCtl.GetArticleDetail)
		api.PUT("/articles/:id/view", articleCtl.AddView)
		api.PUT("/articles/:id/like", articleCtl.Like)
		api.GET("/articles/:id/nav", articleCtl.GetArticleNav)
		api.GET("/articles/:id/comments", commentCtl.GetComments)
		api.POST("/articles/:id/comments", commentCtl.AddComment)
		api.GET("/archives", articleCtl.GetArchives)
		api.GET("/settings", settingCtl.GetSiteSettings)
		api.GET("/quote", settingCtl.GetDailyQuote)

		// 阶段三：用户体系
		// 注册/登录不需要登录
		api.POST("/auth/register", authCtl.Register)
		api.POST("/auth/login", authCtl.Login)

		// 以下接口需要先登录（AuthRequired 解析 token 后，handler 用 GetUserID 取身份）
		authed := api.Group("", middleware.AuthRequired())
		authed.GET("/auth/me", authCtl.Me)
		authed.PUT("/auth/me", authCtl.UpdateProfile)
		authed.PUT("/auth/password", authCtl.ChangePassword)
		// 阶段四：我的投稿
		authed.GET("/my/articles", articleCtl.GetMyArticles)
		authed.POST("/my/articles", articleCtl.CreateMyArticle)
		authed.GET("/my/articles/:id", articleCtl.GetMyArticleDetail)
		authed.PUT("/my/articles/:id", articleCtl.UpdateMyArticle)
		authed.DELETE("/my/articles/:id", articleCtl.DeleteMyArticle)
		authed.POST("/upload", uploadCtl.UploadImage)

		// 后台管理组：编辑及以上（RBAC 双锁：先登录 401，再验角色 403）
		admin := api.Group("/admin", middleware.AuthRequired(), middleware.RequireRole(model.RoleEditor))
		admin.GET("/articles", articleCtl.GetAdminArticles)
		admin.PUT("/articles/:id/approve", articleCtl.ApproveArticle)
		admin.PUT("/articles/:id/reject", articleCtl.RejectArticle)
		// admin 组：AuthRequired(401) → RequireRole(Editor)(403)
		// 子组再叠 RequireRole(Admin)：编辑 role=2 < 3 也被拦
		adminUsers := admin.Group("/users", middleware.RequireRole(model.RoleAdmin))
		adminUsers.GET("", userCtl.GetAdminUsers)
		adminUsers.PUT("/:id/status", userCtl.UpdateUserStatus)
		adminUsers.PUT("/:id/role", userCtl.UpdateUserRole)
		// 分类/标签管理（编辑+，与文章审核同级权限）
		admin.POST("/categories", categoryCtl.CreateCategory)
		admin.PUT("/categories/:id", categoryCtl.UpdateCategory)
		admin.DELETE("/categories/:id", categoryCtl.DeleteCategory)
		admin.POST("/tags", tagCtl.CreateTag)
		admin.PUT("/tags/:id", tagCtl.UpdateTag)
		admin.DELETE("/tags/:id", tagCtl.DeleteTag)
		// 评论管理（编辑+）
		admin.GET("/comments", commentCtl.GetAdminComments)
		admin.PUT("/comments/:id/approve", commentCtl.ApproveComment)
		admin.PUT("/comments/:id/reject", commentCtl.RejectComment)
		admin.DELETE("/comments/:id", commentCtl.DeleteComment)
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
