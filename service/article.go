package service

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"time"

	"blog-system/cache"
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

// articleListCache 文章列表缓存结构（list + total 一起缓存，避免命中时丢总数）
type articleListCache struct {
	List  []model.Article `json:"list"`
	Total int64           `json:"total"`
}

func (s *ArticleService) GetPublishedArticles(keyword string, authorID uint, tag string, categoryID uint, sortBy string, page, pageSize int) ([]model.Article, int64, error) {
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 10
	}
	if page <= 0 {
		page = 1
	}
	// 缓存 key：把筛选参数全拼进去（不同筛选条件 = 不同缓存条目）
	key := fmt.Sprintf("%s%d:%d:%s:%d:%s:%d:%d",
		cache.KeyArticleList, page, pageSize, tag, categoryID, sortBy, authorID, len(keyword))

	// ① 读缓存
	var cached articleListCache
	if cache.Get(key, &cached) {
		return cached.List, cached.Total, nil
	}

	// ② 缓存未命中 → 查数据库
	articles, total, err := s.dao.FindPublished(s.db, keyword, authorID, tag, categoryID, sortBy, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	// ③ 写缓存（短 TTL：新文章 1 分钟内可见）
	if len(articles) > 0 {
		cache.Set(key, articleListCache{List: articles, Total: total}, cache.TTLList)
	}
	return articles, total, nil
}

// GetArticleByID 文章详情（公开接口）：私密文章不返回正文，标记 needPassword
// 缓存策略：详情缓存 5 分钟；阅读量不缓存（读 Redis 实时值覆盖，保证计数即时可见）
func (s *ArticleService) GetArticleByID(id uint) (*model.Article, error) {
	key := cache.KeyArticle + fmt.Sprint(id)

	// ① 读缓存
	var article model.Article
	if cache.Get(key, &article) {
		// 私密文章：缓存里已是脱敏内容，直接返回
		// 阅读量实时化：Redis 里有值就覆盖（没有说明 Redis 刚起步，用缓存里的）
		if v := cache.GetViewCount(id); v >= 0 {
			article.ViewCount = int(v)
		}
		return &article, nil
	}

	// ② 缓存未命中 → 查数据库
	a, err := s.dao.FindByID(s.db, id)
	if err != nil {
		return nil, err
	}
	// 私密文章：告诉前端"需要密码"，并隐藏正文（未解锁前不泄露）
	a.NeedPassword = a.Password != ""
	if a.NeedPassword {
		a.Content = ""
	}
	// ③ 写缓存（文章基本不常变，5 分钟足够）
	cache.Set(key, a, cache.TTLDetail)
	return a, nil
}

func (s *ArticleService) CreateArticle(authorID, categoryID uint, title, content, summary, coverImage, sourceURL, password string, publishAt *time.Time, tagIDs []uint) (*model.Article, error) {
	tags := make([]model.Tag, 0, len(tagIDs))
	for _, id := range tagIDs {
		tags = append(tags, model.Tag{BaseModel: model.BaseModel{ID: id}})
	}
	article := &model.Article{
		Title:      title,
		Content:    content,
		Summary:    summary,
		CoverImage: coverImage,
		SourceURL:  sourceURL,
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
	// 新文章入库 → 列表/分类计数/标签计数全变，清掉相关缓存
	cache.InvalidateArticleRelated(article.ID)
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

func (s *ArticleService) UpdateMyArticle(id, authorID, categoryID uint, title, content, summary, coverImage, sourceURL, password string, publishAt *time.Time, tagIDs []uint) error {
	// 先取当前文章：草稿/已驳回 保存后自动"重新提交审核"（回到待审核）；
	// 已发布文章编辑后保持原状态（作者直接更新内容）
	article, err := s.dao.FindByIDAndAuthor(s.db, id, authorID)
	if err != nil {
		return err
	}
	updates := map[string]interface{}{
		"title":       title,
		"content":     content,
		"summary":     summary,
		"cover_image": coverImage,
		"source_url":  sourceURL,
		"category_id": categoryID,
		"password":    password,  // 修改私密文章密码；空=变回公开
		"publish_at":  publishAt, // 修改排期时间；nil=不排期
	}
	if article.Status == model.ArticleStatusDraft || article.Status == model.ArticleStatusRejected {
		updates["status"] = model.ArticleStatusPending
	}
	if err := s.dao.Update(s.db, id, authorID, updates); err != nil {
		return err
	}

	tags := make([]model.Tag, 0, len(tagIDs))
	for _, tid := range tagIDs {
		tags = append(tags, model.Tag{BaseModel: model.BaseModel{ID: tid}})
	}
	if err := s.db.Model(&model.Article{BaseModel: model.BaseModel{ID: id}}).Association("Tags").Replace(tags); err != nil {
		return err
	}
	// 内容/标签变了 → 失效该文章详情 + 列表缓存
	cache.InvalidateArticleRelated(id)
	return nil
}

func (s *ArticleService) DeleteMyArticle(id, authorID uint) error {
	if err := s.dao.Delete(s.db, id, authorID); err != nil {
		return err
	}
	cache.InvalidateArticleRelated(id)
	return nil
}

func (s *ArticleService) AddView(articleID uint, ip string) (int64, error) {
	// ① 初始化 Redis 计数：首次访问时从 DB 读当前值打底（SetNX 只写一次，不覆盖已有）
	cache.EnsureViewCount(articleID, func() int64 {
		var v int64
		s.db.Model(&model.Article{}).Where("id = ?", articleID).Select("view_count").Scan(&v)
		return v
	})
	// ② Redis 计数 +1（含当日 IP 去重）
	if v, ok := cache.AddView(articleID, ip); ok {
		return v, nil
	}
	// ③ Redis 不可用：降级走原来的数据库计数，保证功能不丢
	return s.dao.AddView(s.db, articleID, ip)
}

func (s *ArticleService) Like(articleID uint, ip string) (*dao.LikeResult, error) {
	result, err := s.dao.Like(s.db, articleID, ip)
	if err != nil {
		return nil, err
	}
	// 点赞数变了 → 失效详情缓存（列表缓存 1 分钟 TTL 自动过期，不用管）
	cache.Del(cache.KeyArticle + fmt.Sprint(articleID))
	return result, nil
}

func (s *ArticleService) GetHotArticles(limit int) ([]model.Article, error) {
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	key := cache.KeyHot + fmt.Sprint(limit)
	var articles []model.Article
	if cache.Get(key, &articles) {
		return articles, nil
	}
	articles, err := s.dao.FindHot(s.db, limit)
	if err != nil {
		return nil, err
	}
	if len(articles) > 0 {
		cache.Set(key, articles, cache.TTLList)
	}
	return articles, nil
}

func (s *ArticleService) GetArchives() ([]model.Archive, error) {
	var archives []model.Archive
	if cache.Get(cache.KeyArchives, &archives) {
		return archives, nil
	}
	archives, err := s.dao.FindArchives(s.db)
	if err != nil {
		return nil, err
	}
	if len(archives) > 0 {
		cache.Set(cache.KeyArchives, archives, cache.TTLStatic)
	}
	return archives, nil
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
		err := s.dao.UpdateStatus(s.db, id, model.ArticleStatusScheduled, "")
		if err == nil {
			cache.InvalidateArticleRelated(id)
		}
		return err
	}
	// 没设时间或时间已过 → 立即发布
	err = s.dao.UpdateStatus(s.db, id, model.ArticleStatusPublished, "")
	if err == nil {
		cache.InvalidateArticleRelated(id)
	}
	return err
}

// RejectArticle 驳回：先确认文章存在，再状态改已驳回、写入原因
func (s *ArticleService) RejectArticle(id uint, reason string) error {
	if _, err := s.dao.FindByID(s.db, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("文章不存在")
		}
		return err
	}
	if err := s.dao.UpdateStatus(s.db, id, model.ArticleStatusRejected, reason); err != nil {
		return err
	}
	cache.InvalidateArticleRelated(id)
	return nil
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

// ------------------------------------------------------------
// 后台文章管理（编辑+）：与"我的文章"不同，管理员可操作任何人的文章
// ------------------------------------------------------------

// GetAdminArticleDetail 后台文章详情（含 Tags；密码由 Controller 单独返回给后台）
func (s *ArticleService) GetAdminArticleDetail(id uint) (*model.Article, error) {
	article, err := s.dao.FindByID(s.db, id) // 只 Preload 了 Author/Category
	if err != nil {
		return nil, err
	}
	// 补 Preload Tags：后台编辑表单要回显标签
	if err := s.db.Preload("Tags").First(article, id).Error; err != nil {
		return nil, err
	}
	return article, nil
}

// AdminCreateArticle 后台代发/新建文章：状态与置顶由后台直接指定
// （和作者投稿不同：投稿固定走"待审核"，管理员自己发可以直接发布）
func (s *ArticleService) AdminCreateArticle(authorID, categoryID uint, title, content, summary, coverImage, sourceURL, password string, publishAt *time.Time, status int, isTop bool, tagIDs []uint) (*model.Article, error) {
	tags := make([]model.Tag, 0, len(tagIDs))
	for _, id := range tagIDs {
		tags = append(tags, model.Tag{BaseModel: model.BaseModel{ID: id}})
	}
	article := &model.Article{
		Title:      title,
		Content:    content,
		Summary:    summary,
		CoverImage: coverImage,
		SourceURL:  sourceURL,
		Password:   password,
		PublishAt:  publishAt,
		Status:     status,
		IsTop:      isTop,
		AuthorID:   authorID,
		CategoryID: categoryID,
		Tags:       tags,
	}
	if err := s.dao.Create(s.db, article); err != nil {
		return nil, err
	}
	// 新文章入库 → 列表/分类计数/标签计数全变，清掉相关缓存
	cache.InvalidateArticleRelated(article.ID)
	return article, nil
}

// AdminUpdateArticle 后台编辑任意文章（含置顶/状态）
func (s *ArticleService) AdminUpdateArticle(id, categoryID uint, title, content, summary, coverImage, sourceURL, password string, publishAt *time.Time, status int, isTop bool, tagIDs []uint) error {
	// 先判存在：GORM 的 Updates 查不到行返回 nil 不报错
	if _, err := s.dao.FindByID(s.db, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("文章不存在")
		}
		return err
	}
	// map 更新才能写入"零值"（is_top=false、status=0）
	updates := map[string]interface{}{
		"title":       title,
		"content":     content,
		"summary":     summary,
		"cover_image": coverImage,
		"source_url":  sourceURL,
		"category_id": categoryID,
		"password":    password,
		"publish_at":  publishAt,
		"status":      status,
		"is_top":      isTop,
	}
	if err := s.dao.UpdateByID(s.db, id, updates); err != nil {
		return err
	}
	// 替换标签（多对多）
	tags := make([]model.Tag, 0, len(tagIDs))
	for _, tid := range tagIDs {
		tags = append(tags, model.Tag{BaseModel: model.BaseModel{ID: tid}})
	}
	if err := s.db.Model(&model.Article{BaseModel: model.BaseModel{ID: id}}).Association("Tags").Replace(tags); err != nil {
		return err
	}
	cache.InvalidateArticleRelated(id)
	return nil
}

// AdminDeleteArticle 后台删除任意文章（软删除）
func (s *ArticleService) AdminDeleteArticle(id uint) error {
	if _, err := s.dao.FindByID(s.db, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("文章不存在")
		}
		return err
	}
	if err := s.db.Delete(&model.Article{}, id).Error; err != nil {
		return err
	}
	cache.InvalidateArticleRelated(id)
	return nil
}

// BatchArticleOp 后台批量操作
// action: publish 发布 / draft 草稿 / top 置顶 / untop 取消置顶 / delete 删除
func (s *ArticleService) BatchArticleOp(ids []uint, action string) (int64, error) {
	if len(ids) == 0 {
		return 0, errors.New("请先勾选文章")
	}
	var (
		affected int64
		err      error
	)
	switch action {
	case "publish":
		affected, err = s.dao.UpdateStatusBatch(s.db, ids, model.ArticleStatusPublished)
	case "draft":
		affected, err = s.dao.UpdateStatusBatch(s.db, ids, model.ArticleStatusDraft)
	case "top":
		affected, err = s.dao.UpdateTopBatch(s.db, ids, true)
	case "untop":
		affected, err = s.dao.UpdateTopBatch(s.db, ids, false)
	case "delete":
		affected, err = s.dao.DeleteBatch(s.db, ids)
	default:
		return 0, errors.New("不支持的操作类型")
	}
	if err != nil {
		return 0, err
	}
	// 批量操作影响多篇文章：全量清掉文章相关缓存（列表/热门/归档/计数）
	for _, id := range ids {
		cache.Del(cache.KeyArticle + fmt.Sprint(id))
	}
	cache.DelPrefix(cache.KeyArticleList)
	cache.Del(cache.KeyHot, cache.KeyArchives, cache.KeyCategories, cache.KeyTags)
	return affected, nil
}
