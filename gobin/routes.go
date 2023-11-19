package gobin

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/stampede"
	"github.com/riandyrn/otelchi"
	slogchi "github.com/samber/slog-chi"
	"github.com/topi314/tint"

	"github.com/topi314/gobin/templates"
)

var (
	ErrDocumentNotFound = errors.New("document not found")
	ErrRateLimit        = errors.New("rate limit exceeded")
	ErrEmptyBody        = errors.New("empty request body")
	ErrContentTooLarge  = func(maxLength int) error {
		return fmt.Errorf("content too large, must be less than %d chars", maxLength)
	}
)

var VersionTimeFormat = "2006-01-02 15:04:05"

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(otelchi.Middleware("gobin", otelchi.WithChiRoutes(r)))
	r.Use(middleware.CleanPath)
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(slogchi.NewWithConfig(slog.Default(), slogchi.Config{
		DefaultLevel:     slog.LevelInfo,
		ClientErrorLevel: slog.LevelDebug,
		ServerErrorLevel: slog.LevelError,
		WithRequestID:    true,
		WithSpanID:       s.cfg.Otel != nil,
		WithTraceID:      s.cfg.Otel != nil,
		Filters: []slogchi.Filter{
			slogchi.IgnorePathPrefix("/assets"),
		},
	}))
	r.Use(cacheControl)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Heartbeat("/ping"))
	if s.cfg.RateLimit != nil {
		r.Use(s.RateLimit)
	}
	r.Use(s.JWTMiddleware)

	if s.cfg.Debug {
		r.Mount("/debug", middleware.Profiler())
	}

	var previewCache func(http.Handler) http.Handler
	previewHandler := func(r chi.Router) {}
	if s.cfg.Preview != nil && s.cfg.Preview.CacheSize > 0 && s.cfg.Preview.CacheTTL > 0 {
		previewCache = stampede.HandlerWithKey(s.cfg.Preview.CacheSize, s.cfg.Preview.CacheTTL, s.cacheKeyFunc)
	}
	if s.cfg.Preview != nil {
		previewHandler = func(r chi.Router) {
			r.Route("/preview", func(r chi.Router) {
				if previewCache != nil {
					r.Use(previewCache)
				}
				r.Get("/", s.GetDocumentPreview)
				r.Head("/", s.GetDocumentPreview)
			})
		}
	}

	r.Mount("/assets", http.FileServer(s.assets))
	r.HandleFunc("/assets/theme.css", s.StyleCSS)
	r.Handle("/favicon.ico", s.file("/assets/favicon.png"))
	r.Handle("/favicon.png", s.file("/assets/favicon.png"))
	r.Handle("/favicon-light.png", s.file("/assets/favicon-light.png"))
	r.Handle("/robots.txt", s.file("/assets/robots.txt"))

	r.Get("/version", s.GetVersion)

	r.Route("/raw/{documentID}", func(r chi.Router) {
		r.Get("/", s.GetRawDocument)
		r.Head("/", s.GetRawDocument)
		r.Route("/files/{fileID}", func(r chi.Router) {
			r.Get("/", s.GetRawDocumentFile)
			r.Head("/", s.GetRawDocumentFile)
			r.Route("/versions/{version}", func(r chi.Router) {
				r.Get("/", s.GetRawDocumentFile)
				r.Head("/", s.GetRawDocumentFile)
			})
		})
	})

	r.Route("/documents", func(r chi.Router) {
		r.Post("/", s.PostDocument)
		r.Route("/{documentID}", func(r chi.Router) {
			r.Get("/", s.GetDocument)
			r.Patch("/", s.PatchDocument)
			r.Delete("/", s.DeleteDocument)
			r.Post("/share", s.PostDocumentShare)

			r.Route("/webhooks", func(r chi.Router) {
				r.Post("/", s.PostDocumentWebhook)
				r.Route("/{webhookID}", func(r chi.Router) {
					r.Get("/", s.GetDocumentWebhook)
					r.Patch("/", s.PatchDocumentWebhook)
					r.Delete("/", s.DeleteDocumentWebhook)
				})
			})

			previewHandler(r)
			r.Route("/files", func(r chi.Router) {
				r.Post("/", s.PostDocumentFile)
				r.Route("/{fileID}", func(r chi.Router) {
					r.Get("/", s.GetDocumentFile)
					r.Patch("/", s.PatchDocumentFile)
					r.Delete("/", s.DeleteDocumentFile)
					r.Route("/versions", func(r chi.Router) {
						r.Get("/", s.DocumentFileVersions)
						r.Route("/{version}", func(r chi.Router) {
							r.Get("/", s.GetDocumentFile)
							r.Delete("/", s.DeleteDocumentFile)
							previewHandler(r)
						})
					})
				})
			})
		})
	})

	r.Route("/{documentID}", func(r chi.Router) {
		r.Get("/", s.GetPrettyDocument)
		r.Head("/", s.GetPrettyDocument)
		previewHandler(r)
		r.Route("/{version}", func(r chi.Router) {
			r.Get("/", s.GetPrettyDocument)
			r.Head("/", s.GetPrettyDocument)
			previewHandler(r)
		})
	})
	r.Get("/", s.GetPrettyDocument)
	r.Head("/", s.GetPrettyDocument)

	r.NotFound(s.redirectRoot)

	if s.cfg.HTTPTimeout > 0 {
		return http.TimeoutHandler(r, s.cfg.HTTPTimeout, "Request timed out")
	}
	return r
}

func (s *Server) StyleCSS(w http.ResponseWriter, r *http.Request) {
	style := getStyle(r)
	cssBuff := s.styleCSS(style)

	w.Header().Set("Content-Type", "text/css; charset=UTF-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(cssBuff)))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write([]byte(cssBuff))
}

func (s *Server) GetVersion(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte(s.version))
}

func (s *Server) parseDocumentVersion(r *http.Request, w http.ResponseWriter) int64 {
	version := chi.URLParam(r, "version")
	if version == "" {
		return 0
	}

	int64Version, err := strconv.ParseInt(version, 10, 64)
	if err != nil {
		s.documentNotFound(w, r)
		return -1
	}
	return int64Version
}

func (s *Server) styleCSS(style *chroma.Style) string {
	cssBuff := new(bytes.Buffer)
	background := style.Get(chroma.Background)
	_, _ = fmt.Fprint(cssBuff, ":root{")
	_, _ = fmt.Fprintf(cssBuff, "--bg-primary: %s;", background.Background.String())
	_, _ = fmt.Fprintf(cssBuff, "--bg-secondary: %s;", background.Background.BrightenOrDarken(0.07).String())
	_, _ = fmt.Fprintf(cssBuff, "--nav-button-bg: %s;", background.Background.BrightenOrDarken(0.12).String())
	_, _ = fmt.Fprintf(cssBuff, "--text-primary: %s;", background.Colour.String())
	_, _ = fmt.Fprintf(cssBuff, "--text-secondary: %s;", background.Colour.BrightenOrDarken(0.2).String())
	_, _ = fmt.Fprintf(cssBuff, "--bg-scrollbar: %s;", background.Background.BrightenOrDarken(0.1).String())
	_, _ = fmt.Fprintf(cssBuff, "--bg-scrollbar-thumb: #%s;", background.Background.BrightenOrDarken(0.2).String())
	_, _ = fmt.Fprintf(cssBuff, "--bg-scrollbar-thumb-hover: %s;", background.Background.BrightenOrDarken(0.3).String())
	_, _ = fmt.Fprint(cssBuff, "}")
	return cssBuff.String()
}

func getStyle(r *http.Request) *chroma.Style {
	var styleName string
	if styleCookie, err := r.Cookie("style"); err == nil {
		styleName = styleCookie.Value
	}
	queryStyle := r.URL.Query().Get("style")
	if queryStyle != "" {
		styleName = queryStyle
	}

	style := styles.Get(styleName)
	if style == nil {
		return styles.Fallback
	}

	return style
}

func (s *Server) readBody(w http.ResponseWriter, r *http.Request) string {
	content, err := io.ReadAll(r.Body)
	if err != nil {
		s.error(w, r, err, http.StatusInternalServerError)
		return ""
	}

	if len(content) == 0 {
		s.error(w, r, ErrEmptyBody, http.StatusBadRequest)
		return ""
	}
	return string(content)
}

func (s *Server) redirectRoot(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) documentNotFound(w http.ResponseWriter, r *http.Request) {
	s.error(w, r, ErrDocumentNotFound, http.StatusNotFound)
}

func (s *Server) rateLimit(w http.ResponseWriter, r *http.Request) {
	s.error(w, r, ErrRateLimit, http.StatusTooManyRequests)
}

func (s *Server) prettyError(w http.ResponseWriter, r *http.Request, err error, status int) {
	w.WriteHeader(status)

	vars := templates.ErrorVars{
		Error:     err.Error(),
		Status:    status,
		RequestID: middleware.GetReqID(r.Context()),
		Path:      r.URL.Path,
	}
	if tmplErr := templates.Error(vars).Render(r.Context(), w); tmplErr != nil && !errors.Is(tmplErr, http.ErrHandlerTimeout) {
		slog.ErrorContext(r.Context(), "failed to execute error template", tint.Err(tmplErr))
	}
}

func (s *Server) error(w http.ResponseWriter, r *http.Request, err error, status int) {
	if errors.Is(err, http.ErrHandlerTimeout) {
		return
	}
	if status == http.StatusInternalServerError {
		slog.ErrorContext(r.Context(), "internal server error", tint.Err(err))
	}
	s.json(w, r, ErrorResponse{
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
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}

	if err := json.NewEncoder(w).Encode(v); err != nil && !errors.Is(err, http.ErrHandlerTimeout) {
		slog.ErrorContext(r.Context(), "failed to encode json", tint.Err(err))
	}
}

func (s *Server) exceedsMaxDocumentSize(w http.ResponseWriter, r *http.Request, content string) bool {
	if s.cfg.MaxDocumentSize > 0 && len([]rune(content)) > s.cfg.MaxDocumentSize {
		s.error(w, r, ErrContentTooLarge(s.cfg.MaxDocumentSize), http.StatusBadRequest)
		return true
	}
	return false
}

func (s *Server) file(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file, err := s.assets.Open(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()
		_, _ = io.Copy(w, file)
	}
}

func (s *Server) shortContent(content string) string {
	if s.cfg.Preview != nil && s.cfg.Preview.MaxLines > 0 {
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
