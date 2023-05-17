package gobin

import (
	"fmt"
	"golang.org/x/exp/slog"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/go-chi/httprate"
	"github.com/go-jose/go-jose/v3"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type ExecuteTemplateFunc func(wr io.Writer, name string, data any) error

func NewServer(version string, debug bool, cfg Config, db *DB, signer jose.Signer, tracer trace.Tracer, meter metric.Meter, assets http.FileSystem, tmpl ExecuteTemplateFunc) *Server {
	s := &Server{
		version: version,
		debug:   debug,
		cfg:     cfg,
		db:      db,
		signer:  signer,
		tracer:  tracer,
		meter:   meter,
		assets:  assets,
		tmpl:    tmpl,
	}

	s.server = &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: s.Routes(),
	}

	if cfg.RateLimit != nil && cfg.RateLimit.Requests > 0 && cfg.RateLimit.Duration > 0 {
		s.rateLimitHandler = httprate.NewRateLimiter(
			cfg.RateLimit.Requests,
			cfg.RateLimit.Duration,
			httprate.WithLimitHandler(s.rateLimit),
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
	db               *DB
	server           *http.Server
	signer           jose.Signer
	tracer           trace.Tracer
	meter            metric.Meter
	assets           http.FileSystem
	tmpl             ExecuteTemplateFunc
	rateLimitHandler func(http.Handler) http.Handler
}

func (s *Server) Start() {
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("Error while listening", slog.Any("err", err))
		os.Exit(1)
	}
}

func (s *Server) Close() {
	if err := s.server.Close(); err != nil {
		slog.Error("Error while closing server", slog.Any("err", err))
	}

	if err := s.db.Close(); err != nil {
		slog.Error("Error while closing database", slog.Any("err", err))
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

func cacheControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=86400")
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		next.ServeHTTP(w, r)
	})
}
