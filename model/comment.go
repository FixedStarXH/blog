package model

// Comment 评论模型（含审核）
//
// 状态机（见 const.go）：待审(0) → 通过(1) / 驳回(2)
//
//	默认待审，后台审核通过后才在前台展示
//
// 游客评论：
//   - UserID 是 *uint（可空）：nil=游客评论
//   - 游客填 Nickname 昵称；登录用户自动带 UserID
//
// 楼中楼：ParentID 指向父评论，nil=顶级评论
type Comment struct {
	BaseModel

	Content   string `gorm:"type:text;not null;comment:评论内容" json:"content"`  // 纯文本存储（前端渲染时转义，防 XSS）
	Status    int    `gorm:"default:0;index;comment:0待审1通过2驳回" json:"status"` // 默认 CommentStatusPending=0
	ArticleID uint   `gorm:"not null;index;comment:文章ID" json:"articleId"`    // 所属文章 FK
	UserID    *uint  `gorm:"index;comment:用户ID" json:"userId"`                // *uint=可空；nil=游客，有值=登录用户
	Nickname  string `gorm:"size:20;comment:游客昵称" json:"nickname"`            // 游客评论时必填
	ParentID  *uint  `gorm:"index;comment:父评论ID" json:"parentId"`             // 楼中楼；nil=顶级评论

	Article Article `gorm:"foreignKey:ArticleID" json:"article,omitempty"` // 关联文章
	User    *User   `gorm:"foreignKey:UserID" json:"user,omitempty"`       // 关联用户；游客时为 nil

	// 以下字段不落库（gorm:"-"），由 Service 填充后返回给前端
	ArticleTitle string `gorm:"-" json:"articleTitle"` // 后台列表用：文章标题（省得前端从 article 里再取一层）
}
