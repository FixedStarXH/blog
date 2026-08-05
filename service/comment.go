package service

import (
	"blog-system/dao"
	"blog-system/model"

	"gorm.io/gorm"
)

type CommentService struct {
	dao *dao.CommentDAO
	db  *gorm.DB
}

func NewCommentService(dao *dao.CommentDAO, db *gorm.DB) *CommentService {
	return &CommentService{dao: dao, db: db}
}

// GetComments 获取文章评论列表
func (s *CommentService) GetComments(articleID uint) ([]model.Comment, error) {
	return s.dao.FindApprovedByArticleID(s.db, articleID)
}
func (s *CommentService) AddComment(articleID uint, content, nickname string) (*model.Comment, error) {
	comment := &model.Comment{
		ArticleID: articleID,                  // 所属文章
		Content:   content,                    // 评论内容
		Nickname:  nickname,                   // 游客昵称
		Status:    model.CommentStatusPending, // 默认待审，后台审核通过才展示
		// UserID 不赋值 → nil，表示游客
	}
	if err := s.dao.Create(s.db, comment); err != nil {
		return nil, err
	}
	return comment, nil
}
