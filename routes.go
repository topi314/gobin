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

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Mount("/assets", s.Assets())
	r.Delete("/documents/{documentID}", s.DeleteDocument)
	r.Patch("/documents/{documentID}", s.PatchDocument)
	r.Post("/documents", s.PostDocument)
	r.Get("/raw/{documentID}", s.GetRawDocument)
	r.Get("/{documentID}", s.GetDocument)
	r.Get("/", s.GetDocument)
	r.NotFound(s.Redirect)

	return r
}

func (s *Server) Assets() http.Handler {
	if s.devMode {
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

type Variables struct {
	ID        string
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time

	Host   string
	Styles []Style
}

func (s *Server) GetDocument(w http.ResponseWriter, r *http.Request) {
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

type DocumentResponse struct {
	Key         string `json:"key"`
	UpdateToken string `json:"update_token"`
}

func (s *Server) PostDocument(w http.ResponseWriter, r *http.Request) {
	content, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(content) == 0 {
		http.Error(w, "empty document", http.StatusBadRequest)
		return
	}

	document, err := s.db.CreateDocument(r.Context(), string(content))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.JSON(w, r, DocumentResponse{
		Key:         document.ID,
		UpdateToken: document.UpdateToken,
	})
}

func (s *Server) PatchDocument(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "documentID")
	updateToken := r.Header.Get("Authorization")

	content, err := io.ReadAll(r.Body)
	if err != nil {
		s.Error(w, r, err)
		return
	}

	document, err := s.db.UpdateDocument(r.Context(), documentID, updateToken, string(content))
	if err != nil {
		s.Error(w, r, err)
		return
	}

	s.JSON(w, r, DocumentResponse{
		Key:         document.ID,
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
}

func (s *Server) Redirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) Unauthorized(w http.ResponseWriter) {
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
}

func (s *Server) Error(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("Error while handling request %s: %s", r.URL, err)
	_ = s.tmpl(w, "error.gohtml", err.Error())
}

func (s *Server) JSON(w http.ResponseWriter, r *http.Request, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		s.Error(w, r, err)
	}
}
