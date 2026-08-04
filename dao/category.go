package dao

import (
	"blog-system/model"

	"gorm.io/gorm"
)

type CategoryDAO struct{}

func NewCategoryDAO() *CategoryDAO {
	return &CategoryDAO{}
}

func (d *CategoryDAO) FindAllWithCount(db *gorm.DB) ([]model.Category, error) {
	var categories []model.Category
	if err := db.Order("id asc").Find(&categories).Error; err != nil {
		return nil, err
	}
	var counts []struct {
		CategoryID uint
		Count      int64
	}
	if err := db.Model(&model.Article{}).
		Select("category_id, COUNT(*) as count").
		Where("status = ?", model.ArticleStatusPublished).
		Group("category_id").
		Scan(&counts).Error; err != nil {
		return nil, err
	}
	countMap := make(map[uint]int64, len(counts))
	for _, c := range counts {
		countMap[c.CategoryID] = c.Count
	}
	for i := range categories {
		categories[i].ArticleCount = countMap[categories[i].ID]
	}
	return categories, nil
}
