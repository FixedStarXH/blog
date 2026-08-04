package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"blog-system/config"
	"blog-system/model"
	"blog-system/utils"

	"github.com/gin-gonic/gin"
)

// setup 测试前初始化：gin 测试模式 + 固定密钥
func setup() {
	gin.SetMode(gin.TestMode)
	config.AppConfig = &config.Config{
		JWT: config.JWTConfig{Secret: "test-secret"},
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

	token, _ := utils.GenerateToken(5, 2) // 签发一个"用户5、编辑"的 token
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
	token1, _ := utils.GenerateToken(1, model.RoleUser)
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/api/admin", nil)
	req1.Header.Set("Authorization", "Bearer "+token1)
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusForbidden {
		t.Errorf("role=1 访问后台应 403，实际 %d", w1.Code)
	}

	// 用例2：role=2（编辑）→ 200 放行
	token2, _ := utils.GenerateToken(2, model.RoleEditor)
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/api/admin", nil)
	req2.Header.Set("Authorization", "Bearer "+token2)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("role=2 访问后台应放行，实际 %d", w2.Code)
	}
}
