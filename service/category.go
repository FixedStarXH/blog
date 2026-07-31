package service

import (
	"blog-system/dao"
	"blog-system/model"

	"gorm.io/gorm"
)

type CategoryService struct {
	dao *dao.CategoryDAO
	db  *gorm.DB
}

func NewCategoryService(dao *dao.CategoryDAO, db *gorm.DB) *CategoryService {
	return &CategoryService{dao: dao, db: db}
}

func (s *CategoryService) GetAllCategories() ([]model.Category, error) {
	return s.dao.FindAll(s.db)
}
