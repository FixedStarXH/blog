package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"blog-system/config"
	"blog-system/model"
	"blog-system/utils"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setup 测试前初始化：gin 测试模式 + 固定密钥 + 内存数据库
//
// AuthRequired 现在会查库验证用户状态（禁用/删除的用户 token 立即失效），
// 所以测试必须有一个真实可查的 DB。这里用纯 Go 的 sqlite 内存库：
// 无需外部 MySQL、无需 CGO 编译，测试自包含。
func setup() {
	gin.SetMode(gin.TestMode)
	config.AppConfig = &config.Config{
		JWT: config.JWTConfig{Secret: "test-secret"},
	}

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		panic("初始化测试内存数据库失败: " + err.Error())
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		panic("测试建表失败: " + err.Error())
	}
	model.DB = db

	// 测试用户（显式指定 ID，对应各用例 GenerateToken 里的用户ID）：
	//   1 = 普通用户  2 = 编辑  5 = 编辑
	// 注意 AuthRequired 会用数据库里的 role 覆盖 token 里的旧值，
	// 所以这里插入的角色必须与用例期望一致。
	users := []model.User{
		{Username: "user1", Email: "u1@test.com", Password: "x", Nickname: "普通用户", Role: model.RoleUser, Status: model.UserStatusActive},
		{Username: "editor", Email: "u2@test.com", Password: "x", Nickname: "编辑", Role: model.RoleEditor, Status: model.UserStatusActive},
		{Username: "editor5", Email: "u5@test.com", Password: "x", Nickname: "编辑5", Role: model.RoleEditor, Status: model.UserStatusActive},
	}
	users[0].ID = 1
	users[1].ID = 2
	users[2].ID = 5
	if err := db.Create(&users).Error; err != nil {
		panic("插入测试用户失败: " + err.Error())
	}
}

// TestAuthRequiredNoToken 无 token 请求 → 必须被 401 拦截
func TestAuthRequiredNoToken(t *testing.T) {
	setup()
	r := gin.New()
	r.GET("/api/me", AuthRequired(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/me", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("无 token 应返回 401，实际 %d", w.Code)
	}
}

// TestAuthRequiredValidToken 有效 token → 放行，且身份信息能被后续 handler 读到
func TestAuthRequiredValidToken(t *testing.T) {
	setup()
	r := gin.New()
	r.GET("/api/me", AuthRequired(), func(c *gin.Context) {
		// 模拟 controller 用 GetUserID/GetRole 取身份
		c.JSON(http.StatusOK, gin.H{"userID": GetUserID(c), "role": GetRole(c)})
	})

	token, _ := utils.GenerateAccessToken(5, 2) // 签发一个"用户5、编辑"的 access token
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("有效 token 应放行，实际 %d", w.Code)
	}
}

// TestRequireRole 角色权限：普通用户(1)访问后台 403，编辑(2)放行
func TestRequireRole(t *testing.T) {
	setup()
	r := gin.New()
	r.GET("/api/admin", AuthRequired(), RequireRole(model.RoleEditor), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// 用例1：role=1（普通用户）→ 403
	token1, _ := utils.GenerateAccessToken(1, model.RoleUser)
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/api/admin", nil)
	req1.Header.Set("Authorization", "Bearer "+token1)
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusForbidden {
		t.Errorf("role=1 访问后台应 403，实际 %d", w1.Code)
	}

	// 用例2：role=2（编辑）→ 200 放行
	token2, _ := utils.GenerateAccessToken(2, model.RoleEditor)
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/api/admin", nil)
	req2.Header.Set("Authorization", "Bearer "+token2)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("role=2 访问后台应放行，实际 %d", w2.Code)
	}
}
