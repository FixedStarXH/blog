package dao

import (
	"blog-system/model"

	"gorm.io/gorm"
)

// LinkDAO 友情链接数据访问
type LinkDAO struct{}

func NewLinkDAO() *LinkDAO {
	return &LinkDAO{}
}

func (d *LinkDAO) Create(db *gorm.DB, link *model.Link) error {
	return db.Create(link).Error
}

// FindAll 全部链接：按 sort 升序、id 升序（后台管理用；前台可用同一方法只取 enabled 的）
func (d *LinkDAO) FindAll(db *gorm.DB) ([]model.Link, error) {
	var links []model.Link
	err := db.Order("sort asc, id asc").Find(&links).Error
	return links, err
}

// FindByID 按 ID 查链接（更新/删除前判存在）
func (d *LinkDAO) FindByID(db *gorm.DB, id uint) (*model.Link, error) {
	var link model.Link
	err := db.First(&link, id).Error
	if err != nil {
		return nil, err
	}
	return &link, nil
}

// Update 更新链接（map 形式：enabled=false 这种"零值"也能更新）
func (d *LinkDAO) Update(db *gorm.DB, id uint, updates map[string]interface{}) error {
	return db.Model(&model.Link{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// Delete 删除链接（软删除）
func (d *LinkDAO) Delete(db *gorm.DB, id uint) error {
	return db.Delete(&model.Link{}, id).Error
}
