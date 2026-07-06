# mini-kb

`mini-kb` 是 M07「记忆系统与 Agentic RAG」配套练习代码。项目实现一条完整知识库问答链路：

1. 遍历本地 `.md` / `.txt` 文档；
2. 使用 `RecursiveChunker` 切分文本；
3. 使用 `Embedder` 生成向量；
4. 写入 PostgreSQL + pgvector；
5. 通过向量检索 + PostgreSQL 全文检索召回候选；
6. 使用 RRF 融合，再用本地 reranker 重排；
7. 输出带来源标注的回答；
8. 将检索能力包装为 `search_knowledge_base` 工具，接入 Agent。

## 目录结构

```text
mini-kb/
├── cmd/minikb/main.go              # 命令行入口
├── docs/                           # 示例知识库文档
├── internal/agent/                 # 最小 Function Calling Agent
├── internal/config/                # 环境变量配置
├── internal/embed/                 # 本地 hash embedding
├── internal/llm/                   # Provider 无关模型类型
├── internal/providers/openai/      # OpenAI 兼容 chat 和 embedding
├── internal/qa/                    # 带来源回答生成
├── internal/rag/                   # chunker、store、retriever、tool
├── internal/schema/                # 最小 JSON Schema 生成器
├── internal/tool/                  # 工具注册表与 typed tool
├── sql/schema.sql                  # 参考建表 SQL
├── docker-compose.yml              # PostgreSQL + pgvector
└── Makefile
```

## 快速运行

启动 PostgreSQL + pgvector：

```bash
make docker-up
```

索引示例文档：

```bash
make reindex
```

检索知识库：

```bash
make search QUESTION="MiniCall 如何配置模型"
```

离线问答摘要：

```bash
make ask QUESTION="v0.3.0 增加了什么能力"
```

对比纯向量检索和 hybrid+rerank：

```bash
make compare QUESTION="知识库回答需要遵守什么规则"
```

## 使用大模型生成综合回答

`ask -llm` 会先检索资料，再把资料交给 OpenAI 兼容 Chat Completions 模型生成回答。

```bash
export LLM_BASE_URL="https://api.deepseek.com"
export LLM_API_KEY="你的 API Key"
export LLM_MODEL="deepseek-chat"

make ask-llm QUESTION="MiniCall 如何配置模型"
```

## 运行 Agentic RAG

`agent` 命令会把检索器包装成 `search_knowledge_base` 工具。模型根据问题自行决定检索时机、查询内容和是否需要多次检索。

```bash
export LLM_BASE_URL="https://api.deepseek.com"
export LLM_API_KEY="你的 API Key"
export LLM_MODEL="deepseek-chat"

make agent QUESTION="MiniCall v0.3.0 增加了什么能力？回答时给出来源"
```

运行时可以看到工具调用轨迹：

```text
[工具调用] search_knowledge_base {"query":"MiniCall v0.3.0 能力","top_k":5}
[工具结果]
[来源 1 | doc=release.md | chunk=0 | score=...]
...
```

## Embedding 配置

默认使用本地 hash embedding，便于直接完成课程练习：

```bash
export MINIKB_EMBEDDER=local
export MINIKB_EMBED_DIM=384
```

也可以切换到 OpenAI 兼容 embedding。切换维度后需要重建索引：

```bash
export MINIKB_EMBEDDER=openai
export MINIKB_EMBED_BASE_URL="https://api.example.com/v1"
export MINIKB_EMBED_API_KEY="你的 API Key"
export MINIKB_EMBED_MODEL="text-embedding-3-small"
export MINIKB_EMBED_DIM=1536

make reindex
```

## 常用命令

```bash
go run ./cmd/minikb init -reset
go run ./cmd/minikb index -docs ./docs
go run ./cmd/minikb search -mode hybrid -top 5 "MiniCall 如何配置模型"
go run ./cmd/minikb search -mode vector -top 5 "MiniCall 如何配置模型"
go run ./cmd/minikb search -mode keyword -top 5 "MiniCall 如何配置模型"
go run ./cmd/minikb ask "知识库回答需要遵守什么规则"
go run ./cmd/minikb ask -llm "知识库回答需要遵守什么规则"
go run ./cmd/minikb agent "MiniCall 如何配置模型？"
go run ./cmd/minikb compare "MiniCall 如何配置模型"
```

## 验收点

- `IndexDir` 完成目录遍历、切分、embedding、`PgVectorStore.Add`。
- `Retriever` 完成向量召回、全文召回、RRF 融合和 rerank。
- `SearchTool` 将 RAG 检索包装成 Agent 可调用工具。
- 回答输出 `[来源 N]` 标注。
- 无命中文档时输出“知识库中未找到足够资料”。
- `go test -race ./...` 覆盖 `RRF` 和 `RecursiveChunker.Split`。
