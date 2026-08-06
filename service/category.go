package service

import (
	"blog-system/dao"
	"blog-system/model"
	"errors"
	"fmt"

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
	return s.dao.FindAllWithCount(s.db)
}

// CreateCategory 新增分类：先查重（Name 唯一），重复就拒绝
func (s *CategoryService) CreateCategory(name, description string, sort int) error {
	exist, err := s.dao.FindByName(s.db, name)
	if err == nil && exist != nil {
		return errors.New("分类名已存在")
	}
	// 查不到的 ErrRecordNotFound 是"正常情况"，放行；其他错误才是真问题
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return s.dao.Create(s.db, &model.Category{
		Name:        name,
		Description: description,
		Sort:        sort,
	})
}

// UpdateCategory 修改分类：先确认存在（GORM 的 Updates 查不到行不报错），
// 改名时查重，但查到的是"自己"就允许
func (s *CategoryService) UpdateCategory(id uint, name, description string, sort int) error {
	if _, err := s.dao.FindByID(s.db, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("分类不存在")
		}
		return err
	}
	if name != "" {
		exist, err := s.dao.FindByName(s.db, name)
		if err == nil {
			if exist.ID != id {
				return errors.New("分类名已存在")
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}
	return s.dao.Update(s.db, id, map[string]interface{}{
		"name":        name,
		"description": description,
		"sort":        sort,
	})
}

// DeleteCategory 删除分类：先确认存在，再查引用，有文章就拒绝
func (s *CategoryService) DeleteCategory(id uint) error {
	if _, err := s.dao.FindByID(s.db, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("分类不存在")
		}
		return err
	}
	count, err := s.dao.CountArticles(s.db, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("该分类下还有 %d 篇文章，无法删除", count)
	}
	return s.dao.Delete(s.db, id)
}
