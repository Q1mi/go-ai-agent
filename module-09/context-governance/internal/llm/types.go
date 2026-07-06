package llm

import "strings"

// Role 表示模型消息角色。
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message 是本练习使用的最小消息结构。
type Message struct {
	Role    Role   `json:"role"`
	Name    string `json:"name,omitempty"`
	Content string `json:"content"`
}

// JoinContent 把消息内容拼成可估算 token 的历史文本。
func JoinContent(messages []Message) string {
	var sb strings.Builder
	for _, msg := range messages {
		if msg.Name != "" {
			sb.WriteString("[")
			sb.WriteString(msg.Name)
			sb.WriteString("] ")
		}
		sb.WriteString(string(msg.Role))
		sb.WriteString(": ")
		sb.WriteString(msg.Content)
		sb.WriteByte('\n')
	}
	return sb.String()
}
