package gobin

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/dustin/go-humanize"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/stampede"
	"github.com/riandyrn/otelchi"
	"github.com/topisenpai/gobin/internal/log"
	"golang.org/x/exp/slices"
	"golang.org/x/exp/slog"
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
	r.Use(middleware.Maybe(
		log.StructuredLogger,
		func(r *http.Request) bool {
			// Don't log requests for assets
			return !strings.HasPrefix(r.URL.Path, "/assets")
		},
	))
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

func (s *Server) DocumentVersions(w http.ResponseWriter, r *http.Request) {
	documentID, _ := parseDocumentID(r)
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
	documentID, _ := parseDocumentID(r)
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

func (s *Server) GetPrettyDocument(w http.ResponseWriter, r *http.Request) {
	documentID, extension := parseDocumentID(r)
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
	for _, documentVersion := range documents {
		versionTime := time.Unix(documentVersion.Version, 0)
		versions = append(versions, DocumentVersion{
			Version: documentVersion.Version,
			Label:   humanize.Time(versionTime),
			Time:    versionTime.Format(VersionTimeFormat),
		})
	}

	formatted, css, language, style, err := s.renderDocument(r, document, "html", extension)
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
		slog.Error("failed to execute template", slog.Any("err", err))
	}
}

func (s *Server) renderDocument(r *http.Request, document Document, formatterName string, extension string) (string, string, string, string, error) {
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
	var lexer chroma.Lexer

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

func (s *Server) GetRawDocument(w http.ResponseWriter, r *http.Request) {
	document, extension := s.getDocument(w, r)
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
		formatted, _, _, _, err = s.renderDocument(r, *document, formatter, extension)
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
	document, extension := s.getDocument(w, r)
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
		formatted, css, language, _, err = s.renderDocument(r, *document, formatter, extension)
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
	document, extension := s.getDocument(w, r)
	if document == nil {
		return
	}

	document.Content = s.shortContent(document.Content)

	formatted, _, _, _, err := s.renderDocument(r, *document, "svg", extension)
	if err != nil {
		s.prettyError(w, r, fmt.Errorf("failed to render document preview: %w", err), http.StatusInternalServerError)
		return
	}

	png, err := s.convertSVG2PNG(r.Context(), formatted)
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
		formatted, css, finalLanguage, _, err = s.renderDocument(r, document, formatter, "")
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

	versionTime := time.Unix(document.Version, 0)
	s.ok(w, r, DocumentResponse{
		Key:          document.ID,
		Version:      document.Version,
		VersionLabel: humanize.Time(versionTime),
		VersionTime:  versionTime.Format(VersionTimeFormat),
		Data:         data,
		Formatted:    template.HTML(formatted),
		CSS:          template.CSS(css),
		Language:     finalLanguage,
		Token:        token,
	})
}

func (s *Server) PatchDocument(w http.ResponseWriter, r *http.Request) {
	documentID, extension := parseDocumentID(r)
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
		if extension != "" {
			lexer = lexers.Match(extension)
		} else {
			lexer = lexers.Analyse(content)
		}
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
		formatted, css, finalLanguage, _, err = s.renderDocument(r, document, formatter, "")
		if err != nil {
			s.error(w, r, fmt.Errorf("failed to render update document"), http.StatusInternalServerError)
			return
		}
		data = document.Content
	}

	versionTime := time.Unix(document.Version, 0)
	s.ok(w, r, DocumentResponse{
		Key:          document.ID,
		Version:      document.Version,
		VersionLabel: humanize.Time(versionTime),
		VersionTime:  versionTime.Format(VersionTimeFormat),
		Data:         data,
		Formatted:    template.HTML(formatted),
		CSS:          template.CSS(css),
		Language:     finalLanguage,
	})
}

func (s *Server) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	documentID, _ := parseDocumentID(r)
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
	documentID, _ := parseDocumentID(r)

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

func (s *Server) prettyError(w http.ResponseWriter, r *http.Request, err error, status int) {
	w.WriteHeader(status)

	vars := TemplateErrorVariables{
		Error:     err.Error(),
		Status:    status,
		RequestID: middleware.GetReqID(r.Context()),
		Path:      r.URL.Path,
	}
	if tmplErr := s.tmpl(w, "error.gohtml", vars); tmplErr != nil && tmplErr != http.ErrHandlerTimeout {
		slog.ErrorCtx(r.Context(), "failed to execute error template", slog.Any("err", tmplErr))
	}
}

func (s *Server) error(w http.ResponseWriter, r *http.Request, err error, status int) {
	if errors.Is(err, http.ErrHandlerTimeout) {
		return
	}
	if status == http.StatusInternalServerError {
		slog.ErrorCtx(r.Context(), "internal server error", slog.Any("err", err))
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
		slog.ErrorCtx(r.Context(), "failed to encode json", slog.Any("err", err))
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
