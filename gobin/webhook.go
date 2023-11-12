package gobin

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/topi314/tint"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

var (
	ErrWebhookNotFound            = errors.New("webhook not found")
	ErrMissingWebhookSecret       = errors.New("missing webhook secret")
	ErrMissingWebhookURL          = errors.New("missing webhook url")
	ErrMissingWebhookEvents       = errors.New("missing webhook events")
	ErrMissingURLOrSecretOrEvents = errors.New("missing url, secret or events")
)

func (s *Server) ExecuteWebhooks(ctx context.Context, event string, document WebhookDocument) {
	s.webhookWaitGroup.Add(1)
	go s.executeWebhooks(context.WithoutCancel(ctx), event, document)
}

func (s *Server) executeWebhooks(ctx context.Context, event string, document WebhookDocument) {
	ctx, span := s.tracer.Start(ctx, "executeWebhooks")
	defer span.End()
	defer s.webhookWaitGroup.Done()

	span.SetAttributes(attribute.String("event", event), attribute.String("document_id", document.Key))

	dbCtx, cancel := context.WithTimeout(ctx, s.cfg.Webhook.Timeout)
	defer cancel()

	var (
		webhooks []Webhook
		err      error
	)
	if event == "delete" {
		webhooks, err = s.db.GetAndDeleteWebhooksByDocumentID(dbCtx, document.Key)
	} else {
		webhooks, err = s.db.GetWebhooksByDocumentID(dbCtx, document.Key)
	}
	if err != nil {
		slog.ErrorContext(dbCtx, "failed to get webhooks by document id", tint.Err(err))
		return
	}

	if len(webhooks) == 0 {
		return
	}

	now := time.Now()
	var wg sync.WaitGroup
	for _, webhook := range webhooks {
		if !slices.Contains(strings.Split(webhook.Events, ","), event) {
			continue
		}

		wg.Add(1)
		go func(webhook Webhook) {
			defer wg.Done()
			s.executeWebhook(ctx, webhook.URL, webhook.Secret, WebhookEventRequest{
				WebhookID: webhook.ID,
				Event:     event,
				CreatedAt: now,
				Document:  document,
			})
		}(webhook)
	}
	wg.Wait()

	slog.DebugContext(ctx, "finished emitting webhooks", slog.String("event", event), slog.Any("document_id", document.Key))
}

func (s *Server) executeWebhook(ctx context.Context, url string, secret string, request WebhookEventRequest) {
	logger := slog.Default().With(slog.String("event", request.Event), slog.Any("webhook_id", request.WebhookID), slog.Any("document_id", request.Document.Key))
	logger.DebugContext(ctx, "emitting webhook", slog.String("url", url))

	ctx, span := s.tracer.Start(ctx, "executeWebhook")
	defer span.End()

	span.SetAttributes(attribute.String("url", url), attribute.String("event", request.Event), attribute.String("document_id", request.Document.Key))

	buff := new(bytes.Buffer)
	if err := json.NewEncoder(buff).Encode(request); err != nil {
		span.SetStatus(codes.Error, "failed to encode document")
		span.RecordError(err)
		logger.ErrorContext(ctx, "failed to encode document", tint.Err(err))
		return
	}

	rq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, buff)
	if err != nil {
		span.SetStatus(codes.Error, "failed to create request")
		span.RecordError(err)
		logger.ErrorContext(ctx, "failed to create request", tint.Err(err))
		return
	}
	rq.Header.Add("Content-Type", "application/json")
	rq.Header.Add("User-Agent", "gobin")
	rq.Header.Add("Authorization", "Secret "+secret)

	for i := 0; i < s.cfg.Webhook.MaxTries; i++ {
		backoff := time.Duration(s.cfg.Webhook.BackoffFactor * float64(s.cfg.Webhook.Backoff) * float64(i))
		if backoff > time.Nanosecond {
			if backoff > s.cfg.Webhook.MaxBackoff {
				backoff = s.cfg.Webhook.MaxBackoff
			}
			logger.DebugContext(ctx, "sleeping backoff", slog.Duration("backoff", backoff))
			time.Sleep(backoff)
		}

		rs, err := s.client.Do(rq)
		if err != nil {
			logger.DebugContext(ctx, "failed to execute request", tint.Err(err))
			continue
		}

		if rs.StatusCode < 200 || rs.StatusCode >= 300 {
			logger.DebugContext(ctx, "invalid status code", slog.Int("status", rs.StatusCode))
			continue
		}

		logger.DebugContext(ctx, "successfully executed webhook", slog.String("status", rs.Status))
		return
	}

	err = errors.New("max tries reached")
	span.SetStatus(codes.Error, "failed to execute webhook")
	span.RecordError(err)
	logger.ErrorContext(ctx, "failed to execute webhook", tint.Err(err))
}

func (s *Server) PostDocumentWebhook(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "documentID")

	var webhookCreate WebhookCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&webhookCreate); err != nil {
		s.error(w, r, err, http.StatusBadRequest)
		return
	}

	if webhookCreate.URL == "" {
		s.error(w, r, ErrMissingWebhookURL, http.StatusBadRequest)
		return
	}

	if webhookCreate.Secret == "" {
		s.error(w, r, ErrMissingWebhookSecret, http.StatusBadRequest)
		return
	}

	if len(webhookCreate.Events) == 0 {
		s.error(w, r, ErrMissingWebhookEvents, http.StatusBadRequest)
		return
	}

	claims := GetClaims(r)
	if !slices.Contains(claims.Permissions, PermissionWebhook) {
		s.error(w, r, ErrPermissionDenied(PermissionWebhook), http.StatusForbidden)
		return
	}

	webhook, err := s.db.CreateWebhook(r.Context(), documentID, webhookCreate.URL, webhookCreate.Secret, webhookCreate.Events)
	if err != nil {
		s.error(w, r, err, http.StatusInternalServerError)
		return
	}

	s.ok(w, r, WebhookResponse{
		ID:          webhook.ID,
		DocumentKey: webhook.DocumentID,
		URL:         webhook.URL,
		Secret:      webhook.Secret,
		Events:      strings.Split(webhook.Events, ","),
	})
}

func (s *Server) GetDocumentWebhook(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "documentID")
	webhookID := chi.URLParam(r, "webhookID")
	secret := GetWebhookSecret(r)
	if secret == "" {
		s.error(w, r, ErrMissingWebhookSecret, http.StatusBadRequest)
		return
	}

	webhook, err := s.db.GetWebhook(r.Context(), documentID, webhookID, secret)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.webhookNotFound(w, r)
			return
		}
		s.error(w, r, err, http.StatusInternalServerError)
		return
	}

	s.ok(w, r, WebhookResponse{
		ID:          webhook.ID,
		DocumentKey: webhook.DocumentID,
		URL:         webhook.URL,
		Secret:      webhook.Secret,
		Events:      strings.Split(webhook.Events, ","),
	})
}

func (s *Server) PatchDocumentWebhook(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "documentID")
	webhookID := chi.URLParam(r, "webhookID")
	secret := GetWebhookSecret(r)
	if secret == "" {
		s.error(w, r, ErrMissingWebhookSecret, http.StatusBadRequest)
		return
	}

	var webhookUpdate WebhookUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&webhookUpdate); err != nil {
		s.error(w, r, err, http.StatusBadRequest)
		return
	}

	if webhookUpdate.URL == "" && webhookUpdate.Secret == "" && len(webhookUpdate.Events) == 0 {
		s.error(w, r, ErrMissingURLOrSecretOrEvents, http.StatusBadRequest)
		return
	}

	webhook, err := s.db.UpdateWebhook(r.Context(), documentID, webhookID, secret, webhookUpdate.URL, webhookUpdate.Secret, webhookUpdate.Events)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.webhookNotFound(w, r)
			return
		}
		s.error(w, r, err, http.StatusInternalServerError)
		return
	}

	s.ok(w, r, WebhookResponse{
		ID:          webhook.ID,
		DocumentKey: webhook.DocumentID,
		URL:         webhook.URL,
		Secret:      webhook.Secret,
		Events:      strings.Split(webhook.Events, ","),
	})
}

func (s *Server) DeleteDocumentWebhook(w http.ResponseWriter, r *http.Request) {
	documentID := chi.URLParam(r, "documentID")
	webhookID := chi.URLParam(r, "webhookID")
	secret := GetWebhookSecret(r)
	if secret == "" {
		s.error(w, r, ErrMissingWebhookSecret, http.StatusBadRequest)
		return
	}

	if err := s.db.DeleteWebhook(r.Context(), documentID, webhookID, secret); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.webhookNotFound(w, r)
			return
		}
		s.error(w, r, err, http.StatusInternalServerError)
		return
	}

	s.ok(w, r, nil)
}

func (s *Server) webhookNotFound(w http.ResponseWriter, r *http.Request) {
	s.error(w, r, ErrWebhookNotFound, http.StatusNotFound)
}
