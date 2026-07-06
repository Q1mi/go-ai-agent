package config

import (
	"os"
	"strconv"
	"strings"
)

// Config 是 mini-kb 运行配置。
type Config struct {
	DSN       string
	Embedder  string
	EmbedDim  int
	ChunkSize int
	Overlap   int
}

// Load 从环境变量读取配置。
func Load() Config {
	return Config{
		DSN:       envOrDefault("MINIKB_DSN", "postgres://postgres:postgres@localhost:5432/minikb?sslmode=disable"),
		Embedder:  envOrDefault("MINIKB_EMBEDDER", "local"),
		EmbedDim:  envInt("MINIKB_EMBED_DIM", 384),
		ChunkSize: envInt("MINIKB_CHUNK_SIZE", 400),
		Overlap:   envInt("MINIKB_CHUNK_OVERLAP", 50),
	}
}

// UseLocalEmbedder 判断是否使用本地 embedding。
func (config Config) UseLocalEmbedder() bool {
	return strings.EqualFold(strings.TrimSpace(config.Embedder), "local")
}

func envOrDefault(name string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func envInt(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
