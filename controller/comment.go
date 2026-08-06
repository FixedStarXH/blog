package controller

import (
	"strconv"

	"blog-system/service"
	"blog-system/utils"

	"github.com/gin-gonic/gin"
)

type CommentController struct {
	service *service.CommentService
}

func NewCommentController(service *service.CommentService) *CommentController {
	return &CommentController{service: service}
}

// 发表评论请求体：昵称 + 内容都必填（游客版）
type addCommentRequest struct {
	Nickname string `json:"nickname" binding:"required"` // 游客昵称，必填
	Content  string `json:"content" binding:"required"`  // 评论内容，必填
}

// GetComments 文章评论列表
// GET /api/articles/:id/comments
func (c *CommentController) GetComments(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(ctx, "无效的文章ID")
		return
	}

	comments, err := c.service.GetComments(uint(id))
	if err != nil {
		utils.Error(ctx, "获取评论失败")
		return
	}
	utils.Success(ctx, comments)
}

// AddComment 发表评论（游客可评，不需要登录）
// POST /api/articles/:id/comments
func (c *CommentController) AddComment(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(ctx, "无效的文章ID")
		return
	}

	var req addCommentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.Fail(ctx, "参数错误："+err.Error())
		return
	}

	comment, err := c.service.AddComment(uint(id), req.Content, req.Nickname)
	if err != nil {
		utils.Fail(ctx, "发表评论失败")
		return
	}
	utils.Success(ctx, comment)
}

// GetAdminComments 后台评论列表（编辑+）
// GET /api/admin/comments?status=0&page=1&pageSize=10
func (c *CommentController) GetAdminComments(ctx *gin.Context) {
	status, _ := strconv.Atoi(ctx.DefaultQuery("status", "-1"))
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "10"))

	comments, total, err := c.service.GetAdminComments(status, page, pageSize)
	if err != nil {
		utils.Error(ctx, "获取评论列表失败")
		return
	}
	utils.Success(ctx, gin.H{"list": comments, "total": total, "page": page, "pageSize": pageSize})
}

// ApproveComment 评论通过
// PUT /api/admin/comments/:id/approve
func (c *CommentController) ApproveComment(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(ctx, "无效的评论ID")
		return
	}
	if err := c.service.ApproveComment(uint(id)); err != nil {
		// 业务拒绝（评论不存在等）用 Fail（code 400），Error 500 留给系统异常
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, nil)
}

// RejectComment 评论驳回
// PUT /api/admin/comments/:id/reject
func (c *CommentController) RejectComment(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(ctx, "无效的评论ID")
		return
	}
	if err := c.service.RejectComment(uint(id)); err != nil {
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, nil)
}

// DeleteComment 删除评论
// DELETE /api/admin/comments/:id
func (c *CommentController) DeleteComment(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(ctx, "无效的评论ID")
		return
	}
	if err := c.service.DeleteComment(uint(id)); err != nil {
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, nil)
}
