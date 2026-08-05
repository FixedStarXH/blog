package dao

import (
	"blog-system/model"

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
