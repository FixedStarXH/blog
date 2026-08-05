package dao

import (
	"blog-system/model"
	"strings"

	"gorm.io/gorm"
)

type SettingDAO struct{}

func NewSettingDAO() *SettingDAO {
	return &SettingDAO{}
}

func (d *SettingDAO) FindAllAsMap(db *gorm.DB) (map[string]string, error) {
	var settings []model.Setting
	if err := db.Find(&settings).Error; err != nil {
		return nil, err
	}
	result := make(map[string]string, len(settings))
	for _, s := range settings {
		result[s.K] = s.V
	}
	return result, nil
}

// GetQuotes 读取名言池配置，按换行拆分成切片返回
func (d *SettingDAO) GetQuotes(db *gorm.DB) ([]string, error) {
	var setting model.Setting
	if err := db.Where("k = ?", "daily_quotes").First(&setting).Error; err != nil {
		return nil, err
	}
	// 按换行拆分："a\nb\nc" → ["a", "b", "c"]
	return strings.Split(setting.V, "\n"), nil
}
