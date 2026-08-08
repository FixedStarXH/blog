package dao

import (
	"blog-system/model"
	"errors"
	"sort"
	"time"

	"gorm.io/gorm"
)

type ArticleDAO struct{}

func NewArticleDAO() *ArticleDAO {
	return &ArticleDAO{}
}

func (d *ArticleDAO) FindPublished(db *gorm.DB, keyword string, authorID uint, tag string, categoryID uint, sortBy string, page, pageSize int) ([]model.Article, int64, error) {
	var articles []model.Article
	var total int64

	query := db.Model(&model.Article{}).Where("status = ?", model.ArticleStatusPublished)
	if keyword != "" {
		query = query.Where("title LIKE ? OR content LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if authorID > 0 {
		query = query.Where("author_id = ?", authorID)
	}
	// 分类过滤：categoryId > 0 时按分类筛选
	if categoryID > 0 {
		query = query.Where("category_id = ?", categoryID)
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
	// Preload Tags：文章详情页要展示标签（缺这行会导致详情接口 tags 永远为空）
	err := db.Preload("Author").Preload("Category").Preload("Tags").First(&article, id).Error
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
func (d *ArticleDAO) AddView(db *gorm.DB, articleID uint, ip string) (int64, error) {
	startOfDay := time.Now().Truncate(24 * time.Hour)

	var count int64
	db.Model(&model.ArticleView{}).
		Where("article_id = ? AND ip = ? AND viewed_at >= ?", articleID, ip, startOfDay).
		Count(&count)

	if count > 0 {
		// 同 IP 当天已访问过：不重复+1，但仍返回当前浏览量
		var v int64
		db.Model(&model.Article{}).Where("id = ?", articleID).
			Select("view_count").Scan(&v)
		return v, nil
	}

	err := db.Transaction(func(tx *gorm.DB) error {
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
	if err != nil {
		return 0, err
	}
	var v int64
	db.Model(&model.Article{}).Where("id = ?", articleID).
		Select("view_count").Scan(&v)
	return v, nil
}

// AddViewDelta 浏览量增量刷库（Redis 定时任务调用）：view_count = view_count + delta
func (d *ArticleDAO) AddViewDelta(db *gorm.DB, articleID uint, delta int64) error {
	return db.Model(&model.Article{}).
		Where("id = ?", articleID).
		UpdateColumn("view_count", gorm.Expr("view_count + ?", delta)).Error
}

// LikeResult 点赞结果：新点赞数 + 是否当前用户已点赞
type LikeResult struct {
	LikeCount int64 `json:"likeCount"`
	Already   bool  `json:"already"` // true=已点过赞（本次是取消）
}

func (d *ArticleDAO) Like(db *gorm.DB, articleID uint, ip string) (*LikeResult, error) {
	var count int64
	db.Model(&model.ArticleLike{}).
		Where("article_id = ? AND ip = ?", articleID, ip).
		Count(&count)

	if count > 0 {
		// 已点过 → 取消点赞
		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Unscoped().Where("article_id = ? AND ip = ?", articleID, ip).
				Delete(&model.ArticleLike{}).Error; err != nil {
				return err
			}
			return tx.Model(&model.Article{}).
				Where("id = ?", articleID).
				UpdateColumn("like_count", gorm.Expr("like_count - 1")).Error
		})
		if err != nil {
			return nil, err
		}
		var v int64
		db.Model(&model.Article{}).Where("id = ?", articleID).
			Select("like_count").Scan(&v)
		return &LikeResult{LikeCount: v, Already: true}, nil
	}

	err := db.Transaction(func(tx *gorm.DB) error {
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
	if err != nil {
		return nil, err
	}
	var v int64
	db.Model(&model.Article{}).Where("id = ?", articleID).
		Select("like_count").Scan(&v)
	return &LikeResult{LikeCount: v, Already: false}, nil
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

// FindArchives 时间归档：按年份分组，返回年份倒序
// 前端契约（archive.html）：[{year, articles}]，所以按年分组而不是按月
func (d *ArticleDAO) FindArchives(db *gorm.DB) ([]model.Archive, error) {
	//查全部已发布文章（带作者/分类/标签，Preload）
	var articles []model.Article
	err := db.Model(&model.Article{}).
		Where("status = ?", model.ArticleStatusPublished).
		Order("created_at desc").
		Preload("Author").
		Preload("Category").
		Preload("Tags").
		Find(&articles).Error
	if err != nil {
		return nil, err
	}

	//用 map 按年份分组
	groups := make(map[string][]model.Article)

	for i := range articles {
		year := articles[i].CreatedAt.Format("2006")
		groups[year] = append(groups[year], articles[i])
	}
	archives := make([]model.Archive, 0, len(groups))
	for year, articleList := range groups {
		archives = append(archives, model.Archive{
			Year:     year,
			Count:    len(articleList),
			Articles: articleList,
		})
	}
	sort.Slice(archives, func(i, j int) bool {
		return archives[i].Year > archives[j].Year
	})

	return archives, nil
}

// FindPrev 上一篇：同分类、已发布、时间比当前早的最近一篇
func (d *ArticleDAO) FindPrev(db *gorm.DB, article *model.Article) (*model.AdjacentArticle, error) {
	var prev model.AdjacentArticle
	// result 接收整个查询结果（不只是 Error）
	result := db.Model(&model.Article{}).
		Select("id, title").
		Where("category_id = ? AND status = ? AND created_at < ?", article.CategoryID, model.ArticleStatusPublished, article.CreatedAt).
		Order("created_at desc").
		Limit(1).
		Scan(&prev)
	if result.Error != nil {
		return nil, result.Error
	}
	// RowsAffected = 实际查到的行数；0 = 没有上一篇 → 返回 nil（JSON 输出 null）
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &prev, nil
}

// FindNext 下一篇：同分类、已发布、时间比当前晚的最近一篇
func (d *ArticleDAO) FindNext(db *gorm.DB, article *model.Article) (*model.AdjacentArticle, error) {
	var next model.AdjacentArticle
	result := db.Model(&model.Article{}).
		Select("id, title").
		Where("category_id = ? AND status = ? AND created_at > ?", article.CategoryID, model.ArticleStatusPublished, article.CreatedAt). // ① > 方向反了
		Order("created_at asc").                                                                                                         // ② asc 升序
		Limit(1).
		Scan(&next)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &next, nil
}

// FindRelated 相关推荐：同分类的其他文章，按浏览量倒序
func (d *ArticleDAO) FindRelated(db *gorm.DB, article *model.Article, limit int) ([]model.Article, error) {
	var related []model.Article
	err := db.Model(&model.Article{}).
		Where("category_id = ? AND id != ? AND status = ?", article.CategoryID, article.ID, model.ArticleStatusPublished).
		Order("view_count desc").
		Limit(limit).
		Preload("Author").
		Preload("Category").
		Preload("Tags").
		Find(&related).Error
	return related, err
}

// FindAll 后台文章管理：按状态/关键词/分类筛选 + 分页（不校验作者，审核员可见所有投稿）
func (d *ArticleDAO) FindAll(db *gorm.DB, status int, keyword string, categoryID uint, page, pageSize int) ([]model.Article, int64, error) {
	var articles []model.Article
	var total int64

	query := db.Model(&model.Article{})
	if status >= 0 {
		query = query.Where("status = ?", status)
	}
	if keyword != "" {
		query = query.Where("title LIKE ? OR content LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if categoryID > 0 {
		query = query.Where("category_id = ?", categoryID)
	}
	query.Count(&total)

	err := query.
		Preload("Author").
		Preload("Category").
		Preload("Tags").
		Order("created_at desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&articles).Error

	return articles, total, err
}

// UpdateStatus 后台审核：直接改状态（不校验作者；驳回时写原因）
func (d *ArticleDAO) UpdateStatus(db *gorm.DB, id uint, status int, rejectReason string) error {
	return db.Model(&model.Article{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":        status,
			"reject_reason": rejectReason, // 通过时传空串，驳回时传原因
		}).Error
}

// UpdateStatusBatch 批量改状态（后台批量发布/草稿/驳回用）
func (d *ArticleDAO) UpdateStatusBatch(db *gorm.DB, ids []uint, status int) (int64, error) {
	result := db.Model(&model.Article{}).
		Where("id IN ?", ids).
		Update("status", status)
	return result.RowsAffected, result.Error
}

// UpdateTopBatch 批量改置顶（后台批量置顶/取消置顶用）
func (d *ArticleDAO) UpdateTopBatch(db *gorm.DB, ids []uint, isTop bool) (int64, error) {
	result := db.Model(&model.Article{}).
		Where("id IN ?", ids).
		Update("is_top", isTop)
	return result.RowsAffected, result.Error
}

// DeleteBatch 批量删除（后台批量删除用，软删除）
func (d *ArticleDAO) DeleteBatch(db *gorm.DB, ids []uint) (int64, error) {
	result := db.Where("id IN ?", ids).Delete(&model.Article{})
	return result.RowsAffected, result.Error
}

// UpdateByID 按 ID 直接更新（后台文章编辑用，不校验作者）
// 注意：updates 是 map 才能更新"零值"（如 is_top=false），struct 的零值会被 GORM 忽略
func (d *ArticleDAO) UpdateByID(db *gorm.DB, id uint, updates map[string]interface{}) error {
	return db.Model(&model.Article{}).Where("id = ?", id).Updates(updates).Error
}

// PublishScheduled 定时任务核心：把已到期的排期文章转已发布
// 条件：status=4(已排期) 且 publish_at 已到（<= 现在）
func (d *ArticleDAO) PublishScheduled(db *gorm.DB) (int64, error) {
	result := db.Model(&model.Article{}).
		Where("status = ? AND publish_at IS NOT NULL AND publish_at <= ?",
			model.ArticleStatusScheduled, time.Now()).
		Update("status", model.ArticleStatusPublished)
	return result.RowsAffected, result.Error
}
