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
	r.Patch("/documents/{documentID}", s.PatchDocument)
	r.Delete("/documents/{documentID}", s.DeleteDocument)
	r.Get("/documents/{documentID}", s.GetDocument)
	r.Head("/documents/{documentID}", s.GetDocument)

	r.Get("/{documentID}", s.GetPrettyDocument)
	r.Get("/", s.GetPrettyDocument)
	r.NotFound(s.Redirect)

	return r
}

func (s *Server) Assets() http.Handler {
	if s.cfg.DevMode {
		return http.FileServer(http.Dir("."))
	}
	return http.FileServer(http.FS(assets))
}

func (s *Server) getDocument(r *http.Request) (Document, error) {
	documentID := chi.URLParam(r, "documentID")
	if documentID == "" {
		return Document{}, nil
	}

	return s.db.GetDocument(r.Context(), documentID)
}

func (s *Server) GetPrettyDocument(w http.ResponseWriter, r *http.Request) {
	document, err := s.getDocument(r)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.Redirect(w, r)
			return
		}
		s.Error(w, r, err)
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

	if err = s.tmpl(w, "document.gohtml", vars); err != nil {
		log.Printf("Error while executing template: %s", err)
		s.Error(w, r, err)
	}
}

func (s *Server) GetRawDocument(w http.ResponseWriter, r *http.Request) {
	document, err := s.getDocument(r)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "document not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte(document.Content))
}

func (s *Server) GetDocument(w http.ResponseWriter, r *http.Request) {
	document, err := s.getDocument(r)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "document not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

	content, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(content) == 0 {
		s.JSONError(w, r, errors.New("empty document"), http.StatusBadRequest)
		return
	}

	document, err := s.db.CreateDocument(r.Context(), string(content), language)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
	updateToken := r.Header.Get("Authorization")
	language := r.Header.Get("Language")

	if updateToken == "" {
		s.JSONError(w, r, errors.New("missing Authorization header"), http.StatusUnauthorized)
		return
	}

	content, err := io.ReadAll(r.Body)
	if err != nil {
		s.JSONError(w, r, err, http.StatusInternalServerError)
		return
	}

	document, err := s.db.UpdateDocument(r.Context(), documentID, updateToken, string(content), language)
	if err != nil {
		s.JSONError(w, r, err, http.StatusInternalServerError)
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
		s.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) Redirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) Unauthorized(w http.ResponseWriter) {
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
}

func (s *Server) Error(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("Error while handling request %s: %s", r.URL, err)
	if tmplErr := s.tmpl(w, "error.gohtml", err.Error()); tmplErr != nil {
		log.Println("Error while executing template:", tmplErr)
	}
}

func (s *Server) JSONError(w http.ResponseWriter, r *http.Request, err error, status int) {
	w.WriteHeader(status)
	s.JSON(w, r, ErrorResponse{
		Message: err.Error(),
	})
}

func (s *Server) JSON(w http.ResponseWriter, r *http.Request, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("Error while encoding JSON %s: %s", r.URL, err)
	}
}
