// Package llm 承接 M02 的统一请求、响应和 Provider 抽象。
//
// M03 文档问答助手继续使用 Message、ChatRequest、ChatResponse 组织模型调用，
// 这样 prompt/context 代码只依赖统一抽象。
package llm
