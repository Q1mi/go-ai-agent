package obs

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "traceagent"

// Tracing 封装本练习的 OTel TracerProvider 与内存 exporter。
type Tracing struct {
	Provider     *sdktrace.TracerProvider
	Exporter     *MemoryExporter
	OTLPEndpoint string
}

// Config 描述 trace 导出配置。
type Config struct {
	ServiceName  string
	OTLPEndpoint string
}

// SetupMemoryTracing 初始化 OTel SDK，并使用内存 exporter 收集 span。
func SetupMemoryTracing(serviceName string) (*Tracing, error) {
	return SetupTracing(context.Background(), Config{ServiceName: serviceName})
}

// SetupTracing 初始化 OTel SDK。内存 exporter 始终启用；OTLPEndpoint 非空时额外导出到 Jaeger/Phoenix 等 OTLP 后端。
func SetupTracing(ctx context.Context, cfg Config) (*Tracing, error) {
	if cfg.ServiceName == "" {
		cfg.ServiceName = "traceagent"
	}
	exporter := NewMemoryExporter()
	res := resource.NewSchemaless(attribute.String("service.name", cfg.ServiceName))
	options := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(exporter)),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	}
	if cfg.OTLPEndpoint != "" {
		otlpExporter, err := otlptracehttp.New(ctx,
			otlptracehttp.WithEndpoint(cfg.OTLPEndpoint),
			otlptracehttp.WithInsecure(),
		)
		if err != nil {
			return nil, err
		}
		options = append(options, sdktrace.WithBatcher(otlpExporter))
	}
	tp := sdktrace.NewTracerProvider(options...)
	otel.SetTracerProvider(tp)
	return &Tracing{Provider: tp, Exporter: exporter, OTLPEndpoint: cfg.OTLPEndpoint}, nil
}

// Shutdown flush 并关闭 TracerProvider。
func (tracing *Tracing) Shutdown(ctx context.Context) error {
	if tracing == nil || tracing.Provider == nil {
		return nil
	}
	return tracing.Provider.Shutdown(ctx)
}

func tracer() trace.Tracer {
	return otel.Tracer(tracerName)
}

// StartAgentSpan 为一次 Agent 运行开一个根 span。
func StartAgentSpan(ctx context.Context, agentName, conversationID string) (context.Context, trace.Span) {
	ctx, span := tracer().Start(ctx, "invoke_agent "+agentName)
	span.SetAttributes(
		attribute.String("gen_ai.operation.name", "invoke_agent"),
		attribute.String("gen_ai.agent.name", agentName),
		attribute.String("gen_ai.conversation.id", conversationID),
	)
	return ctx, span
}

// RecordModelCall 为一次模型调用建立 chat span。
func RecordModelCall(ctx context.Context, provider, model string, fn func(context.Context) (inputTokens, outputTokens int, responseID string, err error)) error {
	ctx, span := tracer().Start(ctx, "chat "+model)
	defer span.End()
	span.SetAttributes(
		attribute.String("gen_ai.operation.name", "chat"),
		attribute.String("gen_ai.provider.name", provider),
		attribute.String("gen_ai.request.model", model),
	)
	inTok, outTok, responseID, err := fn(ctx)
	span.SetAttributes(
		attribute.Int("gen_ai.usage.input_tokens", inTok),
		attribute.Int("gen_ai.usage.output_tokens", outTok),
		attribute.String("gen_ai.response.id", responseID),
		attribute.String("gen_ai.response.model", model),
	)
	if err != nil {
		RecordError(span, err)
		return err
	}
	SetOK(span)
	return nil
}

// RecordToolCall 为一次工具调用建立 execute_tool span。
func RecordToolCall(ctx context.Context, toolName, callID string, fn func(context.Context) error) error {
	ctx, span := tracer().Start(ctx, "execute_tool "+toolName)
	defer span.End()
	span.SetAttributes(
		attribute.String("gen_ai.operation.name", "execute_tool"),
		attribute.String("gen_ai.tool.name", toolName),
		attribute.String("gen_ai.tool.call.id", callID),
		attribute.String("gen_ai.tool.type", "function"),
	)
	if err := fn(ctx); err != nil {
		RecordError(span, err)
		return err
	}
	SetOK(span)
	return nil
}

// AddEvent 在当前 span 上记录事件。
func AddEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span == nil {
		return
	}
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// RecordError 标记 span 错误。
func RecordError(span trace.Span, err error) {
	if span == nil || err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// SetOK 标记 span 成功。
func SetOK(span trace.Span) {
	if span == nil {
		return
	}
	span.SetStatus(codes.Ok, "ok")
}

// TraceURL 给本地演示生成一个可读 trace 标识。
func TraceURL(traceID string) string {
	return fmt.Sprintf("local://trace/%s", traceID)
}
