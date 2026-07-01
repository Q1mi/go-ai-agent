package builtin

import (
	"context"
	"fmt"
	"time"

	"github.com/q1mi/mcptools/internal/tool"
)

// TimeArgs 是 get_time 工具参数。
type TimeArgs struct {
	Timezone string `json:"timezone,omitempty" desc:"IANA 时区名，例如 Asia/Shanghai；为空时使用本地时区"`
}

// NewTimeTool 创建返回当前时间的工具。
func NewTimeTool(clock func() time.Time) tool.Tool {
	if clock == nil {
		clock = time.Now
	}
	return tool.NewTypedTool("get_time", "返回当前时间，支持按时区格式化", func(_ context.Context, args TimeArgs) (string, error) {
		loc := time.Local
		if args.Timezone != "" {
			loaded, err := time.LoadLocation(args.Timezone)
			if err != nil {
				return "", fmt.Errorf("无效时区 %q: %w", args.Timezone, err)
			}
			loc = loaded
		}
		return clock().In(loc).Format(time.RFC3339), nil
	})
}
