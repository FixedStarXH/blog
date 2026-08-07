package dao

import (
	"blog-system/model"

	"gorm.io/gorm"
)

// ImageDAO 图片图库数据访问
type ImageDAO struct{}

func NewImageDAO() *ImageDAO {
	return &ImageDAO{}
}

// Create 记录图片元信息（文件本身在磁盘 uploads/ 目录）
func (d *ImageDAO) Create(db *gorm.DB, image *model.Image) error {
	return db.Create(image).Error
}

// FindAll 图片库分页：按上传时间倒序
func (d *ImageDAO) FindAll(db *gorm.DB, page, pageSize int) ([]model.Image, int64, error) {
	var images []model.Image
	var total int64

	query := db.Model(&model.Image{})
	query.Count(&total)

	err := query.
		Order("created_at desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&images).Error

	return images, total, err
}

// FindByID 按 ID 查图片（删除前判存在：GORM 的 Delete 查不到行不报错）
func (d *ImageDAO) FindByID(db *gorm.DB, id uint) (*model.Image, error) {
	var img model.Image
	err := db.First(&img, id).Error
	if err != nil {
		return nil, err
	}
	return &img, nil
}

// Delete 删除图片记录（磁盘文件由 Service 层顺带删掉）
func (d *ImageDAO) Delete(db *gorm.DB, id uint) error {
	return db.Delete(&model.Image{}, id).Error
}
