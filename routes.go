package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/exp/slices"
)

var (
	ErrDocumentNotFound = errors.New("document not found")
	ErrUnauthorized     = errors.New("unauthorized")
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
	Key      string `json:"key"`
	Data     string `json:"data"`
	Language string `json:"language"`
	Token    string `json:"token,omitempty"`
}

type ShareRequest struct {
	Permissions []Permission `json:"permissions"`
}

type ShareResponse struct {
	Token string `json:"token"`
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
	r.Use(s.JWTMiddleware)

	r.Mount("/assets", s.Assets())
	r.Get("/raw/{documentID}", s.GetRawDocument)
	r.Head("/raw/{documentID}", s.GetRawDocument)

	r.Post("/documents", s.PostDocument)
	r.Get("/documents/{documentID}", s.GetDocument)
	r.Patch("/documents/{documentID}", s.PatchDocument)
	r.Delete("/documents/{documentID}", s.DeleteDocument)

	r.Post("/documents/{documentID}/share", s.PostDocumentShare)

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
	document, ok := s.getDocument(w, r)
	if !ok {
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}

	if _, err := w.Write([]byte(document.Content)); err != nil {
		log.Println("Error while writing response:", err)
	}
}

func (s *Server) GetDocument(w http.ResponseWriter, r *http.Request) {
	document, ok := s.getDocument(w, r)
	if !ok {
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
		s.Error(w, r, err, http.StatusInternalServerError)
		return
	}

	token, err := s.NewToken(document.ID, []Permission{PermissionWrite, PermissionDelete, PermissionShare})
	if err != nil {
		s.Error(w, r, err, http.StatusInternalServerError)
		return
	}

	s.JSON(w, r, DocumentResponse{
		Key:      document.ID,
		Data:     document.Content,
		Language: document.Language,
		Token:    token,
	})
}

func (s *Server) PatchDocument(w http.ResponseWriter, r *http.Request) {
	document, ok := s.getDocument(w, r)
	if !ok {
		println("not ok")
		return
	}

	claims := s.GetClaims(r)

	fmt.Printf("%+v\n", claims)

	if claims.Subject != document.ID || !slices.Contains(claims.Permissions, PermissionWrite) {
		println("not allowed")
		s.Error(w, r, ErrDocumentNotFound, http.StatusNotFound)
		return
	}

	language := r.Header.Get("Language")
	content := s.readBody(w, r)
	if content == "" {
		return
	}

	if s.exceedsMaxDocumentSize(w, r, content) {
		return
	}

	var err error
	document, err = s.db.UpdateDocument(r.Context(), document.ID, content, language)
	if err != nil {
		s.DBError(w, r, err)
		return
	}

	s.JSON(w, r, DocumentResponse{
		Key:      document.ID,
		Data:     document.Content,
		Language: document.Language,
	})
}

func (s *Server) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	document, ok := s.getDocument(w, r)
	if !ok {
		return
	}

	claims := s.GetClaims(r)
	if claims.Subject != document.ID || slices.Contains(claims.Permissions, PermissionDelete) {
		s.NotFound(w, r)
		return
	}

	if err := s.db.DeleteDocument(r.Context(), document.ID); err != nil {
		s.DBError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) PostDocumentShare(w http.ResponseWriter, r *http.Request) {
	document, ok := s.getDocument(w, r)
	if !ok {
		return
	}

	var shareRequest ShareRequest
	if err := json.NewDecoder(r.Body).Decode(&shareRequest); err != nil {
		s.Error(w, r, err, http.StatusBadRequest)
		return
	}

	if len(shareRequest.Permissions) == 0 {
		s.Error(w, r, ErrNoPermissions, http.StatusBadRequest)
		return
	}

	for _, permission := range shareRequest.Permissions {
		if !permission.IsValid() {
			s.Error(w, r, ErrUnknownPermission(permission), http.StatusBadRequest)
			return
		}
	}

	claims := s.GetClaims(r)
	if claims.Subject != document.ID || !slices.Contains(claims.Permissions, PermissionShare) {
		s.NotFound(w, r)
		return
	}

	for _, permission := range shareRequest.Permissions {
		if !slices.Contains(claims.Permissions, permission) {
			s.Error(w, r, ErrPermissionDenied(permission), http.StatusForbidden)
			return
		}
	}

	token, err := s.NewToken(document.ID, shareRequest.Permissions)
	if err != nil {
		s.Error(w, r, err, http.StatusInternalServerError)
		return
	}

	s.JSON(w, r, ShareResponse{
		Token: token,
	})
}

func (s *Server) getDocument(w http.ResponseWriter, r *http.Request) (Document, bool) {
	documentID := chi.URLParam(r, "documentID")
	if documentID == "" {
		println("no document id")
		s.NotFound(w, r)
		return Document{}, false
	}

	document, err := s.db.GetDocument(r.Context(), documentID)
	if err != nil {
		println("db error")
		s.DBError(w, r, err)
		return Document{}, false
	}

	return document, true
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

func (s *Server) NotFound(w http.ResponseWriter, r *http.Request) {
	s.Error(w, r, ErrDocumentNotFound, http.StatusNotFound)
}

func (s *Server) PrettyError(w http.ResponseWriter, r *http.Request, err error, status int) {
	log.Printf("Error while handling request %s: %s\n", r.URL, err)
	w.WriteHeader(status)
	if tmplErr := s.tmpl(w, "error.gohtml", err.Error()); tmplErr != nil {
		log.Println("Error while executing template:", tmplErr)
	}
}

func (s *Server) DBError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		s.Error(w, r, ErrDocumentNotFound, http.StatusNotFound)
		return
	}
	s.Error(w, r, err, http.StatusInternalServerError)
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

func (s *Server) exceedsMaxDocumentSize(w http.ResponseWriter, r *http.Request, content string) bool {
	if s.cfg.MaxDocumentSize > 0 && len([]rune(content)) > s.cfg.MaxDocumentSize {
		s.Error(w, r, ErrContentTooLarge(s.cfg.MaxDocumentSize), http.StatusBadRequest)
		return true
	}
	return false
}
