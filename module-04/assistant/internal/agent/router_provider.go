package agent

import (
	"context"
	"fmt"

	"github.com/q1mi/assistant/internal/llm"
)

// Router 是 M02 多模型路由器在 M04 中需要满足的最小接口。
type Router interface {
	Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, string, error)
}

// RouterProvider 对应课件配套练习中的 M02 router 适配器。
// Agent 只依赖 llm.Provider；router 返回的 provider 名称等额外信息在这里被收拢。
type RouterProvider struct {
	Router Router
}

// Name 返回适配后的 Provider 名称。
func (provider RouterProvider) Name() string {
	return "router"
}

// Capabilities 声明 RouterProvider 支持工具调用。
func (provider RouterProvider) Capabilities() llm.Capability {
	return llm.Capability{Streaming: true, Tools: true}
}

// Chat 调用底层 Router，并丢弃 M02 router 返回的实际供应商名称。
func (provider RouterProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	resp, _, err := provider.Router.Chat(ctx, req)
	return resp, err
}

// ChatStream 保留 llm.Provider 接口中的流式入口。
func (provider RouterProvider) ChatStream(context.Context, llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	return nil, fmt.Errorf("router provider 示例暂未实现 ChatStream")
}
