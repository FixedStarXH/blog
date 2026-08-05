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

func (s *ArticleService) GetHotArticles(limit int) ([]model.Article, error) {
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	return s.dao.FindHot(s.db, limit)
}

func (s *ArticleService) GetArchives() ([]model.Archive, error) {
	return s.dao.FindArchives(s.db)
}

// GetArticleNav 文章导航：上一篇 + 下一篇 + 相关推荐
func (s *ArticleService) GetArticleNav(id uint) (*model.ArticleNav, error) {
	// ① 先查当前文章，拿到它的 CategoryID 和 CreatedAt（三个查询都靠它）
	article, err := s.dao.FindByID(s.db, id)
	if err != nil {
		return nil, err
	}

	// ② 上一篇 / 下一篇（可能为 nil，代表没有了）
	prev, err := s.dao.FindPrev(s.db, article)
	if err != nil {
		return nil, err
	}
	next, err := s.dao.FindNext(s.db, article)
	if err != nil {
		return nil, err
	}

	// ③ 相关推荐：同分类热门 5 条
	related, err := s.dao.FindRelated(s.db, article, 5)
	if err != nil {
		return nil, err
	}

	// ④ 组装返回
	return &model.ArticleNav{
		Prev:    prev,
		Next:    next,
		Related: related,
	}, nil
}

// GetAdminArticles 后台文章列表（编辑+）
func (s *ArticleService) GetAdminArticles(status, page, pageSize int) ([]model.Article, int64, error) {
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 10
	}
	if page <= 0 {
		page = 1
	}
	return s.dao.FindAll(s.db, status, page, pageSize)
}

// ApproveArticle 通过审核：状态改已发布，清空驳回原因
func (s *ArticleService) ApproveArticle(id uint) error {
	return s.dao.UpdateStatus(s.db, id, model.ArticleStatusPublished, "")
}

// RejectArticle 驳回：状态改已驳回，写入原因
func (s *ArticleService) RejectArticle(id uint, reason string) error {
	return s.dao.UpdateStatus(s.db, id, model.ArticleStatusRejected, reason)
}
