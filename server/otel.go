package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv/v1.25.0"
)

func SetupOtel(version string, cfg OtelConfig) error {
	if err := setupTrace(version, cfg); err != nil {
		return fmt.Errorf("failed to setup tracing: %w", err)
	}

	if err := setupMeter(version, cfg); err != nil {
		return fmt.Errorf("failed to setup metrics: %w", err)
	}

	return nil
}

func resources(version string, cfg OtelConfig) *resource.Resource {
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(Name),
		semconv.ServiceNamespace(Namespace),
		semconv.ServiceInstanceID(cfg.InstanceID),
		semconv.ServiceVersion(version),
	)
}

func setupTrace(version string, cfg OtelConfig) error {
	if !cfg.Trace.Enabled {
		return nil
	}

	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(cfg.Trace.Endpoint),
	}
	if cfg.Trace.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	exp, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(resources(version, cfg)),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return nil
}

func setupMeter(version string, cfg OtelConfig) error {
	if !cfg.Metrics.Enabled {
		return nil
	}

	exp, err := prometheus.New()
	if err != nil {
		return err
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exp),
		sdkmetric.WithResource(resources(version, cfg)),
	)
	otel.SetMeterProvider(mp)

	httpServer := &http.Server{
		Addr:    cfg.Metrics.ListenAddr,
		Handler: promhttp.Handler(),
	}

	go func() {
		if listenErr := httpServer.ListenAndServe(); listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			slog.Error("failed to listen metrics server", slog.Any("err", listenErr))
		}
	}()

	return nil
}
