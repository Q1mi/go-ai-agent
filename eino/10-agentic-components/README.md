# Eino AgenticXxx 教学示例

这个示例对应已有的 `chatmodel_chattemplate` 和 `tool` 示例，演示以下三个新组件：

- `AgenticChatTemplate`：格式化并输出 `[]*schema.AgenticMessage`。
- `AgenticModel`：消费和生成 `schema.AgenticMessage`，工具通过 `model.WithTools` 按请求传入。
- `AgenticToolsNode`：执行 `FunctionToolCall` block，并生成 `FunctionToolResult` block。

## 为什么增加 Agentic 系列

`schema.Message` 为不同 Chat API 提供统一的通用字段，适合常规对话、多模态输入和 Function Calling。

新一代模型的 Responses/Agents API 会返回更丰富的原生执行轨迹，例如：

- reasoning 内容及需要回传的签名；
- 模型提供方执行的 Server Tool 调用和结果；
- MCP 调用、结果以及人工审批；
- 客户端 Function Tool 调用；
- 文本、图片、音频、视频和文件形式的工具结果；
- 厂商特有的响应元数据和缓存标识。

`schema.AgenticMessage` 使用有序 `ContentBlocks` 表达这些信息，让它们可以在多轮 Agent 执行中原样回传。AgenticChatTemplate、AgenticModel 和 AgenticToolsNode 统一使用这种消息类型，减少组件之间转换时的信息损失。

## 运行

示例默认使用离线 `demoAgenticModel`，无需 API Key：

```bash
cd agenticxxx
go run .
```

运行后依次观察：

1. AgenticChatTemplate 生成的输入 block；
2. AgenticModel 同时返回 reasoning、Server Tool 和 Function Tool block；
3. AgenticToolsNode 将函数调用转换成类型明确的函数结果；
4. AgenticModel 接收完整轨迹后生成最终回答。

接入真实火山方舟 Responses API 时，可用 `agenticark.New(...)` 创建的模型替换 `demoAgenticModel`；OpenAI Responses API 可使用 `agenticopenai.New(...)`。
