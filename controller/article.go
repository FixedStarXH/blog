package controller

import (
	"blog-system/dao"
	"blog-system/middleware"
	"blog-system/service"
	"blog-system/utils"
	"errors"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type createArticleRequest struct {
	Title      string     `json:"title" binding:"required"`
	Content    string     `json:"content" binding:"required"`
	Summary    string     `json:"summary"`
	CoverImage string     `json:"coverImage"`
	SourceURL  string     `json:"sourceUrl"` // 转载来源链接；可空
	Password   string     `json:"password"`  // 私密文章密码；空=公开
	PublishAt  *time.Time `json:"publishAt"` // 可空；设未来时间=定时发布，不传=审核通过立即发布
	CategoryID uint       `json:"categoryId" binding:"required"`
	TagIDs     []uint     `json:"tagIds"`
}

type updateArticleRequest struct {
	Title      string     `json:"title" binding:"required"`
	Content    string     `json:"content" binding:"required"`
	Summary    string     `json:"summary"`
	CoverImage string     `json:"coverImage"`
	SourceURL  string     `json:"sourceUrl"` // 转载来源链接；可空
	Password   string     `json:"password"`  // 私密文章密码；空=变回公开
	PublishAt  *time.Time `json:"publishAt"` // 可空；修改排期时间
	CategoryID uint       `json:"categoryId" binding:"required"`
	TagIDs     []uint     `json:"tagIds"`
}

type ArticleController struct {
	service *service.ArticleService
}

func NewArticleController(service *service.ArticleService) *ArticleController {
	return &ArticleController{service: service}
}

func (c *ArticleController) GetArticleList(ctx *gin.Context) {
	keyword := ctx.Query("keyword")

	authorID, _ := strconv.ParseUint(ctx.DefaultQuery("authorId", "0"), 10, 64)
	categoryID, _ := strconv.ParseUint(ctx.DefaultQuery("categoryId", "0"), 10, 64)

	tag := ctx.Query("tag")
	sortBy := ctx.Query("sort")

	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "10"))

	articles, total, err := c.service.GetPublishedArticles(keyword, uint(authorID), tag, uint(categoryID), sortBy, page, pageSize)
	if err != nil {
		utils.Error(ctx, "获取文章列表失败")
		return
	}
	utils.Success(ctx, gin.H{"list": articles, "total": total, "page": page, "pageSize": pageSize})
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

// GetMyArticles 我的文章列表
// GET /api/my/articles?status=2&page=1&pageSize=10
func (c *ArticleController) GetMyArticles(ctx *gin.Context) {
	userID := middleware.GetUserID(ctx)

	status, _ := strconv.Atoi(ctx.DefaultQuery("status", "-1"))
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "10"))

	articles, total, err := c.service.GetMyArticles(userID, status, page, pageSize)
	if err != nil {
		utils.Error(ctx, "获取文章列表失败")
		return
	}
	utils.Success(ctx, gin.H{"list": articles, "total": total, "page": page, "pageSize": pageSize})
}

// CreateMyArticle 投稿
// POST /api/my/articles
func (c *ArticleController) CreateMyArticle(ctx *gin.Context) {
	var req createArticleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.Fail(ctx, "参数错误："+err.Error())
		return
	}
	userID := middleware.GetUserID(ctx)

	article, err := c.service.CreateArticle(userID, req.CategoryID, req.Title, req.Content, req.Summary, req.CoverImage, req.SourceURL, req.Password, req.PublishAt, req.TagIDs)
	if err != nil {
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, article)
}

// GetMyArticleDetail 我的文章详情（含驳回原因）
// GET /api/my/articles/:id
func (c *ArticleController) GetMyArticleDetail(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(ctx, "无效的文章ID")
		return
	}
	userID := middleware.GetUserID(ctx)

	article, err := c.service.GetMyArticleDetail(uint(id), userID)
	if err != nil {
		if errors.Is(err, dao.ErrNotAuthor) {
			utils.Forbidden(ctx, "无权查看该文章")
			return
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.Fail(ctx, "文章不存在")
			return
		}
		utils.Error(ctx, "获取文章失败")
		return
	}
	utils.Success(ctx, article)
}

// UpdateMyArticle 编辑我的文章
// PUT /api/my/articles/:id
func (c *ArticleController) UpdateMyArticle(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(ctx, "无效的文章ID")
		return
	}
	var req updateArticleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.Fail(ctx, "参数错误："+err.Error())
		return
	}
	userID := middleware.GetUserID(ctx)

	if err := c.service.UpdateMyArticle(uint(id), userID, req.CategoryID, req.Title, req.Content, req.Summary, req.CoverImage, req.SourceURL, req.Password, req.PublishAt, req.TagIDs); err != nil {
		if errors.Is(err, dao.ErrNotAuthor) {
			utils.Forbidden(ctx, "无权修改该文章")
			return
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.Fail(ctx, "文章不存在")
			return
		}
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, nil)
}

// DeleteMyArticle 删除我的文章
// DELETE /api/my/articles/:id
func (c *ArticleController) DeleteMyArticle(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(ctx, "无效的文章ID")
		return
	}
	userID := middleware.GetUserID(ctx)

	if err := c.service.DeleteMyArticle(uint(id), userID); err != nil {
		if errors.Is(err, dao.ErrNotAuthor) {
			utils.Forbidden(ctx, "无权删除该文章")
			return
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.Fail(ctx, "文章不存在")
			return
		}
		utils.Error(ctx, "删除失败")
		return
	}
	utils.Success(ctx, nil)
}

func (c *ArticleController) AddView(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(ctx, "无效的文章ID")
		return
	}

	ip := ctx.ClientIP()

	viewCount, err := c.service.AddView(uint(id), ip)
	if err != nil {
		utils.Error(ctx, "浏览量更新失败")
		return
	}
	utils.Success(ctx, gin.H{"viewCount": viewCount})
}
func (c *ArticleController) Like(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(ctx, "无效的文章ID")
		return
	}

	ip := ctx.ClientIP()

	result, err := c.service.Like(uint(id), ip)
	if err != nil {
		utils.Error(ctx, "点赞失败")
		return
	}
	utils.Success(ctx, result)
}

func (c *ArticleController) GetHotArticles(ctx *gin.Context) {
	limit, err := strconv.Atoi(ctx.DefaultQuery("limit", "10"))
	if err != nil {
		limit = 10
	}
	articles, err := c.service.GetHotArticles(limit)
	if err != nil {
		utils.Error(ctx, "获取热门文章失败")
		return
	}
	utils.Success(ctx, articles)
}

func (c *ArticleController) GetArchives(ctx *gin.Context) {
	archives, err := c.service.GetArchives()
	if err != nil {
		utils.Error(ctx, "获取归档失败")
		return
	}
	utils.Success(ctx, archives)
}

// GetArticleNav 文章导航（上一篇/下一篇/相关推荐）
// GET /api/articles/:id/nav
func (c *ArticleController) GetArticleNav(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(ctx, "无效的文章ID")
		return
	}

	nav, err := c.service.GetArticleNav(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.Fail(ctx, "文章不存在")
			return
		}
		utils.Error(ctx, "获取文章导航失败")
		return
	}
	utils.Success(ctx, nav)
}

// 驳回请求体：原因必填
type rejectArticleRequest struct {
	Reason string `json:"reason" binding:"required"`
}

// GetAdminArticles 后台文章列表（编辑+）
// GET /api/admin/articles?status=2&page=1&pageSize=10
func (c *ArticleController) GetAdminArticles(ctx *gin.Context) {
	status, _ := strconv.Atoi(ctx.DefaultQuery("status", "-1"))
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "10"))

	articles, total, err := c.service.GetAdminArticles(status, page, pageSize)
	if err != nil {
		utils.Error(ctx, "获取文章列表失败")
		return
	}
	utils.Success(ctx, gin.H{"list": articles, "total": total, "page": page, "pageSize": pageSize})
}

// ApproveArticle 审核通过
// PUT /api/admin/articles/:id/approve
func (c *ArticleController) ApproveArticle(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(ctx, "无效的文章ID")
		return
	}
	if err := c.service.ApproveArticle(uint(id)); err != nil {
		// 业务拒绝（文章不存在等）用 Fail（code 400），不用 Error（500 留给系统异常）
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, nil)
}

// RejectArticle 驳回（原因必填）
// PUT /api/admin/articles/:id/reject
func (c *ArticleController) RejectArticle(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(ctx, "无效的文章ID")
		return
	}
	var req rejectArticleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.Fail(ctx, "参数错误："+err.Error())
		return
	}
	if err := c.service.RejectArticle(uint(id), req.Reason); err != nil {
		// 业务拒绝（文章不存在等）用 Fail（code 400），不用 Error（500 留给系统异常）
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, nil)
}

// 解锁请求体：密码必填
type unlockArticleRequest struct {
	Password string `json:"password" binding:"required"`
}

// UnlockArticle 私密文章解锁（公开接口：游客也能试）
// POST /api/articles/:id/unlock  Body: {"password":"xxx"}
func (c *ArticleController) UnlockArticle(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(ctx, "无效的文章ID")
		return
	}
	var req unlockArticleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.Fail(ctx, "参数错误："+err.Error())
		return
	}
	article, err := c.service.UnlockArticle(uint(id), req.Password)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.Fail(ctx, "文章不存在")
			return
		}
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, article)
}

// ------------------------------------------------------------
// 后台文章管理（编辑+）：管理员可操作任何人的文章
// ------------------------------------------------------------

// adminArticleRequest 后台新建/编辑文章：比作者投稿多 status/isTop（管理员可直接定稿）
type adminArticleRequest struct {
	Title      string     `json:"title" binding:"required"`
	Content    string     `json:"content" binding:"required"`
	Summary    string     `json:"summary"`
	CoverImage string     `json:"coverImage"`
	SourceURL  string     `json:"sourceUrl"` // 转载来源链接；可空
	Status     int        `json:"status"`    // 0草稿 1发布 2待审 3驳回 4已排期
	IsTop      bool       `json:"isTop"`     // 置顶
	Password   string     `json:"password"`  // 私密文章密码；空=公开
	PublishAt  *time.Time `json:"publishAt"` // 定时发布
	CategoryID uint       `json:"categoryId" binding:"required"`
	TagIDs     []uint     `json:"tagIds"`
}

// GetAdminArticleDetail 后台文章详情（编辑+）
// GET /api/admin/articles/:id
// 注意：model.Article 的 Password 是 json:"-"，公开接口绝不外泄；
// 这里后台需要回显密码（编辑表单用），所以单独拼一次响应。
func (c *ArticleController) GetAdminArticleDetail(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(ctx, "无效的文章ID")
		return
	}
	article, err := c.service.GetAdminArticleDetail(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.Fail(ctx, "文章不存在")
			return
		}
		utils.Error(ctx, "获取文章失败")
		return
	}
	utils.Success(ctx, gin.H{
		"id":         article.ID,
		"title":      article.Title,
		"summary":    article.Summary,
		"coverImage": article.CoverImage,
		"sourceUrl":  article.SourceURL,
		"content":    article.Content,
		"status":     article.Status,
		"isTop":      article.IsTop,
		"password":   article.Password,
		"categoryId": article.CategoryID,
		"publishAt":  article.PublishAt,
		"authorId":   article.AuthorID,
		"createdAt":  article.CreatedAt,
		"tags":       article.Tags,
	})
}

// CreateAdminArticle 后台新建/代发文章（编辑+）
// POST /api/admin/articles
func (c *ArticleController) CreateAdminArticle(ctx *gin.Context) {
	var req adminArticleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.Fail(ctx, "参数错误："+err.Error())
		return
	}
	userID := middleware.GetUserID(ctx)
	article, err := c.service.AdminCreateArticle(userID, req.CategoryID, req.Title, req.Content, req.Summary, req.CoverImage, req.SourceURL, req.Password, req.PublishAt, req.Status, req.IsTop, req.TagIDs)
	if err != nil {
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, article)
}

// UpdateAdminArticle 后台编辑文章（编辑+）
// PUT /api/admin/articles/:id  Body 与新建相同（含 isTop/status）
func (c *ArticleController) UpdateAdminArticle(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(ctx, "无效的文章ID")
		return
	}
	var req adminArticleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.Fail(ctx, "参数错误："+err.Error())
		return
	}
	if err := c.service.AdminUpdateArticle(uint(id), req.CategoryID, req.Title, req.Content, req.Summary, req.CoverImage, req.SourceURL, req.Password, req.PublishAt, req.Status, req.IsTop, req.TagIDs); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.Fail(ctx, "文章不存在")
			return
		}
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, nil)
}

// DeleteAdminArticle 后台删除文章（编辑+，软删除）
// DELETE /api/admin/articles/:id
func (c *ArticleController) DeleteAdminArticle(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(ctx, "无效的文章ID")
		return
	}
	if err := c.service.AdminDeleteArticle(uint(id)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.Fail(ctx, "文章不存在")
			return
		}
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, nil)
}

// batchArticleRequest 批量操作：ids 文章ID数组，action: publish/draft/top/untop/delete
type batchArticleRequest struct {
	IDs    []uint `json:"ids" binding:"required"`
	Action string `json:"action" binding:"required"`
}

// BatchArticleOp 后台批量操作（前端 admin/articles.html 契约）
// POST /api/admin/articles/batch  Body: {"ids":[1,2],"action":"delete"}
func (c *ArticleController) BatchArticleOp(ctx *gin.Context) {
	var req batchArticleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.Fail(ctx, "参数错误："+err.Error())
		return
	}
	affected, err := c.service.BatchArticleOp(req.IDs, req.Action)
	if err != nil {
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, gin.H{"affected": affected})
}
