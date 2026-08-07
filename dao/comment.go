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

// FindAll 后台评论列表：按状态筛选 + 分页
// status >= 0 才按状态过滤（-1 表示全部）；Preload 带出文章标题和评论人
func (d *CommentDAO) FindAll(db *gorm.DB, status int, page, pageSize int) ([]model.Comment, int64, error) {
	var comments []model.Comment
	var total int64

	query := db.Model(&model.Comment{})
	if status >= 0 {
		query = query.Where("status = ?", status)
	}
	query.Count(&total)

	err := query.
		Preload("Article"). // 文章标题：Article.Title
		Preload("User").    // 评论人：游客为 nil
		Order("created_at desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&comments).Error

	return comments, total, err
}

// FindByID 按 ID 查评论（审核/删除前判存在，GORM 的 Update/Delete 查不到行不报错）
func (d *CommentDAO) FindByID(db *gorm.DB, id uint) (*model.Comment, error) {
	var c model.Comment
	err := db.First(&c, id).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// UpdateStatus 更新评论状态（通过 1 / 驳回 2）
func (d *CommentDAO) UpdateStatus(db *gorm.DB, id uint, status int) error {
	return db.Model(&model.Comment{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// Delete 删除评论（软删除：BaseModel 自带 DeletedAt，行没被删只是标记）
func (d *CommentDAO) Delete(db *gorm.DB, id uint) error {
	return db.Delete(&model.Comment{}, id).Error
}

// UpdateStatusBatch 批量改状态（后台批量通过/驳回用）
func (d *CommentDAO) UpdateStatusBatch(db *gorm.DB, ids []uint, status int) (int64, error) {
	result := db.Model(&model.Comment{}).
		Where("id IN ?", ids).
		Update("status", status)
	return result.RowsAffected, result.Error
}

// DeleteBatch 批量删除（后台批量删除用，软删除）
func (d *CommentDAO) DeleteBatch(db *gorm.DB, ids []uint) (int64, error) {
	result := db.Where("id IN ?", ids).Delete(&model.Comment{})
	return result.RowsAffected, result.Error
}
