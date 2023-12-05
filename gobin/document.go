package gobin

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"slices"
	"strconv"
	"time"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/dustin/go-humanize"
	"github.com/go-chi/chi/v5"
	"github.com/topi314/gobin/gobin/database"
	"github.com/topi314/gobin/internal/flags"
	"github.com/topi314/gobin/internal/httperr"
	"github.com/topi314/gobin/templates"
	"github.com/topi314/tint"
)

var (
	ErrInvalidMultipartPartName   = errors.New("invalid multipart part name")
	ErrInvalidDocumentFileName    = errors.New("invalid document file name")
	ErrInvalidDocumentFileContent = errors.New("invalid document file content")
)

type (
	DocumentResponse struct {
		Key          string         `json:"key"`
		Version      string         `json:"version"`
		VersionLabel string         `json:"version_label,omitempty"`
		VersionTime  string         `json:"version_time,omitempty"`
		Files        []ResponseFile `json:"files"`
		Token        string         `json:"token,omitempty"`
	}

	ResponseFile struct {
		Name      string `json:"name"`
		Content   string `json:"content,omitempty"`
		Formatted string `json:"formatted,omitempty"`
		Language  string `json:"language"`
	}

	RequestFile struct {
		Name     string
		Content  string
		Language string
	}

	ErrorResponse struct {
		Message   string `json:"message"`
		Status    int    `json:"status"`
		Path      string `json:"path"`
		RequestID string `json:"request_id"`
	}

	ShareRequest struct {
		Permissions []string `json:"permissions"`
	}

	ShareResponse struct {
		Token string `json:"token"`
	}
)

func (s *Server) DocumentVersions(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "documentID")
	withContent := r.URL.Query().Get("withContent") == "true"

	versions, err := s.db.GetDocumentVersionsWithFiles(r.Context(), documentID, withContent)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.error(w, r, httperr.NotFound(err))
			return
		}
		s.error(w, r, fmt.Errorf("failed to get document versions: %w", err))
		return
	}

	formatter := getFormatter(r, false)
	style := getStyle(r)

	var response []DocumentResponse
	for version, dbFiles := range versions {
		files := make([]ResponseFile, len(dbFiles))
		for i, file := range dbFiles {
			var formatted string
			if withContent {
				formatted, err = s.formatFile(file, formatter, style)
				if err != nil {
					s.error(w, r, err)
					return
				}
			}

			files[i] = ResponseFile{
				Name:      file.Name,
				Content:   file.Content,
				Formatted: formatted,
				Language:  file.Language,
			}
		}
		response = append(response, DocumentResponse{
			Key:     documentID,
			Version: strconv.FormatInt(version, 10),
			Files:   nil,
		})
	}

	s.ok(w, r, response)
}

func (s *Server) GetPrettyDocument(w http.ResponseWriter, r *http.Request) {
	document, err := s.getDocument(r)
	if err != nil {
		if !errors.Is(err, ErrDocumentNotFound) {
			s.prettyError(w, r, err)
			return
		}
		if r.URL.Path != "/" {
			s.redirectRoot(w, r)
			return
		}
	}

	if document == nil {
		document = &database.Document{
			Files: []database.File{{
				Name:     "untitled",
				Content:  "",
				Language: "auto",
			}},
		}
	}

	versions, err := s.db.GetDocumentVersions(r.Context(), document.ID)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		s.prettyError(w, r, fmt.Errorf("failed to get document versions: %w", err))
		return
	}

	formatter := getFormatter(r, true)
	style := getStyle(r)

	templateFiles := make([]templates.File, len(document.Files))
	for i, file := range document.Files {
		formatted, err := s.formatFile(file, formatter, style)
		if err != nil {
			s.prettyError(w, r, err)
			return
		}
		templateFiles[i] = templates.File{
			Name:      file.Name,
			Content:   file.Content,
			Formatted: formatted,
			Language:  file.Language,
		}
	}

	templateVersions := make([]templates.DocumentVersion, len(versions))
	for i, v := range versions {
		versionTime := time.UnixMilli(v)
		versionLabel := humanize.Time(versionTime)
		if i == 0 {
			versionLabel += " (current)"
		} else if i == len(versions)-1 {
			versionLabel += " (original)"
		}
		templateVersions[i] = templates.DocumentVersion{
			Version: strconv.FormatInt(v, 10),
			Label:   versionLabel,
			Time:    versionTime.Format(VersionTimeFormat),
		}
	}

	if err = templates.Document(templates.DocumentVars{
		ID:      document.ID,
		Version: strconv.FormatInt(document.Version, 10),
		Edit:    document.ID == "",

		Files:    templateFiles,
		Versions: templateVersions,

		Lexers: lexers.Names(false),
		Styles: s.styles,
		Style:  style.Name,
		Theme:  style.Theme,

		Max:     s.cfg.MaxDocumentSize,
		Host:    r.Host,
		Preview: s.cfg.Preview != nil,
	}).Render(r.Context(), w); err != nil {
		slog.ErrorContext(r.Context(), "failed to execute template", tint.Err(err))
	}
}

func (s *Server) GetDocument(w http.ResponseWriter, r *http.Request) {
	document, err := s.getDocument(r)
	if err != nil {
		s.error(w, r, err)
		return
	}

	formatter := getFormatter(r, false)
	style := getStyle(r)

	response := DocumentResponse{
		Key:     document.ID,
		Version: strconv.FormatInt(document.Version, 10),
		Files:   make([]ResponseFile, len(document.Files)),
	}
	for i, file := range document.Files {
		formatted, err := s.formatFile(file, formatter, style)
		if err != nil {
			s.error(w, r, err)
			return
		}
		response.Files[i] = ResponseFile{
			Name:      file.Name,
			Content:   file.Content,
			Formatted: formatted,
			Language:  file.Language,
		}
	}

	s.ok(w, r, response)
}

func (s *Server) GetRawDocument(w http.ResponseWriter, r *http.Request) {
	document, err := s.getDocument(r)
	if err != nil {
		s.error(w, r, err)
		return
	}

	formatter := getFormatter(r, false)
	style := getStyle(r)

	if len(document.Files) == 1 {
		file := document.Files[0]

		w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{
			"name":     file.Name,
			"filename": file.Name,
		}))

		lexer := lexers.Get(file.Language)
		if lexer == nil {
			lexer = lexers.Fallback
		}
		w.Header().Set("Language", lexer.Config().Name)

		formatted, err := s.formatFile(file, formatter, style)
		if err != nil {
			s.error(w, r, fmt.Errorf("failed to render raw document: %w", err))
			return
		}

		var contentType string
		switch formatter {
		case s.htmlFormatter, s.standaloneHTMLFormatter:
			contentType = "text/html; charset=UTF-8"
		case formatters.SVG:
			contentType = "image/svg+xml"
		case formatters.JSON:
			contentType = "application/json"
		default:
			contentType = "application/octet-stream"
			if len(lexer.Config().MimeTypes) > 0 {
				contentType = lexer.Config().MimeTypes[0]
			}
		}

		w.Header().Set("Content-Type", contentType)
		if _, err = w.Write([]byte(formatted)); err != nil {
			s.error(w, r, err)
		}
		return
	}

	mpw := multipart.NewWriter(w)
	for i, file := range document.Files {
		headers := make(textproto.MIMEHeader, 2)
		headers.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{
			"name":     fmt.Sprintf("file-%d", i),
			"filename": file.Name,
		}))

		lexer := lexers.Get(file.Language)
		if lexer == nil {
			lexer = lexers.Fallback
		}
		headers.Set("Language", lexer.Config().Name)

		formatted, err := s.formatFile(file, formatter, style)
		if err != nil {
			s.error(w, r, fmt.Errorf("failed to render raw document: %w", err))
			return
		}

		var contentType string
		switch formatter {
		case s.htmlFormatter, s.standaloneHTMLFormatter:
			contentType = "text/html; charset=UTF-8"
		case formatters.SVG:
			contentType = "image/svg+xml"
		case formatters.JSON:
			contentType = "application/json"
		default:
			contentType = "application/octet-stream"
			if len(lexer.Config().MimeTypes) > 0 {
				contentType = lexer.Config().MimeTypes[0]
			}
		}

		headers.Set("Content-Type", contentType)

		var part io.Writer
		part, err = mpw.CreatePart(headers)
		if err != nil {
			s.error(w, r, err)
			return
		}
		if _, err = part.Write([]byte(formatted + "\n")); err != nil {
			s.error(w, r, err)
			return
		}
	}

	if err = mpw.Close(); err != nil {
		s.error(w, r, err)
		return
	}
}

func (s *Server) GetDocumentPreview(w http.ResponseWriter, r *http.Request) {
	document, err := s.getDocument(r)
	if err != nil {
		s.error(w, r, err)
	}

	formatter := getFormatter(r, true)
	style := getStyle(r)

	file := document.Files[0]
	file.Content = s.shortContent(document.Files[0].Content)

	formatted, err := s.formatFile(file, formatter, style)
	if err != nil {
		s.prettyError(w, r, fmt.Errorf("failed to render document preview: %w", err))
		return
	}

	png, err := s.convertSVG2PNG(r.Context(), formatted)
	if err != nil {
		s.error(w, r, fmt.Errorf("failed to convert document preview: %w", err))
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

func (s *Server) getDocument(r *http.Request) (*database.Document, error) {
	documentID := chi.URLParam(r, "documentID")

	var version int64
	if versionStr := chi.URLParam(r, "version"); versionStr != "" {
		var err error
		version, err = strconv.ParseInt(versionStr, 10, 64)
		if err != nil {
			return nil, httperr.BadRequest(ErrInvalidDocumentVersion)
		}
	}

	var (
		files []database.File
		err   error
	)
	if version == 0 {
		files, err = s.db.GetDocument(r.Context(), documentID)
	} else {
		files, err = s.db.GetDocumentVersion(r.Context(), documentID, version)
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, httperr.NotFound(ErrDocumentNotFound)
		}
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	return &database.Document{
		ID:      documentID,
		Version: version,
		Files:   files,
	}, nil
}

func (s *Server) GetDocumentFile(w http.ResponseWriter, r *http.Request) {
	file, err := s.getDocumentFile(r)
	if err != nil {
		s.error(w, r, err)
		return
	}

	formatter := getFormatter(r, false)
	style := getStyle(r)

	formatted, err := s.formatFile(*file, formatter, style)
	if err != nil {
		s.error(w, r, err)
		return
	}

	s.ok(w, r, ResponseFile{
		Name:      file.Name,
		Content:   file.Content,
		Formatted: formatted,
		Language:  file.Language,
	})
}

func (s *Server) GetRawDocumentFile(w http.ResponseWriter, r *http.Request) {
	file, err := s.getDocumentFile(r)
	if err != nil {
		s.error(w, r, err)
		return
	}

	if _, err := w.Write([]byte(file.Content)); err != nil {
		s.error(w, r, err)
		return
	}
}

func (s *Server) getDocumentFile(r *http.Request) (*database.File, error) {
	documentID := chi.URLParam(r, "documentID")

	versionStr := chi.URLParam(r, "version")
	var version int64
	if versionStr != "" {
		var err error
		version, err = strconv.ParseInt(versionStr, 10, 64)
		if err != nil {
			return nil, httperr.BadRequest(ErrInvalidDocumentVersion)
		}
	}

	fileName := chi.URLParam(r, "fileName")
	if fileName == "" {
		return nil, httperr.NotFound(ErrDocumentFileNotFound)
	}

	var (
		file *database.File
		err  error
	)
	if version == 0 {
		file, err = s.db.GetDocumentFile(r.Context(), documentID, fileName)
	} else {
		file, err = s.db.GetDocumentFileVersion(r.Context(), documentID, version, fileName)
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, httperr.NotFound(ErrDocumentFileNotFound)
		}
		return nil, fmt.Errorf("failed to get document file: %w", err)
	}

	return file, nil
}

func (s *Server) PostDocument(w http.ResponseWriter, r *http.Request) {
	files, err := parseDocumentFiles(r)
	if err != nil {
		s.error(w, r, err)
		return
	}

	var dbFiles []database.File
	for _, file := range files {
		dbFiles = append(dbFiles, database.File{
			Name:     file.Name,
			Content:  file.Content,
			Language: file.Language,
		})
	}

	documentID, version, err := s.db.CreateDocument(r.Context(), dbFiles)
	if err != nil {
		s.error(w, r, fmt.Errorf("failed to create document: %w", err))
		return
	}

	formatter := getFormatter(r, false)
	style := getStyle(r)

	var rsFiles []ResponseFile
	for _, file := range dbFiles {
		formatted, err := s.formatFile(file, formatter, style)
		if err != nil {
			s.error(w, r, err)
			return
		}
		rsFiles = append(rsFiles, ResponseFile{
			Name:      file.Name,
			Content:   file.Content,
			Formatted: formatted,
			Language:  file.Language,
		})
	}

	token, err := s.NewToken(*documentID, AllPermissions)
	if err != nil {
		s.error(w, r, fmt.Errorf("failed to create jwt token: %w", err))
		return
	}

	versionTime := time.UnixMilli(*version)
	s.json(w, r, DocumentResponse{
		Key:          *documentID,
		Version:      strconv.FormatInt(*version, 10),
		VersionLabel: humanize.Time(versionTime) + " (original)",
		VersionTime:  versionTime.Format(VersionTimeFormat),
		Files:        rsFiles,
		Token:        token,
	}, http.StatusCreated)

}

func (s *Server) PatchDocument(w http.ResponseWriter, r *http.Request) {
	files, err := parseDocumentFiles(r)
	if err != nil {
		s.error(w, r, err)
		return
	}

	claims := GetClaims(r)
	if flags.Misses(claims.Permissions, PermissionWrite) {
		s.error(w, r, httperr.Forbidden(ErrPermissionDenied("webhook")))
		return
	}

	documentID := chi.URLParam(r, "documentID")

	var dbFiles []database.File
	for _, file := range files {
		dbFiles = append(dbFiles, database.File{
			Name:     file.Name,
			Content:  file.Content,
			Language: file.Language,
		})
	}

	version, err := s.db.UpdateDocument(r.Context(), documentID, dbFiles)
	if err != nil {
		s.error(w, r, fmt.Errorf("failed to update document: %w", err))
		return
	}

	formatter := getFormatter(r, false)
	style := getStyle(r)

	var rsFiles []ResponseFile
	for _, file := range dbFiles {
		formatted, err := s.formatFile(file, formatter, style)
		if err != nil {
			s.error(w, r, err)
			return
		}
		rsFiles = append(rsFiles, ResponseFile{
			Name:      file.Name,
			Content:   file.Content,
			Formatted: formatted,
			Language:  file.Language,
		})
	}

	webhooksFiles := make([]WebhookDocumentFile, len(files))
	for i, file := range files {
		webhooksFiles[i] = WebhookDocumentFile{
			Name:     file.Name,
			Content:  file.Content,
			Language: file.Language,
		}
	}
	s.ExecuteWebhooks(r.Context(), WebhookEventUpdate, WebhookDocument{
		Key:     documentID,
		Version: *version,
		Files:   webhooksFiles,
	})

	versionTime := time.UnixMilli(*version)
	s.json(w, r, DocumentResponse{
		Key:          documentID,
		Version:      strconv.FormatInt(*version, 10),
		VersionLabel: humanize.Time(versionTime) + " (current)",
		VersionTime:  versionTime.Format(VersionTimeFormat),
		Files:        rsFiles,
	}, http.StatusOK)
}

func (s *Server) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	claims := GetClaims(r)
	if flags.Misses(claims.Permissions, PermissionDelete) {
		s.error(w, r, httperr.Forbidden(ErrPermissionDenied("webhook")))
		return
	}

	documentID := chi.URLParam(r, "documentID")
	var version int64
	if versionStr := chi.URLParam(r, "version"); versionStr != "" {
		var err error
		version, err = strconv.ParseInt(versionStr, 10, 64)
		if err != nil {
			s.error(w, r, httperr.BadRequest(ErrInvalidDocumentVersion))
			return
		}
	}

	var (
		document *database.Document
		err      error
	)
	if version == 0 {
		document, err = s.db.DeleteDocument(r.Context(), documentID)
	} else {
		document, err = s.db.DeleteDocumentVersion(r.Context(), documentID, version)
	}
	if err != nil {
		s.error(w, r, fmt.Errorf("failed to delete document: %w", err))
		return
	}

	webhooksFiles := make([]WebhookDocumentFile, len(document.Files))
	for i, file := range document.Files {
		webhooksFiles[i] = WebhookDocumentFile{
			Name:     file.Name,
			Content:  file.Content,
			Language: file.Language,
		}
	}
	s.ExecuteWebhooks(r.Context(), WebhookEventDelete, WebhookDocument{
		Key:     document.ID,
		Version: document.Version,
		Files:   webhooksFiles,
	})

	s.ok(w, r, nil)
}

func (s *Server) PostDocumentShare(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "documentID")

	var shareRequest ShareRequest
	if err := json.NewDecoder(r.Body).Decode(&shareRequest); err != nil {
		s.error(w, r, httperr.BadRequest(err))
		return
	}

	if len(shareRequest.Permissions) == 0 {
		s.error(w, r, httperr.BadRequest(ErrNoPermissions))
		return
	}

	for _, permission := range shareRequest.Permissions {
		if !slices.Contains(AllStringPermissions, permission) {
			s.error(w, r, httperr.BadRequest(ErrUnknownPermission(permission)))
			return
		}
	}

	claims := GetClaims(r)
	if claims.Subject != documentID || flags.Misses(claims.Permissions, PermissionShare) {
		s.error(w, r, httperr.Forbidden(ErrPermissionDenied("share")))
		return
	}

	perms, err := parsePermissions(claims.Permissions, shareRequest.Permissions)
	if err != nil {
		s.error(w, r, httperr.Forbidden(err))
		return
	}

	token, err := s.NewToken(documentID, perms)
	if err != nil {
		s.error(w, r, fmt.Errorf("failed to create new token: %w", err))
		return
	}

	s.ok(w, r, ShareResponse{Token: token})
}

func parseDocumentFiles(r *http.Request) ([]RequestFile, error) {
	var files []RequestFile
	contentType := r.Header.Get("Content-Type")
	params := make(map[string]string)
	if contentType != "" {
		var err error
		contentType, params, err = mime.ParseMediaType(contentType)
		if err != nil {
			return nil, fmt.Errorf("failed to parse content type: %w", err)
		}
	}

	if contentType == "multipart/form-data" {
		mr, err := r.MultipartReader()
		if err != nil {
			return nil, fmt.Errorf("failed to get multipart reader: %w", err)
		}
		for i := 0; ; i++ {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("failed to get multipart part: %w", err)
			}

			if part.FormName() != fmt.Sprintf("file-%d", i) {
				return nil, httperr.BadRequest(ErrInvalidMultipartPartName)
			}

			if part.FileName() == "" {
				return nil, httperr.BadRequest(ErrInvalidDocumentFileName)
			}

			data, err := io.ReadAll(part)
			if err != nil {
				return nil, fmt.Errorf("failed to read part data: %w", err)
			}

			if len(data) == 0 {
				return nil, httperr.BadRequest(ErrInvalidDocumentFileContent)
			}

			partContentType := part.Header.Get("Content-Type")
			if partContentType != "" {
				partContentType, _, _ = mime.ParseMediaType(partContentType)
			}

			files = append(files, RequestFile{
				Name:     part.FileName(),
				Content:  string(data),
				Language: getLanguage(part.Header.Get("Language"), partContentType, part.FileName(), string(data)),
			})
		}

	} else {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}

		name := params["filename"]
		if name == "" {
			name = "untitled"
		}

		files = []RequestFile{{
			Name:     name,
			Content:  string(data),
			Language: getLanguage(r.Header.Get("Language"), contentType, params["filename"], string(data)),
		}}
	}
	return files, nil
}

func getLanguage(language string, contentType string, fileName string, content string) string {
	var lexer chroma.Lexer
	if language != "" {
		lexer = lexers.Get(language)
	}
	if lexer != nil {
		return lexer.Config().Name
	}

	if contentType != "" {
		lexer = lexers.MatchMimeType(contentType)
	}
	if lexer != nil {
		return lexer.Config().Name
	}

	if contentType != "" {
		lexer = lexers.Get(contentType)
	}
	if lexer != nil {
		return lexer.Config().Name
	}

	if fileName != "" {
		lexer = lexers.Match(fileName)
	}
	if lexer != nil {
		return lexer.Config().Name
	}

	if len(content) > 0 {
		lexer = lexers.Analyse(content)
	}
	if lexer != nil {
		return lexer.Config().Name
	}

	return "plaintext"
}
