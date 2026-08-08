package service

import (
	"errors"

	"blog-system/cache"
	"blog-system/dao"
	"blog-system/model"
	"blog-system/utils"

	"gorm.io/gorm"
)

type AuthService struct {
	dao *dao.UserDAO
	db  *gorm.DB
}

func NewAuthService(dao *dao.UserDAO, db *gorm.DB) *AuthService {
	return &AuthService{dao: dao, db: db}
}

// registerRefreshToken 把 refresh token 的 jti 登记进 Redis 白名单
// 签发后立刻登记：之后刷新接口靠它校验 + 轮换吊销
func registerRefreshToken(refreshToken string) {
	claims, err := utils.ParseToken(refreshToken, "refresh")
	if err == nil && claims.ID != "" {
		cache.SaveRefreshToken(claims.ID)
	}
}

func (s *AuthService) Register(username, email, password string) (*model.User, string, string, error) {
	if _, err := s.dao.FindByUsername(s.db, username); err == nil {
		return nil, "", "", errors.New("用户名已存在")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", "", err
	}
	if _, err := s.dao.FindByEmail(s.db, email); err == nil {
		return nil, "", "", errors.New("邮箱已存在")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", "", err
	}
	hashed, err := utils.HashPassword(password)
	if err != nil {
		return nil, "", "", err
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
		return nil, "", "", err
	}
	// 注册即登录：签发双 token（access 短效 + refresh 长效）
	access, refresh, err := utils.GenerateTokenPair(user.ID, user.Role)
	if err != nil {
		return nil, "", "", err
	}
	registerRefreshToken(refresh)
	return &user, access, refresh, nil
}

func (s *AuthService) Login(account, password string) (*model.User, string, string, error) {
	user, err := s.dao.FindByUsernameOrEmail(s.db, account)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", "", errors.New("用户名或密码错误")
	}
	if err != nil {
		return nil, "", "", err
	}
	if !utils.CheckPassword(user.Password, password) {
		return nil, "", "", errors.New("用户名或密码错误")
	}
	if user.Status != model.UserStatusActive {
		return nil, "", "", errors.New("账号已被禁用")
	}
	access, refresh, err := utils.GenerateTokenPair(user.ID, user.Role)
	if err != nil {
		return nil, "", "", err
	}
	registerRefreshToken(refresh)
	return user, access, refresh, nil
}

// Refresh 用 refresh token 换新双 token（access 过期后前端自动调用）
//
// 安全设计：
//  1. 验签 + 类型必须是 refresh（access 拿来过不了）
//  2. 查库确认用户仍存在且启用（与鉴权中间件一致，禁用/删除立即失效）
//  3. Redis 白名单校验 jti 有效（防旧 refresh 无限复用）→ 轮换：吊销旧的、签发新的
func (s *AuthService) Refresh(refreshToken string) (string, string, error) {
	claims, err := utils.ParseToken(refreshToken, "refresh")
	if err != nil {
		return "", "", errors.New("登录已过期，请重新登录")
	}
	// 用户状态复查：以数据库为准
	var user model.User
	if err := s.db.Select("id", "role", "status").First(&user, claims.UserID).Error; err != nil {
		return "", "", errors.New("账号不存在，请重新登录")
	}
	if user.Status != model.UserStatusActive {
		return "", "", errors.New("账号已被禁用")
	}
	// 白名单校验：这个 refresh 必须还是"最近一次签发"的（没被轮换过）
	if !cache.CheckRefreshToken(claims.ID) {
		return "", "", errors.New("登录已过期，请重新登录")
	}
	// 轮换：旧 refresh 立即作废，新签发一对
	cache.RemoveRefreshToken(claims.ID)
	access, newRefresh, err := utils.GenerateTokenPair(user.ID, user.Role)
	if err != nil {
		return "", "", err
	}
	registerRefreshToken(newRefresh)
	return access, newRefresh, nil
}

// Logout 退出登录：吊销当前 refresh token（让"已登出"的 refresh 彻底失效）
func (s *AuthService) Logout(refreshToken string) error {
	claims, err := utils.ParseToken(refreshToken, "refresh")
	if err != nil {
		return nil // token 本来就无效：无需吊销
	}
	cache.RemoveRefreshToken(claims.ID)
	return nil
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
