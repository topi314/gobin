package server

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/stampede"
	"github.com/riandyrn/otelchi"
	"github.com/riandyrn/otelchi/metric"
	"github.com/samber/slog-chi"

	"github.com/topi314/gobin/v3/internal/ezhttp"
	"github.com/topi314/gobin/v3/internal/httperr"
	"github.com/topi314/gobin/v3/server/templates"
)

var (
	ErrDocumentNotFound       = errors.New("document not found")
	ErrDocumentFileNotFound   = errors.New("document file not found")
	ErrInvalidDocumentVersion = errors.New("document version is invalid")
	ErrPreviewsDisabled       = errors.New("document previews disabled")
	ErrRateLimit              = errors.New("rate limit exceeded")
)

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(otelchi.Middleware("gobin", otelchi.WithChiRoutes(r)))
	baseCfg := metric.NewBaseConfig("gobin")
	r.Use(metric.NewRequestDurationMillis(baseCfg))
	r.Use(metric.NewRequestInFlight(baseCfg))
	r.Use(metric.NewResponseSizeBytes(baseCfg))
	r.Use(middleware.CleanPath)
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(slogchi.NewWithConfig(slog.Default(), slogchi.Config{
		DefaultLevel:     slog.LevelInfo,
		ClientErrorLevel: slog.LevelDebug,
		ServerErrorLevel: slog.LevelError,
		WithRequestID:    true,
		WithSpanID:       s.cfg.Otel.Enabled,
		WithTraceID:      s.cfg.Otel.Enabled,
		Filters: []slogchi.Filter{
			slogchi.IgnorePathPrefix("/assets"),
		},
	}))
	r.Use(cacheControl)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Heartbeat("/ping"))
	if s.cfg.RateLimit.Enabled {
		r.Use(s.RateLimit)
	}
	r.Use(s.JWTMiddleware)
	r.Use(middleware.GetHead)

	if s.cfg.Debug {
		r.Mount("/debug", middleware.Profiler())
	}

	var previewCache func(http.Handler) http.Handler
	previewHandler := func(r chi.Router) {
		r.Get("/preview", func(w http.ResponseWriter, r *http.Request) {
			s.error(w, r, httperr.NotFound(ErrPreviewsDisabled))
		})
	}
	if s.cfg.Preview.Enabled {
		previewCache = stampede.HandlerWithKey(s.cfg.Preview.CacheSize, time.Duration(s.cfg.Preview.CacheTTL), s.cacheKeyFunc)
	}
	if s.cfg.Preview.Enabled {
		previewHandler = func(r chi.Router) {
			r.Route("/preview", func(r chi.Router) {
				if previewCache != nil {
					r.Use(previewCache)
				}
				r.Get("/", s.GetDocumentPreview)
			})
		}
	}

	r.Mount("/assets", http.FileServer(s.assets))
	r.HandleFunc("/assets/theme.css", s.ThemeCSS)
	r.Handle("/favicon.ico", s.file("/assets/favicon.png"))
	r.Handle("/favicon.png", s.file("/assets/favicon.png"))
	r.Handle("/favicon-light.png", s.file("/assets/favicon-light.png"))
	r.Handle("/robots.txt", s.file("/assets/robots.txt"))

	r.Get("/version", s.GetVersion)

	r.Route("/documents", func(r chi.Router) {
		r.Post("/", s.PostDocument)

		filesHandler := func(r chi.Router) {
			r.Route("/files/{fileName}", func(r chi.Router) {
				r.Get("/", s.GetDocumentFile)
			})
		}
		r.Route("/{documentID}", func(r chi.Router) {
			r.Get("/", s.GetDocument)
			r.Patch("/", s.PatchDocument)
			r.Delete("/", s.DeleteDocument)
			r.Post("/share", s.PostDocumentShare)

			r.Route("/versions", func(r chi.Router) {
				r.Get("/", s.DocumentVersions)
				r.Route("/{version}", func(r chi.Router) {
					r.Get("/", s.GetDocument)
					r.Delete("/", s.DeleteDocument)
				})
			})

			r.Route("/webhooks", func(r chi.Router) {
				r.Post("/", s.PostDocumentWebhook)
				r.Route("/{webhookID}", func(r chi.Router) {
					r.Get("/", s.GetDocumentWebhook)
					r.Patch("/", s.PatchDocumentWebhook)
					r.Delete("/", s.DeleteDocumentWebhook)
				})
			})

			filesHandler(r)
		})
	})

	rawFilesHandler := func(r chi.Router) {
		r.Route("/files/{fileName}", func(r chi.Router) {
			r.Get("/", s.GetRawDocumentFile)
		})
	}
	r.Route("/raw/{documentID}", func(r chi.Router) {
		r.Get("/", s.GetRawDocument)
		r.Route("/versions/{version}", func(r chi.Router) {
			r.Get("/", s.GetRawDocument)
			rawFilesHandler(r)
		})
		rawFilesHandler(r)
	})

	r.Route("/{documentID}", func(r chi.Router) {
		r.Get("/", s.GetPrettyDocument)
		previewHandler(r)
		r.Route("/{version}", func(r chi.Router) {
			r.Get("/", s.GetPrettyDocument)
			previewHandler(r)
		})
	})
	r.Get("/", s.GetPrettyDocument)

	r.NotFound(s.redirectRoot)

	if s.cfg.HTTPTimeout > 0 {
		return http.TimeoutHandler(r, time.Duration(s.cfg.HTTPTimeout), "Request timed out")
	}
	return r
}

func (s *Server) GetVersion(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte(s.version.Format()))
}

func (s *Server) redirectRoot(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) prettyError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	var httpErr *httperr.Error
	if errors.As(err, &httpErr) {
		status = httpErr.Status

		if httpErr.Location != "" {
			http.Redirect(w, r, httpErr.Location, status)
			return
		}
	}

	w.WriteHeader(status)
	if tmplErr := templates.Error(templates.ErrorVars{
		Error:     err.Error(),
		Status:    status,
		RequestID: middleware.GetReqID(r.Context()),
		Path:      r.URL.Path,
	}).Render(r.Context(), w); tmplErr != nil && !errors.Is(tmplErr, http.ErrHandlerTimeout) {
		slog.ErrorContext(r.Context(), "failed to execute error template", slog.Any("err", tmplErr))
	}
}

func (s *Server) error(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, http.ErrHandlerTimeout) {
		return
	}

	status := http.StatusInternalServerError
	var httpErr *httperr.Error
	if errors.As(err, &httpErr) {
		status = httpErr.Status
	}

	if httpErr.Location != "" {
		http.Redirect(w, r, httpErr.Location, status)
		return
	}

	if status == http.StatusInternalServerError {
		slog.ErrorContext(r.Context(), "internal server error", slog.Any("err", err))
	}
	s.json(w, r, ezhttp.ErrorResponse{
		Message:   err.Error(),
		Status:    status,
		Path:      r.URL.Path,
		RequestID: middleware.GetReqID(r.Context()),
	}, status)
}

func (s *Server) ok(w http.ResponseWriter, r *http.Request, v any) {
	if v == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	s.json(w, r, v, http.StatusOK)
}

func (s *Server) json(w http.ResponseWriter, r *http.Request, v any, status int) {
	w.Header().Set(ezhttp.HeaderContentType, ezhttp.ContentTypeJSON)
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}

	if err := json.NewEncoder(w).Encode(v); err != nil && !errors.Is(err, http.ErrHandlerTimeout) {
		slog.ErrorContext(r.Context(), "failed to encode json", slog.Any("err", err))
	}
}

func (s *Server) file(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file, err := s.assets.Open(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer func() {
			_ = file.Close()
		}()
		if _, err = io.Copy(w, file); err != nil {
			slog.ErrorContext(r.Context(), "failed to copy file", slog.Any("err", err))
		}
	}
}

func (s *Server) shortContent(content string) string {
	if s.cfg.Preview.Enabled && s.cfg.Preview.MaxLines > 0 {
		var newLines int
		maxNewLineIndex := strings.IndexFunc(content, func(r rune) bool {
			if r == '\n' {
				newLines++
			}
			return newLines == s.cfg.Preview.MaxLines
		})

		if maxNewLineIndex > 0 {
			content = content[:maxNewLineIndex]
		}
	}
	return content
}
