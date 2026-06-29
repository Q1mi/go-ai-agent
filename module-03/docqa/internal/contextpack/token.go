package contextpack

// EstimateTokens 用一个简单启发式方法估算文本 token 数。
//
// 课程在 M03 关注预算意识和裁剪策略，精确 tokenizer 会在后续模型计量和
// RAG 章节中继续扩展。
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	ascii, nonASCII := 0, 0
	for _, r := range text {
		if r < 128 {
			ascii++
		} else {
			nonASCII++
		}
	}
	return ascii/4 + nonASCII*2/3 + 1
}
