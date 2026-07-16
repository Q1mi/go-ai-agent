package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

// 查询订单的业务逻辑

type OrderArgs struct {
	OrderID string `json:"order_id" jsonschema:"required" jsonschema_description:"订单号，例如 ORD-100111"`
}

type Order struct {
	OrderID string `json:"order_id"`
	Info    string `json:"info"`
	Status  int    `json:"status"`
}

func QueryOrder(ctx context.Context, args OrderArgs) (*Order, error) {
	// mock 查询DB
	order := &Order{
		OrderID: args.OrderID,
		Info:    "北京市北三环精装大平层一套（2000平）",
	}
	return order, nil
}

// 基于已有的函数创建tool
func createTool1() tool.InvokableTool {
	queryOrderTool := utils.NewTool(&schema.ToolInfo{
		Name: "query_order",
		Desc: "查询当前登录客户的订单，包含商品信息、订单状态、金额、运单号等信息。",
		ParamsOneOf: schema.NewParamsOneOfByParams(
			map[string]*schema.ParameterInfo{
				"order_id": &schema.ParameterInfo{
					Type:     schema.String,
					Required: true,
				},
			},
		),
	}, QueryOrder)
	return queryOrderTool
}

func createTool2() tool.InvokableTool {
	queryOrderTool, err := utils.InferTool(
		"query_order",
		"查询当前登录客户的订单，包含商品信息、订单状态、金额、运单号等信息。",
		QueryOrder,
	)
	if err != nil {
		log.Fatalf("inferTool failed, err:%v", err)
	}
	return queryOrderTool
}
