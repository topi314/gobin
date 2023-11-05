package gobin

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (s *Server) PostDocumentWebhook(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "documentID")

	var webhookRq WebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&webhookRq); err != nil {
		s.error(w, r, err, http.StatusBadRequest)
		return
	}

	claims := GetClaims(r)
	if !slices.Contains(claims.Permissions, PermissionWebhook) {
		s.error(w, r, ErrPermissionDenied(PermissionWebhook), http.StatusForbidden)
		return
	}

	webhook, err := s.db.CreateWebhook(r.Context(), documentID, webhookRq.URL, webhookRq.Secret, webhookRq.Events)
	if err != nil {
		s.error(w, r, err, http.StatusInternalServerError)
		return
	}

	s.ok(w, r, WebhookResponse{
		ID:     webhook.ID,
		URL:    webhook.URL,
		Secret: webhook.Secret,
		Events: strings.Split(webhook.Events, ","),
	})
}

func (s *Server) GetDocumentWebhook(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "documentID")
	webhookID := chi.URLParam(r, "webhookID")

	webhook, err := s.db.GetWebhook(r.Context(), webhookID, documentID)

}

func (s *Server) PatchDocumentWebhook(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "documentID")

	var webhook WebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&webhook); err != nil {
		s.error(w, r, err, http.StatusBadRequest)
		return
	}

}

func (s *Server) DeleteDocumentWebhook(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "documentID")
}

func (s *Server) GetFailedWebhookEvents(w http.ResponseWriter, r *http.Request) {
	var webhookEvents FailedWebhookEventsRequest
	if err := json.NewDecoder(r.Body).Decode(&webhookEvents); err != nil {
		s.error(w, r, err, http.StatusBadRequest)
		return
	}
}
