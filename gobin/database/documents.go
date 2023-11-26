package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type File struct {
	DocumentID      string `db:"document_id"`
	DocumentVersion int64  `db:"document_version"`
	Name            string `db:"name"`
	Content         string `db:"content"`
	Language        string `db:"language"`
}

func (d *DB) GetDocument(ctx context.Context, documentID string) ([]File, error) {
	var files []File
	if err := d.dbx.SelectContext(ctx, &files, "SELECT name, document_id, document_version, content, language from (SELECT *, rank() OVER (PARTITION BY document_id ORDER BY document_version DESC) AS rank FROM files) AS f WHERE document_id = $1 AND rank = 1;", documentID); err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	if len(files) == 0 {
		return nil, sql.ErrNoRows
	}
	return files, nil
}

func (d *DB) GetDocumentVersion(ctx context.Context, documentID string, documentVersion int64) ([]File, error) {
	var files []File
	if err := d.dbx.SelectContext(ctx, &files, "SELECT name, document_id, document_version, content, language from files WHERE document_id = $1 AND document_version = $2;", documentID, documentVersion); err != nil {
		return nil, fmt.Errorf("failed to get document version: %w", err)
	}

	if len(files) == 0 {
		return nil, sql.ErrNoRows
	}
	return files, nil
}

func (d *DB) GetDocumentVersions(ctx context.Context, documentID string) ([]int64, error) {
	var versions []int64
	if err := d.dbx.SelectContext(ctx, &versions, "SELECT DISTINCT document_version FROM files WHERE document_id = $1 ORDER BY document_version DESC;", documentID); err != nil {
		return nil, fmt.Errorf("failed to get document versions: %w", err)
	}
	return versions, nil

}

func (d *DB) CreateDocument(ctx context.Context, files []File) (*string, error) {
	documentID := d.randomString(8)
	for i := range files {
		files[i].DocumentID = documentID
		files[i].DocumentVersion = 0
	}

	if _, err := d.dbx.NamedExecContext(ctx, "INSERT INTO files (name, document_id, document_version, content, language) VALUES (:name, :document_id, :document_version, :content, :language);", files); err != nil {
		return nil, fmt.Errorf("failed to create document: %w", err)
	}
	return &documentID, nil
}

func (d *DB) UpdateDocument(ctx context.Context, documentID string, files []File) error {
	version := time.Now().Unix()
	for i := range files {
		files[i].DocumentID = documentID
		files[i].DocumentVersion = version
	}
	if _, err := d.dbx.NamedExecContext(ctx, "INSERT INTO files (name, document_id, document_version, content, language) VALUES (:name, :document_id, :document_version, :content, :language);", files); err != nil {
		return fmt.Errorf("failed to update document: %w", err)
	}
	return nil
}

func (d *DB) DeleteDocument(ctx context.Context, documentID string) error {
	if _, err := d.dbx.ExecContext(ctx, "DELETE FROM files WHERE document_id = $1;", documentID); err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}
	return nil
}

func (d *DB) DeleteDocumentVersion(ctx context.Context, documentID string, documentVersion int) error {
	if _, err := d.dbx.ExecContext(ctx, "DELETE FROM files WHERE document_id = $1 AND document_version = $2;", documentID, documentVersion); err != nil {
		return fmt.Errorf("failed to delete document version: %w", err)
	}
	return nil
}

func (d *DB) DeleteDocumentVersions(ctx context.Context, documentID string) error {
	if _, err := d.dbx.ExecContext(ctx, "DELETE FROM files WHERE document_id = $1;", documentID); err != nil {
		return fmt.Errorf("failed to delete document versions: %w", err)
	}
	return nil
}

func (d *DB) DeleteExpiredDocuments(ctx context.Context, expireAfter time.Duration) error {
	if _, err := d.dbx.ExecContext(ctx, "DELETE FROM files WHERE document_version < $1;", time.Now().Add(expireAfter).Unix()); err != nil {
		return fmt.Errorf("failed to delete expired documents: %w", err)
	}
	return nil
}
