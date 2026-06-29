// Package openai 提供 OpenAI 兼容的大模型 Provider。
//
// M04 默认通过这个 Provider 调用真实模型；请求里会带上本地工具定义，
// 模型可直接回答普通问题，也可在适合工具的场景返回 tool_calls。
package openai
