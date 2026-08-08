package dao

import (
	"blog-system/model"

	"gorm.io/gorm"
)

type TagDAO struct{}

func NewTagDAO() *TagDAO {
	return &TagDAO{}
}

func (d *TagDAO) FindAllWithCount(db *gorm.DB) ([]model.Tag, error) {
	var tags []model.Tag
	if err := db.Order("id asc").Find(&tags).Error; err != nil {
		return nil, err
	}

	var counts []struct {
		TagID uint
		Count int64
	}
	if err := db.Table("article_tags at").
		Select("at.tag_id, COUNT(*) as count").
		// 注意：字符串 JOIN 不会自动注入软删除条件，必须手动加 a.deleted_at IS NULL，
		// 否则已软删除的文章仍会被计入标签文章数（前台标签墙数字虚高）
		Joins("JOIN articles a ON a.id = at.article_id AND a.status = ? AND a.deleted_at IS NULL", model.ArticleStatusPublished).
		Group("at.tag_id").
		Scan(&counts).Error; err != nil {
		return nil, err
	}

	countMap := make(map[uint]int64, len(counts))
	for _, c := range counts {
		countMap[c.TagID] = c.Count
	}
	for i := range tags {
		tags[i].ArticleCount = countMap[tags[i].ID]
	}
	return tags, nil
}

// FindPage 后台标签管理：关键词模糊 + 分页（含每个标签的文章数，不走缓存）
func (d *TagDAO) FindPage(db *gorm.DB, keyword string, page, pageSize int) ([]model.Tag, int64, error) {
	var tags []model.Tag
	var total int64

	query := db.Model(&model.Tag{})
	if keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("id asc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&tags).Error; err != nil {
		return nil, 0, err
	}

	var counts []struct {
		TagID uint
		Count int64
	}
	if err := db.Table("article_tags at").
		Select("at.tag_id, COUNT(*) as count").
		// 同 FindAllWithCount：字符串 JOIN 需手动补软删除条件，避免软删文章计入计数
		Joins("JOIN articles a ON a.id = at.article_id AND a.status = ? AND a.deleted_at IS NULL", model.ArticleStatusPublished).
		Group("at.tag_id").
		Scan(&counts).Error; err != nil {
		return nil, 0, err
	}
	countMap := make(map[uint]int64, len(counts))
	for _, c := range counts {
		countMap[c.TagID] = c.Count
	}
	for i := range tags {
		tags[i].ArticleCount = countMap[tags[i].ID]
	}
	return tags, total, nil
}

// FindByID 按 ID 查标签
func (d *TagDAO) FindByID(db *gorm.DB, id uint) (*model.Tag, error) {
	var t model.Tag
	err := db.First(&t, id).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// FindByName 按名字查标签（新增/改名查重用）
func (d *TagDAO) FindByName(db *gorm.DB, name string) (*model.Tag, error) {
	var t model.Tag
	err := db.Where("name = ?", name).First(&t).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// Create 新增标签
func (d *TagDAO) Create(db *gorm.DB, t *model.Tag) error {
	return db.Create(t).Error
}

// Update 更新标签
func (d *TagDAO) Update(db *gorm.DB, id uint, updates map[string]interface{}) error {
	return db.Model(&model.Tag{}).Where("id = ?", id).Updates(updates).Error
}

// Delete 删除标签：先清中间表 article_tags 的死引用，再删标签本身
// 两步放同一事务：中间表清了、标签删除失败会回滚，不产生孤儿引用
func (d *TagDAO) Delete(db *gorm.DB, id uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM article_tags WHERE tag_id = ?", id).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Tag{}, id).Error
	})
}
