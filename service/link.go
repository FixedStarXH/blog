package service

import (
	"errors"

	"blog-system/dao"
	"blog-system/model"

	"gorm.io/gorm"
)

// LinkService 友情链接业务
type LinkService struct {
	dao *dao.LinkDAO
	db  *gorm.DB
}

func NewLinkService(dao *dao.LinkDAO, db *gorm.DB) *LinkService {
	return &LinkService{dao: dao, db: db}
}

func (s *LinkService) List() ([]model.Link, error) {
	return s.dao.FindAll(s.db)
}

func (s *LinkService) Create(name, url, description string, sort int, enabled bool) (*model.Link, error) {
	link := &model.Link{
		Name:        name,
		URL:         url,
		Description: description,
		Sort:        sort,
		Enabled:     enabled,
	}
	if err := s.dao.Create(s.db, link); err != nil {
		return nil, err
	}
	return link, nil
}

// Update 更新链接：先判存在（GORM 的 Updates 查不到行返回 nil 不报错）
func (s *LinkService) Update(id uint, name, url, description string, sort int, enabled bool) error {
	if _, err := s.dao.FindByID(s.db, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("链接不存在")
		}
		return err
	}
	return s.dao.Update(s.db, id, map[string]interface{}{
		"name":        name,
		"url":         url,
		"description": description,
		"sort":        sort,
		"enabled":     enabled,
	})
}

func (s *LinkService) Delete(id uint) error {
	if _, err := s.dao.FindByID(s.db, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("链接不存在")
		}
		return err
	}
	return s.dao.Delete(s.db, id)
}
