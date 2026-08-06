package service

import (
	"errors"

	"blog-system/dao"
	"blog-system/model"

	"gorm.io/gorm"
)

type UserService struct {
	dao *dao.UserDAO
	db  *gorm.DB
}

func NewUserService(dao *dao.UserDAO, db *gorm.DB) *UserService {
	return &UserService{dao: dao, db: db}
}

// GetAdminUsers 后台用户列表：关键字 + 角色 + 分页
func (s *UserService) GetAdminUsers(keyword string, role, page, pageSize int) ([]model.User, int64, error) {
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 10
	}
	if page <= 0 {
		page = 1
	}
	return s.dao.FindAll(s.db, keyword, role, page, pageSize)
}

// UpdateUserStatus 启用/禁用用户：不能操作自己的账号（防止把自己禁了把系统锁死）
func (s *UserService) UpdateUserStatus(operatorID, targetID uint, status int) error {
	if operatorID == targetID {
		return errors.New("不能操作自己的账号")
	}
	return s.dao.Update(s.db, targetID, map[string]interface{}{"status": status})
}

// UpdateUserRole 修改角色：同样不能改自己（防止管理员手滑把自己降级）
func (s *UserService) UpdateUserRole(operatorID, targetID uint, role int) error {
	if operatorID == targetID {
		return errors.New("不能修改自己的角色")
	}
	return s.dao.Update(s.db, targetID, map[string]interface{}{"role": role})
}
