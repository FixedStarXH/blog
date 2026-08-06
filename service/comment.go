package service

import (
	"blog-system/dao"
	"blog-system/model"
	"errors"

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

// GetAdminComments 后台评论列表：状态筛选 + 分页（编辑+）
func (s *CommentService) GetAdminComments(status, page, pageSize int) ([]model.Comment, int64, error) {
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 10
	}
	if page <= 0 {
		page = 1
	}
	return s.dao.FindAll(s.db, status, page, pageSize)
}

// ApproveComment 通过审核：先判存在，状态 0→1
func (s *CommentService) ApproveComment(id uint) error {
	if _, err := s.dao.FindByID(s.db, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("评论不存在")
		}
		return err
	}
	return s.dao.UpdateStatus(s.db, id, model.CommentStatusApproved)
}

// RejectComment 驳回：先判存在，状态 0→2
func (s *CommentService) RejectComment(id uint) error {
	if _, err := s.dao.FindByID(s.db, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("评论不存在")
		}
		return err
	}
	return s.dao.UpdateStatus(s.db, id, model.CommentStatusRejected)
}

// DeleteComment 删除评论：先判存在，再软删除
func (s *CommentService) DeleteComment(id uint) error {
	if _, err := s.dao.FindByID(s.db, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("评论不存在")
		}
		return err
	}
	return s.dao.Delete(s.db, id)
}
