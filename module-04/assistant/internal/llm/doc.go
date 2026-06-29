// Package llm 定义 M04 Agent 核心需要的大模型统一抽象。
//
// 它在 M02 对话接口基础上加入工具定义、工具调用和 Provider 能力声明，
// 让 Agent 可以在 Function Calling 与 ReAct 两种模式之间切换。
package llm
