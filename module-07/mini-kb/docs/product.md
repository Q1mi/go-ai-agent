# MiniCall 产品手册

MiniCall 是课程中的最小大模型调用程序。它通过 OpenAI 兼容的 `/chat/completions` 接口和模型对话。

默认配置项如下：

- `LLM_BASE_URL`：模型服务地址，例如 `https://api.deepseek.com`
- `LLM_API_KEY`：模型服务密钥
- `LLM_MODEL`：模型名称，例如 `deepseek-chat`

MiniCall 的目标是帮助学员先跑通一次真实模型调用，再逐步理解 Provider、Agent、Tool 和 RAG。

如果程序返回认证失败，通常需要检查 API Key 是否正确、模型名是否存在、Base URL 是否包含多余路径。
