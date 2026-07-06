# debate

`debate` 是 M08「多智能体系统」配套练习代码。项目实现一个多智能体辩论 demo：多个持不同立场的 Agent 围绕同一问题辩论多轮，评审综合最终观点，并与单 Agent 直接回答做成本和质量对照。

## 目录结构

```text
08-debate/
├── cmd/debate/main.go              # 命令行入口
├── internal/llm/                   # Provider 抽象、Usage、Meter
├── internal/mas/                   # MessageBus、Pipeline、Swarm、Debate、Judge
├── internal/providers/offline/     # 本地 deterministic provider
├── internal/providers/openai/      # OpenAI 兼容 provider
├── .env.example
├── Makefile
└── README.md
```

## 快速运行

默认使用本地 `offline` provider，可以直接看到完整流程：

```bash
make offline
```

指定问题和轮数：

```bash
make offline QUESTION="这段架构方案有什么风险？" ROUNDS=2
```

输出 Markdown 记录：

```bash
make transcript QUESTION="我们准备用单体架构起步，后期再拆微服务，有哪些风险？"
```

## 使用 OpenAI 兼容模型

复制环境变量示例并填入模型配置：

```bash
cp .env.example .env
source .env
```

加载环境变量后运行：

```bash
export LLM_BASE_URL="https://api.deepseek.com"
export LLM_API_KEY="你的 API Key"
export LLM_MODEL="deepseek-chat"

make openai QUESTION="我们准备用单体架构起步，后期再拆微服务，有哪些风险？"
```

也可以直接使用命令：

```bash
go run ./cmd/debate \
  -provider openai \
  -rounds 3 \
  -model deepseek-chat \
  "我们准备用单体架构起步、后期再拆微服务，这个决策有哪些风险？"
```

## 命令行参数

```text
-provider string
    Provider：auto、offline、openai，默认 auto

-rounds int
    辩论轮数，默认 3

-model string
    默认模型名称，默认读取 LLM_MODEL

-pragmatic-model string
    务实派模型；为空时使用 -model

-cautious-model string
    谨慎派模型；为空时使用 -model

-data-model string
    数据派模型；为空时使用 -model

-judge-model string
    评审模型；为空时使用 -model

-compare bool
    同时运行单 Agent baseline，默认 true

-transcript string
    把完整辩论记录写入 Markdown 文件

-timeout duration
    总超时时间，默认 2m
```

## 练习验收点对应关系

- 不同 `Persona`：`cmd/debate/main.go` 中内置务实派、谨慎派、数据派。
- 多轮 `Debate`：`internal/mas/debate.go` 的 `DebateWithTranscript` 按轮次并发运行辩手。
- 每轮观点变化：CLI 会打印 `Transcript.Rounds` 中的全部观点。
- `Judge` 综合定稿：`internal/mas/debate.go` 的 `Judge` 基于最终观点生成定稿。
- token 统计：`internal/llm.Meter` 统计调用次数和 usage。
- 单 Agent 对比：`internal/mas.Baseline` 与 `Debate + Judge` 在 CLI 中同时运行并输出成本表。
- 质量对比：`internal/mas.EvaluateQuality` 按落地、维护、风险、数据、执行五个维度输出覆盖情况。
- 轮数上限：`-rounds` 控制辩论轮数，`-timeout` 控制总生命周期。

## 测试与构建

```bash
make fmt
make tidy
make test
make vet
make build
```

`internal/mas` 的测试覆盖：

- `MessageBus` 定向消息与广播；
- `RunBusAgent` 收件箱处理；
- `DebateWithTranscript` 多轮辩论；
- `Judge` 与 `Baseline`；
- `EvaluateQuality`；
- `Stage` channel pipeline；
- `Swarm` handoff 与循环保护。
