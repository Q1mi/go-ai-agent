package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/agenticark"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

/*
	为什么需要 AgenticXxx 系列新组件呢？
	普通组件使用 schema.Message，聚焦跨模型通用的文本、多模态和 Function Calling 字段。
	Agentic 组件使用 schema.AgenticMessage，以有序 ContentBlocks 保存模型原生执行轨迹。
	这些 block 可以表达 reasoning 签名、Server Tool、MCP、审批、函数工具及多模态工具结果。
*/

// Eino AgenticXxx 系列组件：
//   - AgenticChatTemplate：生成 []*schema.AgenticMessage
//   - AgenticModel：消费和生成 *schema.AgenticMessage
//   - AgenticToolsNode：执行 FunctionToolCall block，返回 FunctionToolResult block
func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	// 1. 创建一个普通的本地函数工具。
	orderTool, err := createQueryOrderTool()
	if err != nil {
		return fmt.Errorf("创建订单查询工具: %w", err)
	}
	toolInfo, err := orderTool.Info(ctx)
	if err != nil {
		return fmt.Errorf("读取工具定义: %w", err)
	}

	// 2. AgenticChatTemplate 的模板元素是 AgenticMessage。
	//    模板输出仍保留 ContentBlock 结构，可继续承载推理、文件、工具调用等内容。
	tpl := prompt.FromAgenticMessages(
		schema.FString,
		schema.SystemAgenticMessage("你是订单助手。查询订单时调用 query_order 工具。"),
		schema.AgenticMessagesPlaceholder("history", true),
		schema.UserAgenticMessage("帮我查一下我刚下单的东西到哪儿了？订单号：{order_id}"),
	)

	messages, err := tpl.Format(ctx, map[string]any{
		"history":  []*schema.AgenticMessage{},
		"order_id": "lwz-1234567",
	})
	if err != nil {
		return fmt.Errorf("格式化 Agentic 消息: %w", err)
	}
	printJSON("① AgenticChatTemplate 输出", messages)

	// 3. AgenticModel 在每次请求时通过 model.WithTools 接收工具定义。
	agenticModel, err := agenticark.New(ctx, &agenticark.Config{
		APIKey:  os.Getenv("DOUBAO_API_KEY"),
		BaseURL: os.Getenv("DOUBAO_BASE_URL"),
		Model:   os.Getenv("DOUBAO_MODEL"),
	})
	if err != nil {
		log.Fatalf("create chat model failed, err:%v", err)
	}

	decision, err := agenticModel.Generate(
		ctx,
		messages,
		model.WithTools([]*schema.ToolInfo{toolInfo}),
		model.WithAgenticToolChoice(&schema.AgenticToolChoice{Type: schema.ToolChoiceAllowed}),
	)
	if err != nil {
		return fmt.Errorf("AgenticModel 第一次生成: %w", err)
	}
	printJSON("② AgenticModel 原生块式输出", decision)

	// 4. AgenticToolsNode 只执行 FunctionToolCall block。
	//    reasoning、server_tool_call 等原生 block 会继续保存在 decision 中。
	toolsNode, err := compose.NewAgenticToolsNode(ctx, &compose.ToolsNodeConfig{
		Tools:               []tool.BaseTool{orderTool},
		ExecuteSequentially: true,
	})
	if err != nil {
		return fmt.Errorf("创建 AgenticToolsNode: %w", err)
	}
	toolResults, err := toolsNode.Invoke(ctx, decision)
	if err != nil {
		return fmt.Errorf("执行 AgenticToolsNode: %w", err)
	}
	printJSON("③ AgenticToolsNode 输出", toolResults)

	// 5. 将模型原始输出和工具结果一起回传。
	//    推理签名、服务端工具轨迹、函数调用和函数结果都保持原来的类型与顺序。
	messages = append(messages, decision)
	messages = append(messages, toolResults...)
	answer, err := agenticModel.Generate(
		ctx,
		messages,
		model.WithTools([]*schema.ToolInfo{toolInfo}),
	)
	if err != nil {
		return fmt.Errorf("AgenticModel 第二次生成: %w", err)
	}
	printJSON("④ AgenticModel 最终回答", answer)

	return nil
}

func printJSON(title string, value any) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fmt.Printf("\n===== %s =====\n序列化失败: %v\n", title, err)
		return
	}
	fmt.Printf("\n===== %s =====\n%s\n", title, data)
}
