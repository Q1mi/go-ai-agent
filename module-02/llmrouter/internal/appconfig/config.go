package appconfig

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/q1mi/llmrouter/internal/cost"
)

type ProviderConfig struct {
	Name        string
	BaseURL     string
	APIKey      string
	Model       string
	Pricing     cost.Pricing
	LatencyHint time.Duration
}

type Config struct {
	Providers map[string]ProviderConfig
	Order     []string
	Strategy  string
	Stream    bool
}

func Load() (Config, error) {
	providers := make(map[string]ProviderConfig)

	deepSeek, err := loadProvider(
		"deepseek",
		"DEEPSEEK",
		"https://api.deepseek.com",
		false,
	)
	if err != nil {
		return Config{}, err
	}
	if deepSeek.Model != "" {
		providers[deepSeek.Name] = deepSeek
	}

	doubao, err := loadProvider(
		"doubao",
		"DOUBAO",
		"https://ark.cn-beijing.volces.com/api/v3",
		false,
	)
	if err != nil {
		return Config{}, err
	}
	if doubao.Model != "" {
		providers[doubao.Name] = doubao
	}

	claude, err := loadProvider(
		"claude",
		"CLAUDE",
		"https://api.anthropic.com/v1",
		false,
	)
	if err != nil {
		return Config{}, err
	}
	if claude.Model != "" {
		providers[claude.Name] = claude
	}

	gemini, err := loadProvider(
		"gemini",
		"GEMINI",
		"https://generativelanguage.googleapis.com/v1beta",
		false,
	)
	if err != nil {
		return Config{}, err
	}
	if gemini.Model != "" {
		providers[gemini.Name] = gemini
	}

	ollama, err := loadProvider(
		"ollama",
		"OLLAMA",
		"http://localhost:11434/v1",
		true,
	)
	if err != nil {
		return Config{}, err
	}
	if ollama.Model != "" {
		providers[ollama.Name] = ollama
	}

	order, err := parseOrder(os.Getenv("LLM_PROVIDER_ORDER"), providers)
	if err != nil {
		return Config{}, err
	}
	strategy := strings.ToLower(strings.TrimSpace(os.Getenv("LLM_STRATEGY")))
	if strategy == "" {
		strategy = "priority"
	}
	if strategy != "priority" && strategy != "cheapest" && strategy != "latency" {
		return Config{}, fmt.Errorf("LLM_STRATEGY 必须是 priority、cheapest 或 latency")
	}
	stream, err := parseBoolEnv("LLM_STREAM", false)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Providers: providers,
		Order:     order,
		Strategy:  strategy,
		Stream:    stream,
	}, nil
}

func loadProvider(
	name string,
	prefix string,
	defaultBaseURL string,
	apiKeyOptional bool,
) (ProviderConfig, error) {
	baseURL := strings.TrimSpace(os.Getenv(prefix + "_BASE_URL"))
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	apiKey := strings.TrimSpace(os.Getenv(prefix + "_API_KEY"))
	model := strings.TrimSpace(os.Getenv(prefix + "_MODEL"))

	if model == "" && apiKey == "" {
		return ProviderConfig{Name: name}, nil
	}
	if model == "" {
		return ProviderConfig{}, fmt.Errorf("%s_API_KEY 已配置，但 %s_MODEL 为空", prefix, prefix)
	}
	if !apiKeyOptional && apiKey == "" {
		return ProviderConfig{}, fmt.Errorf("%s_MODEL 已配置，但 %s_API_KEY 为空", prefix, prefix)
	}

	inputPrice, err := parseFloatEnv(prefix + "_INPUT_PER_1M_USD")
	if err != nil {
		return ProviderConfig{}, err
	}
	outputPrice, err := parseFloatEnv(prefix + "_OUTPUT_PER_1M_USD")
	if err != nil {
		return ProviderConfig{}, err
	}
	latencyHint, err := parseMillisecondsEnv(prefix + "_LATENCY_HINT_MS")
	if err != nil {
		return ProviderConfig{}, err
	}

	return ProviderConfig{
		Name:        name,
		BaseURL:     strings.TrimRight(baseURL, "/"),
		APIKey:      apiKey,
		Model:       model,
		LatencyHint: latencyHint,
		Pricing: cost.Pricing{
			InputPer1M:  inputPrice,
			OutputPer1M: outputPrice,
			Currency:    "USD",
		},
	}, nil
}

func parseOrder(value string, providers map[string]ProviderConfig) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		value = "deepseek,doubao,claude,gemini,ollama"
	}

	known := map[string]bool{
		"deepseek": true,
		"doubao":   true,
		"claude":   true,
		"gemini":   true,
		"ollama":   true,
	}
	seen := make(map[string]bool)
	var order []string
	for _, raw := range strings.Split(value, ",") {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" {
			continue
		}
		if !known[name] {
			return nil, fmt.Errorf("LLM_PROVIDER_ORDER 包含未知 Provider %q", name)
		}
		if seen[name] {
			return nil, fmt.Errorf("LLM_PROVIDER_ORDER 重复配置 %q", name)
		}
		seen[name] = true
		if _, enabled := providers[name]; enabled {
			order = append(order, name)
		}
	}
	for _, name := range []string{"deepseek", "doubao", "claude", "gemini", "ollama"} {
		if _, enabled := providers[name]; enabled && !seen[name] {
			order = append(order, name)
		}
	}
	if len(order) == 0 {
		return nil, fmt.Errorf("没有启用任何 Provider，请配置 API key/model 或 OLLAMA_MODEL")
	}
	return order, nil
}

func parseFloatEnv(name string) (float64, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s 必须是非负数字", name)
	}
	return parsed, nil
}

func parseMillisecondsEnv(name string) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s 必须是非负整数", name)
	}
	return time.Duration(parsed) * time.Millisecond, nil
}

func parseBoolEnv(name string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s 必须是布尔值", name)
	}
	return parsed, nil
}
