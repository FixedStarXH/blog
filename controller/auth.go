package controller

import (
	"blog-system/middleware"
	"blog-system/model"
	"blog-system/service"
	"blog-system/utils"

	"github.com/gin-gonic/gin"
)

type AuthController struct {
	service *service.AuthService
}

func NewAuthController(service *service.AuthService) *AuthController {
	return &AuthController{service: service}
}

type registerRequest struct {
	Username string `json:"username" binding:"required,min=2,max=50"` // 必填 + 长度 2-50（与 DB size 对齐，防超长触发 MySQL 报错）
	Email    string `json:"email" binding:"required,email"`           // 必填 + 邮箱格式
	Password string `json:"password" binding:"required,min=6"`        // 必填 + 至少6位
}

type loginRequest struct {
	Username string `json:"username" binding:"required"` // 用户名或邮箱（接口文档约定的字段名）
	Password string `json:"password" binding:"required"`
}

type updateProfileRequest struct {
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Bio      string `json:"bio"`
}

type changePasswordRequest struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=6"`
}

// refreshRequest 刷新 token 请求体：携带 refresh token 换新
type refreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

// Register 注册
func (c *AuthController) Register(ctx *gin.Context) {
	var req registerRequest
	// 第一步：绑定 JSON + 校验（binding 不通过会返回 err）
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.Fail(ctx, "参数错误："+err.Error())
		return
	}
	// 第二步：调 service（service 返回什么错误，前端就收到什么文案）
	user, access, refresh, err := c.service.Register(req.Username, req.Email, req.Password)
	if err != nil {
		utils.Fail(ctx, err.Error())
		return
	}
	// 第三步：成功响应（双 token：access 业务用，refresh 过期后换新）
	utils.Success(ctx, gin.H{"accessToken": access, "refreshToken": refresh, "user": user})
}

// Login 登录
func (c *AuthController) Login(ctx *gin.Context) {
	var req loginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.Fail(ctx, "参数错误："+err.Error())
		return
	}
	user, access, refresh, err := c.service.Login(req.Username, req.Password)
	if err != nil {
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, gin.H{"accessToken": access, "refreshToken": refresh, "user": user})
}

// Refresh 刷新双 token：access 过期后，前端用 refresh 换新
// POST /api/auth/refresh  Body: {"refreshToken":"..."}
func (c *AuthController) Refresh(ctx *gin.Context) {
	var req refreshRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.Fail(ctx, "参数错误："+err.Error())
		return
	}
	access, refresh, err := c.service.Refresh(req.RefreshToken)
	if err != nil {
		// 401：refresh 无效/过期/被轮换 → 前端清 token 回登录页
		utils.Unauthorized(ctx, err.Error())
		return
	}
	utils.Success(ctx, gin.H{"accessToken": access, "refreshToken": refresh})
}

// Logout 退出登录：吊销当前 refresh token（登出后旧 refresh 无法再换新）
// POST /api/auth/logout  Body: {"refreshToken":"..."}
func (c *AuthController) Logout(ctx *gin.Context) {
	var req refreshRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.Fail(ctx, "参数错误："+err.Error())
		return
	}
	if err := c.service.Logout(req.RefreshToken); err != nil {
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, nil)
}

// AdminLogin 后台登录（前端 admin/login.html 契约）
// POST /api/admin/login
// 和普通登录的区别：登录成功后校验角色，编辑(2)及以上才能进管理后台
func (c *AuthController) AdminLogin(ctx *gin.Context) {
	var req loginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.Fail(ctx, "参数错误："+err.Error())
		return
	}
	user, access, refresh, err := c.service.Login(req.Username, req.Password)
	if err != nil {
		utils.Fail(ctx, err.Error())
		return
	}
	// 普通用户（role=1）不允许进后台：登录本身成功，但被权限拒绝（403）
	if user.Role < model.RoleEditor {
		utils.Forbidden(ctx, "该账号无后台管理权限")
		return
	}
	utils.Success(ctx, gin.H{"accessToken": access, "refreshToken": refresh, "user": user})
}

// Me 当前用户信息（需要登录）
func (c *AuthController) Me(ctx *gin.Context) {
	// 从 context 取身份：中间件 AuthRequired 已经解析过 token 放进去了
	userID := middleware.GetUserID(ctx)
	user, err := c.service.Me(userID)
	if err != nil {
		utils.Fail(ctx, "用户不存在")
		return
	}
	utils.Success(ctx, user)
}

// UpdateProfile 更新资料（需要登录）
func (c *AuthController) UpdateProfile(ctx *gin.Context) {
	var req updateProfileRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.Fail(ctx, "参数错误："+err.Error())
		return
	}
	userID := middleware.GetUserID(ctx)
	user, err := c.service.UpdateProfile(userID, req.Nickname, req.Avatar, req.Bio)
	if err != nil {
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, user)
}

// ChangePassword 修改密码（需要登录）
func (c *AuthController) ChangePassword(ctx *gin.Context) {
	var req changePasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.Fail(ctx, "参数错误："+err.Error())
		return
	}
	userID := middleware.GetUserID(ctx)
	if err := c.service.ChangePassword(userID, req.OldPassword, req.NewPassword); err != nil {
		utils.Fail(ctx, err.Error())
		return
	}
	utils.Success(ctx, nil)
}
