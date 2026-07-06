package mas

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/q1mi/debate/internal/llm"
)

// Debater 是参与辩论的 Agent。
type Debater struct {
	Name     string
	Provider llm.Provider
	Model    string
	Persona  string
}

// Answer 记录某一轮某位辩手的观点。
type Answer struct {
	Name    string
	Content string
}

// Round 记录一轮辩论的全部观点。
type Round struct {
	Number  int
	Answers []Answer
}

// Transcript 是完整辩论记录。
type Transcript struct {
	Question     string
	Rounds       []Round
	FinalAnswers map[string]string
}

// Debate 进行 rounds 轮辩论，返回每位辩手的最终答案。
func Debate(ctx context.Context, debaters []Debater, question string, rounds int) (map[string]string, error) {
	transcript, err := DebateWithTranscript(ctx, debaters, question, rounds)
	if err != nil {
		return nil, err
	}
	return transcript.FinalAnswers, nil
}

// DebateWithTranscript 进行多轮辩论，并保留每轮观点变化。
func DebateWithTranscript(ctx context.Context, debaters []Debater, question string, rounds int) (Transcript, error) {
	if err := validateDebateInput(debaters, question, rounds); err != nil {
		return Transcript{}, err
	}
	answers := make(map[string]string)
	transcript := Transcript{
		Question:     strings.TrimSpace(question),
		FinalAnswers: make(map[string]string),
	}

	for round := 1; round <= rounds; round++ {
		prev := cloneAnswers(answers)
		current, err := runRound(ctx, debaters, question, round, prev)
		if err != nil {
			return Transcript{}, err
		}
		for name, content := range current {
			answers[name] = content
		}
		transcript.Rounds = append(transcript.Rounds, Round{
			Number:  round,
			Answers: orderedAnswers(current),
		})
	}

	for name, content := range answers {
		transcript.FinalAnswers[name] = content
	}
	return transcript, nil
}

func validateDebateInput(debaters []Debater, question string, rounds int) error {
	if strings.TrimSpace(question) == "" {
		return fmt.Errorf("question 不能为空")
	}
	if rounds <= 0 {
		return fmt.Errorf("rounds 必须大于 0")
	}
	if len(debaters) == 0 {
		return fmt.Errorf("debaters 不能为空")
	}
	seen := map[string]bool{}
	for i, debater := range debaters {
		if strings.TrimSpace(debater.Name) == "" {
			return fmt.Errorf("debaters[%d].Name 不能为空", i)
		}
		if seen[debater.Name] {
			return fmt.Errorf("重复辩手名称: %s", debater.Name)
		}
		seen[debater.Name] = true
		if debater.Provider == nil {
			return fmt.Errorf("debaters[%d].Provider 不能为空", i)
		}
		if strings.TrimSpace(debater.Persona) == "" {
			return fmt.Errorf("debaters[%d].Persona 不能为空", i)
		}
	}
	return nil
}

func runRound(ctx context.Context, debaters []Debater, question string, round int, prev map[string]string) (map[string]string, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	current := make(map[string]string, len(debaters))
	errCh := make(chan error, len(debaters))

	for _, debater := range debaters {
		debater := debater
		wg.Add(1)
		go func() {
			defer wg.Done()
			system := debater.Persona + "\n你正在参加多智能体辩论。请给出清晰结论、主要理由和需要保留的风险。"
			answer, err := chat(ctx, debater.Provider, debater.Model, system, debatePrompt(question, debater.Name, round, prev))
			if err != nil {
				errCh <- fmt.Errorf("%s 第 %d 轮失败: %w", debater.Name, round, err)
				return
			}
			mu.Lock()
			current[debater.Name] = answer
			mu.Unlock()
		}()
	}
	wg.Wait()
	close(errCh)
	if err := firstErr(errCh); err != nil {
		return nil, err
	}
	return current, nil
}

func debatePrompt(question, self string, round int, prev map[string]string) string {
	if len(prev) == 0 {
		return fmt.Sprintf("问题：%s\n\n这是第 %d 轮。请先独立给出你的回答和理由。", strings.TrimSpace(question), round)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "问题：%s\n\n这是第 %d 轮。\n\n其他成员上一轮的观点：\n", strings.TrimSpace(question), round)
	names := sortedKeys(prev)
	for _, name := range names {
		if name == self {
			continue
		}
		fmt.Fprintf(&sb, "- %s：%s\n", name, prev[name])
	}
	sb.WriteString("\n请批判性地参考其他观点，修订并强化你的回答。指出你采纳了什么、保留了什么分歧。")
	return sb.String()
}

// Judge 综合各辩手的最终答案，给出定稿。
func Judge(ctx context.Context, provider llm.Provider, model, question string, answers map[string]string) (string, error) {
	if provider == nil {
		return "", fmt.Errorf("provider 不能为空")
	}
	if strings.TrimSpace(question) == "" {
		return "", fmt.Errorf("question 不能为空")
	}
	if len(answers) == 0 {
		return "", fmt.Errorf("answers 不能为空")
	}
	var sb strings.Builder
	for _, name := range sortedKeys(answers) {
		fmt.Fprintf(&sb, "【%s】\n%s\n\n", name, answers[name])
	}
	return chat(ctx, provider, model,
		"你是评审。综合各位专家的最终回答，给出准确、全面、平衡的定稿。请说明关键结论、主要依据、仍需验证的信息。",
		"问题："+strings.TrimSpace(question)+"\n\n各方回答：\n"+sb.String())
}

// Baseline 让单个 Agent 直接回答同一个问题，作为成本和质量对照。
func Baseline(ctx context.Context, provider llm.Provider, model, question string) (string, error) {
	if provider == nil {
		return "", fmt.Errorf("provider 不能为空")
	}
	return chat(ctx, provider, model,
		"你是资深架构顾问。请直接回答用户问题，给出结构化建议和风险提示。",
		"问题："+strings.TrimSpace(question))
}

func cloneAnswers(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func orderedAnswers(in map[string]string) []Answer {
	names := sortedKeys(in)
	out := make([]Answer, 0, len(names))
	for _, name := range names {
		out = append(out, Answer{Name: name, Content: in[name]})
	}
	return out
}

func sortedKeys(in map[string]string) []string {
	names := make([]string, 0, len(in))
	for name := range in {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func firstErr(errCh <-chan error) error {
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}
