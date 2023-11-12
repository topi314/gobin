package gobin

import (
	"html/template"
	"time"
)

type (
	DocumentResponse struct {
		Key          string        `json:"key,omitempty"`
		Version      int64         `json:"version"`
		VersionLabel string        `json:"version_label,omitempty"`
		VersionTime  string        `json:"version_time,omitempty"`
		Data         string        `json:"data,omitempty"`
		Formatted    template.HTML `json:"formatted,omitempty"`
		CSS          template.CSS  `json:"css,omitempty"`
		ThemeCSS     template.CSS  `json:"theme_css,omitempty"`
		Language     string        `json:"language"`
		Token        string        `json:"token,omitempty"`
	}

	ShareRequest struct {
		Permissions []Permission `json:"permissions"`
	}

	ShareResponse struct {
		Token string `json:"token"`
	}

	DeleteResponse struct {
		Versions int `json:"versions"`
	}

	ErrorResponse struct {
		Message   string `json:"message"`
		Status    int    `json:"status"`
		Path      string `json:"path"`
		RequestID string `json:"request_id"`
	}

	WebhookCreateRequest struct {
		URL    string   `json:"url"`
		Secret string   `json:"secret"`
		Events []string `json:"events"`
	}

	WebhookUpdateRequest struct {
		URL    string   `json:"url"`
		Secret string   `json:"secret"`
		Events []string `json:"events"`
	}

	WebhookResponse struct {
		ID          string   `json:"id"`
		DocumentKey string   `json:"document_key"`
		URL         string   `json:"url"`
		Secret      string   `json:"secret"`
		Events      []string `json:"events"`
	}

	WebhookEventRequest struct {
		WebhookID string          `json:"webhook_id"`
		Event     string          `json:"event"`
		CreatedAt time.Time       `json:"created_at"`
		Document  WebhookDocument `json:"document"`
	}

	WebhookDocument struct {
		Key      string `json:"key"`
		Version  int64  `json:"version"`
		Language string `json:"language"`
		Data     string `json:"data"`
	}
)

const (
	WebhookEventUpdate string = "update"
	WebhookEventDelete string = "delete"
)
