// Package review 组合 M05 模式原语，实现小型 Go 代码审查 Agent。
//
// 主流程：
//   - IntentRouter 判断输入是否进入审查流程；
//   - Planner 产出审查维度；
//   - Generator 按维度并行审查；
//   - Evaluator 检查报告是否充分，不足时反馈给下一轮 Generator。
package review
