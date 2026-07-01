# M05 Agent 设计模式配套练习：代码审查 Agent

这个目录实现 M05 的课后练习：用 Planner / Generator / Evaluator 三 Agent 范式审查 Go 代码片段。

## 目录对应关系

| 课件章节 | 代码 |
| --- | --- |
| 5.2 增强型 LLM | `internal/patterns/complete.go` |
| 5.3 Prompt Chaining | `internal/patterns/chain.go` |
| 5.4 Routing | `internal/patterns/router.go` |
| 5.5 Parallelization | `internal/patterns/parallel.go` |
| 5.6 Evaluator-Optimizer | `internal/patterns/evaluator.go` |
| 5.7 Orchestrator-Workers | `internal/patterns/orchestrator.go` |
| 配套练习代码审查 Agent | `internal/review`、`cmd/reviewagent` |

## 运行方式

先配置一个 OpenAI 兼容模型：

```bash
cp .env.example .env
export LLM_BASE_URL=https://api.deepseek.com
export LLM_API_KEY=sk-...
export LLM_MODEL=deepseek-chat
```

审查文件：

```bash
go run ./cmd/reviewagent --file ./example.go
```

从 stdin 输入：

```bash
cat ./example.go | go run ./cmd/reviewagent
```

普通问题会先经过 `IntentRouter`，然后走普通回答：

```bash
go run ./cmd/reviewagent "请解释 M05 的三 Agent 范式"
```

输出 JSON：

```bash
go run ./cmd/reviewagent --format=json --file ./example.go
```

## 这个任务真的需要三 Agent 吗？

短小代码片段通常用一次精心设计的代码审查提示词就够，速度更快，token 成本更低。

三 Agent 方案适合这些情况：

- 代码较长，容易漏掉不同维度的问题；
- 希望 Planner 先明确审查清单；
- 希望 Generator 按维度并行审查，降低单一路径漏报；
- 希望 Evaluator 对照计划做补漏检查；
- 接受更多 LLM 调用带来的成本和延迟。

工程取舍很直接：先用单次调用建立基线。如果漏报率、报告可操作性或稳定性达不到要求，再升级到三 Agent 流程。

## 验证

```bash
go build ./...
go test ./...
go test -race ./...
```
