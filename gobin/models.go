package gobin

import "html/template"

type (
	TemplateVariables struct {
		ID        string
		Version   int64
		Content   template.HTML
		Formatted template.HTML
		CSS       template.CSS
		Language  string

		Versions []DocumentVersion
		Lexers   []string
		Styles   []string
		Style    string
		Theme    string

		Max        int
		Host       string
		Preview    bool
		PreviewAlt string
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
)
