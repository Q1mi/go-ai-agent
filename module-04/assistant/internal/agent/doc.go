// Package agent 对应课件 M04 的 Agent 核心运行时。
//
// 代码按讲义顺序拆成几个文件：
//   - state.go：4.2 状态机抽象
//   - react.go：4.4 ReAct 文本格式解析
//   - agent.go：4.5 Function Calling 循环与 4.8 RunStream
//   - budget.go：4.6 停止条件与预算
//   - store.go：4.10 状态持久化
//
// 这样安排的目的是让学员能把课件中的片段逐段对应到完整项目。
package agent
