package gobin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptrace"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/go-chi/httprate"
	"github.com/go-jose/go-jose/v3"
	"github.com/topi314/gobin/internal/httperr"
	"github.com/topi314/tint"
	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/topi314/gobin/gobin/database"
	"github.com/topi314/gobin/templates"
)

func NewServer(version string, debug bool, cfg Config, db *database.DB, signer jose.Signer, tracer trace.Tracer, meter metric.Meter, assets http.FileSystem, htmlFormatter *html.Formatter) *Server {
	var allStyles []templates.Style
	for _, name := range styles.Names() {
		allStyles = append(allStyles, templates.Style{
			Name:  name,
			Theme: styles.Get(name).Theme,
		})
	}

	var client *http.Client
	if cfg.Webhook != nil {
		client = &http.Client{
			Transport: otelhttp.NewTransport(
				http.DefaultTransport,
				otelhttp.WithClientTrace(func(ctx context.Context) *httptrace.ClientTrace {
					return otelhttptrace.NewClientTrace(ctx)
				}),
			),
			Timeout: cfg.Webhook.Timeout,
		}
	}

	s := &Server{
		version:       version,
		debug:         debug,
		cfg:           cfg,
		db:            db,
		client:        client,
		signer:        signer,
		tracer:        tracer,
		meter:         meter,
		assets:        assets,
		styles:        allStyles,
		htmlFormatter: htmlFormatter,
	}

	s.server = &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: s.Routes(),
	}

	if cfg.RateLimit != nil && cfg.RateLimit.Requests > 0 && cfg.RateLimit.Duration > 0 {
		s.rateLimitHandler = httprate.NewRateLimiter(
			cfg.RateLimit.Requests,
			cfg.RateLimit.Duration,
			httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
				s.error(w, r, httperr.TooManyRequests(ErrRateLimit))
			}),
			httprate.WithKeyFuncs(
				httprate.KeyByIP,
				httprate.KeyByEndpoint,
			),
		).Handler
	}

	return s
}

type Server struct {
	version          string
	debug            bool
	cfg              Config
	db               *database.DB
	server           *http.Server
	client           *http.Client
	signer           jose.Signer
	tracer           trace.Tracer
	meter            metric.Meter
	assets           http.FileSystem
	htmlFormatter    *html.Formatter
	styles           []templates.Style
	rateLimitHandler func(http.Handler) http.Handler
	webhookWaitGroup sync.WaitGroup
}

func (s *Server) Start() {
	if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("Error while listening", tint.Err(err))
		os.Exit(1)
	}
}

func (s *Server) Close() {
	if err := s.server.Close(); err != nil {
		slog.Error("Error while closing server", tint.Err(err))
	}

	s.webhookWaitGroup.Wait()

	if err := s.db.Close(); err != nil {
		slog.Error("Error while closing database", tint.Err(err))
	}
}

func FormatBuildVersion(version string, commit string, buildTime time.Time) string {
	if len(commit) > 7 {
		commit = commit[:7]
	}

	buildTimeStr := "unknown"
	if !buildTime.IsZero() {
		buildTimeStr = buildTime.Format(time.ANSIC)
	}
	return fmt.Sprintf("Go Version: %s\nVersion: %s\nCommit: %s\nBuild Time: %s\nOS/Arch: %s/%s\n", runtime.Version(), version, commit, buildTimeStr, runtime.GOOS, runtime.GOARCH)
}
