package database

import (
	"time"
)

type File struct {
	DocumentID      string     `db:"document_id"`
	DocumentVersion int64      `db:"document_version"`
	Name            string     `db:"name"`
	Content         string     `db:"content"`
	Language        string     `db:"language"`
	ExpiresAt       *time.Time `db:"expires_at"`
	OrderIndex      int        `db:"order_index"`
}

type Document struct {
	ID      string
	Version int64
	Files   []File
}

type Webhook struct {
	ID         string `db:"id"`
	DocumentID string `db:"document_id"`
	URL        string `db:"url"`
	Secret     string `db:"secret"`
	Events     string `db:"events"`
}

type WebhookUpdate struct {
	ID         string `db:"id"`
	DocumentID string `db:"document_id"`
	Secret     string `db:"secret"`

	NewURL    string `db:"new_url"`
	NewSecret string `db:"new_secret"`
	NewEvents string `db:"new_events"`
}
