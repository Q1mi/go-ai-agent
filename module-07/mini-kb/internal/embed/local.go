package embed

import (
	"context"
	"encoding/binary"
	"hash/fnv"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"
)

// HashEmbedder 是本地 deterministic embedder，便于无外部 API 时完成练习。
type HashEmbedder struct {
	dim int
}

// NewHashEmbedder 创建本地向量化器。
func NewHashEmbedder(dim int) *HashEmbedder {
	if dim <= 0 {
		dim = 384
	}
	return &HashEmbedder{dim: dim}
}

// Dim 返回向量维度。
func (embedder *HashEmbedder) Dim() int {
	return embedder.dim
}

// Embed 对文本做特征哈希并归一化。
func (embedder *HashEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, 0, len(texts))
	for _, text := range texts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		out = append(out, embedder.embedOne(text))
	}
	return out, nil
}

func (embedder *HashEmbedder) embedOne(text string) []float32 {
	vec := make([]float32, embedder.dim)
	for _, token := range tokens(text) {
		addFeature(vec, "tok:"+token, 1)
	}
	runes := []rune(strings.ToLower(text))
	for n := 2; n <= 3; n++ {
		for i := 0; i+n <= len(runes); i++ {
			gram := strings.TrimSpace(string(runes[i : i+n]))
			if gram != "" {
				addFeature(vec, "gram:"+gram, 0.35)
			}
		}
	}
	normalize(vec)
	return vec
}

func tokens(text string) []string {
	var out []string
	var b strings.Builder
	flush := func() {
		if b.Len() > 0 {
			out = append(out, b.String())
			b.Reset()
		}
	}
	for len(text) > 0 {
		r, size := utf8.DecodeRuneInString(text)
		text = text[size:]
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		flush()
		if unicode.Is(unicode.Han, r) {
			out = append(out, string(r))
		}
	}
	flush()
	return out
}

func addFeature(vec []float32, feature string, weight float32) {
	h := fnv.New64a()
	_, _ = h.Write([]byte(feature))
	sum := h.Sum64()
	idx := int(sum % uint64(len(vec)))
	signBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(signBytes, sum)
	if signBytes[0]&1 == 0 {
		vec[idx] += weight
		return
	}
	vec[idx] -= weight
}

func normalize(vec []float32) {
	var norm float64
	for _, value := range vec {
		norm += float64(value * value)
	}
	if norm == 0 {
		return
	}
	scale := float32(1 / math.Sqrt(norm))
	for i := range vec {
		vec[i] *= scale
	}
}
