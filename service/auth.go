package service

import (
	"blog-system/dao"
	"blog-system/model"
	"blog-system/utils"
	"errors"

	"gorm.io/gorm"
)

type AuthService struct {
	dao *dao.UserDAO
	db  *gorm.DB
}

func NewAuthService(dao *dao.UserDAO, db *gorm.DB) *AuthService {
	return &AuthService{dao: dao, db: db}
}

func (s *AuthService) Register(username, email, password string) (*model.User, string, error) {
	if _, err := s.dao.FindByUsername(s.db, username); err == nil {
		return nil, "", errors.New("用户名已存在")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", err
	}
	if _, err := s.dao.FindByEmail(s.db, email); err == nil {
		return nil, "", errors.New("邮箱已存在")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", err
	}
	hashed, err := utils.HashPassword(password)
	if err != nil {
		return nil, "", err
	}

	user := model.User{
		Username: username,
		Email:    email,
		Password: hashed,
		Nickname: username,
		Role:     model.RoleUser,
		Status:   model.UserStatusActive,
	}
	if err := s.dao.Create(s.db, &user); err != nil {
		return nil, "", err
	}
	// 签发 token：注册即登录
	token, err := utils.GenerateToken(user.ID, user.Role)
	if err != nil {
		return nil, "", err
	}
	return &user, token, nil
}

func (s *AuthService) Login(account, password string) (*model.User, string, error) {
	user, err := s.dao.FindByUsernameOrEmail(s.db, account)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", errors.New("用户名或密码错误")
	}
	if err != nil {
		return nil, "", err
	}
	if !utils.CheckPassword(user.Password, password) {
		return nil, "", errors.New("用户名或密码错误")
	}
	if user.Status != model.UserStatusActive {
		return nil, "", errors.New("账号已被禁用")
	}
	token, err := utils.GenerateToken(user.ID, user.Role)
	if err != nil {
		return nil, "", err
	}
	return user, token, nil
}

// Me 当前登录用户信息：从 token 拿 userID，查库返回最新数据
func (s *AuthService) Me(userID uint) (*model.User, error) {
	user, err := s.dao.FindByID(s.db, userID)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// UpdateProfile 更新资料：只允许改 nickname/avatar/bio 三个字段
func (s *AuthService) UpdateProfile(userID uint, nickname, avatar, bio string) (*model.User, error) {
	// 用 map 更新：零值（空字符串）也会生效，能把 bio 清空
	updates := map[string]interface{}{
		"nickname": nickname,
		"avatar":   avatar,
		"bio":      bio,
	}
	if err := s.dao.Update(s.db, userID, updates); err != nil {
		return nil, err
	}
	// 重新查库返回最新数据（而不是内存拼的，保证和数据库一致）
	return s.dao.FindByID(s.db, userID)
}

// ChangePassword 修改密码：先验旧密码，再存新密码哈希
func (s *AuthService) ChangePassword(userID uint, oldPassword, newPassword string) error {
	// 1. 查用户，拿库里存的旧密码哈希
	user, err := s.dao.FindByID(s.db, userID)
	if err != nil {
		return err
	}
	// 2. 校验旧密码：错了直接返回，不继续
	if !utils.CheckPassword(user.Password, oldPassword) {
		return errors.New("原密码错误")
	}
	// 3. 新密码哈希（新密码同样不能存明文）
	hashed, err := utils.HashPassword(newPassword)
	if err != nil {
		return err
	}
	// 4. 单独更新 password 字段
	return s.dao.UpdatePassword(s.db, userID, hashed)
}
