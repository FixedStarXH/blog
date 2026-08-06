package dao

import (
	"blog-system/model"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

// Upsert 批量保存设置：K 是主键，存在则更新 V，不存在则插入（ON DUPLICATE KEY UPDATE）
func (d *SettingDAO) Upsert(db *gorm.DB, kv map[string]string) error {
	for k, v := range kv {
		s := model.Setting{K: k, V: v}
		if err := db.Clauses(clause.OnConflict{
			UpdateAll: true, // 主键冲突时更新所有列（这里就是 V）
		}).Create(&s).Error; err != nil {
			return err
		}
	}
	return nil
}
