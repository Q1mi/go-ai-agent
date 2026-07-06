package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// EmbedderConfig 描述 OpenAI 兼容 embedding 配置。
type EmbedderConfig struct {
	BaseURL    string
	APIKey     string
	Model      string
	Dim        int
	HTTPClient *http.Client
}

// Embedder 调用 OpenAI 兼容 /embeddings。
type Embedder struct {
	baseURL    string
	apiKey     string
	model      string
	dim        int
	httpClient *http.Client
}

// NewEmbedderFromEnv 从 MINIKB_EMBED_* 环境变量创建 Embedder。
func NewEmbedderFromEnv() (*Embedder, error) {
	return NewEmbedder(EmbedderConfig{
		BaseURL: strings.TrimSpace(firstEnv("MINIKB_EMBED_BASE_URL", "LLM_BASE_URL")),
		APIKey:  strings.TrimSpace(firstEnv("MINIKB_EMBED_API_KEY", "LLM_API_KEY")),
		Model:   strings.TrimSpace(os.Getenv("MINIKB_EMBED_MODEL")),
		Dim:     envInt("MINIKB_EMBED_DIM", 1536),
	})
}

// NewEmbedder 创建 Embedder。
func NewEmbedder(config EmbedderConfig) (*Embedder, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" {
		return nil, errors.New("MINIKB_EMBED_BASE_URL 不能为空")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("无效 MINIKB_EMBED_BASE_URL: %q", baseURL)
	}
	apiKey := strings.TrimSpace(config.APIKey)
	if apiKey == "" && !allowEmptyAPIKey(parsed.Host) {
		return nil, errors.New("MINIKB_EMBED_API_KEY 不能为空")
	}
	model := strings.TrimSpace(config.Model)
	if model == "" {
		return nil, errors.New("MINIKB_EMBED_MODEL 不能为空")
	}
	dim := config.Dim
	if dim <= 0 {
		return nil, errors.New("MINIKB_EMBED_DIM 必须大于 0")
	}
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	return &Embedder{baseURL: baseURL, apiKey: apiKey, model: model, dim: dim, httpClient: client}, nil
}

// Dim 返回向量维度。
func (embedder *Embedder) Dim() int { return embedder.dim }

// Embed 批量向量化文本。
func (embedder *Embedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	body, err := json.Marshal(map[string]any{
		"model": embedder.model,
		"input": texts,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, embedder.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if embedder.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+embedder.apiKey)
	}
	resp, err := embedder.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, parseAPIError("embedding", resp)
	}
	var payload struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	out := make([][]float32, 0, len(payload.Data))
	for i, item := range payload.Data {
		if len(item.Embedding) != embedder.dim {
			return nil, fmt.Errorf("embedding[%d] 维度=%d，期望=%d", i, len(item.Embedding), embedder.dim)
		}
		out = append(out, item.Embedding)
	}
	return out, nil
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func envInt(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	var value int
	if _, err := fmt.Sscanf(raw, "%d", &value); err != nil || value <= 0 {
		return fallback
	}
	return value
}
