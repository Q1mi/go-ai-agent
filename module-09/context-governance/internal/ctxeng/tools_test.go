package ctxeng

import (
	"context"
	"testing"

	"github.com/q1mi/ctxagent/internal/tool"
)

func TestSelectTools(t *testing.T) {
	noOp := func(ctx context.Context, input string) (string, error) { return "", nil }
	all := []tool.Tool{
		tool.New("read_doc", "读取 文档 长文 方案 风险", noOp),
		tool.New("read_memory", "读取 外置 内容 全文", noOp),
		tool.New("calc_risk_score", "计算 风险 分数 指标", noOp),
		tool.New("create_ticket", "创建 工单 issue", noOp),
	}
	tests := []struct {
		name  string
		query string
		maxN  int
		want  []string
	}{
		{
			name:  "risk document query",
			query: "请读取文档并计算风险分数",
			maxN:  2,
			want:  []string{"read_doc", "calc_risk_score"},
		},
		{
			name:  "memory query",
			query: "读取外置内容全文",
			maxN:  1,
			want:  []string{"read_memory"},
		},
		{
			name:  "zero max",
			query: "读取文档",
			maxN:  0,
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SelectTools(tt.query, all, tt.maxN)
			if len(got) != len(tt.want) {
				t.Fatalf("len(SelectTools())=%d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i].Name() != tt.want[i] {
					t.Fatalf("tool[%d]=%s, want %s", i, got[i].Name(), tt.want[i])
				}
			}
		})
	}
}
