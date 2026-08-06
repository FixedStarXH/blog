package controller

import (
	"strconv"

	"blog-system/middleware"
	"blog-system/service"
	"blog-system/utils"

	"github.com/gin-gonic/gin"
)

// 改状态请求体：oneof 限定值只能是 0 或 1（禁用/正常）
// 注意用 *int 指针：required 对数值 0 判空（0=禁用正好踩坑），
// 指针为 nil 才算"没传"，指向 0 不算空 → {"status":0} 能正常通过校验
type updateUserStatusRequest struct {
	Status *int `json:"status" binding:"required,oneof=0 1"`
}

// 改角色请求体：只能 1/2/3（防提权：DTO 白名单，不接收 username/password 等其他字段）
type updateUserRoleRequest struct {
	Role int `json:"role" binding:"required,oneof=1 2 3"`
}

type UserController struct {
	service *service.UserService
}

func NewUserController(service *service.UserService) *UserController {
	return &UserController{service: service}
}

// GetAdminUsers 用户列表
// GET /api/admin/users?keyword=李&role=1&page=1&pageSize=10
func (c *UserController) GetAdminUsers(ctx *gin.Context) {
	keyword := ctx.Query("keyword")
	role, _ := strconv.Atoi(ctx.DefaultQuery("role", "0"))
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "10"))

	users, total, err := c.service.GetAdminUsers(keyword, role, page, pageSize)
	if err != nil {
		utils.Error(ctx, "获取用户列表失败")
		return
	}
	utils.Success(ctx, gin.H{"list": users, "total": total, "page": page, "pageSize": pageSize})
}

// UpdateUserStatus 启用/禁用
// PUT /api/admin/users/:id/status   Body: {"status": 0}
func (c *UserController) UpdateUserStatus(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(ctx, "无效的用户ID")
		return
	}
	var req updateUserStatusRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.Fail(ctx, "参数错误："+err.Error())
		return
	}
	operatorID := middleware.GetUserID(ctx)
	if err := c.service.UpdateUserStatus(operatorID, uint(id), *req.Status); err != nil {
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, nil)
}

// UpdateUserRole 修改角色
// PUT /api/admin/users/:id/role   Body: {"role": 2}
func (c *UserController) UpdateUserRole(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(ctx, "无效的用户ID")
		return
	}
	var req updateUserRoleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.Fail(ctx, "参数错误："+err.Error())
		return
	}
	operatorID := middleware.GetUserID(ctx)
	if err := c.service.UpdateUserRole(operatorID, uint(id), req.Role); err != nil {
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, nil)
}
