package gobin

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/stampede"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/exp/slices"
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

type (
	TemplateVariables struct {
		ID        string
		Version   int64
		Content   template.HTML
		Formatted template.HTML
		CSS       template.CSS
		Language  string

		Versions []DocumentVersion
		Lexers   []string
		Styles   []string
		Style    string
		Theme    string

		Max        int
		Host       string
		Preview    bool
		PreviewAlt string
	}
	DocumentVersion struct {
		Version int64
		Label   string
		Time    string
	}
	DocumentResponse struct {
		Key          string        `json:"key,omitempty"`
		Version      int64         `json:"version"`
		VersionLabel string        `json:"version_label,omitempty"`
		VersionTime  string        `json:"version_time,omitempty"`
		Data         string        `json:"data,omitempty"`
		Formatted    template.HTML `json:"formatted,omitempty"`
		CSS          template.CSS  `json:"css,omitempty"`
		Language     string        `json:"language"`
		Token        string        `json:"token,omitempty"`
	}
	ShareRequest struct {
		Permissions []Permission `json:"permissions"`
	}
	ShareResponse struct {
		Token string `json:"token"`
	}
	DeleteResponse struct {
		Versions int `json:"versions"`
	}
	ErrorResponse struct {
		Message   string `json:"message"`
		Status    int    `json:"status"`
		Path      string `json:"path"`
		RequestID string `json:"request_id"`
	}
)

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.CleanPath)
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.Maybe(
		middleware.RequestLogger(&middleware.DefaultLogFormatter{
			Logger: log.Default(),
		}),
		func(r *http.Request) bool {
			// Don't log requests for assets
			return !strings.HasPrefix(r.URL.Path, "/assets")
		},
	))
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
	if s.cfg.Preview != nil && s.cfg.Preview.CacheSize > 0 && s.cfg.Preview.CacheTTL > 0 {
		previewCache = stampede.HandlerWithKey(s.cfg.Preview.CacheSize, s.cfg.Preview.CacheTTL, s.cacheKeyFunc)
	}

	r.Mount("/assets", http.FileServer(s.assets))
	r.Handle("/favicon.ico", s.file("/assets/favicon.png"))
	r.Handle("/favicon.png", s.file("/assets/favicon.png"))
	r.Handle("/favicon-light.png", s.file("/assets/favicon-light.png"))
	r.Handle("/robots.txt", s.file("/assets/robots.txt"))
	r.Group(func(r chi.Router) {
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
				if s.cfg.Preview != nil {
					r.Route("/preview", func(r chi.Router) {
						if previewCache != nil {
							r.Use(previewCache)
						}
						r.Get("/", s.GetDocumentPreview)
						r.Head("/", s.GetDocumentPreview)
					})
				}
				r.Route("/versions", func(r chi.Router) {
					r.Get("/", s.DocumentVersions)
					r.Route("/{version}", func(r chi.Router) {
						r.Get("/", s.GetDocument)
						r.Delete("/", s.DeleteDocument)
						if s.cfg.Preview != nil {
							r.Route("/preview", func(r chi.Router) {
								if previewCache != nil {
									r.Use(previewCache)
								}
								r.Get("/", s.GetDocumentPreview)
								r.Head("/", s.GetDocumentPreview)
							})
						}
					})
				})
			})
		})
		r.Get("/version", s.GetVersion)
		r.Route("/{documentID}", func(r chi.Router) {
			r.Get("/", s.GetPrettyDocument)
			r.Head("/", s.GetPrettyDocument)
			if s.cfg.Preview != nil {
				r.Route("/preview", func(r chi.Router) {
					if previewCache != nil {
						r.Use(previewCache)
					}
					r.Get("/", s.GetDocumentPreview)
					r.Head("/", s.GetDocumentPreview)
				})
			}

			r.Route("/{version}", func(r chi.Router) {
				r.Get("/", s.GetPrettyDocument)
				r.Head("/", s.GetPrettyDocument)
				if s.cfg.Preview != nil {
					r.Route("/preview", func(r chi.Router) {
						if previewCache != nil {
							r.Use(previewCache)
						}
						r.Get("/", s.GetDocumentPreview)
						r.Head("/", s.GetDocumentPreview)
					})
				}
			})
		})
		r.Get("/", s.GetPrettyDocument)
		r.Head("/", s.GetPrettyDocument)
	})
	r.NotFound(s.redirectRoot)

	if s.cfg.HTTPTimeout > 0 {
		return http.TimeoutHandler(r, s.cfg.HTTPTimeout, "Request timed out")
	}
	return otelhttp.NewHandler(r, "gobin-http")
}

func (s *Server) cacheKeyFunc(r *http.Request) uint64 {
	return stampede.BytesToHash([]byte(r.Method), []byte(chi.URLParam(r, "documentID")), []byte(chi.URLParam(r, "version")), []byte(r.URL.RawQuery))
}

func (s *Server) DocumentVersions(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "documentID")
	withContent := r.URL.Query().Get("withData") == "true"

	versions, err := s.db.GetDocumentVersions(r.Context(), documentID, withContent)
	if err != nil {
		s.error(w, r, fmt.Errorf("failed to get document versions: %w", err), http.StatusInternalServerError)
		return
	}
	if len(versions) == 0 {
		s.documentNotFound(w, r)
		return
	}
	var response []DocumentResponse
	for _, version := range versions {
		response = append(response, DocumentResponse{
			Version:  version.Version,
			Data:     version.Content,
			Language: version.Language,
		})
	}
	s.ok(w, r, response)
}

func (s *Server) GetDocumentVersion(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "documentID")
	version := parseDocumentVersion(r, s, w)
	if version == -1 {
		return
	}

	document, err := s.db.GetDocumentVersion(r.Context(), documentID, version)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.documentNotFound(w, r)
			return
		}
		s.error(w, r, fmt.Errorf("failed to get document version: %w", err), http.StatusInternalServerError)
		return
	}

	s.ok(w, r, DocumentResponse{
		Key:      document.ID,
		Version:  document.Version,
		Data:     document.Content,
		Language: document.Language,
	})
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

func (s *Server) GetPrettyDocument(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "documentID")
	version := parseDocumentVersion(r, s, w)
	if version == -1 {
		return
	}

	var (
		document  Document
		documents []Document
		err       error
	)
	if documentID != "" {
		if version == 0 {
			document, err = s.db.GetDocument(r.Context(), documentID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					s.redirectRoot(w, r)
					return
				}
				s.prettyError(w, r, fmt.Errorf("failed to get pretty document: %w", err), http.StatusInternalServerError)
				return
			}
		} else {
			document, err = s.db.GetDocumentVersion(r.Context(), documentID, version)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					s.redirectRoot(w, r)
					return
				}
				s.prettyError(w, r, fmt.Errorf("failed to get pretty document: %w", err), http.StatusInternalServerError)
				return
			}
		}
		documents, err = s.db.GetDocumentVersions(r.Context(), documentID, false)
		if err != nil {
			s.prettyError(w, r, fmt.Errorf("failed to get pretty document versions: %w", err), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}

	versions := make([]DocumentVersion, 0, len(documents))
	now := time.Now()
	for _, documentVersion := range documents {
		label, timeStr := FormatDocumentVersion(now, documentVersion.Version)
		versions = append(versions, DocumentVersion{
			Version: documentVersion.Version,
			Label:   label,
			Time:    timeStr,
		})
	}

	formatted, css, language, style, err := s.renderDocument(r, document, "html")
	if err != nil {
		s.prettyError(w, r, fmt.Errorf("failed to render document: %w", err), http.StatusInternalServerError)
		return
	}

	theme := "dark"
	if themeCookie, err := r.Cookie("theme"); err == nil && themeCookie.Value != "" {
		theme = themeCookie.Value
	}

	vars := TemplateVariables{
		ID:        document.ID,
		Version:   document.Version,
		Content:   template.HTML(document.Content),
		Formatted: template.HTML(formatted),
		CSS:       template.CSS(css),
		Language:  language,

		Versions: versions,
		Lexers:   lexers.Names(false),
		Styles:   styles.Names(),
		Style:    style,
		Theme:    theme,

		Max:        s.cfg.MaxDocumentSize,
		Host:       r.Host,
		Preview:    s.cfg.Preview != nil,
		PreviewAlt: template.HTMLEscapeString(s.shortContent(document.Content)),
	}
	if err = s.tmpl(w, "document.gohtml", vars); err != nil {
		log.Println("failed to execute template:", err)
	}
}

func (s *Server) renderDocument(r *http.Request, document Document, formatterName string) (string, string, string, string, error) {
	var (
		styleName    string
		languageName = document.Language
	)
	if styleCookie, err := r.Cookie("style"); err == nil {
		styleName = styleCookie.Value
	}
	queryStyle := r.URL.Query().Get("style")
	if queryStyle != "" {
		styleName = queryStyle
	}

	style := styles.Get(styleName)
	if style == nil {
		style = styles.Fallback
	}
	lexer := lexers.Get(languageName)
	if lexer == nil {
		lexer = lexers.Fallback
	}

	iterator, err := lexer.Tokenise(nil, document.Content)
	if err != nil {
		return "", "", "", "", err
	}

	formatter := formatters.Get(formatterName)
	if formatter == nil {
		formatter = formatters.Fallback
	}

	buff := new(bytes.Buffer)
	if err = formatter.Format(buff, style, iterator); err != nil {
		return "", "", "", "", err
	}

	cssBuff := new(bytes.Buffer)
	if htmlFormatter, ok := formatter.(*html.Formatter); ok {
		if err = htmlFormatter.WriteCSS(cssBuff, style); err != nil {
			return "", "", "", "", err
		}
	}

	language := lexer.Config().Name
	if document.ID == "" {
		language = "auto"
	}
	return buff.String(), cssBuff.String(), language, style.Name, nil
}

func (s *Server) GetVersion(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte(s.version))
}

func FormatDocumentVersion(now time.Time, versionRaw int64) (string, string) {
	version := time.Unix(versionRaw, 0)
	timeStr := version.Format("02/01/2006 15:04:05")
	if version.Year() < now.Year() {
		return fmt.Sprintf("%d years ago", now.Year()-version.Year()), timeStr
	}
	if version.Month() < now.Month() {
		return fmt.Sprintf("%d months ago", now.Month()-version.Month()), timeStr
	}
	if version.Day() < now.Day() {
		return fmt.Sprintf("%d days ago", now.Day()-version.Day()), timeStr
	}
	if version.Hour() < now.Hour() {
		return fmt.Sprintf("%d hours ago", now.Hour()-version.Hour()), timeStr
	}
	if version.Minute() < now.Minute() {
		return fmt.Sprintf("%d minutes ago", now.Minute()-version.Minute()), timeStr
	}
	return fmt.Sprintf("%d seconds ago", now.Second()-version.Second()), timeStr
}

func (s *Server) GetRawDocument(w http.ResponseWriter, r *http.Request) {
	document := s.getDocument(w, r)
	if document == nil {
		return
	}

	var formatted string
	query := r.URL.Query()
	formatter := query.Get("formatter")
	if formatter != "" {
		if formatter == "html" {
			formatter = "html-standalone"
		}
		if query.Get("language") != "" {
			document.Language = query.Get("language")
		}
		var err error
		formatted, _, _, _, err = s.renderDocument(r, *document, formatter)
		if err != nil {
			s.error(w, r, fmt.Errorf("failed to render raw document: %w", err), http.StatusInternalServerError)
			return
		}
	}

	content := document.Content
	if formatted != "" {
		content = formatted
	}

	var contentType string
	switch formatter {
	case "html", "html-standalone":
		contentType = "text/html; charset=UTF-8"
	case "svg":
		contentType = "image/svg+xml"
	case "json":
		contentType = "application/json"
	default:
		contentType = "text/plain; charset=UTF-8"
	}

	w.Header().Set("Content-Type", contentType)
	if r.Method == http.MethodHead {
		w.Header().Set("Content-Length", strconv.Itoa(len([]byte(content))))
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = w.Write([]byte(content))
}

func (s *Server) GetDocument(w http.ResponseWriter, r *http.Request) {
	document := s.getDocument(w, r)
	if document == nil {
		return
	}

	var (
		formatted string
		css       string
		language  string
	)
	query := r.URL.Query()
	formatter := query.Get("formatter")
	if formatter != "" {
		if query.Get("language") != "" {
			document.Language = query.Get("language")
		}
		var err error
		formatted, css, language, _, err = s.renderDocument(r, *document, formatter)
		if err != nil {
			s.error(w, r, fmt.Errorf("failed to render document: %w", err), http.StatusInternalServerError)
			return
		}
	}

	var version int64
	if chi.URLParam(r, "version") != "" {
		version = document.Version
	}

	s.ok(w, r, DocumentResponse{
		Key:       document.ID,
		Version:   version,
		Data:      document.Content,
		Formatted: template.HTML(formatted),
		CSS:       template.CSS(css),
		Language:  language,
	})
}

func (s *Server) GetDocumentPreview(w http.ResponseWriter, r *http.Request) {
	document := s.getDocument(w, r)
	if document == nil {
		return
	}

	document.Content = s.shortContent(document.Content)

	formatted, _, _, _, err := s.renderDocument(r, *document, "svg")
	if err != nil {
		s.prettyError(w, r, fmt.Errorf("failed to render document preview: %w", err), http.StatusInternalServerError)
		return
	}

	png, err := s.convertSVG2PNG(formatted)
	if err != nil {
		s.error(w, r, fmt.Errorf("failed to convert document preview: %w", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	if r.Method == http.MethodHead {
		w.Header().Set("Content-Length", strconv.Itoa(len(png)))
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = w.Write(png)
}

func (s *Server) PostDocument(w http.ResponseWriter, r *http.Request) {
	language := r.URL.Query().Get("language")
	content := s.readBody(w, r)
	if content == "" {
		return
	}

	if s.exceedsMaxDocumentSize(w, r, content) {
		return
	}

	var lexer chroma.Lexer
	if language == "auto" || language == "" {
		lexer = lexers.Analyse(content)
	} else {
		lexer = lexers.Get(language)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}

	document, err := s.db.CreateDocument(r.Context(), content, lexer.Config().Name)
	if err != nil {
		s.error(w, r, fmt.Errorf("failed to create document: %w", err), http.StatusInternalServerError)
		return
	}

	var (
		data          string
		formatted     string
		css           string
		finalLanguage string
	)
	formatter := r.URL.Query().Get("formatter")
	if formatter != "" {
		formatted, css, finalLanguage, _, err = s.renderDocument(r, document, formatter)
		if err != nil {
			s.error(w, r, fmt.Errorf("failed to render document: %w", err), http.StatusInternalServerError)
			return
		}
		data = document.Content
	}

	token, err := s.NewToken(document.ID, []Permission{PermissionWrite, PermissionDelete, PermissionShare})
	if err != nil {
		s.error(w, r, fmt.Errorf("failed to create jwt token: %w", err), http.StatusInternalServerError)
		return
	}

	versionLabel, versionTime := FormatDocumentVersion(time.Now(), document.Version)
	s.ok(w, r, DocumentResponse{
		Key:          document.ID,
		Version:      document.Version,
		VersionLabel: versionLabel,
		VersionTime:  versionTime,
		Data:         data,
		Formatted:    template.HTML(formatted),
		CSS:          template.CSS(css),
		Language:     finalLanguage,
		Token:        token,
	})
}

func (s *Server) PatchDocument(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "documentID")
	language := r.URL.Query().Get("language")

	claims := s.GetClaims(r)
	if claims.Subject != documentID || !slices.Contains(claims.Permissions, PermissionWrite) {
		s.documentNotFound(w, r)
		return
	}

	content := s.readBody(w, r)
	if content == "" {
		return
	}

	if s.exceedsMaxDocumentSize(w, r, content) {
		return
	}

	var lexer chroma.Lexer
	if language == "auto" || language == "" {
		lexer = lexers.Analyse(content)
	} else {
		lexer = lexers.Get(language)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}

	document, err := s.db.UpdateDocument(r.Context(), documentID, content, lexer.Config().Name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.documentNotFound(w, r)
			return
		}
		s.error(w, r, err, http.StatusInternalServerError)
		return
	}

	var (
		data          string
		formatted     string
		css           string
		finalLanguage string
	)
	formatter := r.URL.Query().Get("formatter")
	if formatter != "" {
		formatted, css, finalLanguage, _, err = s.renderDocument(r, document, formatter)
		if err != nil {
			s.error(w, r, fmt.Errorf("failed to render update document"), http.StatusInternalServerError)
			return
		}
		data = document.Content
	}

	versionLabel, versionTime := FormatDocumentVersion(time.Now(), document.Version)
	s.ok(w, r, DocumentResponse{
		Key:          document.ID,
		Version:      document.Version,
		VersionLabel: versionLabel,
		VersionTime:  versionTime,
		Data:         data,
		Formatted:    template.HTML(formatted),
		CSS:          template.CSS(css),
		Language:     finalLanguage,
	})
}

func (s *Server) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "documentID")
	version := parseDocumentVersion(r, s, w)
	if version == -1 {
		return
	}

	claims := s.GetClaims(r)
	if claims.Subject != documentID || !slices.Contains(claims.Permissions, PermissionDelete) {
		s.documentNotFound(w, r)
		return
	}

	var err error
	if version == 0 {
		err = s.db.DeleteDocument(r.Context(), documentID)
	} else {
		err = s.db.DeleteDocumentByVersion(r.Context(), documentID, version)
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.documentNotFound(w, r)
			return
		}
		s.error(w, r, err, http.StatusInternalServerError)
		return
	}
	if version == 0 {
		w.WriteHeader(http.StatusNoContent)
	}

	count, err := s.db.GetVersionCount(r.Context(), documentID)
	if err != nil {
		s.error(w, r, err, http.StatusInternalServerError)
		return
	}
	s.ok(w, r, DeleteResponse{
		Versions: count,
	})
}

func (s *Server) PostDocumentShare(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "documentID")

	var shareRequest ShareRequest
	if err := json.NewDecoder(r.Body).Decode(&shareRequest); err != nil {
		s.error(w, r, err, http.StatusBadRequest)
		return
	}

	if len(shareRequest.Permissions) == 0 {
		s.error(w, r, ErrNoPermissions, http.StatusBadRequest)
		return
	}

	for _, permission := range shareRequest.Permissions {
		if !permission.IsValid() {
			s.error(w, r, ErrUnknownPermission(permission), http.StatusBadRequest)
			return
		}
	}

	claims := s.GetClaims(r)
	if claims.Subject != documentID || !slices.Contains(claims.Permissions, PermissionShare) {
		s.documentNotFound(w, r)
		return
	}

	for _, permission := range shareRequest.Permissions {
		if !slices.Contains(claims.Permissions, permission) {
			s.error(w, r, ErrPermissionDenied(permission), http.StatusForbidden)
			return
		}
	}

	token, err := s.NewToken(documentID, shareRequest.Permissions)
	if err != nil {
		s.error(w, r, err, http.StatusInternalServerError)
		return
	}

	s.ok(w, r, ShareResponse{
		Token: token,
	})
}

func (s *Server) getDocument(w http.ResponseWriter, r *http.Request) *Document {
	documentID := chi.URLParam(r, "documentID")
	if documentID == "" {
		return &Document{}
	}

	version := parseDocumentVersion(r, s, w)
	if version == -1 {
		return nil
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
			return nil
		}
		s.error(w, r, err, http.StatusInternalServerError)
		return nil
	}
	return &document
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

func (s *Server) RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only apply rate limiting to POST, PATCH, and DELETE requests
		if r.Method != http.MethodPost && r.Method != http.MethodPatch && r.Method != http.MethodDelete {
			next.ServeHTTP(w, r)
			return
		}
		remoteAddr := strings.SplitN(r.RemoteAddr, ":", 2)[0]
		// Filter whitelisted IPs
		if slices.Contains(s.cfg.RateLimit.Whitelist, remoteAddr) {
			next.ServeHTTP(w, r)
			return
		}
		// Filter blacklisted IPs
		if slices.Contains(s.cfg.RateLimit.Blacklist, remoteAddr) {
			retryAfter := maxUnix - int(time.Now().Unix())
			w.Header().Set("X-RateLimit-Limit", "0")
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", strconv.Itoa(maxUnix))
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			w.WriteHeader(http.StatusTooManyRequests)
			s.rateLimit(w, r)
			return
		}
		if s.rateLimitHandler == nil {
			next.ServeHTTP(w, r)
			return
		}
		s.rateLimitHandler(next).ServeHTTP(w, r)
	})
}

func (s *Server) log(r *http.Request, logType string, err error) {
	if errors.Is(err, context.DeadlineExceeded) {
		return
	}
	log.Printf("Error while handling %s(%s) %s: %s\n", logType, middleware.GetReqID(r.Context()), r.RequestURI, err)
}

func (s *Server) prettyError(w http.ResponseWriter, r *http.Request, err error, status int) {
	if status == http.StatusInternalServerError {
		s.log(r, "pretty request", err)
	}
	w.WriteHeader(status)

	vars := map[string]any{
		"Error":     err.Error(),
		"Status":    status,
		"RequestID": middleware.GetReqID(r.Context()),
		"Path":      r.URL.Path,
	}
	if tmplErr := s.tmpl(w, "error.gohtml", vars); tmplErr != nil && tmplErr != http.ErrHandlerTimeout {
		s.log(r, "template", tmplErr)
	}
}

func (s *Server) error(w http.ResponseWriter, r *http.Request, err error, status int) {
	if errors.Is(err, http.ErrHandlerTimeout) {
		return
	}
	if status == http.StatusInternalServerError {
		s.log(r, "request", err)
	}
	s.json(w, r, ErrorResponse{
		Message:   err.Error(),
		Status:    status,
		Path:      r.URL.Path,
		RequestID: middleware.GetReqID(r.Context()),
	}, status)
}

func (s *Server) ok(w http.ResponseWriter, r *http.Request, v any) {
	s.json(w, r, v, http.StatusOK)
}

func (s *Server) json(w http.ResponseWriter, r *http.Request, v any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}

	if err := json.NewEncoder(w).Encode(v); err != nil && err != http.ErrHandlerTimeout {
		s.log(r, "json", err)
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
