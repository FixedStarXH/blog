package service

import (
	"errors"
	"os"
	"strings"

	"blog-system/dao"
	"blog-system/model"

	"gorm.io/gorm"
)

// ImageService 图片图库业务
type ImageService struct {
	dao *dao.ImageDAO
	db  *gorm.DB
}

func NewImageService(dao *dao.ImageDAO, db *gorm.DB) *ImageService {
	return &ImageService{dao: dao, db: db}
}

// List 图片库分页（默认一页 24 张）
func (s *ImageService) List(page, pageSize int) ([]model.Image, int64, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 24
	}
	if page <= 0 {
		page = 1
	}
	return s.dao.FindAll(s.db, page, pageSize)
}

// Delete 删除图片：先删数据库记录，再删磁盘文件（记录没了文件还在 = 垃圾）
func (s *ImageService) Delete(id uint) error {
	img, err := s.dao.FindByID(s.db, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("图片不存在")
		}
		return err
	}
	if err := s.dao.Delete(s.db, id); err != nil {
		return err
	}
	// URL 形如 /uploads/xxx.jpg，去掉开头的 / 就是磁盘相对路径
	path := strings.TrimPrefix(img.URL, "/")
	if path != "" {
		os.Remove(path) // 文件不存在也不报错（幂等）
	}
	return nil
}
