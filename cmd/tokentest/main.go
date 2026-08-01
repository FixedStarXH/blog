// 阶段二验证工具：手动签发/解析一个 JWT，亲眼看 token 结构和防篡改效果
// 用法：go run ./cmd/tokentest
package main

import (
	"fmt"

	"blog-system/config"
	"blog-system/utils"
)

func main() {
	// 手动初始化配置（正常项目启动时由 main 调用 config.Init()）
	if err := config.Init(); err != nil {
		panic(err)
	}

	// 1. 签发：模拟"用户1、管理员"登录成功
	token, err := utils.GenerateToken(1, 3)
	if err != nil {
		panic(err)
	}
	fmt.Println("签发的 token（复制到 https://jwt.io 可以解码看三段结构）:")
	fmt.Println(token)
	fmt.Println()

	// 2. 解析：模拟中间件验证
	claims, err := utils.ParseToken(token)
	if err != nil {
		panic(err)
	}
	fmt.Printf("解析成功 → userID=%d, role=%d（这就是中间件塞进 context 的身份）\n", claims.UserID, claims.Role)
	fmt.Println()

	// 3. 防篡改演示：改动最后一个字符，验证必须失败
	tampered := token[:len(token)-1] + "x"
	if _, err := utils.ParseToken(tampered); err != nil {
		fmt.Println("篡改后的 token 解析失败 →", err)
		fmt.Println("（防篡改机制生效：正文一改，签名就对不上）")
	}
}
