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

// 发表评论请求体：昵称可选（免昵称），内容必填
type addCommentRequest struct {
	Nickname string `json:"nickname"`                   // 游客昵称，可选
	Content  string `json:"content" binding:"required"` // 评论内容，必填
	ParentID *uint  `json:"parentId"`                   // 楼中楼：回复哪条评论，nil=顶级评论
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
	// 前端详情页需要显示"共 N 条"，统一返回分页结构（评论不分页，pageSize 给大值即可）
	utils.Success(ctx, gin.H{"list": comments, "total": len(comments)})
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

	comment, err := c.service.AddComment(uint(id), req.Content, req.Nickname, req.ParentID)
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

// commentStatusRequest 后台通用改状态：status 0待审 1通过 2驳回
type commentStatusRequest struct {
	Status int `json:"status" binding:"required"`
}

// UpdateCommentStatus 后台通用改状态（前端 comments.html 契约）
// PUT /api/admin/comments/:id/status  Body: {"status":1}
func (c *CommentController) UpdateCommentStatus(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(ctx, "无效的评论ID")
		return
	}
	var req commentStatusRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.Fail(ctx, "参数错误："+err.Error())
		return
	}
	if err := c.service.SetCommentStatus(uint(id), req.Status); err != nil {
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, nil)
}

// batchCommentRequest 批量操作：ids 评论ID数组，action: approve 通过 / delete 删除
type batchCommentRequest struct {
	IDs    []uint `json:"ids" binding:"required"`
	Action string `json:"action" binding:"required"`
}

// BatchCommentOp 批量通过/删除（前端 comments.html 契约）
// POST /api/admin/comments/batch  Body: {"ids":[1,2],"action":"approve"}
func (c *CommentController) BatchCommentOp(ctx *gin.Context) {
	var req batchCommentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.Fail(ctx, "参数错误："+err.Error())
		return
	}
	if err := c.service.BatchCommentOp(req.IDs, req.Action); err != nil {
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, nil)
}
