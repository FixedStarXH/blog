package dao

import (
	"blog-system/model"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 业务哨兵错误：用于在 DAO 内部区分"正常业务分支"与"真实错误"
var (
	errAlreadyViewed = errors.New("该IP当天已浏览过") // AddView：唯一约束命中，视为已访问
	errAlreadyLiked  = errors.New("已点过赞")      // Like：唯一约束命中，视为已点赞
)

// isDuplicateEntry 判断错误是否为 MySQL 唯一键冲突（Duplicate entry，错误码 1062）
// 用途：点赞/浏览的防重不再靠"先查后插"（有竞态），而是靠数据库唯一索引兜底——
// 并发下只有一个请求插入成功，其余命中 Duplicate entry，在这里识别并转入业务分支
func isDuplicateEntry(err error) bool {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1062
	}
	return false
}

type ArticleDAO struct{}

func NewArticleDAO() *ArticleDAO {
	return &ArticleDAO{}
}

// FindPublished 已发布文章列表
// featured=true 时走首页精选：ORDER BY RAND(seed) 随机展示。
// seed 由前端每次进入页面生成一次：同一次访问内翻页顺序稳定不重叠，每次刷新（新种子）顺序不同。
func (d *ArticleDAO) FindPublished(db *gorm.DB, keyword string, authorID uint, tag string, categoryID uint, sortBy string, page, pageSize int, featured bool, seed int64) ([]model.Article, int64, error) {
	var articles []model.Article
	var total int64

	// 列表不返回正文 Content（longtext 可达 200KB/篇）：
	// 前端列表只显示标题/摘要/封面，排除正文能大幅减小响应体积和 Redis 缓存占用。
	// 正文只在详情接口（FindPublishedByID）返回。
	query := db.Model(&model.Article{}).Omit("content").Where("status = ?", model.ArticleStatusPublished)
	if featured {
		query = query.Where("is_featured = ?", true)
	}
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
	if featured {
		// 首页精选随机抽取：每次刷新首页展示不同的精选文章（池子=后台勾选 is_featured 的文章）
		// RAND(seed)：同一种子下顺序确定，保证同一访问内翻页不重叠；seed 每次进入页面都不同
		orderBy = fmt.Sprintf("RAND(%d)", seed)
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

// FindPublishedByID 按 ID 查【已发布】文章（公开接口专用）
// 区别 FindByID：多了 status 过滤，草稿/待审核/已驳回/已排期文章前台一律查不到，
// 防止游客遍历 ID 读到未发布的正文（IDOR 漏洞修复点）
func (d *ArticleDAO) FindPublishedByID(db *gorm.DB, id uint) (*model.Article, error) {
	var article model.Article
	err := db.Preload("Author").Preload("Category").Preload("Tags").
		Where("status = ?", model.ArticleStatusPublished).
		First(&article, id).Error
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
//
// 并发安全设计（修复"先查后插"竞态）：
// 旧写法在事务外先 Count 判断"今天是否访问过"，两个并发请求都能通过 count==0 的检查，
// 各自插一条明细并 +1，导致重复计数。新写法在事务内先 SELECT ... FOR UPDATE
// 锁住文章的计数行（悲观锁）：同一篇文章的浏览请求被串行化，
// 后到的请求在锁内重新查明细，发现已有记录就不再计数。
// 注意：这是 Redis 缓存不可用时的降级路径，流量低，悲观锁开销可忽略。
func (d *ArticleDAO) AddView(db *gorm.DB, articleID uint, ip string) (int64, error) {
	// 当天零点：不能用 time.Now().Truncate(24*time.Hour)——
	// 它按 UTC 对齐，在 +08 时区会截到本地 08:00，导致 0:00-8:00 的访问漏判去重。
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var current int64

	err := db.Transaction(func(tx *gorm.DB) error {
		// ① 锁住文章行（FOR UPDATE）：把并发浏览串行化，锁内再判断
		var article model.Article
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "view_count").
			First(&article, articleID).Error; err != nil {
			return err
		}
		current = int64(article.ViewCount)

		// ② 锁内查"今天是否已访问过"（此时并发请求都在排队，结果可靠）
		var count int64
		if err := tx.Model(&model.ArticleView{}).
			Where("article_id = ? AND ip = ? AND viewed_at >= ?", articleID, ip, startOfDay).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return nil // 已访问过：不加数，current 已取到当前值
		}

		// ③ 首次访问：插明细 + 计数 +1（同一事务，要么都成功要么都回滚）
		if err := tx.Create(&model.ArticleView{
			ArticleID: articleID,
			IP:        ip,
			ViewedAt:  now,
		}).Error; err != nil {
			return err
		}
		current++
		return tx.Model(&model.Article{}).
			Where("id = ?", articleID).
			UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
	})
	if err != nil {
		return 0, err
	}
	return current, nil
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

// Like 点赞/取消点赞（幂等 + 并发安全）
//
// 设计要点（修复旧版两个问题）：
//   - 旧版在事务外先 Count 判断"是否已赞"，两个并发"取消"请求都能读到 count>0，
//     都执行 DELETE + like_count-1，第二个 DELETE 删 0 行照样减 1，计数被多减。
//     新版：先幂等 DELETE，用 RowsAffected 判断"是否真的取消了一行"，删 0 行就不减。
//   - 减计数带下限 GREATEST(like_count-1, 0)：即使计数初始不一致，也永远不会变负数。
//   - 点赞靠 (article_id, ip) 联合唯一索引兜底并发：并发点赞只有一个插入成功，
//     其余命中 Duplicate entry（isDuplicateEntry）→ 视为已点赞，不重复加数。
func (d *ArticleDAO) Like(db *gorm.DB, articleID uint, ip string) (*LikeResult, error) {
	// 先尝试取消（幂等）：DELETE 返回实际删除的行数
	res := db.Unscoped().
		Where("article_id = ? AND ip = ?", articleID, ip).
		Delete(&model.ArticleLike{})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected > 0 {
		// 确实取消了一行 → 减计数（带下限，防并发下多减成负数）
		// 用 CASE WHEN 而非 GREATEST：语义一致，且 SQLite（测试库）不支持 GREATEST
		if err := db.Model(&model.Article{}).
			Where("id = ?", articleID).
			UpdateColumn("like_count", gorm.Expr("CASE WHEN like_count > 0 THEN like_count - 1 ELSE 0 END")).Error; err != nil {
			return nil, err
		}
		var v int64
		db.Model(&model.Article{}).Where("id = ?", articleID).Select("like_count").Scan(&v)
		return &LikeResult{LikeCount: v, Already: true}, nil
	}

	// 没取消到任何行 → 尝试点赞：唯一索引兜底，Duplicate 说明别人刚点过
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&model.ArticleLike{
			ArticleID: articleID,
			IP:        ip,
		}).Error; err != nil {
			if isDuplicateEntry(err) {
				return errAlreadyLiked
			}
			return err
		}
		return tx.Model(&model.Article{}).
			Where("id = ?", articleID).
			UpdateColumn("like_count", gorm.Expr("like_count + 1")).Error
	})
	if err != nil {
		if errors.Is(err, errAlreadyLiked) {
			// 并发下已有人点赞：视为已点赞，不重复加数
			var v int64
			db.Model(&model.Article{}).Where("id = ?", articleID).Select("like_count").Scan(&v)
			return &LikeResult{LikeCount: v, Already: true}, nil
		}
		return nil, err
	}
	var v int64
	db.Model(&model.Article{}).Where("id = ?", articleID).Select("like_count").Scan(&v)
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

// DeleteBatch 批量软删除：同一事务内先清多对多中间表再删主表，
// 避免残留 article_tags 死引用（软删除的文章仍占着标签关联记录）
func (d *ArticleDAO) DeleteBatch(db *gorm.DB, ids []uint) (int64, error) {
	var affected int64
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM article_tags WHERE article_id IN ?", ids).Error; err != nil {
			return err
		}
		result := tx.Where("id IN ?", ids).Delete(&model.Article{})
		affected = result.RowsAffected
		return result.Error
	})
	return affected, err
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
