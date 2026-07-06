package rag

// Document 表示知识库中的一个文本片段。
type Document struct {
	ID         string
	DocID      string
	ChunkIndex int
	Content    string
	Score      float32
}

// SourceLabel 返回适合展示给用户的来源标识。
func (doc Document) SourceLabel() string {
	if doc.DocID == "" {
		return doc.ID
	}
	return doc.DocID
}
