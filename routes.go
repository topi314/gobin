package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
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

type Variables struct {
	ID        string
	Content   string
	Language  string
	CreatedAt time.Time
	UpdatedAt time.Time

	Host   string
	Styles []Style
}

type DocumentResponse struct {
	Key         string `json:"key"`
	Data        string `json:"data"`
	Language    string `json:"language"`
	UpdateToken string `json:"update_token,omitempty"`
}

type ErrorResponse struct {
	Message   string `json:"message"`
	Status    int    `json:"status"`
	Path      string `json:"path"`
	RequestID string `json:"request_id"`
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(middleware.Compress(5))
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Heartbeat("/ping"))

	if s.cfg.Debug {
		r.Mount("/debug", middleware.Profiler())
	}

	r.Mount("/assets", s.Assets())

	r.Get("/raw/{documentID}", s.GetRawDocument)
	r.Head("/raw/{documentID}", s.GetRawDocument)

	r.Get("/documents/{documentID}", s.GetDocument)
	r.Group(func(r chi.Router) {
		if s.cfg.RateLimit != nil && s.cfg.RateLimit.Requests > 0 && s.cfg.RateLimit.Duration > 0 {
			r.Use(httprate.Limit(s.cfg.RateLimit.Requests, s.cfg.RateLimit.Duration, httprate.WithLimitHandler(s.RateLimit), httprate.WithKeyFuncs(httprate.KeyByIP, httprate.KeyByEndpoint)))
		}

		r.Post("/documents", s.PostDocument)
		r.Patch("/documents/{documentID}", s.PatchDocument)
		r.Delete("/documents/{documentID}", s.DeleteDocument)
	})

	r.Get("/{documentID}", s.GetPrettyDocument)
	r.Head("/{documentID}", s.GetPrettyDocument)

	r.Get("/", s.GetPrettyDocument)
	r.Head("/", s.GetPrettyDocument)

	r.NotFound(s.Redirect)

	return r
}

func (s *Server) Assets() http.Handler {
	if s.cfg.DevMode {
		return http.FileServer(http.Dir("."))
	}
	return http.FileServer(http.FS(assets))
}

func (s *Server) GetPrettyDocument(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "documentID")
	var document Document
	if documentID != "" {
		var err error
		document, err = s.db.GetDocument(r.Context(), documentID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				s.Redirect(w, r)
				return
			}
			s.PrettyError(w, r, err, http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}

	vars := Variables{
		ID:        document.ID,
		Content:   document.Content,
		Language:  document.Language,
		CreatedAt: document.CreatedAt,
		UpdatedAt: document.UpdatedAt,
		Host:      r.Host,
		Styles:    Styles,
	}
	if err := s.tmpl(w, "document.gohtml", vars); err != nil {
		log.Println("Error while executing template:", err)
	}
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

	s.JSON(w, r, DocumentResponse{
		Key:      document.ID,
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
		log.Println("Error while creating document:", err)
		s.Error(w, r, err, http.StatusInternalServerError)
		return
	}

	s.JSON(w, r, DocumentResponse{
		Key:         document.ID,
		Data:        document.Content,
		Language:    document.Language,
		UpdateToken: document.UpdateToken,
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
			s.Error(w, r, ErrDocumentNotFound, http.StatusNotFound)
			return
		}
		s.Error(w, r, err, http.StatusInternalServerError)
		return
	}

	s.JSON(w, r, DocumentResponse{
		Key:         document.ID,
		Data:        document.Content,
		Language:    document.Language,
		UpdateToken: document.UpdateToken,
	})
}

func (s *Server) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "documentID")
	updateToken := r.Header.Get("Authorization")

	if err := s.db.DeleteDocument(r.Context(), documentID, updateToken); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.Error(w, r, ErrDocumentNotFound, http.StatusNotFound)
			return
		}
		s.Error(w, r, err, http.StatusInternalServerError)
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
			s.Error(w, r, ErrDocumentNotFound, http.StatusNotFound)
			return nil
		}
		s.Error(w, r, err, http.StatusInternalServerError)
		return nil
	}
	return &document
}

func (s *Server) getUpdateToken(w http.ResponseWriter, r *http.Request) string {
	updateToken := r.Header.Get("Authorization")
	if updateToken == "" {
		s.Unauthorized(w, r)
		return ""
	}
	return updateToken
}

func (s *Server) readBody(w http.ResponseWriter, r *http.Request) string {
	content, err := io.ReadAll(r.Body)
	if err != nil {
		s.Error(w, r, err, http.StatusInternalServerError)
		return ""
	}

	if len(content) == 0 {
		s.Error(w, r, ErrEmptyBody, http.StatusBadRequest)
		return ""
	}
	return string(content)
}

func (s *Server) Redirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) Unauthorized(w http.ResponseWriter, r *http.Request) {
	s.Error(w, r, ErrUnauthorized, http.StatusUnauthorized)
}

func (s *Server) RateLimit(w http.ResponseWriter, r *http.Request) {
	s.Error(w, r, ErrRateLimit, http.StatusTooManyRequests)
}

func (s *Server) Log(r *http.Request, logType string, err error) {
	log.Printf("Error while handling %s(%s) %s: %s\n", logType, middleware.GetReqID(r.Context()), r.RequestURI, err)
}

func (s *Server) PrettyError(w http.ResponseWriter, r *http.Request, err error, status int) {
	s.Log(r, "pretty request", err)
	w.WriteHeader(status)

	vars := map[string]any{
		"Error":     err.Error(),
		"Status":    status,
		"RequestID": middleware.GetReqID(r.Context()),
		"Path":      r.URL.Path,
	}
	if tmplErr := s.tmpl(w, "error.gohtml", vars); tmplErr != nil {
		s.Log(r, "template", tmplErr)
	}
}

func (s *Server) Error(w http.ResponseWriter, r *http.Request, err error, status int) {
	s.Log(r, "request", err)
	s.json(w, r, ErrorResponse{
		Message:   err.Error(),
		Status:    status,
		Path:      r.URL.Path,
		RequestID: middleware.GetReqID(r.Context()),
	}, status)
}

func (s *Server) JSON(w http.ResponseWriter, r *http.Request, v any) {
	s.json(w, r, v, http.StatusOK)
}

func (s *Server) json(w http.ResponseWriter, r *http.Request, v any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}

	if err := json.NewEncoder(w).Encode(v); err != nil {
		s.Log(r, "json", err)
	}
}

func (s *Server) exceedsMaxDocumentSize(w http.ResponseWriter, r *http.Request, content string) bool {
	if s.cfg.MaxDocumentSize > 0 && len([]rune(content)) > s.cfg.MaxDocumentSize {
		s.Error(w, r, ErrContentTooLarge(s.cfg.MaxDocumentSize), http.StatusBadRequest)
		return true
	}
	return false
}
