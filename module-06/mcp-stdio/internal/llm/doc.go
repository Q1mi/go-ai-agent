// Package llm 定义 M06 练习使用的中立模型抽象。
//
// 这里沿用 M04 的 Function Calling 边界：业务层只看 ToolDef、ToolCall 和
// Message；Provider 在边界处把它们翻译成 OpenAI 兼容协议等厂商格式。
package llm
