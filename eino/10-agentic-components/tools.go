package main

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type OrderArgs struct {
	OrderID string `json:"order_id" jsonschema:"required" jsonschema_description:"订单号，例如 ORD-1001"`
}

type Order struct {
	OrderID string `json:"order_id"`
	Info    string `json:"info"`
	Status  int    `json:"status"`
}

func QueryOrder(ctx context.Context, args OrderArgs) (Order, error) {
	order := Order{
		OrderID: args.OrderID,
		Info:    "北京市北三环精装大平层一套（2000平方）",
	}
	return order, nil
}

func createQueryOrderTool() (tool.InvokableTool, error) {
	return utils.InferTool(
		"query_order",
		"查询当前登录客户的订单，包含商品信息、订单状态、金额和运单号等。",
		QueryOrder,
	)
}
