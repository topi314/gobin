package gobin

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/stampede"
	"github.com/riandyrn/otelchi"
	"github.com/samber/slog-chi"
)

const maxUnix = int(^int32(0))

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
		r.Route("/versions/{version}", func(r chi.Router) {
			r.Get("/", s.GetRawDocument)
			r.Head("/", s.GetRawDocument)
		})
	})

	r.Route("/documents", func(r chi.Router) {
		r.Post("/", s.PostDocument)
		r.Route("/{documentID}", func(r chi.Router) {
			r.Get("/", s.GetDocument)
			r.Patch("/", s.PatchDocument)
			r.Delete("/", s.DeleteDocument)
			r.Post("/share", s.PostDocumentShare)
			previewHandler(r)
			r.Route("/versions", func(r chi.Router) {
				r.Get("/", s.DocumentVersions)
				r.Route("/{version}", func(r chi.Router) {
					r.Get("/", s.GetDocument)
					r.Delete("/", s.DeleteDocument)
					previewHandler(r)
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

func (s *Server) cacheKeyFunc(r *http.Request) uint64 {
	return stampede.BytesToHash([]byte(r.Method), []byte(chi.URLParam(r, "documentID")), []byte(chi.URLParam(r, "version")), []byte(r.URL.RawQuery))
}

func parseDocumentVersion(r *http.Request, s *Server, w http.ResponseWriter) int64 {
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

func parseDocumentID(r *http.Request) (string, string) {
	documentID := chi.URLParam(r, "documentID")
	if documentID == "" {
		return "", ""
	}

	// get the filename and extension from the documentID
	filename := documentID
	extension := ""
	if index := strings.LastIndex(documentID, "."); index != -1 {
		filename = documentID[:index]
		extension = documentID[index+1:]
	}

	return filename, extension
}

func (s *Server) renderDocument(r *http.Request, document Document, formatterName string, extension string) (string, string, string, *chroma.Style, error) {
	var (
		languageName = document.Language
		lexer        chroma.Lexer
	)

	if s.cfg.MaxHighlightSize > 0 && len([]rune(document.Content)) > s.cfg.MaxHighlightSize {
		lexer = lexers.Get("plaintext")
	} else if extension != "" {
		lexer = lexers.Match(fmt.Sprintf("%s.%s", document.ID, extension))
	} else {
		lexer = lexers.Get(languageName)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}

	iterator, err := lexer.Tokenise(nil, document.Content)
	if err != nil {
		return "", "", "", nil, err
	}

	formatter := formatters.Get(formatterName)
	if formatter == nil {
		formatter = formatters.Fallback
	}

	style := getStyle(r)

	buff := new(bytes.Buffer)
	if err = formatter.Format(buff, style, iterator); err != nil {
		return "", "", "", nil, err
	}

	cssBuff := new(bytes.Buffer)
	if htmlFormatter, ok := formatter.(*html.Formatter); ok {
		if err = htmlFormatter.WriteCSS(cssBuff, style); err != nil {
			return "", "", "", nil, err
		}
	}

	language := lexer.Config().Name
	if document.ID == "" {
		language = "auto"
	}
	return buff.String(), cssBuff.String(), language, style, nil
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
		styleName, _ = url.QueryUnescape(styleCookie.Value)
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

func (s *Server) getDocument(w http.ResponseWriter, r *http.Request) (*Document, string) {
	documentID, extension := parseDocumentID(r)
	if documentID == "" {
		return &Document{}, ""
	}

	version := parseDocumentVersion(r, s, w)
	if version == -1 {
		return nil, ""
	}

	var (
		document Document
		err      error
	)
	if version == 0 {
		document, err = s.db.GetDocument(r.Context(), documentID)
	} else {
		document, err = s.db.GetDocumentVersion(r.Context(), documentID, version)
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.documentNotFound(w, r)
			return nil, ""
		}
		s.error(w, r, err, http.StatusInternalServerError)
		return nil, ""
	}
	return &document, extension
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
