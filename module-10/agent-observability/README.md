# agent-observability

`agent-observability` 是 M10「可观测性、评估与安全」配套练习代码。项目实现一个手写 Agent，并通过 OpenAI-compatible 协议请求真实 DeepSeek 模型，同时为 Agent 根入口、模型调用和工具调用接入 OpenTelemetry trace。项目还包含最小评估集、确定性检查、轻量判官、轨迹评估、Prompt Injection 防线、限流和 token 配额示例。

## 课堂快速演示

先配置真实模型：

```bash
export DEEPSEEK_API_KEY=sk-...
export DEEPSEEK_MODEL=deepseek-chat
```

运行一次 Agent，并直接打印 trace 树：

```bash
make demo
```

示例输出会包含：

```text
trace <trace-id>
└─ invoke_agent kbot
   ├─ chat deepseek-chat
   ├─ execute_tool get_weather
   └─ chat deepseek-chat
```

## 使用 Docker Compose 启动 Jaeger

启动临时 Jaeger：

```bash
make jaeger-up
```

打开 UI：

```text
http://localhost:16686
```

发送一条 trace 到 Jaeger：

```bash
make demo-jaeger
```

在 Jaeger UI 中选择服务 `traceagent`，点击 `Find Traces` 即可看到 `invoke_agent kbot` 根 span，以及内部的 `chat deepseek-chat` 和 `execute_tool get_weather` 子 span。

运行评估集并把多条 trace 发送到 Jaeger：

```bash
make eval-jaeger
```

关闭临时 Jaeger：

```bash
make jaeger-down
```

运行评估集：

```bash
make eval
```

打印每条评估样本的 trace：

```bash
make eval-trace
```

演示 Prompt Injection 拦截：

```bash
make attack
```

导出 span JSON：

```bash
make json JSON=spans.json
```

## 目录结构

```text
10-agent-observability/
├── cmd/traceagent/main.go          # CLI：demo / eval / attack
├── internal/agent/                 # 手写 Agent，埋点位置在这里
├── internal/eval/                  # 确定性检查、轻量判官、轨迹评估
├── internal/llm/                   # 真实 OpenAI-compatible 模型 Provider
├── internal/obs/                   # OTel 初始化、GenAI span、内存 exporter、指标
├── internal/security/              # Prompt Injection、边界包裹、限流、配额
├── internal/tool/                  # get_weather / search_kb 工具
├── Makefile
└── README.md
```

## 与课件验收点对应

- Agent 根入口创建 `invoke_agent` span：
  - [internal/agent/agent.go](./internal/agent/agent.go)
  - [internal/obs/tracing.go](./internal/obs/tracing.go)

- 模型调用创建 `chat {model}` span：
  - `obs.RecordModelCall`

- 工具调用创建 `execute_tool {name}` span：
  - `obs.RecordToolCall`

- GenAI 语义属性：
  - `gen_ai.operation.name`
  - `gen_ai.provider.name`
  - `gen_ai.request.model`
  - `gen_ai.usage.input_tokens`
  - `gen_ai.usage.output_tokens`
  - `gen_ai.tool.name`
  - `gen_ai.tool.call.id`
  - `gen_ai.agent.name`

- 最小评估集：
  - `weather_umbrella`
  - `refund_policy`
  - `prompt_injection`

- 评估输出：
  - 每条样本的 pass / score / reason；
  - 每条样本对应的 trace_id；
  - 轨迹四维分：任务完成率、步骤效率、工具准确率、行动推进度。

## 常用命令

```bash
export DEEPSEEK_API_KEY=sk-...
go run ./cmd/traceagent demo -q "北京今天需要带伞吗？"
go run ./cmd/traceagent demo -exporter jaeger -otlp-endpoint localhost:4318 -q "北京今天需要带伞吗？"
go run ./cmd/traceagent demo -q "退款政策是什么？"
go run ./cmd/traceagent eval
go run ./cmd/traceagent eval -exporter jaeger
go run ./cmd/traceagent eval -judge
go run ./cmd/traceagent eval -show-trace
go run ./cmd/traceagent attack
```

## 课堂讲解重点

建议按以下顺序讲：

1. 先运行 `make demo`，看 trace 树；
2. 打开 `internal/agent/agent.go`，指出根 span、模型 span、工具 span 的位置；
3. 打开 `internal/obs/tracing.go`，讲 GenAI 属性；
4. 打开 `internal/obs/memory_exporter.go`，讲课堂本地 trace 展示；
5. 运行 `make jaeger-up && make demo-jaeger`，在 Jaeger UI 中查看真实 trace；
6. 运行 `make eval`，讲“看见 → 判断”的闭环；
7. 打开 `internal/eval/eval.go`，讲结果评估和轨迹评估；
8. 运行 `make attack`，讲安全拦截和 trace 里的安全事件。

## 验证

```bash
make fmt
make tidy
make test
make vet
make build
```

单元测试使用 `httptest` 模拟 OpenAI-compatible HTTP 服务，因此 `make test` 不消耗模型额度。运行 `demo`、`eval`、`judge`、`json` 等命令会请求真实模型。

本练习默认使用内存 exporter 打印 trace 树。需要 UI 时，使用 Docker Compose 启动 Jaeger，并通过 OTLP HTTP 端口 `4318` 导出 trace。接入 Phoenix 或 Langfuse 时，可以保留 `internal/obs/tracing.go` 中的 span 埋点代码，替换 OTLP endpoint。

## 模型环境变量

默认使用 DeepSeek：

```bash
export DEEPSEEK_API_KEY=sk-...
export DEEPSEEK_BASE_URL=https://api.deepseek.com
export DEEPSEEK_MODEL=deepseek-chat
```

也可以改用其他 OpenAI-compatible 服务：

```bash
export LLM_PROVIDER_NAME=openai-compatible
export LLM_BASE_URL=https://api.openai.com/v1
export LLM_API_KEY=sk-...
export LLM_MODEL=gpt-4.1-mini
```
