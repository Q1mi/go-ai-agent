package mas

import (
	"context"
	"fmt"
	"sync"
)

// Message 是 Agent 间传递的消息。
type Message struct {
	From    string         // 发送者 Agent 名
	To      string         // 收件人；空或 "*" 表示广播
	Content string         // 文本内容
	Meta    map[string]any // 可选结构化负载
}

// MessageBus 为每个 Agent 维护一个收件箱 channel。
type MessageBus struct {
	mu      sync.RWMutex
	inboxes map[string]chan Message
	buffer  int
	closed  bool
}

// NewMessageBus 创建消息总线。
func NewMessageBus(buffer int) *MessageBus {
	if buffer < 0 {
		buffer = 0
	}
	return &MessageBus{
		inboxes: make(map[string]chan Message),
		buffer:  buffer,
	}
}

// Subscribe 为某个 Agent 注册收件箱，返回只读 channel。
func (bus *MessageBus) Subscribe(name string) (<-chan Message, error) {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	if bus.closed {
		return nil, fmt.Errorf("message bus 已关闭")
	}
	if name == "" {
		return nil, fmt.Errorf("agent 名称不能为空")
	}
	if _, ok := bus.inboxes[name]; ok {
		return nil, fmt.Errorf("agent 已订阅: %s", name)
	}
	ch := make(chan Message, bus.buffer)
	bus.inboxes[name] = ch
	return ch, nil
}

// Publish 把消息路由给收件人；广播会发给除发送者外的所有订阅者。
func (bus *MessageBus) Publish(ctx context.Context, msg Message) error {
	targets, err := bus.targets(msg)
	if err != nil {
		return err
	}
	for _, ch := range targets {
		select {
		case ch <- msg:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (bus *MessageBus) targets(msg Message) ([]chan Message, error) {
	bus.mu.RLock()
	defer bus.mu.RUnlock()
	if bus.closed {
		return nil, fmt.Errorf("message bus 已关闭")
	}
	if msg.To != "" && msg.To != "*" {
		ch, ok := bus.inboxes[msg.To]
		if !ok {
			return nil, fmt.Errorf("收件人不存在: %s", msg.To)
		}
		return []chan Message{ch}, nil
	}
	targets := make([]chan Message, 0, len(bus.inboxes))
	for name, ch := range bus.inboxes {
		if name == msg.From {
			continue
		}
		targets = append(targets, ch)
	}
	return targets, nil
}

// Close 关闭所有收件箱。
func (bus *MessageBus) Close() {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	if bus.closed {
		return
	}
	for _, ch := range bus.inboxes {
		close(ch)
	}
	bus.inboxes = make(map[string]chan Message)
	bus.closed = true
}

// RunBusAgent 启动一个持续读取收件箱的 Agent goroutine。
func RunBusAgent(ctx context.Context, bus *MessageBus, name string, handle func(context.Context, Message) (*Message, error)) error {
	inbox, err := bus.Subscribe(name)
	if err != nil {
		return err
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-inbox:
				if !ok {
					return
				}
				reply, err := handle(ctx, msg)
				if err != nil || reply == nil {
					continue
				}
				if reply.From == "" {
					reply.From = name
				}
				_ = bus.Publish(ctx, *reply)
			}
		}
	}()
	return nil
}
