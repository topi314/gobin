package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
)

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

func (d *DB) GetWebhook(ctx context.Context, documentID string, webhookID string, secret string) (*Webhook, error) {
	var webhook Webhook
	err := d.dbx.GetContext(ctx, &webhook, "SELECT * FROM webhooks WHERE document_id = $1 AND id = $2 AND secret = $3", documentID, webhookID, secret)
	if err != nil {
		return nil, err
	}

	return &webhook, nil
}

func (d *DB) GetWebhooksByDocumentID(ctx context.Context, documentID string) ([]Webhook, error) {
	var webhooks []Webhook
	err := d.dbx.SelectContext(ctx, &webhooks, "SELECT * FROM webhooks WHERE document_id = $1", documentID)
	if err != nil {
		return nil, err
	}

	return webhooks, nil
}

func (d *DB) GetAndDeleteWebhooksByDocumentID(ctx context.Context, documentID string) ([]Webhook, error) {
	var webhooks []Webhook
	err := d.dbx.SelectContext(ctx, &webhooks, "DELETE FROM webhooks WHERE document_id = $1 RETURNING *", documentID)
	if err != nil {
		return nil, err
	}

	return webhooks, nil
}

func (d *DB) CreateWebhook(ctx context.Context, documentID string, url string, secret string, events []string) (*Webhook, error) {
	webhook := Webhook{
		ID:         d.randomString(8),
		DocumentID: documentID,
		URL:        url,
		Secret:     secret,
		Events:     strings.Join(events, ","),
	}

	if _, err := d.dbx.NamedExecContext(ctx, "INSERT INTO webhooks (id, document_id, url, secret, events) VALUES (:id, :document_id, :url, :secret, :events)", webhook); err != nil {
		return nil, fmt.Errorf("failed to insert webhook: %w", err)
	}

	return &webhook, nil
}

func (d *DB) UpdateWebhook(ctx context.Context, documentID string, webhookID string, secret string, newURL string, newSecret string, newEvents []string) (*Webhook, error) {
	webhookUpdate := WebhookUpdate{
		ID:         webhookID,
		DocumentID: documentID,
		Secret:     secret,
		NewURL:     newURL,
		NewSecret:  newSecret,
		NewEvents:  strings.Join(newEvents, ","),
	}

	query, args, err := sqlx.Named(`UPDATE webhooks SET 
                    url = CASE WHEN :new_url = '' THEN url ELSE :new_url END,
                    secret = CASE WHEN :new_secret = '' THEN secret ELSE :new_secret END,
                    events = CASE WHEN :new_events = '' THEN events ELSE :new_events END
                WHERE document_id = :document_id AND id = :id AND secret = :secret returning *`, webhookUpdate)
	if err != nil {
		return nil, err
	}

	var webhook Webhook
	if err = d.dbx.GetContext(ctx, webhook, query, args...); err != nil {
		return nil, err
	}

	return &webhook, nil
}

func (d *DB) DeleteWebhook(ctx context.Context, documentID string, webhookID string, secret string) error {
	res, err := d.dbx.ExecContext(ctx, "DELETE FROM webhooks WHERE document_id = $1 AND id = $2 AND secret = $3", documentID, webhookID, secret)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}
