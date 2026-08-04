package service

import (
	"blog-system/dao"
	"blog-system/model"

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
