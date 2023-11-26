package database

import (
	"context"
	"fmt"
)

func (d *DB) GetDocumentFile(ctx context.Context, documentID string, fileName string) (*File, error) {
	var file File
	if err := d.dbx.GetContext(ctx, &file, "SELECT id, document_id, document_version, name, content, language from (SELECT *, rank() OVER (PARTITION BY id ORDER BY document_version DESC) AS rank FROM files) AS f WHERE document_id = $1 AND name = $2 AND rank = 1;", documentID, fileName); err != nil {
		return nil, fmt.Errorf("failed to get document file: %w", err)
	}

	return &file, nil
}

func (d *DB) GetDocumentFileVersion(ctx context.Context, documentID string, documentVersion int64, fileName string) (*File, error) {
	var file File
	if err := d.dbx.GetContext(ctx, &file, "SELECT id, document_id, document_version, name, content, language from files WHERE document_id = $1 AND document_version = $2 AND name = $3;", documentID, documentVersion, fileName); err != nil {
		return nil, fmt.Errorf("failed to get document file version: %w", err)
	}

	return &file, nil
}

func (d *DB) DeleteDocumentFile(ctx context.Context, documentID string, fileName string) error {
	if _, err := d.dbx.ExecContext(ctx, "DELETE FROM files WHERE document_id = $1 AND name = $2;", documentID, fileName); err != nil {
		return fmt.Errorf("failed to delete document file: %w", err)
	}

	return nil
}

func (d *DB) DeleteDocumentVersionFile(ctx context.Context, documentID string, documentVersion int64, fileName string) error {
	if _, err := d.dbx.ExecContext(ctx, "DELETE FROM files WHERE document_id = $1 AND document_version = $2 AND name = $3;", documentID, documentVersion, fileName); err != nil {
		return fmt.Errorf("failed to delete document version file: %w", err)
	}

	return nil
}
