package service

import (
	"blog-system/cache"
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
	// 标签墙数据：低频，缓存 5 分钟
	var tags []model.Tag
	if cache.Get(cache.KeyTags, &tags) {
		return tags, nil
	}
	tags, err := s.dao.FindAllWithCount(s.db)
	if err != nil {
		return nil, err
	}
	if len(tags) > 0 {
		cache.Set(cache.KeyTags, tags, cache.TTLStatic)
	}
	return tags, nil
}

// GetAdminTags 后台标签分页列表（实时性要求高，不走缓存）
func (s *TagService) GetAdminTags(keyword string, page, pageSize int) ([]model.Tag, int64, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 12
	}
	if page <= 0 {
		page = 1
	}
	return s.dao.FindPage(s.db, keyword, page, pageSize)
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
	if err := s.dao.Create(s.db, &model.Tag{Name: name}); err != nil {
		return err
	}
	cache.InvalidateTaxonomy()
	return nil
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
	// name 为空表示"不改名"，绝不能覆盖数据库里的原名
	if name != "" {
		dup, err := s.dao.FindByName(s.db, name)
		if err == nil {
			if dup.ID != exist.ID {
				return errors.New("标签名已存在")
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := s.dao.Update(s.db, id, map[string]interface{}{"name": name}); err != nil {
			return err
		}
	}
	// name 为空时无需更新（标签只有 name 一个业务字段），直接结束
	cache.InvalidateTaxonomy()
	return nil
}

// DeleteTag 删除标签：先确认存在，再清中间表 + 删标签
func (s *TagService) DeleteTag(id uint) error {
	if _, err := s.dao.FindByID(s.db, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("标签不存在")
		}
		return err
	}
	if err := s.dao.Delete(s.db, id); err != nil {
		return err
	}
	cache.InvalidateTaxonomy()
	return nil
}
