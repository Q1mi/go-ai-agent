package obs

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// SpanRecord 是可序列化的 span 快照。
type SpanRecord struct {
	TraceID      string            `json:"trace_id"`
	SpanID       string            `json:"span_id"`
	ParentSpanID string            `json:"parent_span_id,omitempty"`
	Name         string            `json:"name"`
	Start        time.Time         `json:"start"`
	End          time.Time         `json:"end"`
	DurationMS   int64             `json:"duration_ms"`
	Attributes   map[string]string `json:"attributes,omitempty"`
	Events       []EventRecord     `json:"events,omitempty"`
	Status       string            `json:"status,omitempty"`
}

// EventRecord 是可序列化的 span event。
type EventRecord struct {
	Name       string            `json:"name"`
	Time       time.Time         `json:"time"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// MemoryExporter 实现 sdktrace.SpanExporter，用于课堂本地查看 trace。
type MemoryExporter struct {
	mu    sync.Mutex
	spans []SpanRecord
}

// NewMemoryExporter 创建内存 exporter。
func NewMemoryExporter() *MemoryExporter {
	return &MemoryExporter{}
}

// ExportSpans 收集 span 快照。
func (exporter *MemoryExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	exporter.mu.Lock()
	defer exporter.mu.Unlock()
	for _, span := range spans {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		exporter.spans = append(exporter.spans, snapshot(span))
	}
	return nil
}

// Shutdown 关闭 exporter。
func (exporter *MemoryExporter) Shutdown(ctx context.Context) error {
	return ctx.Err()
}

// Spans 返回所有 span。
func (exporter *MemoryExporter) Spans() []SpanRecord {
	exporter.mu.Lock()
	defer exporter.mu.Unlock()
	out := make([]SpanRecord, len(exporter.spans))
	copy(out, exporter.spans)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out
}

// Reset 清空记录。
func (exporter *MemoryExporter) Reset() {
	exporter.mu.Lock()
	defer exporter.mu.Unlock()
	exporter.spans = nil
}

// TraceIDs 返回已收集的 trace id。
func (exporter *MemoryExporter) TraceIDs() []string {
	seen := map[string]bool{}
	var ids []string
	for _, span := range exporter.Spans() {
		if !seen[span.TraceID] {
			seen[span.TraceID] = true
			ids = append(ids, span.TraceID)
		}
	}
	return ids
}

// Tree 返回指定 trace 的树形文本。
func (exporter *MemoryExporter) Tree(traceID string) string {
	spans := exporter.Spans()
	if traceID == "" {
		for _, span := range spans {
			traceID = span.TraceID
			break
		}
	}
	var filtered []SpanRecord
	for _, span := range spans {
		if span.TraceID == traceID {
			filtered = append(filtered, span)
		}
	}
	if len(filtered) == 0 {
		return ""
	}
	children := map[string][]SpanRecord{}
	var roots []SpanRecord
	for _, span := range filtered {
		if span.ParentSpanID == "" {
			roots = append(roots, span)
			continue
		}
		children[span.ParentSpanID] = append(children[span.ParentSpanID], span)
	}
	sortByStart(roots)
	for parent := range children {
		sortByStart(children[parent])
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "trace %s\n", traceID)
	for i, root := range roots {
		renderSpan(&sb, root, children, "", i == len(roots)-1)
	}
	return strings.TrimRight(sb.String(), "\n")
}

// JSON 返回全部 span 的 JSON。
func (exporter *MemoryExporter) JSON() ([]byte, error) {
	return json.MarshalIndent(exporter.Spans(), "", "  ")
}

func snapshot(span sdktrace.ReadOnlySpan) SpanRecord {
	record := SpanRecord{
		TraceID:      span.SpanContext().TraceID().String(),
		SpanID:       span.SpanContext().SpanID().String(),
		ParentSpanID: parentID(span.Parent()),
		Name:         span.Name(),
		Start:        span.StartTime(),
		End:          span.EndTime(),
		DurationMS:   span.EndTime().Sub(span.StartTime()).Milliseconds(),
		Attributes:   attrs(span.Attributes()),
		Status:       span.Status().Code.String(),
	}
	for _, event := range span.Events() {
		record.Events = append(record.Events, EventRecord{
			Name:       event.Name,
			Time:       event.Time,
			Attributes: attrs(event.Attributes),
		})
	}
	return record
}

func parentID(sc trace.SpanContext) string {
	if !sc.IsValid() {
		return ""
	}
	return sc.SpanID().String()
}

func attrs(kvs []attribute.KeyValue) map[string]string {
	if len(kvs) == 0 {
		return nil
	}
	out := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		out[string(kv.Key)] = kv.Value.AsString()
		if out[string(kv.Key)] == "" {
			out[string(kv.Key)] = kv.Value.Emit()
		}
	}
	return out
}

func sortByStart(spans []SpanRecord) {
	sort.SliceStable(spans, func(i, j int) bool { return spans[i].Start.Before(spans[j].Start) })
}

func renderSpan(sb *strings.Builder, span SpanRecord, children map[string][]SpanRecord, prefix string, last bool) {
	branch := "├─ "
	nextPrefix := prefix + "│  "
	if last {
		branch = "└─ "
		nextPrefix = prefix + "   "
	}
	fmt.Fprintf(sb, "%s%s%s (%dms)", prefix, branch, span.Name, span.DurationMS)
	if op := span.Attributes["gen_ai.operation.name"]; op != "" {
		fmt.Fprintf(sb, " %s", op)
	}
	if model := span.Attributes["gen_ai.request.model"]; model != "" {
		fmt.Fprintf(sb, " model=%s", model)
	}
	if tool := span.Attributes["gen_ai.tool.name"]; tool != "" {
		fmt.Fprintf(sb, " tool=%s", tool)
	}
	if in := span.Attributes["gen_ai.usage.input_tokens"]; in != "" {
		fmt.Fprintf(sb, " in=%s", in)
	}
	if out := span.Attributes["gen_ai.usage.output_tokens"]; out != "" {
		fmt.Fprintf(sb, " out=%s", out)
	}
	if span.Status != "" && span.Status != "Unset" {
		fmt.Fprintf(sb, " status=%s", span.Status)
	}
	sb.WriteByte('\n')
	for _, event := range span.Events {
		fmt.Fprintf(sb, "%s   • event %s", nextPrefix, event.Name)
		if text := event.Attributes["text"]; text != "" {
			fmt.Fprintf(sb, " text=%q", shorten(text, 90))
		}
		sb.WriteByte('\n')
	}
	kids := children[span.SpanID]
	for i, child := range kids {
		renderSpan(sb, child, children, nextPrefix, i == len(kids)-1)
	}
}

func shorten(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}
