# docqa-context

M03「Prompt 与上下文工程基础」完整配套练习。

这个项目在前两章代码形态上继续演进：

- 沿用 M01 的 `transport.NewClient().Do(req)` 风格 HTTP 客户端；
- 内置 M02 的大模型网关代码，沿用 `llm.Provider`、`ChatRequest`、`ChatResponse` 抽象；
- 沿用 M02 的 `schema.Generate` 生成结构化输出 Schema；
- 在 M03 新增 `prompt.Template`、文档知识库检索、上下文预算和 Prompt Caching 排布。

完整链路：

```text
用户问题
  -> knowledge.LoadDir 读取本地文档
  -> knowledge.Retrieve 检索 top-k 资料
  -> contextpack.BuildPlan 组装 system/user messages
  -> gateway.NewFromEnv 创建 M02 大模型网关
  -> gateway.Chat 调用 OpenAI 兼容模型
  -> 输出答案、token、命中文档
```

项目完成课后练习要求：

- 用 `prompt.Template` 渲染文档问答助手 System Prompt；
- 模板启用 `missingkey=error`；
- Prompt 包含角色、规则、资料占位和 few-shot 示例；
- 用 `schema.Generate` 生成 `{level, reason}` 结构化输出 Schema；
- 估算 System、Schema、示例历史、检索片段、用户问题的 token；
- 输出预算表、Prompt Caching 稳定前缀说明、当前时间放置策略和超长资料处理策略。
- 支持使用真实大模型和本地文档知识库运行文档问答。

## 配置模型

03 练习代码内置 M02 的网关实现，支持通用 `LLM_*` 配置，也支持
`DEEPSEEK_*`、`OPENAI_*`、`DOUBAO_*`、`OLLAMA_*` 这类 M02 网关配置。

使用 OpenAI 兼容接口时，以 DeepSeek 为例：

```bash
export LLM_BASE_URL="https://api.deepseek.com"
export LLM_API_KEY="你的 API Key"
export LLM_MODEL="deepseek-chat"
```

也可以换成其他 OpenAI 兼容平台，只需要调整 `LLM_BASE_URL` 和 `LLM_MODEL`。

如果希望验证网关的多 Provider 配置，可以这样启用 DeepSeek：

```bash
export DEEPSEEK_BASE_URL="https://api.deepseek.com"
export DEEPSEEK_API_KEY="你的 API Key"
export DEEPSEEK_MODEL="deepseek-chat"
```

示例知识库在 [knowledge](./knowledge) 目录，支持 `.md` 和 `.txt`。

## 运行

调用模型回答：

```bash
go run ./cmd/docqa "如何修改默认超时？"
```

可选参数：

```bash
go run ./cmd/docqa \
  -product "示例网关" \
  -docs ./knowledge \
  -top-k 3 \
  -question "如何修改默认超时？"
```

只查看组装后的上下文和预算：

```bash
go run ./cmd/docqa -dry-run "如何修改默认超时？"
```

## 验证

```bash
make test
make vet
make build
```
