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
		Joins("JOIN articles a ON a.id = at.article_id AND a.status = ?", model.ArticleStatusPublished).
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
func (d *TagDAO) Delete(db *gorm.DB, id uint) error {
	if err := db.Exec("DELETE FROM article_tags WHERE tag_id = ?", id).Error; err != nil {
		return err
	}
	return db.Delete(&model.Tag{}, id).Error
}
