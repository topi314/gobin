package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var (
	ErrDocumentNotFound = errors.New("document not found")
	ErrUnauthorized     = errors.New("unauthorized")
	ErrEmptyBody        = errors.New("empty request body")
	ErrContentTooLarge  = errors.New("content too large")
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
	Message string `json:"message"`
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Mount("/assets", s.Assets())
	r.Get("/raw/{documentID}", s.GetRawDocument)
	r.Head("/raw/{documentID}", s.GetRawDocument)

	r.Post("/documents", s.PostDocument)
	r.Head("/documents/{documentID}", s.GetDocument)
	r.Get("/documents/{documentID}", s.GetDocument)
	r.Patch("/documents/{documentID}", s.PatchDocument)
	r.Delete("/documents/{documentID}", s.DeleteDocument)

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
	w.WriteHeader(http.StatusOK)
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
	if content == nil {
		return
	}

	if s.cfg.MaxContentLength > 0 && len([]rune(string(content))) > s.cfg.MaxContentLength {
		s.Error(w, r, ErrContentTooLarge, http.StatusBadRequest)
		return
	}

	document, err := s.db.CreateDocument(r.Context(), string(content), language)
	if err != nil {
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
	if content == nil {
		return
	}

	document, err := s.db.UpdateDocument(r.Context(), documentID, updateToken, string(content), language)
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

func (s *Server) readBody(w http.ResponseWriter, r *http.Request) []byte {
	content, err := io.ReadAll(r.Body)
	if err != nil {
		s.Error(w, r, err, http.StatusInternalServerError)
		return nil
	}

	if len(content) == 0 {
		s.Error(w, r, ErrEmptyBody, http.StatusBadRequest)
		return nil
	}
	return content
}

func (s *Server) Redirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) Unauthorized(w http.ResponseWriter, r *http.Request) {
	s.Error(w, r, errors.New("unauthorized"), http.StatusUnauthorized)
}

func (s *Server) PrettyError(w http.ResponseWriter, r *http.Request, err error, status int) {
	log.Printf("Error while handling request %s: %s\n", r.URL, err)
	w.WriteHeader(status)
	if tmplErr := s.tmpl(w, "error.gohtml", err.Error()); tmplErr != nil {
		log.Println("Error while executing template:", tmplErr)
	}
}

func (s *Server) Error(w http.ResponseWriter, r *http.Request, err error, status int) {
	log.Printf("Error while handling request %s: %s\n", r.URL, err)
	s.json(w, r, ErrorResponse{
		Message: err.Error(),
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
		log.Printf("Error while encoding JSON %s: %s\n", r.URL, err)
	}
}
