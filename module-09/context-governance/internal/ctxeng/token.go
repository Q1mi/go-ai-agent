package ctxeng

import "strings"

// EstimateTokens 粗略估算 token：英文约 4 字符/token，中文约 1.5 字符/token。
func EstimateTokens(s string) int {
	if strings.TrimSpace(s) == "" {
		return 0
	}
	ascii, cjk := 0, 0
	for _, r := range s {
		if r < 128 {
			ascii++
			continue
		}
		cjk++
	}
	return ascii/4 + cjk*2/3 + 1
}
