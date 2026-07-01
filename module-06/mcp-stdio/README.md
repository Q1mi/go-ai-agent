# M06 配套练习：Tools、MCP 与 Skills

本目录是课件 M06 的完整练习代码。它从前面课程的 `llm.Provider`、`tool.Tool`、`agent.Agent` 继续演进，实现一个可运行的 MCP stdio Server，并通过 MCP Client + Bridge 接入到 Agent。

## 目录结构

```text
.
├── cmd
│   ├── mcpagent    # 启动 MCP Server，把工具桥接进 Agent，再调用大模型
│   ├── mcpclient   # 独立 MCP Client，便于调试 tools/list 和 tools/call
│   └── mcpserver   # 最小 MCP stdio Server
├── internal
│   ├── agent       # M04 风格 Function Calling Agent
│   ├── builtin     # 内置工具：get_time、calc，以及安全示例
│   ├── llm         # Provider 无关类型
│   ├── mcp         # JSON-RPC、Server、Client、Bridge
│   ├── providers   # OpenAI 兼容模型接入
│   ├── schema      # 从 Go 结构体生成 JSON Schema
│   └── tool        # Tool 接口、TypedTool、Registry、输出清洗
├── Makefile
├── go.mod
└── .env.example
```

## 快速运行

先进入练习目录：

```bash
cd code/06-tools-mcp-skills
```

运行测试：

```bash
make test
```

列出 MCP Server 暴露的工具：

```bash
make client-list
```

直接调用 `calc` 工具：

```bash
make client-call
```

运行 Agent 前，需要配置一个支持 Function Calling 的 OpenAI 兼容模型：

```bash
cp .env.example .env
export LLM_BASE_URL="https://api.deepseek.com"
export LLM_API_KEY="你的 API Key"
export LLM_MODEL="deepseek-chat"
```

再执行：

```bash
make agent QUESTION="现在几点了？顺便计算 12*(3+4)"
```

也可以直接运行：

```bash
go run ./cmd/mcpagent "现在几点了？顺便计算 12*(3+4)"
```

## 本练习覆盖的课件要点

- `cmd/mcpserver` 实现了最小 stdio MCP Server，处理 `initialize`、`tools/list`、`tools/call`。
- `internal/mcp.StdioClient` 使用子进程 stdin/stdout 与 MCP Server 通信。
- `internal/mcp.BridgeAll` 把 MCP 工具转换为本项目的 `tool.Tool`。
- `llm.ToolDef.Parameters` 和 `MCPTool.InputSchema` 都使用 `json.RawMessage`，可以保留 MCP 与厂商 JSON Schema 的扩展字段。
- `cmd/mcpagent` 启动 MCP Server，完成初始化，桥接工具，使用 `tool.Sanitized` 清洗工具输出，再注册到 Agent。
- `internal/providers/openai` 负责把中立工具声明映射到 OpenAI 兼容 Function Calling 协议。

## 安全边界

MCP Server 可能来自本地项目、第三方包或外部团队。接入前应明确三点：

1. 固定 server 命令与版本，避免运行来源不明的可执行文件。
2. 检查工具描述、参数 Schema 和权限范围，高危工具需要最小权限。
3. 工具输出进入模型上下文前做长度限制和边界标记，本练习通过 `tool.Sanitized` 完成。

文件系统、数据库、HTTP 请求、命令执行类工具都应做显式白名单和审计记录。`internal/builtin` 中包含路径围栏和只读 SQL 校验示例，供后续扩展参考。
