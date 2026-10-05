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
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
)

func SetupOtel(version string, cfg OtelConfig) (func(context.Context) error, error) {
	noop := func(context.Context) error { return nil }
	if !cfg.Enabled {
		return noop, nil
	}

	res, err := resources(version, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	var (
		tp          *sdktrace.TracerProvider
		mp          *sdkmetric.MeterProvider
		metricServer *http.Server
	)

	if cfg.Trace.Enabled {
		tp, err = setupTrace(cfg, res)
		if err != nil {
			return nil, fmt.Errorf("failed to setup tracing: %w", err)
		}
	}

	if cfg.Metrics.Enabled {
		mp, metricServer, err = setupMeter(cfg, res)
		if err != nil {
			return nil, fmt.Errorf("failed to setup metrics: %w", err)
		}
	}

	return func(ctx context.Context) error {
		var errs []error
		if tp != nil {
			errs = append(errs, tp.Shutdown(ctx))
		}
		if mp != nil {
			errs = append(errs, mp.Shutdown(ctx))
		}
		if metricServer != nil {
			errs = append(errs, metricServer.Shutdown(ctx))
		}
		return errors.Join(errs...)
	}, nil
}

func resources(version string, cfg OtelConfig) (*resource.Resource, error) {
	return resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(Name),
			semconv.ServiceNamespace(Namespace),
			semconv.ServiceInstanceID(cfg.InstanceID),
			semconv.ServiceVersion(version),
		),
	)
}

func setupTrace(cfg OtelConfig, res *resource.Resource) (*sdktrace.TracerProvider, error) {
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
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp, nil
}

func setupMeter(cfg OtelConfig, res *resource.Resource) (*sdkmetric.MeterProvider, *http.Server, error) {
	exp, err := prometheus.New()
	if err != nil {
		return nil, nil, err
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exp),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	httpServer := &http.Server{
		Addr:              cfg.Metrics.ListenAddr,
		Handler:           promhttp.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if listenErr := httpServer.ListenAndServe(); listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			slog.Error("failed to listen metrics server", slog.Any("err", listenErr))
		}
	}()

	return mp, httpServer, nil
}
