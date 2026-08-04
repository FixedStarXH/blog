package dao

import (
	"blog-system/model"
	"errors"
	"time"

	"gorm.io/gorm"
)

type ArticleDAO struct{}

func NewArticleDAO() *ArticleDAO {
	return &ArticleDAO{}
}

func (d *ArticleDAO) FindPublished(db *gorm.DB, keyword string, authorID uint, tag string, sortBy string, page, pageSize int) ([]model.Article, int64, error) {
	var articles []model.Article
	var total int64

	query := db.Model(&model.Article{}).Where("status = ?", model.ArticleStatusPublished)
	if keyword != "" {
		query = query.Where("title LIKE ? OR content LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if authorID > 0 {
		query = query.Where("author_id = ?", authorID)
	}
	if tag != "" {
		query = query.Joins("JOIN article_tags at ON at.article_id = articles.id").
			Joins("JOIN tags t ON t.id = at.tag_id").
			Where("t.name = ?", tag)
	}
	orderBy := "created_at desc"
	if sortBy == "hot" {
		orderBy = "view_count desc"
	}
	query = query.Order(orderBy)
	query.Count(&total)
	err := query.
		Preload("Author").
		Preload("Category").
		Preload("Tags").
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

// AddView 浏览量 +1（IP 防刷 + 事务）
func (d *ArticleDAO) AddView(db *gorm.DB, articleID uint, ip string) error {
	startOfDay := time.Now().Truncate(24 * time.Hour)

	var count int64
	db.Model(&model.ArticleView{}).
		Where("article_id = ? AND ip = ? AND viewed_at >= ?", articleID, ip, startOfDay).
		Count(&count)

	if count > 0 {
		return nil
	}

	return db.Transaction(func(tx *gorm.DB) error {
		// A. 插明细
		if err := tx.Create(&model.ArticleView{
			ArticleID: articleID,
			IP:        ip,
			ViewedAt:  time.Now(),
		}).Error; err != nil {
			return err
		}

		if err := tx.Model(&model.Article{}).
			Where("id = ?", articleID).
			UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error; err != nil {
			return err
		}
		return nil
	})
}

func (d *ArticleDAO) Like(db *gorm.DB, articleID uint, ip string) error {
	var count int64
	db.Model(&model.ArticleLike{}).
		Where("article_id = ? AND ip = ?", articleID, ip).
		Count(&count)

	if count > 0 {
		return db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Unscoped().Where("article_id = ? AND ip = ?", articleID, ip).
				Delete(&model.ArticleLike{}).Error; err != nil {
				return err
			}
			return tx.Model(&model.Article{}).
				Where("id = ?", articleID).
				UpdateColumn("like_count", gorm.Expr("like_count - 1")).Error
		})
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&model.ArticleLike{
			ArticleID: articleID,
			IP:        ip,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&model.Article{}).
			Where("id = ?", articleID).
			UpdateColumn("like_count", gorm.Expr("like_count + 1")).Error
	})
}

func (d *ArticleDAO) FindHot(db *gorm.DB, limit int) ([]model.Article, error) {
	var articles []model.Article

	err := db.Model(&model.Article{}).
		Where("status = ?", model.ArticleStatusPublished).
		Order("like_count desc").
		Limit(limit).
		Preload("Author").
		Preload("Category").
		Preload("Tags").
		Find(&articles).Error

	return articles, err
}
