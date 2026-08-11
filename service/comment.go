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

// AddComment 发表评论（游客可评，不需要登录；登录用户自动识别身份）
// userID=0 表示游客；userID>0 时昵称留空默认用账号名称（昵称→用户名），并绑定 UserID
func (s *CommentService) AddComment(articleID uint, content, nickname string, parentID *uint, userID uint) (*model.Comment, error) {
	comment := &model.Comment{
		ArticleID: articleID,                   // 所属文章
		Content:   content,                     // 评论内容
		Status:    model.CommentStatusApproved, // 免审核：默认通过，直接展示
		ParentID:  parentID,                    // 楼中楼：nil=顶级评论，有值=回复
	}
	// 登录用户：昵称留空默认用账号名称（优先昵称，其次用户名），并绑定 UserID
	if userID > 0 {
		var u model.User
		if err := s.db.Select("nickname", "username").First(&u, userID).Error; err == nil {
			if nickname == "" {
				nickname = u.Nickname
				if nickname == "" {
					nickname = u.Username
				}
			}
			comment.UserID = &userID
		}
	}
	// 游客兜底：仍是空昵称时给默认值"游客"
	if nickname == "" {
		nickname = "游客"
	}
	comment.Nickname = nickname
	// 校验文章存在且已发布（防止对任意 ID 产生孤儿评论）
	var article model.Article
	if err := s.db.Select("id").Where("status = ?", model.ArticleStatusPublished).First(&article, articleID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("文章不存在或未发布")
		}
		return nil, err
	}
	// 楼中楼：校验父评论存在且属于同一篇文章（防止跨文章"楼中楼"回复）
	if parentID != nil {
		var parent model.Comment
		if err := s.db.Select("id", "article_id").First(&parent, *parentID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("被回复的评论不存在")
			}
			return nil, err
		}
		if parent.ArticleID != articleID {
			return nil, errors.New("被回复的评论不属于这篇文章")
		}
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
	comments, total, err := s.dao.FindAll(s.db, status, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	// 填充 ArticleTitle（前端列表直接用 c.articleTitle，不用再取 c.article.title）
	for i := range comments {
		comments[i].ArticleTitle = comments[i].Article.Title
	}
	return comments, total, nil
}

// SetCommentStatus 后台通用改状态（前端 /status 接口：0待审 1通过 2驳回）
func (s *CommentService) SetCommentStatus(id uint, status int) error {
	if _, err := s.dao.FindByID(s.db, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("评论不存在")
		}
		return err
	}
	return s.dao.UpdateStatus(s.db, id, status)
}

// BatchCommentOp 批量操作：approve=批量通过，delete=批量删除
func (s *CommentService) BatchCommentOp(ids []uint, action string) error {
	if len(ids) == 0 {
		return errors.New("请先勾选评论")
	}
	switch action {
	case "approve":
		_, err := s.dao.UpdateStatusBatch(s.db, ids, model.CommentStatusApproved)
		return err
	case "delete":
		_, err := s.dao.DeleteBatch(s.db, ids)
		return err
	default:
		return errors.New("不支持的操作类型")
	}
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
