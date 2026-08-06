package service

import (
	"blog-system/dao"
	"errors"
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

// 允许修改的设置键（白名单：防止往 KV 表乱写 key 污染数据）
var allowedSettingKeys = map[string]bool{
	"site_title":       true,
	"site_description": true,
	"daily_quotes":     true,
}

// UpdateSettings 更新站点设置：只保留白名单内的键，其余忽略
func (s *SettingService) UpdateSettings(kv map[string]string) error {
	filtered := make(map[string]string)
	for k, v := range kv {
		if allowedSettingKeys[k] {
			filtered[k] = v
		}
	}
	if len(filtered) == 0 {
		return errors.New("没有可更新的设置项")
	}
	return s.dao.Upsert(s.db, filtered)
}
