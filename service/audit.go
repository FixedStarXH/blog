package service

import (
	"blog-system/model"

	"gorm.io/gorm"
)

// AuditService 操作审计（记录 + 查询 + 清空）
type AuditService struct {
	db *gorm.DB
}

func NewAuditService(db *gorm.DB) *AuditService {
	return &AuditService{db: db}
}

// AuditRecord 审计条目（前端契约：admin/audit.html）
type AuditRecord struct {
	model.AuditLog
	RoleName string `json:"roleName"` // 角色中文名（1用户/2编辑/3管理员）
}

// Record 追加一条审计记录（写库失败只记日志，不影响主流程）
func (s *AuditService) Record(operatorID uint, username string, role int, action, method, path, targetType string, targetID uint, detail, ip string, ok bool) {
	status := 1
	if !ok {
		status = 0
	}
	if len(detail) > 500 {
		detail = detail[:500] // 截断超长详情，防止撑爆表字段
	}
	err := s.db.Create(&model.AuditLog{
		OperatorID: operatorID,
		Username:   username,
		Role:       role,
		Action:     action,
		Method:     method,
		Path:       path,
		TargetType: targetType,
		TargetID:   targetID,
		Detail:     detail,
		IP:         ip,
		Status:     status,
	}).Error
	if err != nil {
		// 审计失败绝不能影响业务：仅输出告警日志
		println("[audit] 写入审计日志失败:", err.Error())
	}
}

// List 分页查询审计记录（按时间倒序，可按动作筛选）
func (s *AuditService) List(page, pageSize int, action string) ([]AuditRecord, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	q := s.db.Model(&model.AuditLog{})
	if action != "" {
		q = q.Where("action = ?", action)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []model.AuditLog
	if err := q.Order("created_at desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	out := make([]AuditRecord, 0, len(logs))
	for i := range logs {
		out = append(out, AuditRecord{AuditLog: logs[i], RoleName: roleName(logs[i].Role)})
	}
	return out, total, nil
}

// Clear 清空审计记录（仅管理员，权限在 Controller 校验）
func (s *AuditService) Clear() error {
	return s.db.Where("1 = 1").Delete(&model.AuditLog{}).Error
}

// roleName 角色常量 → 中文名
func roleName(role int) string {
	switch role {
	case model.RoleAdmin:
		return "管理员"
	case model.RoleEditor:
		return "编辑"
	case model.RoleUser:
		return "用户"
	default:
		return "游客"
	}
}
