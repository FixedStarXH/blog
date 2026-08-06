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

// FindByID 按 ID 查分类
func (d *CategoryDAO) FindByID(db *gorm.DB, id uint) (*model.Category, error) {
	var c model.Category
	err := db.First(&c, id).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// FindByName 按名字查分类（新增/改名查重用；查不到返回 ErrRecordNotFound）
func (d *CategoryDAO) FindByName(db *gorm.DB, name string) (*model.Category, error) {
	var c model.Category
	err := db.Where("name = ?", name).First(&c).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// Create 新增分类
func (d *CategoryDAO) Create(db *gorm.DB, c *model.Category) error {
	return db.Create(c).Error
}

// Update 更新分类（map 更新：sort 改成 0 也能生效）
func (d *CategoryDAO) Update(db *gorm.DB, id uint, updates map[string]interface{}) error {
	return db.Model(&model.Category{}).Where("id = ?", id).Updates(updates).Error
}

// CountArticles 统计分类下的文章数（删除前检查引用：有文章不能删）
func (d *CategoryDAO) CountArticles(db *gorm.DB, id uint) (int64, error) {
	var count int64
	err := db.Model(&model.Article{}).
		Where("category_id = ?", id).
		Count(&count).Error
	return count, err
}

// Delete 删除分类
func (d *CategoryDAO) Delete(db *gorm.DB, id uint) error {
	return db.Delete(&model.Category{}, id).Error
}
