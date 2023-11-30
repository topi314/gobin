package gobin

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/dustin/go-humanize"
	"github.com/go-chi/chi/v5"
	"github.com/topi314/gobin/gobin/database"
	"github.com/topi314/gobin/internal/httperr"
	"github.com/topi314/gobin/templates"
	"github.com/topi314/tint"
)

type (
	DocumentResponse struct {
		Key     string         `json:"key"`
		Version int64          `json:"version"`
		Files   []ResponseFile `json:"files"`
		Token   string         `json:"token,omitempty"`
	}

	ResponseFile struct {
		Name      string `json:"name"`
		Content   string `json:"content"`
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
)

func (s *Server) GetPrettyDocument(w http.ResponseWriter, r *http.Request) {
	documentID, version, files, err := s.getDocument(r)
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

	if len(files) == 0 {
		files = []database.File{{
			Name:     "untitled",
			Content:  "",
			Language: "plaintext",
		}}
	}

	versions, err := s.db.GetDocumentVersions(r.Context(), documentID)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		s.prettyError(w, r, fmt.Errorf("failed to get document versions: %w", err))
		return
	}

	formatter := s.getFormatter(r)
	style := getStyle(r)

	templateFiles := make([]templates.File, len(files))
	for i, file := range files {
		formatted, err := s.formatFile(files[i], formatter, style)
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
		versionTime := time.Unix(v, 0)
		versionLabel := humanize.Time(versionTime)
		if i == 0 {
			versionLabel += " (current)"
		} else if i == len(versions)-1 {
			versionLabel += " (original)"
		}
		templateVersions[i] = templates.DocumentVersion{
			Version: v,
			Label:   versionLabel,
			Time:    versionTime.Format(VersionTimeFormat),
		}
	}

	vars := templates.DocumentVars{
		ID:      documentID,
		Version: version,
		Edit:    documentID == "",

		Files:    templateFiles,
		Versions: templateVersions,

		Lexers: lexers.Names(false),
		Styles: s.styles,
		Style:  style.Name,
		Theme:  style.Theme,

		Max:     s.cfg.MaxDocumentSize,
		Host:    r.Host,
		Preview: s.cfg.Preview != nil,
	}
	if err = templates.Document(vars).Render(r.Context(), w); err != nil {
		slog.ErrorContext(r.Context(), "failed to execute template", tint.Err(err))
	}
}

func (s *Server) GetDocument(w http.ResponseWriter, r *http.Request) {
	documentID, version, files, err := s.getDocument(r)
	if err != nil {
		s.error(w, r, err)
		return
	}

	formatter := s.getFormatter(r)
	style := getStyle(r)

	response := DocumentResponse{
		Key:     documentID,
		Version: version,
		Files:   make([]ResponseFile, len(files)),
	}
	for i, file := range files {
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

func (s *Server) getDocument(r *http.Request) (string, int64, []database.File, error) {
	documentID := chi.URLParam(r, "documentID")

	versionStr := chi.URLParam(r, "version")
	var version int64
	if versionStr != "" {
		var err error
		version, err = strconv.ParseInt(versionStr, 10, 64)
		if err != nil {
			return "", 0, nil, httperr.BadRequest(ErrInvalidDocumentVersion)
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
			return "", 0, nil, httperr.NotFound(ErrDocumentNotFound)
		}
		return "", 0, nil, fmt.Errorf("failed to get document: %w", err)
	}

	return documentID, version, files, nil
}

func (s *Server) GetDocumentFile(w http.ResponseWriter, r *http.Request) {
	file, err := s.getDocumentFile(r)
	if err != nil {
		s.error(w, r, err)
		return
	}

	formatter := s.getFormatter(r)
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

	documentID, err := s.db.CreateDocument(r.Context(), dbFiles)
	if err != nil {
		s.error(w, r, fmt.Errorf("failed to create document: %w", err))
		return
	}

	var rsFiles []ResponseFile
	for _, file := range dbFiles {
		rsFiles = append(rsFiles, ResponseFile{
			Name:     file.Name,
			Content:  file.Content,
			Language: file.Language,
		})
	}

	s.json(w, r, DocumentResponse{
		Key:     *documentID,
		Version: 0,
		Files:   rsFiles,
		Token:   "todo",
	}, http.StatusCreated)

}

func (s *Server) PatchDocument(w http.ResponseWriter, r *http.Request) {
	files, err := parseDocumentFiles(r)
	if err != nil {
		s.error(w, r, err)
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

	if err = s.db.UpdateDocument(r.Context(), documentID, dbFiles); err != nil {
		s.error(w, r, fmt.Errorf("failed to update document: %w", err))
		return
	}

	var rsFiles []ResponseFile
	for _, file := range dbFiles {
		rsFiles = append(rsFiles, ResponseFile{
			Name:     file.Name,
			Content:  file.Content,
			Language: file.Language,
		})
	}

	s.json(w, r, DocumentResponse{
		Key:     documentID,
		Version: 0,
		Files:   rsFiles,
		Token:   "todo",
	}, http.StatusOK)
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
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("failed to get multipart part: %w", err)
			}

			data, err := io.ReadAll(part)
			if err != nil {
				return nil, fmt.Errorf("failed to read part data: %w", err)
			}

			language := part.Header.Get("Language")
			if language == "" {
				partContentType := part.Header.Get("Content-Type")
				if partContentType != "" {
					partContentType, _, _ = mime.ParseMediaType(partContentType)
				}
				language = getLanguage(partContentType, part.FileName(), string(data))
			}
			files = append(files, RequestFile{
				Name:     part.FileName(),
				Content:  string(data),
				Language: language,
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
			Language: getLanguage(contentType, params["filename"], string(data)),
		}}
	}
	return files, nil
}

func getLanguage(contentType string, fileName string, content string) string {
	fmt.Printf("contentType: %s, fileName: %s, content: %s\n", contentType, fileName, content)
	var lexer chroma.Lexer
	if contentType != "" {
		lexer = lexers.MatchMimeType(contentType)
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

func (s *Server) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "documentID")

	if err := s.db.DeleteDocument(r.Context(), documentID); err != nil {
		s.error(w, r, fmt.Errorf("failed to delete document: %w", err))
		return
	}

	s.ok(w, r, nil)
}
