package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/topisenpai/gobin/gobin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv/v1.18.0"
	"go.opentelemetry.io/otel/trace"
)

func newTracer(cfg gobin.OtelConfig) (trace.Tracer, error) {
	if cfg.Trace == nil {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(cfg.Trace.Endpoint),
	}
	if cfg.Trace.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	exp, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceNamespace(serviceNamespace),
			semconv.ServiceInstanceID(cfg.InstanceID),
			semconv.ServiceVersion(version),
		)),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return otel.Tracer(serviceName), nil
}

func newMeter(cfg gobin.OtelConfig) (metric.Meter, error) {
	if cfg.Metrics == nil {
		return nil, nil
	}
	exp, err := prometheus.New()
	if err != nil {
		return nil, err
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exp),
		sdkmetric.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceNamespace(serviceNamespace),
			semconv.ServiceInstanceID(cfg.InstanceID),
			semconv.ServiceVersion(version),
		)),
	)
	global.SetMeterProvider(mp)

	go func() {
		server := &http.Server{
			Addr:    cfg.Metrics.ListenAddr,
			Handler: promhttp.Handler(),
		}
		if listenErr := server.ListenAndServe(); listenErr != nil && listenErr != http.ErrServerClosed {
			log.Println("failed to listen metrics server:", listenErr)
		}
	}()

	return mp.Meter(serviceName), nil
}
