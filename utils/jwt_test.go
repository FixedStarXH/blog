package utils

import (
	"testing"
	"time"

	"blog-system/config"

	"github.com/golang-jwt/jwt/v5"
)

// init 在测试前设置配置（GenerateToken/ParseToken 都要读 jwt.secret）
// 测试用固定密钥，不依赖 config.yaml
func init() {
	config.AppConfig = &config.Config{
		JWT: config.JWTConfig{Secret: "test-secret"},
	}
}

// TestGenerateAndParse 核心用例：签发 → 解析，身份信息要能还原
func TestGenerateAndParse(t *testing.T) {
	token, err := GenerateToken(1, 3)
	if err != nil {
		t.Fatalf("GenerateToken 失败: %v", err)
	}
	claims, err := ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken 失败: %v", err)
	}
	if claims.UserID != 1 || claims.Role != 3 {
		t.Errorf("解析结果不对: userID=%d role=%d, 期望 userID=1 role=3", claims.UserID, claims.Role)
	}
}

// TestParseTampered 防篡改用例：改动 token 任何一个字符，验签必须失败
// 这验证的是 JWT 的签名机制（小课第 3 节：正文一改，签名就对不上）
func TestParseTampered(t *testing.T) {
	token, err := GenerateToken(1, 1)
	if err != nil {
		t.Fatalf("GenerateToken 失败: %v", err)
	}
	// 篡改最后一个字符（破坏签名段）
	tampered := token[:len(token)-1] + "x"
	if _, err := ParseToken(tampered); err == nil {
		t.Error("篡改的 token 竟然通过了校验！签名机制失效")
	}
}

// TestParseExpired 过期用例：手工构造已过期的 token，解析必须失败
func TestParseExpired(t *testing.T) {
	claims := Claims{
		UserID: 1,
		Role:   1,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)), // 1 小时前就过期了
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("构造 token 失败: %v", err)
	}
	if _, err := ParseToken(token); err == nil {
		t.Error("过期 token 竟然通过了校验！exp 未生效")
	}
}
