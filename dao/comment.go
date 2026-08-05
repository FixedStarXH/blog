package dao

import (
	"blog-system/model"

	"gorm.io/gorm"
)

type CommentDAO struct{}

func NewCommentDAO() *CommentDAO {
	return &CommentDAO{}
}

func (d *CommentDAO) FindApprovedByArticleID(db *gorm.DB, articleID uint) ([]model.Comment, error) {
	var comments []model.Comment
	err := db.Where("article_id = ? AND status = ?", articleID, model.CommentStatusApproved).
		Order("created_at asc").
		Find(&comments).Error
	return comments, err
}

func (d *CommentDAO) Create(db *gorm.DB, comment *model.Comment) error {
	return db.Create(comment).Error
}
