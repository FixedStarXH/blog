package model

// ArticleChunk 文章向量分块（RAG 问答的数据源）
//
// 背景：AI 问答要"基于文章内容回答"，但大模型不知道你写了什么，
// 所以提前把每篇文章切成小块、每块算一个 embedding 向量存这里。
// 用户提问时：问题向量化 → 检索最相似的块 → 拼进 prompt 交给大模型。
//
// 设计说明：
//   - 向量用二进制 blob 存 float32 数组（encoding/binary 序列化），紧凑、可入库检索
//   - 不用软删除：块是"派生数据"，重建索引 = 清空旧块重新写入，软删除只会留垃圾
//   - 文章数量几百篇以内，全量加载到内存算余弦相似度完全够用，不引入向量数据库
type ArticleChunk struct {
	ID         uint   `gorm:"primarykey" json:"id"`
	ArticleID  uint   `gorm:"index" json:"articleId"` // 属于哪篇文章（硬关联，文章软删/重建索引时清理）
	ChunkIndex int    `gorm:"index" json:"chunkIndex"` // 块序号：切块顺序，检索后按序拼接上下文
	Content    string `gorm:"type:text" json:"content"` // 块原文（拼 prompt 用）
	Embedding  []byte `gorm:"type:blob" json:"-"`       // 向量二进制，不给前端
}
