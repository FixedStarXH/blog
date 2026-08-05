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
