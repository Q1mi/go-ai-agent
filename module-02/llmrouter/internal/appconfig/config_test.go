package appconfig

import "testing"

func clearProviderEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"DEEPSEEK_API_KEY", "DEEPSEEK_MODEL", "DEEPSEEK_BASE_URL",
		"DEEPSEEK_INPUT_PER_1M_USD", "DEEPSEEK_OUTPUT_PER_1M_USD",
		"DEEPSEEK_LATENCY_HINT_MS",
		"DOUBAO_API_KEY", "DOUBAO_MODEL", "DOUBAO_BASE_URL",
		"CLAUDE_API_KEY", "CLAUDE_MODEL", "CLAUDE_BASE_URL",
		"GEMINI_API_KEY", "GEMINI_MODEL", "GEMINI_BASE_URL",
		"GEMINI_INPUT_PER_1M_USD", "GEMINI_OUTPUT_PER_1M_USD",
		"GEMINI_LATENCY_HINT_MS",
		"OLLAMA_API_KEY", "OLLAMA_MODEL", "OLLAMA_BASE_URL",
		"LLM_PROVIDER_ORDER", "LLM_STRATEGY", "LLM_STREAM",
	} {
		t.Setenv(name, "")
	}
}

func TestLoadKeepsConfiguredOrder(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("DEEPSEEK_API_KEY", "key")
	t.Setenv("DEEPSEEK_MODEL", "deepseek-model")
	t.Setenv("OLLAMA_MODEL", "local-model")
	t.Setenv("LLM_PROVIDER_ORDER", "ollama,deepseek")

	config, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Order) != 2 || config.Order[0] != "ollama" || config.Order[1] != "deepseek" {
		t.Fatalf("Order = %#v", config.Order)
	}
}

func TestLoadRequiresModelWithKey(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("DEEPSEEK_API_KEY", "key")
	if _, err := Load(); err == nil {
		t.Fatal("期望缺少模型错误")
	}
}

func TestLoadRejectsUnknownStrategy(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("OLLAMA_MODEL", "local-model")
	t.Setenv("LLM_STRATEGY", "random")
	if _, err := Load(); err == nil {
		t.Fatal("期望 strategy 错误")
	}
}

func TestLoadIncludesGemini(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("GEMINI_API_KEY", "key")
	t.Setenv("GEMINI_MODEL", "gemini-test")

	config, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Order) != 1 || config.Order[0] != "gemini" {
		t.Fatalf("Order = %#v", config.Order)
	}
	if got := config.Providers["gemini"].BaseURL; got != "https://generativelanguage.googleapis.com/v1beta" {
		t.Fatalf("Gemini BaseURL = %q", got)
	}
}
