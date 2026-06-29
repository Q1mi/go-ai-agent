# assistant

M04「Agent 核心架构」配套练习：命令行 AI 助手。

这个项目按课件 04 的展开顺序组织，目标是让学员从讲义中的片段自然过渡到完整可运行代码。

## 从课件到代码

| 课件小节 | 讲义里的关键代码 | 练习代码位置 |
| --- | --- | --- |
| 4.2 状态机抽象 | `Phase`、`State` | [internal/agent/state.go](./internal/agent/state.go) |
| 4.2 Agent 结构 | `Agent`、`New`、`Option` | [internal/agent/agent.go](./internal/agent/agent.go) |
| 4.2 工具注册表 | `tool.Tool`、`tool.Registry` | [internal/tool/tool.go](./internal/tool/tool.go) |
| 4.4 ReAct 循环 | `reactSystemTmpl`、`parseReact`、`runReAct` | [internal/agent/react.go](./internal/agent/react.go)、[internal/agent/agent.go](./internal/agent/agent.go) |
| 4.5 Function Calling 循环 | `ToolDef`、`ToolCall`、`runFunctionCalling`、`toolDefs` | [internal/llm/types.go](./internal/llm/types.go)、[internal/agent/agent.go](./internal/agent/agent.go) |
| 4.6 停止条件与预算 | `Budget.Exceeded` | [internal/agent/budget.go](./internal/agent/budget.go) |
| 4.8 AgentEvent 事件流 | `AgentEvent`、`RunStream` | [internal/agent/event.go](./internal/agent/event.go)、[internal/agent/agent.go](./internal/agent/agent.go) |
| 4.9 Plan-and-Execute | `plan.Task`、`plan.Levels`、`plan.Execute` | [internal/plan](./internal/plan) |
| 4.10 状态持久化 | `Store`、`FileStore` | [internal/agent/store.go](./internal/agent/store.go) |
| 配套练习 | `assistant` CLI、OpenAI 兼容 Provider、`calculator`、`now` | [cmd/assistant](./cmd/assistant)、[internal/providers/openai](./internal/providers/openai)、[internal/tools](./internal/tools) |

建议阅读路径：

1. 先看 [internal/agent/state.go](./internal/agent/state.go)，确认 Agent 运行态如何保存。
2. 再看 [internal/tool/tool.go](./internal/tool/tool.go) 和 [internal/tools](./internal/tools)，确认工具如何声明和执行。
3. 然后看 [internal/agent/agent.go](./internal/agent/agent.go) 的 `runFunctionCalling`，这是主循环。
4. 接着看 [internal/agent/react.go](./internal/agent/react.go) 和 `runReAct`，理解文本格式兜底。
5. 最后看 [cmd/assistant/main.go](./cmd/assistant/main.go)，了解 CLI 如何接入事件流、预算、会话存储和 Ctrl+C。

## 实现内容

- 状态机：`thinking`、`acting`、`done`、`error`
- Function Calling 循环
- ReAct 文本格式兜底
- `AgentEvent` 事件流
- `Budget` 步数与 token 预算
- `FileStore` 会话保存和恢复
- 工具注册表
- 内置 `calculator` 与 `now` 两个工具
- `schema.Generate` 生成工具参数 Schema
- `plan.Levels` 拓扑分层与计划执行骨架
- OpenAI 兼容大模型 Provider，支持普通对话和 Function Calling

## 模型出口说明

课件练习要求接入 M02 的 router 作为模型出口。本项目内置 [internal/providers/openai](./internal/providers/openai)，默认通过 OpenAI 兼容接口调用真实大模型，并把 `calculator`、`now` 两个工具作为 Function Calling 工具传给模型。

运行逻辑是：

```text
用户输入
  -> 大模型 Provider 对话
  -> 模型直接回答普通问题
  -> 模型返回 tool_calls 时执行本地工具
  -> 工具结果回填给模型
  -> 大模型整理最终回答
```

真实接入 M02 router 时，保留 `agent.Agent` 主体，只把 provider 替换为 [internal/agent/router_provider.go](./internal/agent/router_provider.go) 中的适配器即可。这个设计对应课件 4.2 里“Agent 只依赖 `llm.Provider` 接口”的部分。

## 配置模型

运行前需要配置 OpenAI 兼容模型：

```bash
export LLM_BASE_URL="https://api.deepseek.com"
export LLM_API_KEY="你的 API Key"
export LLM_MODEL="deepseek-chat"
```

也可以换成其他 OpenAI 兼容平台，只需要调整 `LLM_BASE_URL` 和 `LLM_MODEL`。如果 `LLM_BASE_URL` 为空，默认使用 `https://api.deepseek.com`。

## 运行

单轮：

```bash
go run ./cmd/assistant "现在几点？顺便计算 1+2*3"
```

ReAct 兜底模式，适合演示不支持原生工具调用的模型路径：

```bash
go run ./cmd/assistant -mode react "计算 (1+2)*3"
```

交互模式：

```bash
go run ./cmd/assistant
```

会话保存和恢复：

```bash
go run ./cmd/assistant --session my-session "现在几点？"
go run ./cmd/assistant --session my-session "刚才你查到了什么？"
```

运行时输出事件与课件 4.8 一一对应：

```text
[思考] ...
[调用工具] calculator({"expr":"1+2*3"})
[工具结果] 1+2*3 = 7
最终答案
[完成]
```

## 验证

```bash
make test
make vet
make build
```
