package service

import (
	"blog-system/dao"
	"blog-system/model"
	"errors"

	"gorm.io/gorm"
)

// TagService 标签业务层（唯一职责：转发 DAO 结果）
type TagService struct {
	dao *dao.TagDAO
	db  *gorm.DB
}

func NewTagService(dao *dao.TagDAO, db *gorm.DB) *TagService {
	return &TagService{dao: dao, db: db}
}

// GetAllTags 获取全部标签（含每个标签的文章数）
func (s *TagService) GetAllTags() ([]model.Tag, error) {
	return s.dao.FindAllWithCount(s.db)
}

// CreateTag 新增标签：查重
func (s *TagService) CreateTag(name string) error {
	exist, err := s.dao.FindByName(s.db, name)
	if err == nil && exist != nil {
		return errors.New("标签名已存在")
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return s.dao.Create(s.db, &model.Tag{Name: name})
}

// UpdateTag 修改标签：先确认存在（GORM 的 Updates 查不到行不报错），再查重（排除自己）
func (s *TagService) UpdateTag(id uint, name string) error {
	exist, err := s.dao.FindByID(s.db, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("标签不存在")
		}
		return err
	}
	if name != "" {
		dup, err := s.dao.FindByName(s.db, name)
		if err == nil {
			if dup.ID != exist.ID {
				return errors.New("标签名已存在")
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}
	return s.dao.Update(s.db, id, map[string]interface{}{"name": name})
}

// DeleteTag 删除标签：先确认存在，再清中间表 + 删标签
func (s *TagService) DeleteTag(id uint) error {
	if _, err := s.dao.FindByID(s.db, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("标签不存在")
		}
		return err
	}
	return s.dao.Delete(s.db, id)
}
