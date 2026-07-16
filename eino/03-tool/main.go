package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// eino components: tool

/*
调用工具的流程

1. 定义一个tool
2. 告诉大模型我有这么一个tool
3. 大模型分析决策出是否需要调用tool,以及调用什么tool,参数是什么
4. 解析大模型回复的结果，如果需要工具调用那么就调用工具
5. 把调用结果告诉大模型
6. 大模型根据用户的问题+工具的执行结果生成回答

*/

func main() {
	// mockToolDemo(context.Background())

	runTool(context.Background())
}

func mockToolDemo(ctx context.Context) {
	// 创建工具
	queryOrderTool := createTool2()

	// 创建 ToolsNode
	toolsNode, err := compose.NewToolNode(
		ctx,
		&compose.ToolsNodeConfig{
			Tools: []tool.BaseTool{
				queryOrderTool,
				// .... 支持多个工具
			},
			ExecuteSequentially: true, // 是否按顺序调用工具
		},
	)
	if err != nil {
		log.Fatalf("creatreToolsNode failed, err:%v", err)
	}

	// 调用
	// mock 大模型输出的工具调用消息
	input := &schema.Message{
		Role: schema.Assistant,
		ToolCalls: []schema.ToolCall{
			{
				Function: schema.FunctionCall{
					Name:      "query_order",
					Arguments: `{"order_id": "lwz-1234567"}`,
				},
			},
		},
	}

	toolMessages, err := toolsNode.Invoke(ctx, input)
	if err != nil {
		log.Fatalf("invoke failed, err:%v", err)
	}

	for _, msg := range toolMessages {
		fmt.Printf("msg:%#v\n", msg)
	}
}

func runTool(ctx context.Context) {
	// 大模型
	chatModel, err := ark.NewChatModel(ctx, &ark.ChatModelConfig{
		APIKey:  os.Getenv("DOUBAO_API_KEY"),
		Model:   os.Getenv("DOUBAO_MODEL"),
		BaseURL: os.Getenv("DOUBAO_BASE_URL"),
	})
	if err != nil {
		log.Fatalf("初始化模型失败: %w", err)
	}

	// 工具
	queryOrderTool := createTool2()

	// adk
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "order_assistant",
		Description: "可以查询订单的智能助手",
		Instruction: "你是一个智能助手，用户查询订单是必须调用 query_order 工具，并且根据工具的返回结果回答。",
		Model:       chatModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: []tool.BaseTool{
					queryOrderTool,
				},
			},
		},
		MaxIterations: 3, // 限制循环次数
	})

	if err != nil {
		log.Fatalf("new chat agent failed, err:%v", err)
	}

	// 创建运行器
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: true,
	})

	// 运行
	events := runner.Run(
		ctx,
		[]*schema.Message{
			schema.UserMessage("帮我查一下我刚才下单的东西到哪儿了？ 订单号：lwz-1234567"),
		},
	)

	// 遍历流式事件
	for {
		event, ok := events.Next()
		if !ok {
			break // 消费完了，退出
		}
		if event == nil {
			fmt.Println("event == nil")
			return
		}
		if event.Err != nil {
			fmt.Printf("event.Err:%v\n", event.Err)
			return
		}
		if event.Output != nil && event.Output.MessageOutput != nil {
			printMessage(event.Output.MessageOutput)
		}
	}
}

func printMessage(output *adk.MessageVariant) error {
	message, err := output.GetMessage()
	if err != nil {
		return fmt.Errorf("读取流消息失败：%w", err)
	}
	if message == nil {
		return nil
	}
	role := output.Role
	if role == "" {
		role = message.Role
	}
	switch role {
	case schema.Assistant:
		if message.Content != "" {
			fmt.Printf("[模型] %s\n", message.Content)
		}
		for _, call := range message.ToolCalls {
			fmt.Printf("[模型调用工具] %s,参数:%s\n", call.Function.Name, call.Function.Arguments)
		}
	case schema.Tool:
		toolName := output.ToolName
		if toolName == "" {
			toolName = message.ToolName
		}
		fmt.Printf("[工具 %s] %s\n", toolName, message.Content)
	default:
		if message.Content != "" {
			fmt.Printf("[%s] %s\n", role, message.Content)
		}
	}
	return nil
}
