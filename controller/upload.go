package controller

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"time"

	"blog-system/utils"

	"github.com/gin-gonic/gin"
)

// 允许的图片扩展名（白名单，防上传 exe/php 等可执行文件）
var allowedExt = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".webp": true,
}

const maxUploadSize = 5 << 20 // 5MB（5 << 20 = 5 * 1024 * 1024 字节）

type UploadController struct{}

func NewUploadController() *UploadController {
	return &UploadController{}
}

// UploadImage 图片上传（登录用户可用，写文章插图是作者功能）
// POST /api/upload   Body: form-data 的 file 字段放图片
func (c *UploadController) UploadImage(ctx *gin.Context) {
	// ① 取文件：form-data 里字段名 "file"
	file, err := ctx.FormFile("file")
	if err != nil {
		utils.Fail(ctx, "请选择要上传的图片")
		return
	}

	// ② 大小校验：超过 5MB 拒绝
	if file.Size > maxUploadSize {
		utils.Fail(ctx, "图片不能超过 5MB")
		return
	}

	// ③ 扩展名白名单：从原始文件名取后缀，转小写再查表
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedExt[ext] {
		utils.Fail(ctx, "仅支持 jpg/png/gif/webp 格式")
		return
	}

	// ④ 生成全新文件名（绝不用用户文件名：防路径穿越 + 防重名覆盖）
	newName := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), randomString(8), ext)
	dir := "uploads"
	if err := os.MkdirAll(dir, 0o755); err != nil { // 目录不存在就创建
		utils.Error(ctx, "创建上传目录失败")
		return
	}
	dst := filepath.Join(dir, newName)
	if err := ctx.SaveUploadedFile(file, dst); err != nil {
		utils.Error(ctx, "保存文件失败")
		return
	}

	// ⑤ 返回访问 URL：/uploads 已被 r.Static 映射到本地目录
	utils.Success(ctx, gin.H{"url": "/uploads/" + newName})
}

// randomString 生成 n 位随机小写字母+数字（math/rand/v2 自带随机源，不用手动 seed）
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.IntN(len(letters))]
	}
	return string(b)
}
