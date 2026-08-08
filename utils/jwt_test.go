package utils

import (
	"testing"
	"time"

	"blog-system/config"

	"github.com/golang-jwt/jwt/v5"
)

// init 在测试前设置配置（GenerateTokenPair/ParseToken 都要读 jwt.secret）
// 测试用固定密钥，不依赖 config.yaml
func init() {
	config.AppConfig = &config.Config{
		JWT: config.JWTConfig{Secret: "test-secret"},
	}
}

// TestGenerateAndParse 核心用例：签发 → 解析，身份信息要能还原
func TestGenerateAndParse(t *testing.T) {
	access, refresh, err := GenerateTokenPair(1, 3)
	if err != nil {
		t.Fatalf("GenerateTokenPair 失败: %v", err)
	}
	claims, err := ParseToken(access, "access")
	if err != nil {
		t.Fatalf("ParseToken 失败: %v", err)
	}
	if claims.UserID != 1 || claims.Role != 3 {
		t.Errorf("解析结果不对: userID=%d role=%d, 期望 userID=1 role=3", claims.UserID, claims.Role)
	}
	// refresh 也要能解析，且类型是 refresh
	rclaims, err := ParseToken(refresh, "refresh")
	if err != nil {
		t.Fatalf("ParseToken(refresh) 失败: %v", err)
	}
	if rclaims.TokenType != "refresh" {
		t.Errorf("refresh 类型不对: %q, 期望 refresh", rclaims.TokenType)
	}
	// jti 必须存在：refresh 轮换吊销靠它
	if rclaims.ID == "" {
		t.Error("refresh token 缺少 jti（唯一ID），轮换吊销将失效")
	}
}

// TestTokenTypeMismatch 类型串用用例：access 不能当 refresh 用，反之亦然
func TestTokenTypeMismatch(t *testing.T) {
	access, _, _ := GenerateTokenPair(1, 1)
	// 拿 access 去按 refresh 解析 → 必须失败
	if _, err := ParseToken(access, "refresh"); err == nil {
		t.Error("access token 竟然通过了 refresh 校验！类型隔离失效")
	}
}

// TestParseTampered 防篡改用例：改动 token 任何一个字符，验签必须失败
// 这验证的是 JWT 的签名机制（小课第 3 节：正文一改，签名就对不上）
func TestParseTampered(t *testing.T) {
	access, _, _ := GenerateTokenPair(1, 1)
	// 篡改最后一个字符（破坏签名段）
	tampered := access[:len(access)-1] + "x"
	if _, err := ParseToken(tampered, "access"); err == nil {
		t.Error("篡改的 token 竟然通过了校验！签名机制失效")
	}
}

// TestParseExpired 过期用例：手工构造已过期的 token，解析必须失败
func TestParseExpired(t *testing.T) {
	claims := Claims{
		UserID:    1,
		Role:      1,
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)), // 1 小时前就过期了
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("构造 token 失败: %v", err)
	}
	if _, err := ParseToken(token, "access"); err == nil {
		t.Error("过期 token 竟然通过了校验！exp 未生效")
	}
}
