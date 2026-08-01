package model

import "time"

// Article 文章模型（含投稿审核工作流）
//
// 状态机（见 const.go）：
//   草稿(0) → 待审核(2) → 已发布(1)
//                      ↘ 已驳回(3) → 修改重提 → 待审核(2)
// 特色字段：
//   - RejectReason 驳回原因（状态=3 时给作者看）
//   - IsTop 置顶（首页头条）
//   - PublishAt 发布时间（支持定时发布）
//   - Password 私密文章密码（解锁后可见）
type Article struct {
	BaseModel

	Title        string     `gorm:"not null;size:200;index;comment:标题" json:"title"`        // 必填，加索引便于搜索
	Content      string     `gorm:"type:longtext;not null;comment:正文HTML" json:"content"`  // 必填，前端渲染 HTML
	Summary      string     `gorm:"size:500;comment:摘要" json:"summary"`                    // 列表页显示，可空
	CoverImage   string     `gorm:"size:255;comment:封面图URL" json:"coverImage"`             // 列表页缩略图，可空
	ViewCount    int        `gorm:"default:0;comment:阅读量" json:"viewCount"`                // 冗余计数（明细在 article_views）
	LikeCount    int        `gorm:"default:0;comment:点赞数" json:"likeCount"`                // 冗余计数（明细在 article_likes）
	Status       int        `gorm:"default:0;index;comment:0草稿1发布2待审3驳回" json:"status"`  // 默认 ArticleStatusDraft=0
	RejectReason string     `gorm:"size:255;comment:驳回原因" json:"rejectReason"`             // 状态=3 时填写
	IsTop        bool       `gorm:"default:false;index;comment:是否置顶" json:"isTop"`         // 首页头条用，加索引方便筛选
	PublishAt    *time.Time `json:"publishAt"`                                              // 发布时间；指针类型=可空，支持定时发布
	Password     string     `gorm:"size:255;comment:私密文章密码" json:"-"`                     // json:"-" 密码永不返回；空=公开文章
	AuthorID     uint       `gorm:"not null;index;comment:作者ID" json:"authorId"`           // 作者 FK（多作者）
	CategoryID   uint       `gorm:"not null;index;comment:分类ID" json:"categoryId"`         // 分类 FK（必选）
	ColumnID     *uint      `gorm:"index;comment:专栏ID" json:"columnId"`                    // 专栏 FK；*uint=可空（文章可不属于专栏）

	// 关联对象（查询时用 Preload 预加载，见 dao）
	Author   User     `gorm:"foreignKey:AuthorID" json:"author"`          // 作者
	Category Category `gorm:"foreignKey:CategoryID" json:"category"`      // 分类
	Column   *Column  `gorm:"foreignKey:ColumnID" json:"column"`          // 专栏（可空）
	Tags     []Tag    `gorm:"many2many:article_tags" json:"tags"`         // 多对多标签
}
