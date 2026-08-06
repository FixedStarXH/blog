package dao

import (
	"blog-system/model"

	"gorm.io/gorm"
)

type UserDAO struct{}

func NewUserDAO() *UserDAO {
	return &UserDAO{}
}

func (d *UserDAO) Create(db *gorm.DB, user *model.User) error {
	return db.Create(user).Error
}

func (d *UserDAO) FindByUsername(db *gorm.DB, username string) (*model.User, error) {
	var user model.User
	err := db.Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (d *UserDAO) FindByEmail(db *gorm.DB, email string) (*model.User, error) {
	var user model.User
	err := db.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (d *UserDAO) FindByUsernameOrEmail(db *gorm.DB, account string) (*model.User, error) {
	var user model.User
	err := db.Where("username = ? OR email = ?", account, account).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (d *UserDAO) FindByID(db *gorm.DB, id uint) (*model.User, error) {
	var user model.User
	err := db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (d *UserDAO) Update(db *gorm.DB, id uint, updates map[string]interface{}) error {
	return db.Model(&model.User{}).Where("id = ?", id).Updates(updates).Error
}

func (d *UserDAO) UpdatePassword(db *gorm.DB, id uint, hashed string) error {
	return db.Model(&model.User{}).Where("id = ?", id).Update("password", hashed).Error
}

// FindAll 后台用户列表：关键字（用户名/昵称/邮箱模糊匹配）+ 角色筛选 + 分页
// keyword 空则不过滤；role<=0 表示"全部角色"
func (d *UserDAO) FindAll(db *gorm.DB, keyword string, role int, page, pageSize int) ([]model.User, int64, error) {
	var users []model.User
	var total int64

	query := db.Model(&model.User{})
	if keyword != "" {
		// LIKE '%xx%'：三个地方都模糊匹配，任意命中就算
		query = query.Where("username LIKE ? OR nickname LIKE ? OR email LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	if role > 0 {
		query = query.Where("role = ?", role)
	}
	query.Count(&total)

	err := query.
		Order("id asc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&users).Error

	return users, total, err
}
