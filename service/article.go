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

func (s *ArticleService) GetPublishedArticles(keyword string, authorID uint, tag, sortBy string, page, pageSize int) ([]model.Article, int64, error) {
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 10
	}
	if page <= 0 {
		page = 1
	}
	return s.dao.FindPublished(s.db, keyword, authorID, tag, sortBy, page, pageSize)

}

func (s *ArticleService) GetArticleByID(id uint) (*model.Article, error) {
	return s.dao.FindByID(s.db, id)
}

func (s *ArticleService) CreateArticle(authorID, categoryID uint, title, content, summary, coverImage string, tagIDs []uint) (*model.Article, error) {
	tags := make([]model.Tag, 0, len(tagIDs))
	for _, id := range tagIDs {
		tags = append(tags, model.Tag{BaseModel: model.BaseModel{ID: id}})
	}
	article := &model.Article{
		Title:      title,
		Content:    content,
		Summary:    summary,
		CoverImage: coverImage,
		Status:     model.ArticleStatusPending,
		AuthorID:   authorID,
		CategoryID: categoryID,
		Tags:       tags,
	}

	if err := s.dao.Create(s.db, article); err != nil {
		return nil, err
	}
	return article, nil
}

func (s *ArticleService) GetMyArticles(authorID uint, status, page, pageSize int) ([]model.Article, int64, error) {
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 10
	}
	if page <= 0 {
		page = 1
	}
	return s.dao.FindByAuthorID(s.db, authorID, status, page, pageSize)
}

func (s *ArticleService) GetMyArticleDetail(id, authorID uint) (*model.Article, error) {
	return s.dao.FindByIDAndAuthor(s.db, id, authorID)
}

func (s *ArticleService) UpdateMyArticle(id, authorID, categoryID uint, title, content, summary, coverImage string, tagIDs []uint) error {
	updates := map[string]interface{}{
		"title":       title,
		"content":     content,
		"summary":     summary,
		"cover_image": coverImage,
		"category_id": categoryID,
	}
	if err := s.dao.Update(s.db, id, authorID, updates); err != nil {
		return err
	}

	article := &model.Article{BaseModel: model.BaseModel{ID: id}}
	tags := make([]model.Tag, 0, len(tagIDs))
	for _, tid := range tagIDs {
		tags = append(tags, model.Tag{BaseModel: model.BaseModel{ID: tid}})

	}
	return s.db.Model(article).Association("Tags").Replace(tags)
}

func (s *ArticleService) DeleteMyArticle(id, authorID uint) error {
	return s.dao.Delete(s.db, id, authorID)
}

func (s *ArticleService) AddView(articleID uint, ip string) error {
	return s.dao.AddView(s.db, articleID, ip)
}

func (s *ArticleService) Like(articleID uint, ip string) error {
	return s.dao.Like(s.db, articleID, ip)
}
