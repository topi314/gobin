package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

var _ DB = (*postgresDB)(nil)

func newPostgresDB(db *sqlx.DB) *postgresDB {
	return &postgresDB{db}
}

type postgresDB struct {
	*sqlx.DB
}

func (d *postgresDB) GetDocument(ctx context.Context, documentID string) ([]File, error) {
	var files []File
	if err := d.SelectContext(ctx, &files, "SELECT name, document_id, document_version, content, language, expires_at from (SELECT *, rank() OVER (PARTITION BY document_id ORDER BY document_version DESC) AS rank FROM files) AS f WHERE document_id = $1 AND rank = 1 ORDER BY order_index;", documentID); err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	if len(files) == 0 {
		return nil, sql.ErrNoRows
	}
	return files, nil
}

func (d *postgresDB) GetDocumentVersion(ctx context.Context, documentID string, documentVersion int64) ([]File, error) {
	var files []File
	if err := d.SelectContext(ctx, &files, "SELECT name, document_id, document_version, content, language, expires_at from files WHERE document_id = $1 AND document_version = $2 ORDER BY order_index;", documentID, documentVersion); err != nil {
		return nil, fmt.Errorf("failed to get document version: %w", err)
	}

	if len(files) == 0 {
		return nil, sql.ErrNoRows
	}
	return files, nil
}

func (d *postgresDB) GetVersionCount(ctx context.Context, documentID string) (int, error) {
	var count int
	err := d.GetContext(ctx, &count, "SELECT COUNT(DISTINCT document_version) FROM files WHERE document_id = $1;", documentID)
	return count, err
}

func (d *postgresDB) GetDocumentVersions(ctx context.Context, documentID string) ([]int64, error) {
	var versions []int64
	if err := d.SelectContext(ctx, &versions, "SELECT DISTINCT document_version FROM files WHERE document_id = $1 ORDER BY document_version DESC;", documentID); err != nil {
		return nil, fmt.Errorf("failed to get document versions: %w", err)
	}
	return versions, nil

}

func (d *postgresDB) GetDocumentVersionsWithFiles(ctx context.Context, documentID string, withContent bool) (map[int64][]File, error) {
	var query string
	if withContent {
		query = "SELECT name, document_id, document_version, content, language, expires_at WHERE document_id = $1 ORDER BY document_version DESC;"
	} else {
		query = "SELECT name, document_id, document_version, language, expires_at WHERE document_id = $1 ORDER BY document_version DESC;"
	}

	var files []File
	if err := d.SelectContext(ctx, &files, query, documentID); err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	if len(files) == 0 {
		return nil, sql.ErrNoRows
	}

	mapFiles := make(map[int64][]File)
	for _, file := range files {
		mapFiles[file.DocumentVersion] = append(mapFiles[file.DocumentVersion], file)
	}
	return mapFiles, nil

}

func (d *postgresDB) CreateDocument(ctx context.Context, files []File) (*string, *int64, error) {
	documentID := randomString(8)
	version := time.Now().UnixMilli()
	for i := range files {
		files[i].DocumentID = documentID
		files[i].DocumentVersion = version
	}

	if _, err := d.NamedExecContext(ctx, "INSERT INTO files (name, document_id, document_version, content, language, expires_at, order_index) VALUES (:name, :document_id, :document_version, :content, :language, :expires_at, :order_index);", files); err != nil {
		return nil, nil, fmt.Errorf("failed to create document: %w", err)
	}
	return &documentID, &version, nil
}

func (d *postgresDB) UpdateDocument(ctx context.Context, documentID string, files []File) (*int64, error) {
	version := time.Now().UnixMilli()
	for i := range files {
		files[i].DocumentID = documentID
		files[i].DocumentVersion = version
	}
	if _, err := d.NamedExecContext(ctx, "INSERT INTO files (name, document_id, document_version, content, language, expires_at) VALUES (:name, :document_id, :document_version, :content, :language, :expires_at);", files); err != nil {
		return nil, fmt.Errorf("failed to update document: %w", err)
	}
	return &version, nil
}

func (d *postgresDB) DeleteDocument(ctx context.Context, documentID string) (*Document, error) {
	var files []File
	if err := d.SelectContext(ctx, &files, "DELETE FROM files WHERE document_id = $1 RETURNING *", documentID); err != nil {
		return nil, fmt.Errorf("failed to delete document: %w", err)
	}

	if len(files) == 0 {
		return nil, sql.ErrNoRows
	}

	var lastDeletedFiles []File
	for i := len(files) - 1; i >= 0; i-- {
		if files[i].DocumentVersion != files[len(files)-1].DocumentVersion {
			break
		}
		lastDeletedFiles = append(lastDeletedFiles, files[i])
	}

	return &Document{
		ID:      documentID,
		Version: files[len(files)-1].DocumentVersion,
		Files:   lastDeletedFiles,
	}, nil
}

func (d *postgresDB) DeleteDocumentVersion(ctx context.Context, documentID string, documentVersion int64) (*Document, error) {
	var files []File
	if err := d.SelectContext(ctx, &files, "DELETE FROM files WHERE document_id = $1 AND document_version = $2 RETURNING *;", documentID, documentVersion); err != nil {
		return nil, fmt.Errorf("failed to delete document version: %w", err)
	}

	if len(files) == 0 {
		return nil, sql.ErrNoRows
	}

	var lastDeletedFiles []File
	for i := len(files) - 1; i >= 0; i-- {
		if files[i].DocumentVersion != files[len(files)-1].DocumentVersion {
			break
		}
		lastDeletedFiles = append(lastDeletedFiles, files[i])
	}

	return &Document{
		ID:      documentID,
		Version: documentVersion,
		Files:   lastDeletedFiles,
	}, nil
}

func (d *postgresDB) DeleteDocumentVersions(ctx context.Context, documentID string) error {
	if _, err := d.ExecContext(ctx, "DELETE FROM files WHERE document_id = $1;", documentID); err != nil {
		return fmt.Errorf("failed to delete document versions: %w", err)
	}
	return nil
}

func (d *postgresDB) DeleteExpiredDocuments(ctx context.Context, expireAfter time.Duration) ([]Document, error) {
	now := time.Now()
	query := "DELETE FROM files WHERE expires_at < $1"
	args := []interface{}{now}
	if expireAfter > 0 {
		query += " OR document_version < $2"
		args = append(args, now.Add(expireAfter).UnixMilli())
	}
	query += " RETURNING *;"
	var files []File
	if err := d.SelectContext(ctx, &files, query, args...); err != nil {
		return nil, fmt.Errorf("failed to delete expired documents: %w", err)
	}

	documents := make(map[string]Document)
	for _, file := range files {
		document, ok := documents[file.DocumentID]
		if !ok || file.DocumentVersion > document.Version {
			document = Document{
				ID:      file.DocumentID,
				Version: file.DocumentVersion,
			}
		}
		if file.DocumentVersion < document.Version {
			continue
		}

		document.Files = append(document.Files, file)
		documents[file.DocumentID] = document
	}

	documentsSlice := make([]Document, 0, len(documents))
	for _, document := range documents {
		documentsSlice = append(documentsSlice, document)
	}
	return documentsSlice, nil
}

func (d *postgresDB) GetDocumentFile(ctx context.Context, documentID string, fileName string) (*File, error) {
	var file File
	if err := d.GetContext(ctx, &file, "SELECT name, document_id, document_version, content, language, expires_at from (SELECT *, rank() OVER (PARTITION BY document_id ORDER BY document_version DESC) AS rank FROM files) AS f WHERE document_id = $1 AND name = $2 AND rank = 1;", documentID, fileName); err != nil {
		return nil, fmt.Errorf("failed to get document file: %w", err)
	}

	return &file, nil
}

func (d *postgresDB) GetDocumentFileVersion(ctx context.Context, documentID string, documentVersion int64, fileName string) (*File, error) {
	var file File
	if err := d.GetContext(ctx, &file, "SELECT name, document_id, document_version, content, language, expires_at from files WHERE document_id = $1 AND document_version = $2 AND name = $3;", documentID, documentVersion, fileName); err != nil {
		return nil, fmt.Errorf("failed to get document file version: %w", err)
	}

	return &file, nil
}

func (d *postgresDB) DeleteDocumentFile(ctx context.Context, documentID string, fileName string) error {
	if _, err := d.ExecContext(ctx, "DELETE FROM files WHERE document_id = $1 AND name = $2;", documentID, fileName); err != nil {
		return fmt.Errorf("failed to delete document file: %w", err)
	}

	return nil
}

func (d *postgresDB) DeleteDocumentVersionFile(ctx context.Context, documentID string, documentVersion int64, fileName string) error {
	if _, err := d.ExecContext(ctx, "DELETE FROM files WHERE document_id = $1 AND document_version = $2 AND name = $3;", documentID, documentVersion, fileName); err != nil {
		return fmt.Errorf("failed to delete document version file: %w", err)
	}

	return nil
}

func (d *postgresDB) GetWebhook(ctx context.Context, documentID string, webhookID string, secret string) (*Webhook, error) {
	var webhook Webhook
	err := d.GetContext(ctx, &webhook, "SELECT * FROM webhooks WHERE document_id = $1 AND id = $2 AND secret = $3", documentID, webhookID, secret)
	if err != nil {
		return nil, err
	}

	return &webhook, nil
}

func (d *postgresDB) GetWebhooksByDocumentID(ctx context.Context, documentID string) ([]Webhook, error) {
	var webhooks []Webhook
	err := d.SelectContext(ctx, &webhooks, "SELECT * FROM webhooks WHERE document_id = $1", documentID)
	if err != nil {
		return nil, err
	}

	return webhooks, nil
}

func (d *postgresDB) GetAndDeleteWebhooksByDocumentID(ctx context.Context, documentID string) ([]Webhook, error) {
	var webhooks []Webhook
	err := d.SelectContext(ctx, &webhooks, "DELETE FROM webhooks WHERE document_id = $1 RETURNING *", documentID)
	if err != nil {
		return nil, err
	}

	return webhooks, nil
}

func (d *postgresDB) CreateWebhook(ctx context.Context, documentID string, url string, secret string, events []string) (*Webhook, error) {
	webhook := Webhook{
		ID:         randomString(8),
		DocumentID: documentID,
		URL:        url,
		Secret:     secret,
		Events:     strings.Join(events, ","),
	}

	if _, err := d.NamedExecContext(ctx, "INSERT INTO webhooks (id, document_id, url, secret, events) VALUES (:id, :document_id, :url, :secret, :events)", webhook); err != nil {
		return nil, fmt.Errorf("failed to insert webhook: %w", err)
	}

	return &webhook, nil
}

func (d *postgresDB) UpdateWebhook(ctx context.Context, documentID string, webhookID string, secret string, newURL string, newSecret string, newEvents []string) (*Webhook, error) {
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
	if err = d.GetContext(ctx, webhook, query, args...); err != nil {
		return nil, err
	}

	return &webhook, nil
}

func (d *postgresDB) DeleteWebhook(ctx context.Context, documentID string, webhookID string, secret string) error {
	res, err := d.ExecContext(ctx, "DELETE FROM webhooks WHERE document_id = $1 AND id = $2 AND secret = $3", documentID, webhookID, secret)
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
