# minicall

M01「Go 语言 AI 开发基础」的完整配套练习。它通过 OpenAI 兼容的
`/chat/completions` 接口发送一次非流式请求，并打印模型回答和 token 用量。

## 运行

需要 Go 1.22 或更高版本，以及一个可用的 OpenAI 兼容 API Key。

```bash
export LLM_BASE_URL="https://api.deepseek.com"
export LLM_API_KEY="你的 API Key"
export LLM_MODEL="deepseek-v4-pro"

go run ./cmd/minicall "用一句话解释什么是 AI Agent"
```

也可以通过 Makefile 运行：

```bash
make run QUESTION="为什么 Go 适合开发 AI Agent？"
```

不要把真实 API Key 写入源码或提交到 Git。

## 验证

```bash
make test
make build
```

测试使用 `httptest` 模拟模型服务，不需要 API Key，也不会产生模型调用费用。
