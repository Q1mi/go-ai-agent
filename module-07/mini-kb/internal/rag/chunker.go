package rag

import "strings"

// RecursiveChunker 按自然边界递归切分文本，并为相邻块添加重叠内容。
type RecursiveChunker struct {
	ChunkSize  int
	Overlap    int
	Separators []string
}

// NewRecursiveChunker 创建适合中文知识库的递归切分器。
func NewRecursiveChunker(chunkSize, overlap int) *RecursiveChunker {
	if chunkSize <= 0 {
		chunkSize = 400
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= chunkSize {
		overlap = chunkSize / 4
	}
	return &RecursiveChunker{
		ChunkSize:  chunkSize,
		Overlap:    overlap,
		Separators: []string{"\n\n", "\n", "。", "！", "？", "; ", " "},
	}
}

// Split 把文本切成若干 chunk。
func (chunker *RecursiveChunker) Split(text string) []string {
	atoms := chunker.recurse(text, 0)
	return chunker.addOverlap(atoms)
}

func (chunker *RecursiveChunker) recurse(text string, sepIdx int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if runeLen(text) <= chunker.ChunkSize {
		return []string{text}
	}
	if sepIdx >= len(chunker.Separators) {
		return hardSplit([]rune(text), chunker.ChunkSize)
	}

	sep := chunker.Separators[sepIdx]
	parts := strings.Split(text, sep)
	var atoms []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if runeLen(part) > chunker.ChunkSize {
			atoms = append(atoms, chunker.recurse(part, sepIdx+1)...)
			continue
		}
		atoms = append(atoms, part)
	}
	return mergeAtoms(atoms, sep, chunker.ChunkSize)
}

func hardSplit(runes []rune, size int) []string {
	var out []string
	for i := 0; i < len(runes); i += size {
		end := i + size
		if end > len(runes) {
			end = len(runes)
		}
		text := strings.TrimSpace(string(runes[i:end]))
		if text != "" {
			out = append(out, text)
		}
	}
	return out
}

func mergeAtoms(atoms []string, sep string, chunkSize int) []string {
	var out []string
	var cur strings.Builder
	curLen := 0
	flush := func() {
		if curLen > 0 {
			out = append(out, strings.TrimSpace(cur.String()))
			cur.Reset()
			curLen = 0
		}
	}
	sepLen := runeLen(sep)
	for _, atom := range atoms {
		atom = strings.TrimSpace(atom)
		if atom == "" {
			continue
		}
		atomLen := runeLen(atom)
		addLen := atomLen
		if curLen > 0 {
			addLen += sepLen
		}
		if curLen > 0 && curLen+addLen > chunkSize {
			flush()
			addLen = atomLen
		}
		if curLen > 0 {
			cur.WriteString(sep)
		}
		cur.WriteString(atom)
		curLen += addLen
	}
	flush()
	return out
}

func (chunker *RecursiveChunker) addOverlap(chunks []string) []string {
	if chunker.Overlap <= 0 || len(chunks) <= 1 {
		return chunks
	}
	out := make([]string, len(chunks))
	out[0] = chunks[0]
	for i := 1; i < len(chunks); i++ {
		prev := []rune(chunks[i-1])
		tail := prev
		if len(prev) > chunker.Overlap {
			tail = prev[len(prev)-chunker.Overlap:]
		}
		out[i] = strings.TrimSpace(string(tail) + chunks[i])
	}
	return out
}

func runeLen(s string) int {
	return len([]rune(s))
}
