# context-governance

`context-governance` 是 M09「Context Engineering（上下文工程）」配套练习代码。项目实现一个会累积历史、并反复读取长文档的简易 Agent，然后对比治理前后的 token 占用曲线。

## 覆盖能力

- `ctxeng.EstimateTokens`：低成本 token 估算；
- `ctxeng.Budget`：上下文分项预算和总预算门控；
- `ctxeng.Compact`：历史超预算时压缩早期消息；
- `ctxeng.FileMemory`：大段工具结果写入文件，上下文保留引用和摘要；
- `read_memory` 工具：按 id 读取外置全文；
- `ctxeng.SelectTools`：按当前问题动态暴露工具；
- `ctxeng.Note`：结构化工作笔记类型；
- ASCII token 曲线与 CSV 输出。

## 目录结构

```text
09-context-governance/
├── cmd/ctxdemo/main.go             # 演示入口
├── docs/architecture-plan.md       # read_doc 返回的长文档
├── internal/ctxeng/                # 上下文治理核心包
├── internal/demo/                  # 未治理 / 已治理对比模拟
├── internal/llm/                   # 最小消息类型
├── internal/tool/                  # 最小工具接口
├── Makefile
└── README.md
```

## 快速运行

```bash
make run
```

输出包括：

- 每轮未治理 total token；
- 每轮已治理 total token；
- history 分项 token；
- 每轮动态暴露的工具；
- `read_doc` 外置动作；
- 历史压缩动作；
- ASCII token 曲线。

增加轮数观察增长趋势：

```bash
make run ROUNDS=12
```

写出 CSV：

```bash
make csv ROUNDS=12 CSV=token-curve.csv
```

读取外置内容：

```bash
make read-memory ID=mem-xxxxxxxxxxxxxxxx
```

## 关键命令

```bash
go run ./cmd/ctxdemo \
  -rounds 8 \
  -history-budget 900 \
  -tools-budget 160 \
  -total-budget 1500 \
  -offload-threshold 260 \
  -keep-recent 6 \
  -max-tools 2
```

常用参数：

```text
-rounds
    模拟对话轮数。

-history-budget
    历史消息预算，超过后触发 Compact。

-tools-budget
    工具定义预算。演示中通过 SelectTools 降低工具定义占用。

-total-budget
    总输入预算。

-offload-threshold
    工具结果超过该估算 token 后写入 FileMemory。

-keep-recent
    历史压缩后保留最近多少条原文消息。

-max-tools
    每轮最多暴露多少个工具。

-read-memory
    读取外置内容 id。
```

## 课堂讲解路径

建议按以下顺序讲：

1. 先运行 `make run`，观察未治理曲线快速增长；
2. 打开 `internal/demo/demo.go`，解释两条链路的差异；
3. 打开 `internal/ctxeng/budget.go`，讲预算门控；
4. 打开 `internal/ctxeng/file_memory.go`，讲工具结果外置；
5. 打开 `internal/ctxeng/compact.go`，讲历史压缩；
6. 打开 `internal/ctxeng/tools.go`，讲动态工具暴露；
7. 用 `make csv` 导出数据，让学员画图或做进一步评估。

## 测试

```bash
make test
make vet
make build
```

测试覆盖：

- `EstimateTokens` 表驱动测试；
- `Budget.Over`；
- `Compact` 注入假 summarize；
- `AssembleWithReport`；
- `FileMemory.Offload` 与 `Read`；
- `SelectTools` 表驱动测试。

## 课堂提示

本练习中的 token 估算、压缩摘要和工具选择都采用教学级实现。生产系统应接入供应商 tokenizer、真实 usage、模型摘要、检索式工具路由、外置内容生命周期管理和评估集。重点是理解治理流程：先估算，再发现超标来源，然后对相应部分执行压缩、外置或动态选择。
