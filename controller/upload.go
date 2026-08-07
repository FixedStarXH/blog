package controller

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"time"

	"blog-system/dao"
	"blog-system/middleware"
	"blog-system/model"
	"blog-system/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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

type UploadController struct {
	imageDAO *dao.ImageDAO // 上传成功后记录到图库表（后台图库管理用）
	db       *gorm.DB
}

func NewUploadController(imageDAO *dao.ImageDAO, db *gorm.DB) *UploadController {
	return &UploadController{imageDAO: imageDAO, db: db}
}

// UploadImage 图片上传（登录用户可用）
// POST /api/upload 或 POST /api/admin/upload  Body: form-data 的 file 字段放图片
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

	// ⑤ 记录到图库表（元信息入库，后台图库页靠它列出所有图）
	url := "/uploads/" + newName
	_ = c.imageDAO.Create(c.db, &model.Image{
		Name:       file.Filename,                        // 原始文件名（展示用）
		URL:        url,                                  // 访问路径
		Size:       file.Size,                            // 字节大小
		Ext:        strings.TrimPrefix(ext, "."),         // 去掉点的扩展名
		UploaderID: middleware.GetUserID(ctx),            // 上传者（admin 组必登录，可拿 ID）
	})

	// ⑥ 返回访问 URL：/uploads 已被 r.Static 映射到本地目录
	utils.Success(ctx, gin.H{"url": url})
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
