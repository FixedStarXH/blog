package service

import (
	"blog-system/dao"
	"math/rand/v2"

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

// GetDailyQuote 随机返回一条名言
func (s *SettingService) GetDailyQuote() (string, error) {
	quotes, err := s.dao.GetQuotes(s.db)
	if err != nil {
		return "", err
	}
	// rand.IntN(3) 返回 0/1/2 的随机整数，正好当切片下标
	return quotes[rand.IntN(len(quotes))], nil
}
