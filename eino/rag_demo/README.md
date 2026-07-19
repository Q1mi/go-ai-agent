# Eino + Milvus RAG 示例

这个示例实现一条可直接运行的文档入库与语义检索链路：

```text
docs 文档 -> Loader -> Markdown 分块 -> Ark Embedding -> Milvus -> 相似度检索
```

## 目录职责

- `loader.go`：递归加载 `docs` 下的文件，记录来源并生成稳定文档 ID。
- `transformer.go`：先按 Markdown 标题切分，再对长章节执行带重叠切分。
- `embedding.go`：调用火山方舟 Embedding，分批生成文档向量和查询向量。
- `indexer.go`：通过 Eino Milvus2 Indexer 以 Upsert 方式写入本地 Milvus。
- `retriever.go`：将问题向量化，执行 COSINE 检索并还原 Eino Document。
- `main.go`：提供 `index`、`query`、`all` 三种运行模式。

## 运行

进入当前目录并启动 Milvus：

```bash
cd rag_demo
docker compose -f ../rag/docker-compose.yml up -d
docker compose -f ../rag/docker-compose.yml ps
```

配置火山方舟。默认配置沿用本示例原先使用的多模态 Embedding 模型和 2048 维向量：

```bash
export ARK_API_KEY="你的 API Key"
export ARK_EMBEDDING_MODEL="doubao-embedding-vision-251215"
export ARK_EMBEDDING_API_TYPE="multimodal"
export EMBEDDING_DIMENSION="2048"
```

如果使用文本 Embedding 模型，将模型名、API 类型和维度切换到对应配置：

```bash
export ARK_EMBEDDING_MODEL="你的文本 Embedding 模型或推理接入点 ID"
export ARK_EMBEDDING_API_TYPE="text"
export EMBEDDING_DIMENSION="模型输出维度"
```

首次运行完整链路：

```bash
go run . -mode all -query "Aurora 手机的保修期是多久？" -topk 3
```

后续可以分别执行入库和检索：

```bash
go run . -mode index -docs ../rag/docs
go run . -mode query -query "产品进水后可以免费维修吗？" -topk 3
```

`index` 使用稳定 chunk ID 和 Milvus Upsert，重复执行会更新对应文本块。更换 Embedding 模型或向量维度时，请使用新的 `MILVUS_COLLECTION`，也可以先手动删除旧 collection 后重新入库。

## 常用环境变量

| 变量 | 默认值 | 用途 |
| --- | --- | --- |
| `ARK_API_KEY` | 无 | 火山方舟 API Key，也兼容 `DOUBAO_API_KEY` |
| `ARK_EMBEDDING_MODEL` | `doubao-embedding-vision-251215` | Embedding 模型或推理接入点 ID |
| `ARK_EMBEDDING_API_TYPE` | `multimodal` | `text` 或 `multimodal` |
| `EMBEDDING_DIMENSION` | `2048` | 向量维度，需与模型输出一致 |
| `MILVUS_ADDRESS` | `localhost:19530` | Milvus 地址 |
| `MILVUS_COLLECTION` | `eino_rag_demo` | collection 名称 |
| `RAG_DOCS_DIR` | `../rag/docs` | 文档目录 |
| `RAG_CHUNK_SIZE` | `1200` | 单块最大 rune 数 |
| `RAG_CHUNK_OVERLAP` | `150` | 相邻块重叠 rune 数 |

查看 Milvus 日志或停止服务：

```bash
docker compose -f ../rag/docker-compose.yml logs -f standalone
docker compose -f ../rag/docker-compose.yml down
```
