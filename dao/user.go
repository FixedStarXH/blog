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
