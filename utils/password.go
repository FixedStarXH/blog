package utils

import "golang.org/x/crypto/bcrypt"

// HashPassword 密码加密（BCrypt）
// 注册/改密时调用，把明文密码变成不可逆的哈希存库
// 为什么用 BCrypt？
//  1. 自带"盐"：即使两人密码相同，哈希也不同，防彩虹表破解
//  2. 故意慢：单次几十毫秒，暴力破解成本指数上升
//  3. 不可逆：数据库泄露也拿不回明文
func HashPassword(plain string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// CheckPassword 密码校验（登录时调用）
// 返回 true = 密码正确。注意：不是"解密对比"，
// 而是把明文 + 库里存的盐 重新算一遍哈希再比较（bcrypt.CompareHashAndPassword 封装了这个过程）
func CheckPassword(hashed, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain)) == nil
}
