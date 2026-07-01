# M06 扩展练习：使用 mcp-go 编写 MCP Server

这个项目是 M06 的独立扩展练习，目标是使用社区库 `github.com/mark3labs/mcp-go` 快速搭建一个 stdio MCP Server，并用 MCP Inspector 调试。

## 目录结构

```text
.
├── cmd/mcpgo-server      # mcp-go stdio MCP Server
├── internal/calc         # 四则运算表达式解析
├── mcp.json              # Inspector / MCP Client 配置示例
├── Makefile
├── go.mod
└── README.md
```

## 工具列表

Server 暴露两个工具：

- `get_time`：返回当前时间，支持 `timezone` 参数，例如 `Asia/Shanghai`。
- `calc`：计算只包含数字、括号、`+`、`-`、`*`、`/` 的表达式。

## 运行测试

```bash
cd code/06-mcp-go-inspector
make test
```

## 本地启动 Server

```bash
make run
```

stdio Server 会占用当前终端等待 MCP Client 输入 JSON-RPC 消息。普通用户通常通过 Inspector 或其他 MCP Client 连接它。

## 使用 MCP Inspector 连接

方式一：直接指定启动命令。

```bash
make inspector
```

等终端输出 Inspector 地址后，打开浏览器访问。进入页面后执行：

1. 点击连接；
2. 执行 `List Tools`；
3. 选择 `get_time` 或 `calc`；
4. 输入参数并调用。

方式二：使用 `mcp.json`。

```bash
make inspector-config
```

`mcp.json` 内容如下：

```json
{
  "mcpServers": {
    "m06-mcp-go": {
      "type": "stdio",
      "command": "go",
      "args": ["run", "./cmd/mcpgo-server"]
    }
  }
}
```

## 环境要求

- Go 1.23 或更高版本；
- Node.js 22.7.5 或更高版本，用于运行 MCP Inspector。

`mark3labs/mcp-go` 在较新版本中提升了 Go 版本要求。本项目锁定 `v0.45.0`，兼顾当前 MCP 生态和课程环境门槛。

## 与手写版本的关系

`code/06-tools-mcp-skills` 保留手写 JSON-RPC / stdio MCP Server，用来理解协议细节。

本项目展示工程库封装后的写法：用 `mcp.NewTool` 声明工具，用 `s.AddTool` 注册处理函数，用 `server.ServeStdio` 启动服务。两个项目可以配合学习：先理解协议，再使用库提升开发效率。
