# llmrouter

M02「LLM 全平台接入」的完整配套练习，包含：

- OpenAI 兼容 Provider：同步与 SSE 流式调用；
- Anthropic Messages Provider：请求适配、同步与流式调用；
- Google Gemini GenerateContent Provider：`contents` / `parts` / `systemInstruction` 适配；
- DeepSeek、豆包、Gemini 和 Ollama 的配置接入；
- 按优先级、成本或延迟排序的故障转移路由；
- token 用量、预估成本以及 P50/P95 延迟统计；
- 基于反射的基础 JSON Schema 生成器。

实现只使用 Go 标准库。测试通过 `httptest` 模拟服务，不需要 API Key。

## 配置

复制 `.env.example` 中需要的配置到 shell 环境。至少启用一个 Provider：

```bash
export DEEPSEEK_API_KEY="你的 API Key"
export DEEPSEEK_MODEL="控制台中的模型名"
export DEEPSEEK_INPUT_PER_1M_USD="0.00"
export DEEPSEEK_OUTPUT_PER_1M_USD="0.00"
```

价格变化较快，示例不硬编码厂商价格。请从供应商官方价格页读取当前单价，并通过
`*_INPUT_PER_1M_USD` 和 `*_OUTPUT_PER_1M_USD` 配置。

启用多个 Provider 时，每个 Provider 使用自己的默认模型。这样故障转移到 Claude、
Gemini 或 Ollama 时，不会错误沿用 DeepSeek 的模型名。

Gemini 使用原生 GenerateContent REST 协议：

```bash
export GEMINI_API_KEY="你的 API Key"
export GEMINI_MODEL="gemini-3.5-flash"
export GEMINI_BASE_URL="https://generativelanguage.googleapis.com/v1beta"
```

## 运行

```bash
go run ./cmd/llmrouter "为什么 Go 适合开发 AI Agent？"
```

常用选项：

```bash
go run ./cmd/llmrouter -stream "介绍一下 SSE"
go run ./cmd/llmrouter -strategy cheapest "解释什么是 Provider"
go run ./cmd/llmrouter -strategy latency "给出一个简短回答"
go run ./cmd/llmrouter -system "只回答一句话" -max-tokens 200 "什么是 Agent？"
```

`priority` 按 `LLM_PROVIDER_ORDER` 尝试；`cheapest` 按配置的输入、输出单价之和排序；
`latency` 优先使用已有 P50 统计，没有样本时使用 `*_LATENCY_HINT_MS`，仍相同时保持
配置顺序。

流式模式只在“建立流之前”做故障转移。某个 Provider 已经输出部分文本后再断流，
程序会报告错误而不会切换到另一家重答，避免把两家模型的文本拼在一起。

## 验证

```bash
make test
make vet
make build
```

课件代码片段的审查结果见 [COURSE_REVIEW.md](./COURSE_REVIEW.md)。
