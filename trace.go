package main

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv/v1.18.0"
)

func newExporter(ctx context.Context) (*otlptrace.Exporter, error) {
	return otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint("192.168.178.73:4318"),
		otlptracehttp.WithInsecure(),
	)
}

func newTraceProvider(exp sdktrace.SpanExporter) *sdktrace.TracerProvider {
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("gobin-server"),
			semconv.ServiceNamespace("github.com/topisenpai/gobin"),
			semconv.ServiceInstanceID("gobin-server"),
			semconv.ServiceVersion("v0.0.1"),
		)),
	)
	otel.SetTracerProvider(provider)
	return provider
}
