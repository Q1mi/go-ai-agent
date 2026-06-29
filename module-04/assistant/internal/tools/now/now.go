package now

import (
	"context"
	"encoding/json"
	"time"

	"github.com/q1mi/assistant/internal/schema"
)

// Args 是 now 工具的 JSON 参数。
type Args struct{}

// Tool 对应 M04 配套练习的 now 工具。
// 它没有参数，用于演示空对象 Schema 和工具观察结果回填。
type Tool struct {
	location   *time.Location
	clock      func() time.Time
	parameters json.RawMessage
}

// New 创建 now 工具。
//
// location 和 clock 可注入，方便测试固定时间和时区。
func New(location *time.Location, clock func() time.Time) (*Tool, error) {
	if location == nil {
		location = time.Local
	}
	if clock == nil {
		clock = time.Now
	}
	parameters, err := schema.Generate(Args{})
	if err != nil {
		return nil, err
	}
	return &Tool{location: location, clock: clock, parameters: schema.MustJSON(parameters)}, nil
}

// MustNew 创建 now 工具，失败时 panic。
func MustNew() *Tool {
	tool, err := New(time.Local, time.Now)
	if err != nil {
		panic(err)
	}
	return tool
}

// Name 返回工具名。
func (tool *Tool) Name() string {
	return "now"
}

// Description 返回工具说明。
func (tool *Tool) Description() string {
	return "查询当前时间"
}

// Parameters 返回工具参数 Schema。
func (tool *Tool) Parameters() json.RawMessage {
	return tool.parameters
}

// Call 返回当前时间的 RFC3339 表示。
func (tool *Tool) Call(context.Context, json.RawMessage) (string, error) {
	return tool.clock().In(tool.location).Format(time.RFC3339), nil
}
