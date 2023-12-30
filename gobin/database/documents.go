package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type File struct {
	DocumentID      string     `db:"document_id"`
	DocumentVersion int64      `db:"document_version"`
	Name            string     `db:"name"`
	Content         string     `db:"content"`
	Language        string     `db:"language"`
	ExpiresAt       *time.Time `db:"expires_at"`
}

type Document struct {
	ID      string
	Version int64
	Files   []File
}

func (d *DB) GetDocument(ctx context.Context, documentID string) ([]File, error) {
	var files []File
	if err := d.dbx.SelectContext(ctx, &files, "SELECT name, document_id, document_version, content, language, expires_at from (SELECT *, rank() OVER (PARTITION BY document_id ORDER BY document_version DESC) AS rank FROM files) AS f WHERE document_id = $1 AND rank = 1;", documentID); err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	if len(files) == 0 {
		return nil, sql.ErrNoRows
	}
	return files, nil
}

func (d *DB) GetDocumentVersion(ctx context.Context, documentID string, documentVersion int64) ([]File, error) {
	var files []File
	if err := d.dbx.SelectContext(ctx, &files, "SELECT name, document_id, document_version, content, language, expires_at from files WHERE document_id = $1 AND document_version = $2;", documentID, documentVersion); err != nil {
		return nil, fmt.Errorf("failed to get document version: %w", err)
	}

	if len(files) == 0 {
		return nil, sql.ErrNoRows
	}
	return files, nil
}

func (d *DB) GetVersionCount(ctx context.Context, documentID string) (int, error) {
	var count int
	err := d.dbx.GetContext(ctx, &count, "SELECT COUNT(DISTINCT document_version) FROM files WHERE document_id = $1;", documentID)
	return count, err
}

func (d *DB) GetDocumentVersions(ctx context.Context, documentID string) ([]int64, error) {
	var versions []int64
	if err := d.dbx.SelectContext(ctx, &versions, "SELECT DISTINCT document_version FROM files WHERE document_id = $1 ORDER BY document_version DESC;", documentID); err != nil {
		return nil, fmt.Errorf("failed to get document versions: %w", err)
	}
	return versions, nil

}

func (d *DB) GetDocumentVersionsWithFiles(ctx context.Context, documentID string, withContent bool) (map[int64][]File, error) {
	var query string
	if withContent {
		query = "SELECT name, document_id, document_version, content, language, expires_at WHERE document_id = $1 ORDER BY document_version DESC;"
	} else {
		query = "SELECT name, document_id, document_version, language, expires_at WHERE document_id = $1 ORDER BY document_version DESC;"
	}

	var files []File
	if err := d.dbx.SelectContext(ctx, &files, query, documentID); err != nil {
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

func (d *DB) CreateDocument(ctx context.Context, files []File) (*string, *int64, error) {
	documentID := d.randomString(8)
	version := time.Now().UnixMilli()
	for i := range files {
		files[i].DocumentID = documentID
		files[i].DocumentVersion = version
	}

	if _, err := d.dbx.NamedExecContext(ctx, "INSERT INTO files (name, document_id, document_version, content, language, expires_at) VALUES (:name, :document_id, :document_version, :content, :language, :expires_at);", files); err != nil {
		return nil, nil, fmt.Errorf("failed to create document: %w", err)
	}
	return &documentID, &version, nil
}

func (d *DB) UpdateDocument(ctx context.Context, documentID string, files []File) (*int64, error) {
	version := time.Now().UnixMilli()
	for i := range files {
		files[i].DocumentID = documentID
		files[i].DocumentVersion = version
	}
	if _, err := d.dbx.NamedExecContext(ctx, "INSERT INTO files (name, document_id, document_version, content, language, expires_at) VALUES (:name, :document_id, :document_version, :content, :language, :expires_at);", files); err != nil {
		return nil, fmt.Errorf("failed to update document: %w", err)
	}
	return &version, nil
}

func (d *DB) DeleteDocument(ctx context.Context, documentID string) (*Document, error) {
	var files []File
	if err := d.dbx.SelectContext(ctx, &files, "DELETE FROM files WHERE document_id = $1 RETURNING *", documentID); err != nil {
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

func (d *DB) DeleteDocumentVersion(ctx context.Context, documentID string, documentVersion int64) (*Document, error) {
	var files []File
	if err := d.dbx.SelectContext(ctx, &files, "DELETE FROM files WHERE document_id = $1 AND document_version = $2 RETURNING *;", documentID, documentVersion); err != nil {
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
		Files:   files,
	}, nil
}

func (d *DB) DeleteDocumentVersions(ctx context.Context, documentID string) error {
	if _, err := d.dbx.ExecContext(ctx, "DELETE FROM files WHERE document_id = $1;", documentID); err != nil {
		return fmt.Errorf("failed to delete document versions: %w", err)
	}
	return nil
}

func (d *DB) DeleteExpiredDocuments(ctx context.Context, expireAfter time.Duration) ([]Document, error) {
	now := time.Now()
	query := "DELETE FROM files WHERE expires_at < $1"
	args := []interface{}{now}
	if expireAfter > 0 {
		query += " OR document_version < $2"
		args = append(args, now.Add(expireAfter).UnixMilli())
	}
	query += " RETURNING *;"
	var files []File
	if err := d.dbx.SelectContext(ctx, &files, query, args...); err != nil {
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
