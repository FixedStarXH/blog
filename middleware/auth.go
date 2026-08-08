package middleware

import (
	"strings"

	"blog-system/model"
	"blog-system/utils"

	"github.com/gin-gonic/gin"
)

// 下面两个"从 context 取用户"的辅助函数，给 controller 用
// 为什么存在？中间件解析完 token 后，把 userID/role 放进 gin.Context，
// 后续 handler 想拿身份信息，就用这两个函数取（类型断言包了一层，controller 更干净）

// GetUserID 从 context 取当前登录用户ID（必须在 AuthRequired 之后用）
func GetUserID(c *gin.Context) uint {
	// c.Get 返回 (value, exists)；断言失败时 v=nil，取零值 0 兜底
	v, _ := c.Get("userID")
	if id, ok := v.(uint); ok {
		return id
	}
	return 0
}

// GetRole 从 context 取当前登录用户角色
func GetRole(c *gin.Context) int {
	v, _ := c.Get("role")
	if r, ok := v.(int); ok {
		return r
	}
	return 0
}

// AuthRequired 登录鉴权中间件：必须携带有效 token 才放行
//
// 用法：需要登录的接口分组上挂它，如
//
//	authed := api.Group("", middleware.AuthRequired())
//
// 流程（JWT 小课第 4 节）：
//  1. 从 Header 取 "Authorization: Bearer <token>"
//  2. ParseToken 验签 + 查过期
//  3. 成功 → 把 userID/role 塞进 context → c.Next() 放行
//  4. 失败 → 返回 401 并 c.Abort() 拦截（后续 handler 不再执行）
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 取 Authorization 头，标准格式 "Bearer xxx"
		auth := c.GetHeader("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			utils.Unauthorized(c, "未登录")
			c.Abort() // 拦截：不再往下执行任何 handler
			return
		}
		tokenString := strings.TrimPrefix(auth, "Bearer ")

		// 2. 验签 + 过期校验（库内部完成）
		claims, err := utils.ParseToken(tokenString)
		if err != nil {
			utils.Unauthorized(c, "登录已过期，请重新登录")
			c.Abort()
			return
		}

		// 3. 查库确认用户真实状态：
		//    JWT 本身无状态，若只靠验签，禁用/删除的用户旧 token 依然有效，
		//    角色被降级后旧 token 也继续持有旧权限。这里以数据库为准：
		//    - 用户不存在（被删）→ 401
		//    - 用户被禁用（status=0）→ 401
		//    - role 用数据库里的最新值覆盖 token 里的旧值（权限变更即时生效）
		var user model.User
		if err := model.DB.Select("id", "role", "status").First(&user, claims.UserID).Error; err != nil {
			utils.Unauthorized(c, "账号不存在，请重新登录")
			c.Abort()
			return
		}
		if user.Status != model.UserStatusActive {
			utils.Unauthorized(c, "账号已被禁用")
			c.Abort()
			return
		}

		// 4. 把身份信息放进 context，后续 handler 用 GetUserID/GetRole 取
		c.Set("userID", user.ID)
		c.Set("role", user.Role)

		// 5. 放行，继续执行后面的 handler
		c.Next()
	}
}

// RequireRole 角色权限中间件：要求角色 >= minRole 才放行（RBAC）
//
// 用法：必须放在 AuthRequired 之后（它要读 context 里的 role），如
//
//	admin := api.Group("/admin", middleware.AuthRequired(), middleware.RequireRole(model.RoleEditor))
//
// 为什么用 ">=" 而不是 "=="？
//
//	权限是越级包含的：管理员(3)天然拥有编辑(2)的权限。
//	RequireRole(RoleEditor) 表示"编辑及以上"，管理员也能过——避免每个接口都写死角色
func RequireRole(minRole int) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := GetRole(c)
		if role < minRole {
			utils.Forbidden(c, "权限不足")
			c.Abort()
			return
		}
		c.Next()
	}
}
