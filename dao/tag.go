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
