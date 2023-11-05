package gobin

import (
	"html/template"
	"time"
)

type (
	TemplateVariables struct {
		ID        string
		Version   int64
		Content   template.HTML
		Formatted template.HTML
		CSS       template.CSS
		ThemeCSS  template.CSS
		Language  string

		Versions []DocumentVersion
		Lexers   []string
		Styles   []TemplateStyle
		Style    string
		Theme    string

		Max        int
		Host       string
		Preview    bool
		PreviewAlt string
	}

	TemplateStyle struct {
		Name  string
		Theme string
	}

	TemplateErrorVariables struct {
		Error     string
		Status    int
		RequestID string
		Path      string
	}

	DocumentVersion struct {
		Version int64
		Label   string
		Time    string
	}

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

	WebhookRequest struct {
		URL    string   `json:"url"`
		Secret string   `json:"secret"`
		Events []string `json:"events"`
	}

	WebhookResponse struct {
		ID     int      `json:"id"`
		URL    string   `json:"url"`
		Secret string   `json:"secret"`
		Events []string `json:"events"`
	}

	FailedWebhookEventsRequest []FailedWebhook

	FailedWebhook struct {
		ID     int    `json:"id"`
		Secret string `json:"secret"`
	}

	FailWebhookEventsResponse []WebhookEventRequest

	WebhookEventRequest struct {
		WebhookID int             `json:"webhook_id"`
		Event     string          `json:"event"`
		Retries   int             `json:"retries"`
		CreatedAt time.Time       `json:"created_at"`
		LastRetry time.Time       `json:"last_retry"`
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
