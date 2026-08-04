package dao

import (
	"blog-system/model"
	"errors"

	"gorm.io/gorm"
)

type ArticleDAO struct{}

func NewArticleDAO() *ArticleDAO {
	return &ArticleDAO{}
}

func (d *ArticleDAO) FindPublished(db *gorm.DB, page, pageSize int) ([]model.Article, int64, error) {
	var articles []model.Article
	var total int64

	db.Model(&model.Article{}).Where("status = ?", 1).Count(&total)

	err := db.Where("status = ?", 1).
		Preload("Author").
		Preload("Category").
		Order("created_at desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&articles).Error

	return articles, total, err
}

func (d *ArticleDAO) FindByID(db *gorm.DB, id uint) (*model.Article, error) {
	var article model.Article
	err := db.Preload("Author").Preload("Category").First(&article, id).Error
	if err != nil {
		return nil, err
	}
	return &article, nil
}

func (d *ArticleDAO) FindByAuthorID(db *gorm.DB, authorID uint, status, page, pageSize int) ([]model.Article, int64, error) {
	var articles []model.Article
	var total int64

	query := db.Model(&model.Article{}).Where("author_id = ?", authorID)

	if status >= 0 {
		query = query.Where("status = ?", status)
	}
	query.Count(&total)

	err := query.
		Preload("Category").
		Preload("Tags").
		Order("created_at desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&articles).Error

	return articles, total, err
}

func (d *ArticleDAO) Create(db *gorm.DB, article *model.Article) error {
	return db.Create(article).Error
}

var ErrNotAuthor = errors.New("无权操作该文章")

func (d *ArticleDAO) FindByIDAndAuthor(db *gorm.DB, id, authorID uint) (*model.Article, error) {
	var article model.Article

	err := db.Preload("Category").Preload("Tags").First(&article, id).Error
	if err != nil {
		return nil, err
	}
	if article.AuthorID != authorID {
		return nil, ErrNotAuthor
	}
	return &article, nil
}

func (d *ArticleDAO) Update(db *gorm.DB, id, authorID uint, updates map[string]interface{}) error {
	if _, err := d.FindByIDAndAuthor(db, id, authorID); err != nil {
		return err
	}
	return db.Model(&model.Article{}).Where("id = ?", id).Updates(updates).Error
}
func (d *ArticleDAO) Delete(db *gorm.DB, id, authorID uint) error {
	if _, err := d.FindByIDAndAuthor(db, id, authorID); err != nil {
		return err
	}
	return db.Delete(&model.Article{}, id).Error
}
