// Package knowledge 是 M03 在前两章模型调用能力之上新增的本地文档知识库。
//
// 它负责加载 .md/.txt 文档，并用简单词项匹配完成 top-k 检索。
// 后续 M07 Agentic RAG 会把这里替换成向量检索和更完整的召回链路。
package knowledge
