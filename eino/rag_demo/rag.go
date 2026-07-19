package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	defaultDocsDir          = "../rag/docs"
	defaultMilvusAddress    = "localhost:19530"
	defaultCollection       = "eino_rag_demo"
	defaultVectorField      = "vector"
	defaultEmbeddingModel   = "doubao-embedding-vision-251215"
	defaultEmbeddingDim     = 2048
	defaultEmbeddingBatch   = 16
	defaultChunkSize        = 1200
	defaultChunkOverlap     = 150
	defaultTopK             = 3
	defaultEmbeddingAPIType = "multimodal"
)

type appConfig struct {
	DocsDir            string
	MilvusAddress      string
	Collection         string
	VectorField        string
	EmbeddingAPIKey    string
	EmbeddingModel     string
	EmbeddingBaseURL   string
	EmbeddingAPIType   string
	EmbeddingDimension int
	EmbeddingBatchSize int
	ChunkSize          int
	ChunkOverlap       int
}

func loadConfig() (appConfig, error) {
	cfg := appConfig{
		DocsDir:            envOr("RAG_DOCS_DIR", defaultDocsDir),
		MilvusAddress:      envOr("MILVUS_ADDRESS", defaultMilvusAddress),
		Collection:         envOr("MILVUS_COLLECTION", defaultCollection),
		VectorField:        envOr("MILVUS_VECTOR_FIELD", defaultVectorField),
		EmbeddingAPIKey:    firstNonEmpty(os.Getenv("ARK_API_KEY"), os.Getenv("DOUBAO_API_KEY")),
		EmbeddingModel:     firstNonEmpty(os.Getenv("ARK_EMBEDDING_MODEL"), os.Getenv("DOUBAO_EMBEDDING_MODEL"), defaultEmbeddingModel),
		EmbeddingBaseURL:   firstNonEmpty(os.Getenv("ARK_BASE_URL"), os.Getenv("DOUBAO_BASE_URL")),
		EmbeddingAPIType:   strings.ToLower(envOr("ARK_EMBEDDING_API_TYPE", defaultEmbeddingAPIType)),
		EmbeddingDimension: defaultEmbeddingDim,
		EmbeddingBatchSize: defaultEmbeddingBatch,
		ChunkSize:          defaultChunkSize,
		ChunkOverlap:       defaultChunkOverlap,
	}

	var err error
	if cfg.EmbeddingDimension, err = positiveEnvInt("EMBEDDING_DIMENSION", defaultEmbeddingDim); err != nil {
		return appConfig{}, err
	}
	if cfg.EmbeddingBatchSize, err = positiveEnvInt("EMBEDDING_BATCH_SIZE", defaultEmbeddingBatch); err != nil {
		return appConfig{}, err
	}
	if cfg.ChunkSize, err = positiveEnvInt("RAG_CHUNK_SIZE", defaultChunkSize); err != nil {
		return appConfig{}, err
	}
	if cfg.ChunkOverlap, err = nonNegativeEnvInt("RAG_CHUNK_OVERLAP", defaultChunkOverlap); err != nil {
		return appConfig{}, err
	}
	if cfg.ChunkOverlap >= cfg.ChunkSize {
		return appConfig{}, fmt.Errorf("RAG_CHUNK_OVERLAP (%d) 必须小于 RAG_CHUNK_SIZE (%d)", cfg.ChunkOverlap, cfg.ChunkSize)
	}

	return cfg, nil
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func positiveEnvInt(key string, fallback int) (int, error) {
	value, err := nonNegativeEnvInt(key, fallback)
	if err != nil {
		return 0, err
	}
	if value == 0 {
		return 0, fmt.Errorf("环境变量 %s 必须大于 0", key)
	}
	return value, nil
}

func nonNegativeEnvInt(key string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("环境变量 %s 必须是非负整数，当前值为 %q", key, raw)
	}
	return value, nil
}
