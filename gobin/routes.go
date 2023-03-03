package gobin

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
)

var (
	ErrDocumentNotFound = errors.New("document not found")
	ErrUnauthorized     = errors.New("unauthorized")
	ErrRateLimit        = errors.New("rate limit exceeded")
	ErrEmptyBody        = errors.New("empty request body")
	ErrContentTooLarge  = func(maxLength int) error {
		return fmt.Errorf("content too large, must be less than %d chars", maxLength)
	}
)

type (
	TemplateVariables struct {
		ID       string
		Version  int64
		Content  string
		Language string

		Versions []DocumentVersion

		Host   string
		Styles []Style
	}
	DocumentVersion struct {
		Version int64
		Label   string
		Time    string
	}
	DocumentResponse struct {
		Key          string `json:"key,omitempty"`
		Version      int64  `json:"version"`
		VersionLabel string `json:"version_label,omitempty"`
		VersionTime  string `json:"version_time,omitempty"`
		Data         string `json:"data,omitempty"`
		Language     string `json:"language"`
		UpdateToken  string `json:"update_token,omitempty"`
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
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(middleware.Compress(5))
	r.Use(middleware.Maybe(
		middleware.Logger,
		func(r *http.Request) bool {
			// Don't log requests for assets
			return !strings.HasPrefix(r.URL.Path, "/assets")
		},
	))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Heartbeat("/ping"))

	if s.cfg.RateLimit != nil && s.cfg.RateLimit.Requests > 0 && s.cfg.RateLimit.Duration > 0 {
		rateLimiter := httprate.NewRateLimiter(
			s.cfg.RateLimit.Requests,
			s.cfg.RateLimit.Duration,
			httprate.WithLimitHandler(s.rateLimit),
			httprate.WithKeyFuncs(
				httprate.KeyByIP,
				httprate.KeyByEndpoint,
			),
		)
		r.Use(middleware.Maybe(
			rateLimiter.Handler,
			func(r *http.Request) bool {
				// Only apply rate limiting to POST, PATCH, and DELETE requests
				return r.Method == http.MethodPost || r.Method == http.MethodPatch || r.Method == http.MethodDelete
			},
		))
	}

	if s.cfg.Debug {
		r.Mount("/debug", middleware.Profiler())
	}

	r.Mount("/assets", http.FileServer(s.assets))
	r.Group(func(r chi.Router) {
		r.Route("/raw/{documentID}", func(r chi.Router) {
			r.Get("/", s.GetRawDocument)
			r.Head("/", s.GetRawDocument)
			r.Route("/versions/{version}", func(r chi.Router) {
				r.Get("/", s.GetRawDocumentVersion)
				r.Head("/", s.GetRawDocumentVersion)
			})
		})
		r.Route("/documents", func(r chi.Router) {
			r.Post("/", s.PostDocument)
			r.Route("/{documentID}", func(r chi.Router) {
				r.Get("/", s.GetDocument)
				r.Patch("/", s.PatchDocument)
				r.Delete("/", s.DeleteDocument)
				r.Route("/versions", func(r chi.Router) {
					r.Get("/", s.DocumentVersions)
					r.Route("/{version}", func(r chi.Router) {
						r.Get("/", s.GetDocumentVersion)
						r.Delete("/", s.DeleteDocumentVersion)
					})
				})
			})
		})
		r.Get("/{documentID}", s.GetPrettyDocument)
		r.Head("/{documentID}", s.GetPrettyDocument)
		r.Get("/", s.GetPrettyDocument)
		r.Head("/", s.GetPrettyDocument)
	})
	r.NotFound(s.redirectRoot)

	return r
}

func (s *Server) DocumentVersions(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "documentID")
	withContent := r.URL.Query().Get("withData") == "true"

	versions, err := s.db.GetDocumentVersions(r.Context(), documentID, withContent)
	if err != nil {
		s.log(r, "get document versions", err)
		s.error(w, r, err, http.StatusInternalServerError)
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
	documentID, version := parseDocumentVersion(r, s, w)
	if documentID == "" {
		return
	}

	document, err := s.db.GetDocumentVersion(r.Context(), documentID, version)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.error(w, r, ErrDocumentNotFound, http.StatusNotFound)
			return
		}
		s.log(r, "get document version", err)
		s.error(w, r, err, http.StatusInternalServerError)
		return
	}

	s.ok(w, r, DocumentResponse{
		Key:      document.ID,
		Version:  document.Version,
		Data:     document.Content,
		Language: document.Language,
	})
}

func (s *Server) DeleteDocumentVersion(w http.ResponseWriter, r *http.Request) {
	documentID, version := parseDocumentVersion(r, s, w)
	if documentID == "" {
		return
	}

	if err := s.db.DeleteDocumentByVersion(r.Context(), version, documentID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.error(w, r, ErrDocumentNotFound, http.StatusNotFound)
			return
		}
		s.log(r, "delete document version", err)
		s.error(w, r, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) GetRawDocumentVersion(w http.ResponseWriter, r *http.Request) {
	documentID, version := parseDocumentVersion(r, s, w)
	if documentID == "" {
		return
	}
	document, err := s.db.GetDocumentVersion(r.Context(), documentID, version)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.error(w, r, ErrDocumentNotFound, http.StatusNotFound)
			return
		}
		s.log(r, "get document version", err)
		s.error(w, r, err, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	if r.Method == http.MethodHead {
		w.Header().Set("Content-Length", strconv.Itoa(len([]byte(document.Content))))
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = w.Write([]byte(document.Content))
}

func parseDocumentVersion(r *http.Request, s *Server, w http.ResponseWriter) (string, int64) {
	documentID := chi.URLParam(r, "documentID")
	version := chi.URLParam(r, "version")
	if documentID == "" || version == "" {
		s.error(w, r, ErrDocumentNotFound, http.StatusNotFound)
		return "", -1
	}

	int64Version, err := strconv.ParseInt(version, 10, 64)
	if err != nil {
		s.error(w, r, ErrDocumentNotFound, http.StatusNotFound)
		return "", -1
	}
	return documentID, int64Version
}

func (s *Server) GetPrettyDocument(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "documentID")

	var (
		document  Document
		documents []Document
		err       error
	)
	if documentID != "" {
		document, err = s.db.GetDocument(r.Context(), documentID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				s.redirectRoot(w, r)
				return
			}
			s.log(r, "get pretty document", err)
			s.prettyError(w, r, err, http.StatusInternalServerError)
			return
		}

		documents, err = s.db.GetDocumentVersions(r.Context(), documentID, false)
		if err != nil {
			s.log(r, "get pretty document versions", err)
			s.prettyError(w, r, err, http.StatusInternalServerError)
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
		label, timeStr := formatVersion(now, documentVersion.Version)
		versions = append(versions, DocumentVersion{
			Version: documentVersion.Version,
			Label:   label,
			Time:    timeStr,
		})
	}

	vars := TemplateVariables{
		ID:       document.ID,
		Version:  document.Version,
		Content:  document.Content,
		Language: document.Language,
		Versions: versions,
		Host:     r.Host,
		Styles:   Styles,
	}
	if err = s.tmpl(w, "document.gohtml", vars); err != nil {
		log.Println("error while executing template:", err)
	}
}

func formatVersion(now time.Time, versionRaw int64) (string, string) {
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

	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	if r.Method == http.MethodHead {
		w.Header().Set("Content-Length", strconv.Itoa(len([]byte(document.Content))))
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = w.Write([]byte(document.Content))
}

func (s *Server) GetDocument(w http.ResponseWriter, r *http.Request) {
	document := s.getDocument(w, r)
	if document == nil {
		return
	}

	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}

	s.ok(w, r, DocumentResponse{
		Key:      document.ID,
		Version:  document.Version,
		Data:     document.Content,
		Language: document.Language,
	})
}

func (s *Server) PostDocument(w http.ResponseWriter, r *http.Request) {
	language := r.Header.Get("Language")
	content := s.readBody(w, r)
	if content == "" {
		return
	}

	if s.exceedsMaxDocumentSize(w, r, content) {
		return
	}

	document, err := s.db.CreateDocument(r.Context(), content, language)
	if err != nil {
		s.log(r, "creating document", err)
		s.error(w, r, err, http.StatusInternalServerError)
		return
	}

	versionLabel, versionTime := formatVersion(time.Now(), document.Version)
	s.ok(w, r, DocumentResponse{
		Key:          document.ID,
		Version:      document.Version,
		VersionLabel: versionLabel,
		VersionTime:  versionTime,
		UpdateToken:  document.UpdateToken,
	})
}

func (s *Server) PatchDocument(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "documentID")
	language := r.Header.Get("Language")

	updateToken := s.getUpdateToken(w, r)
	if updateToken == "" {
		return
	}

	content := s.readBody(w, r)
	if content == "" {
		return
	}

	if s.exceedsMaxDocumentSize(w, r, content) {
		return
	}

	document, err := s.db.UpdateDocument(r.Context(), documentID, updateToken, content, language)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.error(w, r, ErrDocumentNotFound, http.StatusNotFound)
			return
		}
		s.error(w, r, err, http.StatusInternalServerError)
		return
	}

	versionLabel, versionTime := formatVersion(time.Now(), document.Version)
	s.ok(w, r, DocumentResponse{
		Key:          document.ID,
		Version:      document.Version,
		VersionLabel: versionLabel,
		VersionTime:  versionTime,
	})
}

func (s *Server) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "documentID")
	updateToken := r.Header.Get("Authorization")

	if err := s.db.DeleteDocument(r.Context(), documentID, updateToken); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.error(w, r, ErrDocumentNotFound, http.StatusNotFound)
			return
		}
		s.error(w, r, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getDocument(w http.ResponseWriter, r *http.Request) *Document {
	documentID := chi.URLParam(r, "documentID")
	if documentID == "" {
		return &Document{}
	}

	document, err := s.db.GetDocument(r.Context(), documentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.error(w, r, ErrDocumentNotFound, http.StatusNotFound)
			return nil
		}
		s.error(w, r, err, http.StatusInternalServerError)
		return nil
	}
	return &document
}

func (s *Server) getUpdateToken(w http.ResponseWriter, r *http.Request) string {
	updateToken := r.Header.Get("Authorization")
	if updateToken == "" {
		s.unauthorized(w, r)
		return ""
	}
	return updateToken
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

func (s *Server) unauthorized(w http.ResponseWriter, r *http.Request) {
	s.error(w, r, ErrUnauthorized, http.StatusUnauthorized)
}

func (s *Server) rateLimit(w http.ResponseWriter, r *http.Request) {
	s.error(w, r, ErrRateLimit, http.StatusTooManyRequests)
}

func (s *Server) log(r *http.Request, logType string, err error) {
	log.Printf("Error while handling %s(%s) %s: %s\n", logType, middleware.GetReqID(r.Context()), r.RequestURI, err)
}

func (s *Server) prettyError(w http.ResponseWriter, r *http.Request, err error, status int) {
	s.log(r, "pretty request", err)
	w.WriteHeader(status)

	vars := map[string]any{
		"Error":     err.Error(),
		"Status":    status,
		"RequestID": middleware.GetReqID(r.Context()),
		"Path":      r.URL.Path,
	}
	if tmplErr := s.tmpl(w, "error.gohtml", vars); tmplErr != nil {
		s.log(r, "template", tmplErr)
	}
}

func (s *Server) error(w http.ResponseWriter, r *http.Request, err error, status int) {
	s.log(r, "request", err)
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

	if err := json.NewEncoder(w).Encode(v); err != nil {
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
