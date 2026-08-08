package utils

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"blog-system/config"

	"github.com/golang-jwt/jwt/v5"
)

// Claims 自定义 JWT 载荷（Payload 的内容）
//
// 登录成功后把"用户ID + 角色 + token类型"写进 token，服务器之后不用查数据库，
// 直接从 token 里读出这些值（JWT 小课第 2 节）
//
// RegisteredClaims 是官方内置字段（iss/exp/iat...），嵌入它就有 exp 过期校验
type Claims struct {
	UserID    uint   `json:"user_id"`    // 用户ID（注意 json 标签，和前端/数据库无关，纯粹 token 内部格式）
	Role      int    `json:"role"`       // 角色：1普通 2编辑 3管理员（RBAC 权限判断直接读它）
	TokenType string `json:"token_type"` // access / refresh：防止拿 refresh 冒充 access（或反过来）
	jwt.RegisteredClaims                  // 内置：exp 过期时间、iat 签发时间、jti 唯一ID
}

// 双 token 有效期设计：
//   - access token  2 小时：业务接口鉴权用。短 = 泄漏后攻击窗口小
//   - refresh token 7 天：只用于"换新 access"，不参与业务鉴权。长 = 用户不用频繁登录
const (
	AccessTokenTTL  = 2 * time.Hour
	RefreshTokenTTL = 7 * 24 * time.Hour
)

// generateToken 签发指定类型、指定有效期的 token
func generateToken(userID uint, role int, tokenType string, ttl time.Duration) (string, error) {
	claims := Claims{
		UserID:    userID,
		Role:      role,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        randomJTI(), // 唯一ID：refresh 轮换吊销时用它在 Redis 里登记/查找
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	// 用 HS256 算法创建 token（对称加密：同一个密钥既签名又验签）
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 用 config.yaml 里的 jwt.secret 给 token 签名（密钥只有服务器知道）
	return token.SignedString([]byte(config.AppConfig.JWT.Secret))
}

// GenerateAccessToken 签发 access token（业务接口鉴权，短有效期）
func GenerateAccessToken(userID uint, role int) (string, error) {
	return generateToken(userID, role, "access", AccessTokenTTL)
}

// GenerateRefreshToken 签发 refresh token（只用于刷新换新，长有效期）
func GenerateRefreshToken(userID uint, role int) (string, error) {
	return generateToken(userID, role, "refresh", RefreshTokenTTL)
}

// GenerateTokenPair 一次签发双 token（登录/注册成功后调用）
func GenerateTokenPair(userID uint, role int) (access, refresh string, err error) {
	access, err = GenerateAccessToken(userID, role)
	if err != nil {
		return "", "", err
	}
	refresh, err = GenerateRefreshToken(userID, role)
	if err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

// ParseToken 解析并校验 token：鉴权中间件/刷新接口里调用
// expectedType 传 "access" 或 "refresh"：类型不符直接拒绝
// 返回载荷（里面能取 user_id / role / jti）
func ParseToken(tokenString, expectedType string) (*Claims, error) {
	// jwt.ParseWithClaims 会自动校验 exp（过期）——这是 JWT 库的内置行为
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		// 防"算法混淆攻击"：必须校验签名算法真的是 HS256
		// 如果不校验，攻击者可以伪造一个 alg=none 或 HS512 的 token
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("非法的签名算法")
		}
		return []byte(config.AppConfig.JWT.Secret), nil
	})
	if err != nil {
		return nil, err
	}

	// token.Valid 为 true 说明签名正确且未过期
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("token 无效")
	}
	// 类型校验：access 不能冒充 refresh，反之亦然（双 token 的核心防护）
	if claims.TokenType != expectedType {
		return nil, errors.New("token 类型错误")
	}
	return claims, nil
}

// randomJTI 生成随机唯一ID（jti）：refresh token 轮换吊销的凭据
func randomJTI() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
