package utils

import (
	"errors"
	"time"

	"blog-system/config"

	"github.com/golang-jwt/jwt/v5"
)

// Claims 自定义 JWT 载荷（Payload 的内容）
//
// 登录成功后把"用户ID + 角色"写进 token，服务器之后不用查数据库，
// 直接从 token 里读出这两个值（JWT 小课第 2 节）
//
// RegisteredClaims 是官方内置字段（iss/exp/iat...），嵌入它就有 exp 过期校验
type Claims struct {
	UserID               uint `json:"user_id"` // 用户ID（注意 json 标签，和前端/数据库无关，纯粹 token 内部格式）
	Role                 int  `json:"role"`    // 角色：1普通 2编辑 3管理员（RBAC 权限判断直接读它）
	jwt.RegisteredClaims      // 内置：exp 过期时间、iat 签发时间
}

// tokenTTL token 有效期：72 小时
// 为什么是 72 小时？博客不是银行，一次登录用几天合理；
// 用户被踢下线只能等过期（JWT 特性），时间太长有风险、太短老要重新登录
const tokenTTL = 72 * time.Hour

// GenerateToken 签发 token：登录成功后调用
// 入参：用户ID + 角色；出参：token 字符串
func GenerateToken(userID uint, role int) (string, error) {
	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			// 过期时间：当前时间 + 72 小时（Unix 秒）
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenTTL)),
			// 签发时间：当前时间
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
	}

	// 用 HS256 算法创建 token（对称加密：同一个密钥既签名又验签）
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 用 config.yaml 里的 jwt.secret 给 token 签名（密钥只有服务器知道）
	return token.SignedString([]byte(config.AppConfig.JWT.Secret))
}

// ParseToken 解析并校验 token：鉴权中间件里调用
// 返回载荷（里面能取 user_id / role）
func ParseToken(tokenString string) (*Claims, error) {
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
	return claims, nil
}
