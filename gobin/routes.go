package gobin

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"slices"
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
	"github.com/topi314/gobin/internal/log"
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

	vars := TemplateVariables{
		ID:        document.ID,
		Version:   document.Version,
		Content:   template.HTML(document.Content),
		Formatted: template.HTML(formatted),
		CSS:       template.CSS(css),
		ThemeCSS:  template.CSS(s.styleCSS(style)),
		Language:  language,

		Versions: versions,
		Lexers:   lexers.Names(false),
		Styles:   s.styles,
		Style:    style.Name,
		Theme:    style.Theme,

		Max:        s.cfg.MaxDocumentSize,
		Host:       r.Host,
		Preview:    s.cfg.Preview != nil,
		PreviewAlt: template.HTMLEscapeString(s.shortContent(document.Content)),
	}
	if err = s.tmpl(w, "document.gohtml", vars); err != nil {
		slog.Error("failed to execute template", slog.Any("err", err))
	}
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

	style := getStyle(r)
	s.ok(w, r, DocumentResponse{
		Key:       document.ID,
		Version:   version,
		Data:      document.Content,
		Formatted: template.HTML(formatted),
		CSS:       template.CSS(css),
		ThemeCSS:  template.CSS(s.styleCSS(style)),
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
