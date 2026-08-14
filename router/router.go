package router

import (
	"net/http"
	"net/http/pprof"
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
	// 请求日志中间件（洋葱最外层：所有请求都会先经过它）
	r.Use(middleware.RequestLogger())

	articleDAO := dao.NewArticleDAO()
	categoryDAO := dao.NewCategoryDAO()
	userDAO := dao.NewUserDAO()
	tagDAO := dao.NewTagDAO()
	commentDAO := dao.NewCommentDAO()
	settingDAO := dao.NewSettingDAO()
	imageDAO := dao.NewImageDAO()
	linkDAO := dao.NewLinkDAO()

	articleSvc := service.NewArticleService(articleDAO, model.DB)
	categorySvc := service.NewCategoryService(categoryDAO, model.DB)
	authSvc := service.NewAuthService(userDAO, model.DB)
	tagSvc := service.NewTagService(tagDAO, model.DB)
	commentSvc := service.NewCommentService(commentDAO, model.DB)
	settingSvc := service.NewSettingService(settingDAO, model.DB)
	imageSvc := service.NewImageService(imageDAO, model.DB)
	linkSvc := service.NewLinkService(linkDAO, model.DB)
	dashboardSvc := service.NewDashboardService(model.DB)

	articleCtl := controller.NewArticleController(articleSvc)
	userSvc := service.NewUserService(userDAO, model.DB)
	userCtl := controller.NewUserController(userSvc)
	categoryCtl := controller.NewCategoryController(categorySvc)
	authCtl := controller.NewAuthController(authSvc)
	tagCtl := controller.NewTagController(tagSvc)
	commentCtl := controller.NewCommentController(commentSvc)
	settingCtl := controller.NewSettingController(settingSvc)
	uploadCtl := controller.NewUploadController(imageDAO, model.DB)
	feedCtl := controller.NewFeedController(articleSvc)
	imageCtl := controller.NewImageController(imageSvc)
	linkCtl := controller.NewLinkController(linkSvc)
	dashboardCtl := controller.NewDashboardController(dashboardSvc)
	rssCtl := controller.NewRSSController(articleSvc, settingSvc)
	aiSvc := service.NewAIService(model.DB)
	aiCtl := controller.NewAIController(aiSvc)
	auditSvc := service.NewAuditService(model.DB)
	auditCtl := controller.NewAuditController(auditSvc)

	api := r.Group("/api")
	{
		// 全局 API 宽松限流：100/秒、突发 200（按 IP 维度，防止单个 IP 打爆全站）
		api.Use(middleware.RateLimitByIP(100, 200))

		api.GET("/articles", articleCtl.GetArticleList)
		api.GET("/categories", categoryCtl.GetCategoryList)
		api.GET("/tags", tagCtl.GetTagList)
		api.GET("/articles/hot", articleCtl.GetHotArticles)
		api.GET("/articles/:id", articleCtl.GetArticleDetail)
		api.GET("/articles/:id/export", articleCtl.ExportArticle) // 导出 Markdown（仅已发布公开文章）
		api.PUT("/articles/:id/view", articleCtl.AddView)
		api.PUT("/articles/:id/like", articleCtl.Like)
		api.GET("/articles/:id/nav", articleCtl.GetArticleNav)
		api.POST("/articles/:id/unlock", middleware.RateLimitByIP(5, 10), articleCtl.UnlockArticle) // 私密文章解锁（游客也能试）
		api.GET("/articles/:id/comments", commentCtl.GetComments)
		// 评论发表限流（防刷屏）+ 可选登录（带 token 识别用户：昵称留空自动用账号名，游客默认"游客"）
		api.POST("/articles/:id/comments", middleware.RateLimitByIP(10, 20), middleware.OptionalAuth(), commentCtl.AddComment)
		api.GET("/sensitive/words", commentCtl.GetSensitiveWords) // 敏感词库（前端提交评论前即时检测）
		// 评论点赞（原子自增 +1；IP 限流防刷屏）
		api.PUT("/articles/:id/comments/:cid/like", middleware.RateLimitByIP(10, 20), commentCtl.LikeComment)
		api.GET("/archives", articleCtl.GetArchives)
		api.GET("/settings", settingCtl.GetSiteSettings)
		api.GET("/quote", settingCtl.GetDailyQuote)
		api.GET("/links", linkCtl.GetPublicLinks) // 公开友链列表（前台 about 页用）
		api.GET("/rss.xml", rssCtl.Feed)          // RSS 2.0 订阅源

		// 联调补齐：前端页面使用的路径/结构与后端契约（home.js/common.js/about.html/archive.html）
		api.GET("/site", settingCtl.GetSiteInfo)            // 前台站点信息（含统计数字）
		api.GET("/quote/random", settingCtl.GetRandomQuote) // 前台每日一言（结构化 {content, author}）
		api.GET("/archive", articleCtl.GetArchives)         // 归档别名（前端用单数）
		// AI 智能问答（公开接口；严格限流：AI 每次调用都花钱，防刷接口烧额度）
		api.POST("/ai/ask", middleware.RateLimitByIP(5, 10), aiCtl.Ask)
		// 前台文章"一键总结本文"（公开接口；同样严格限流，未配置 key 时后端自动降级）
		api.POST("/articles/:id/summary", middleware.RateLimitByIP(5, 10), aiCtl.SummarizeArticle)

		// 阶段三：用户体系
		// 注册/登录不需要登录；注册限流防批量注册垃圾账号（与登录同级别：5/秒、突发 10）
		// LoginAudit：认证尝试全记录（谁/成功失败/IP），防暴力破解留痕
		api.POST("/auth/register", middleware.RateLimitByIP(5, 10), middleware.LoginAudit(), authCtl.Register)
		// 登录接口严格限流（防暴力破解密码：5/秒、突发 10，按 IP 维度）
		api.POST("/auth/login", middleware.RateLimitByIP(5, 10), middleware.LoginAudit(), authCtl.Login)
		// 刷新双 token（access 过期后用 refresh 换新；限流防暴力遍历 refresh token）
		api.POST("/auth/refresh", middleware.RateLimitByIP(10, 20), authCtl.Refresh)
		// 退出登录（吊销 refresh token）
		api.POST("/auth/logout", middleware.LoginAudit(), authCtl.Logout)
		// 后台登录（前端 admin/login.html 契约；编辑+ 才能进后台，角色校验在 handler 内）
		api.POST("/admin/login", middleware.RateLimitByIP(5, 10), middleware.LoginAudit(), authCtl.AdminLogin)

		// 以下接口需要先登录（AuthRequired 解析 token 后，handler 用 GetUserID 取身份）
		authed := api.Group("", middleware.AuthRequired())
		authed.GET("/auth/me", authCtl.Me)
		authed.PUT("/auth/me", authCtl.UpdateProfile)
		authed.PUT("/auth/password", authCtl.ChangePassword)
		// AI 能力（登录用户）：投稿/写文章页生成摘要、流式润色（严格限流，AI 每次调用都花钱）
		authed.POST("/ai/summary", middleware.RateLimitByIP(5, 10), aiCtl.GenerateSummaryByContent)
		authed.POST("/ai/polish", middleware.RateLimitByIP(5, 10), aiCtl.Polish)
		// 阶段四：我的投稿
		authed.GET("/my/articles", articleCtl.GetMyArticles)
		authed.POST("/my/articles", articleCtl.CreateMyArticle)
		authed.GET("/my/articles/:id", articleCtl.GetMyArticleDetail)
		authed.PUT("/my/articles/:id", articleCtl.UpdateMyArticle)
		authed.DELETE("/my/articles/:id", articleCtl.DeleteMyArticle)
		authed.POST("/upload", uploadCtl.UploadImage)
		// 后台改密码（前端 admin/settings.html 契约：改的是当前登录用户的密码）
		authed.PUT("/admin/password", authCtl.ChangePassword)

		// 后台管理组：编辑及以上（RBAC 双锁：先登录 401，再验角色 403）
		admin := api.Group("/admin", middleware.AuthRequired(), middleware.RequireRole(model.RoleEditor))
		// 操作审计（AdminAudit）：挂在整组，自动记录所有写操作（新建/修改/删除/审核），
		// 谁在什么时候做了什么、成功失败、来源 IP 全落库，后台可查（见 admin/audit.html）
		admin.Use(middleware.AdminAudit())
		admin.GET("/articles", articleCtl.GetAdminArticles)
		admin.PUT("/articles/:id/approve", articleCtl.ApproveArticle)
		admin.PUT("/articles/:id/reject", articleCtl.RejectArticle)
		// 联调补齐：后台文章完整管理（列表已有，补详情/新建/编辑/删除，管理员可操作任何人的文章）
		admin.GET("/articles/:id", articleCtl.GetAdminArticleDetail)
		admin.POST("/articles", articleCtl.CreateAdminArticle)
		admin.PUT("/articles/:id", articleCtl.UpdateAdminArticle)
		admin.DELETE("/articles/:id", articleCtl.DeleteAdminArticle)
		// 批量操作（前端 admin/articles.html 契约：勾选 → 批量发布/草稿/置顶/删除）
		admin.POST("/articles/batch", articleCtl.BatchArticleOp)
		// 仪表盘统计（编辑+）
		admin.GET("/dashboard", dashboardCtl.GetDashboard)
		// 仪表盘：浏览量趋势（编辑+，近 N 天折线图）
		admin.GET("/dashboard/views-trend", dashboardCtl.GetViewsTrend)
		// 后台分类下拉（前端 admin/articles.html 用 id/name 填充下拉框）
		admin.GET("/categories", categoryCtl.GetCategoryList)
		// 站点设置（编辑+）：GET 回显 + PUT 保存
		admin.GET("/settings", settingCtl.GetAdminSettings)
		admin.PUT("/settings", settingCtl.UpdateSettings)
		// 图库管理（编辑+）：上传写库 + 列表 + 删除
		admin.POST("/upload", uploadCtl.UploadImage)
		admin.GET("/images", imageCtl.GetImages)
		admin.DELETE("/images/:id", imageCtl.DeleteImage)
		// 友情链接（编辑+）：完整 CRUD
		admin.GET("/links", linkCtl.GetLinks)
		admin.POST("/links", linkCtl.CreateLink)
		admin.PUT("/links/:id", linkCtl.UpdateLink)
		admin.DELETE("/links/:id", linkCtl.DeleteLink)
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
		admin.GET("/tags", tagCtl.GetAdminTagList) // 后台标签管理列表（分页，admin/tags.html 契约）
		admin.POST("/tags", tagCtl.CreateTag)
		admin.PUT("/tags/:id", tagCtl.UpdateTag)
		admin.DELETE("/tags/:id", tagCtl.DeleteTag)
		// 评论管理（编辑+）：原有通过/驳回/删除 + 通用改状态 + 批量操作
		admin.GET("/comments", commentCtl.GetAdminComments)
		admin.PUT("/comments/:id/approve", commentCtl.ApproveComment)
		admin.PUT("/comments/:id/reject", commentCtl.RejectComment)
		admin.DELETE("/comments/:id", commentCtl.DeleteComment)
		admin.PUT("/comments/:id/status", commentCtl.UpdateCommentStatus)
		admin.POST("/comments/batch", commentCtl.BatchCommentOp)
		// AI 能力（编辑+）：摘要 / 润色 / RAG 索引管理
		admin.POST("/ai/summary", aiCtl.GenerateSummary)
		admin.POST("/ai/polish", aiCtl.Polish)
		admin.POST("/ai/index/:id", aiCtl.IndexArticle)
		admin.POST("/ai/index-all", aiCtl.IndexAll)
		admin.GET("/ai/index-status", aiCtl.IndexStatus)
		// 操作审计日志（编辑+ 可查，管理员可清空）
		admin.GET("/audit-logs", auditCtl.GetAuditLogs)
		admin.DELETE("/audit-logs", middleware.RequireRole(model.RoleAdmin), auditCtl.ClearAuditLogs)
		// 性能分析 pprof（仅管理员，生产环境可安全查看内存/CPU profile）
		// 用法：go tool pprof http://<host>/api/admin/debug/pprof/heap
		adminDebug := admin.Group("/debug/pprof", middleware.RequireRole(model.RoleAdmin))
		adminDebug.GET("/*any", func(c *gin.Context) {
			// pprof.Index 按 /debug/pprof/<name> 解析 profile 名，这里把前缀重写回去
			c.Request.URL.Path = "/debug/pprof" + c.Param("any")
			pprof.Index(c.Writer, c.Request)
		})
	}

	// 前端构建产物托管(dist/ 目录，由 web/ Vite 工程构建生成，源文件在 web/)
	webDir := "./dist"
	r.Static("/web", webDir)
	r.Static("/uploads", "./uploads")

	// RSS 订阅 + Sitemap（放根路径：搜索引擎/订阅器习惯请求域名根路径）
	r.GET("/feed.xml", feedCtl.Rss)
	r.GET("/sitemap.xml", feedCtl.Sitemap)

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
