package service

import (
	"crypto/subtle"
	"errors"
	"time"

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

// GetArticleByID 文章详情（公开接口）：私密文章不返回正文，标记 needPassword
func (s *ArticleService) GetArticleByID(id uint) (*model.Article, error) {
	article, err := s.dao.FindByID(s.db, id)
	if err != nil {
		return nil, err
	}
	// 私密文章：告诉前端"需要密码"，并隐藏正文（未解锁前不泄露）
	article.NeedPassword = article.Password != ""
	if article.NeedPassword {
		article.Content = ""
	}
	return article, nil
}

func (s *ArticleService) CreateArticle(authorID, categoryID uint, title, content, summary, coverImage, password string, publishAt *time.Time, tagIDs []uint) (*model.Article, error) {
	tags := make([]model.Tag, 0, len(tagIDs))
	for _, id := range tagIDs {
		tags = append(tags, model.Tag{BaseModel: model.BaseModel{ID: id}})
	}
	article := &model.Article{
		Title:      title,
		Content:    content,
		Summary:    summary,
		CoverImage: coverImage,
		Password:   password,  // 私密文章密码；空=公开
		PublishAt:  publishAt, // 作者可设定时发布；nil=不排期，审核通过立即发布
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

func (s *ArticleService) UpdateMyArticle(id, authorID, categoryID uint, title, content, summary, coverImage, password string, publishAt *time.Time, tagIDs []uint) error {
	updates := map[string]interface{}{
		"title":       title,
		"content":     content,
		"summary":     summary,
		"cover_image": coverImage,
		"category_id": categoryID,
		"password":    password,  // 修改私密文章密码；空=变回公开
		"publish_at":  publishAt, // 修改排期时间；nil=不排期
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

// ApproveArticle 通过审核：先确认文章存在，
// 再判断：设了未来 PublishAt → 转已排期(4) 等定时任务；否则立即发布(1)
func (s *ArticleService) ApproveArticle(id uint) error {
	article, err := s.dao.FindByID(s.db, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("文章不存在")
		}
		return err
	}
	// 定时发布：PublishAt 是未来时间 → 已排期，由 scheduler 每 30 秒扫描后自动转发布
	if article.PublishAt != nil && article.PublishAt.After(time.Now()) {
		return s.dao.UpdateStatus(s.db, id, model.ArticleStatusScheduled, "")
	}
	// 没设时间或时间已过 → 立即发布
	return s.dao.UpdateStatus(s.db, id, model.ArticleStatusPublished, "")
}

// RejectArticle 驳回：先确认文章存在，再状态改已驳回、写入原因
func (s *ArticleService) RejectArticle(id uint, reason string) error {
	if _, err := s.dao.FindByID(s.db, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("文章不存在")
		}
		return err
	}
	return s.dao.UpdateStatus(s.db, id, model.ArticleStatusRejected, reason)
}

// UnlockArticle 私密文章解锁：密码正确返回全文
// 用 crypto/subtle 恒定时间比较，防时序攻击（响应时间与"错几位"无关）
func (s *ArticleService) UnlockArticle(id uint, password string) (*model.Article, error) {
	article, err := s.dao.FindByID(s.db, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("文章不存在")
		}
		return nil, err
	}
	// 公开文章：直接返回全文（解锁接口对公开文章无害）
	if article.Password == "" {
		return article, nil
	}
	// 私密文章：恒定时间比较密码
	if subtle.ConstantTimeCompare([]byte(password), []byte(article.Password)) != 1 {
		return nil, errors.New("密码错误")
	}
	return article, nil
}
