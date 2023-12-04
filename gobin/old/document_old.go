package old

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/dustin/go-humanize"
	"github.com/go-chi/chi/v5"
	"github.com/topi314/tint"

	"github.com/topi314/gobin/gobin/database"
	"github.com/topi314/gobin/templates"
)

type (
	ShareRequest struct {
		Permissions []Permission `json:"permissions"`
	}

	ShareResponse struct {
		Token string `json:"token"`
	}

	DeleteResponse struct {
		Versions int `json:"versions"`
	}
)

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

func (s *Server) GetPrettyDocument(w http.ResponseWriter, r *http.Request) {
	documentID, extension := parseDocumentID(r)
	version := s.parseDocumentVersion(r, w)
	if version == -1 {
		return
	}

	var (
		document  database.Document
		documents []database.Document
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

	versions := make([]templates.DocumentVersion, 0, len(documents))
	for i, documentVersion := range documents {
		versionTime := time.UnixMilli(documentVersion.Version)
		versionLabel := humanize.Time(versionTime)
		if i == 0 {
			versionLabel += " (current)"
		} else if i == len(documents)-1 {
			versionLabel += " (original)"
		}
		versions = append(versions, templates.DocumentVersion{
			Version: documentVersion.Version,
			Label:   versionLabel,
			Time:    versionTime.Format(VersionTimeFormat),
		})
	}

	formatted, css, language, style, err := s.renderDocument(r, document, "html", extension)
	if err != nil {
		s.prettyError(w, r, fmt.Errorf("failed to render document: %w", err), http.StatusInternalServerError)
		return
	}

	vars := templates.DocumentVars{
		ID:        document.ID,
		Version:   document.Version,
		Content:   document.Content,
		Formatted: formatted,
		CSS:       css,
		ThemeCSS:  s.styleCSS(style),
		Language:  language,

		Versions: versions,
		Lexers:   lexers.Names(false),
		Styles:   s.styles,
		Style:    style.Name,
		Theme:    style.Theme,

		Max:        s.cfg.MaxDocumentSize,
		Host:       r.Host,
		Preview:    s.cfg.Preview != nil,
		PreviewAlt: s.shortContent(document.Content),
	}
	if err = templates.Document(vars).Render(r.Context(), w); err != nil {
		slog.ErrorContext(r.Context(), "failed to execute template", tint.Err(err))
	}
}

func (s *Server) GetRawDocumentFile(w http.ResponseWriter, r *http.Request) {

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

	style := getStyle(r)
	s.ok(w, r, DocumentResponse{
		Key:       document.ID,
		Version:   version,
		Data:      document.Content,
		Formatted: formatted,
		CSS:       css,
		ThemeCSS:  s.styleCSS(style),
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
		formatted     string
		css           string
		finalLanguage = document.Language
	)
	formatter := r.URL.Query().Get("formatter")
	if formatter != "" {
		formatted, css, finalLanguage, _, err = s.renderDocument(r, document, formatter, "")
		if err != nil {
			s.error(w, r, fmt.Errorf("failed to render document: %w", err), http.StatusInternalServerError)
			return
		}
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
		VersionLabel: humanize.Time(versionTime) + " (original)",
		VersionTime:  versionTime.Format(VersionTimeFormat),
		Data:         document.Content,
		Formatted:    formatted,
		CSS:          css,
		Language:     finalLanguage,
		Token:        token,
	})
}

func (s *Server) PatchDocument(w http.ResponseWriter, r *http.Request) {
	documentID, extension := parseDocumentID(r)
	language := r.URL.Query().Get("language")

	claims := GetClaims(r)
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
		formatted     string
		css           string
		finalLanguage = document.Language
	)
	formatter := r.URL.Query().Get("formatter")
	if formatter != "" {
		formatted, css, finalLanguage, _, err = s.renderDocument(r, document, formatter, "")
		if err != nil {
			s.error(w, r, fmt.Errorf("failed to render update document"), http.StatusInternalServerError)
			return
		}
	}

	s.ExecuteWebhooks(r.Context(), WebhookEventUpdate, WebhookDocument{
		Key:      document.ID,
		Version:  document.Version,
		Language: finalLanguage,
		Data:     document.Content,
	})

	versionTime := time.Unix(document.Version, 0)
	s.ok(w, r, DocumentResponse{
		Key:          document.ID,
		Version:      document.Version,
		VersionLabel: humanize.Time(versionTime) + " (current)",
		VersionTime:  versionTime.Format(VersionTimeFormat),
		Data:         document.Content,
		Formatted:    formatted,
		CSS:          css,
		Language:     finalLanguage,
	})
}

func (s *Server) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	documentID, _ := parseDocumentID(r)
	version := s.parseDocumentVersion(r, w)
	if version == -1 {
		return
	}

	claims := GetClaims(r)
	if claims.Subject != documentID || !slices.Contains(claims.Permissions, PermissionDelete) {
		s.documentNotFound(w, r)
		return
	}

	var (
		document database.Document
		err      error
	)
	if version == 0 {
		document, err = s.db.DeleteDocument(r.Context(), documentID)
	} else {
		document, err = s.db.DeleteDocumentByVersion(r.Context(), documentID, version)
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

	s.ExecuteWebhooks(r.Context(), WebhookEventDelete, WebhookDocument{
		Key:      document.ID,
		Version:  document.Version,
		Language: document.Language,
		Data:     document.Content,
	})

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

	claims := GetClaims(r)
	if claims.Subject != documentID || !slices.Contains(claims.Permissions, PermissionShare) {
		s.documentNotFound(w, r)
		return
	}

	for _, permission := range shareRequest.Permissions {
		if !slices.Contains(claims.Permissions, permission) {
			s.error(w, r, ErrPermissionDenied(permission), http.StatusBadRequest)
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

func (s *Server) renderDocument(r *http.Request, document database.Document, formatterName string, extension string) (string, string, string, *chroma.Style, error) {
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
