package service

import (
	"blog-system/dao"
	"blog-system/model"

	"gorm.io/gorm"
)

type ArticleService struct {
	dao *dao.ArticleDAO
	db  *gorm.DB
}

func NewArticleService(dao *dao.ArticleDAO, db *gorm.DB) *ArticleService {
	return &ArticleService{dao: dao, db: db}
}

func (s *ArticleService) GetPublishedArticles(page, pageSize int) ([]model.Article, int64, error) {
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 10
	}
	if page <= 0 {
		page = 1
	}
	return s.dao.FindPublished(s.db, page, pageSize)

}

func (s *ArticleService) GetArticleByID(id uint) (*model.Article, error) {
	return s.dao.FindByID(s.db, id)
}
