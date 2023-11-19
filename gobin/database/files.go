package database

import (
	"context"
	"fmt"
)

type File struct {
	ID         string `db:"id"`
	DocumentID string `db:"document_id"`
	Name       string `db:"name"`
	Version    int64  `db:"version"`
	Content    string `db:"content"`
	Language   string `db:"language"`
}

type Files []File

type FileVersions map[int64][]File

func (d *DB) GetDocument(ctx context.Context, documentID string) (Files, error) {
	var files Files
	if err := d.dbx.SelectContext(ctx, &files, "SELECT id, document_id, name, version, content, language from (SELECT *, rank() OVER (PARTITION BY id ORDER BY version DESC ) AS rank FROM files) AS f WHERE document_id = $1 AND rank = 1;", documentID); err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}
	return files, nil
}

func (d *DB) GetDocumentVersion(ctx context.Context, documentID string, version int64) (Files, error) {

}

func (d *DB) GetDocumentVersions(ctx context.Context, documentID string, withContent bool) ([]Document, error) {

}
