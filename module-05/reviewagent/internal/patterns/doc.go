// Package patterns 实现 M05 课件中的 Agent 设计模式原语。
//
// 这些原语故意保持小而清晰：一次模型调用、线性链、路由、并行执行、
// 评估优化循环，以及编排者-工作者。代码审查 Agent 会组合其中的
// IntentRouter、Sectioning 和 EvaluatorOptimizer。
package patterns
