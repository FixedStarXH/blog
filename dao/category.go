package dao

import (
	"blog-system/model"

	"gorm.io/gorm"
)

type CategoryDAO struct{}

func NewCategoryDAO() *CategoryDAO {
	return &CategoryDAO{}
}

func (d *CategoryDAO) FindAll(db *gorm.DB) ([]model.Category, error) {
	var categories []model.Category
	err := db.Order("id asc").Find(&categories).Error
	return categories, err
}
