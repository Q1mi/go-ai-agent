package factory

import (
	"testing"

	"github.com/q1mi/llmrouter/internal/appconfig"
)

func TestBuildAllPreservesOrder(t *testing.T) {
	config := appconfig.Config{
		Providers: map[string]appconfig.ProviderConfig{
			"ollama": {
				Name:    "ollama",
				BaseURL: "http://localhost:11434/v1",
				Model:   "local-model",
			},
			"deepseek": {
				Name:    "deepseek",
				BaseURL: "https://api.deepseek.com",
				APIKey:  "key",
				Model:   "cloud-model",
			},
			"gemini": {
				Name:    "gemini",
				BaseURL: "https://generativelanguage.googleapis.com/v1beta",
				APIKey:  "key",
				Model:   "gemini-test",
			},
		},
		Order:    []string{"ollama", "gemini", "deepseek"},
		Strategy: "priority",
	}

	candidates, err := BuildAll(config)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 3 {
		t.Fatalf("len = %d", len(candidates))
	}
	if candidates[0].Provider.Name() != "ollama" ||
		candidates[1].Provider.Name() != "gemini" ||
		candidates[2].Provider.Name() != "deepseek" {
		t.Fatalf(
			"order = [%s, %s, %s]",
			candidates[0].Provider.Name(),
			candidates[1].Provider.Name(),
			candidates[2].Provider.Name(),
		)
	}
}
