package service

import (
	"blog-system/dao"

	"gorm.io/gorm"
)

type SettingService struct {
	dao *dao.SettingDAO
	db  *gorm.DB
}

func NewSettingService(dao *dao.SettingDAO, db *gorm.DB) *SettingService {
	return &SettingService{dao: dao, db: db}
}

// GetSiteSettings 获取站点全部配置（KV map）
func (s *SettingService) GetSiteSettings() (map[string]string, error) {
	return s.dao.FindAllAsMap(s.db)
}
