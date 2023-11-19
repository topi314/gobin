package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"modernc.org/sqlite"
)

type Document struct {
	ID    string `db:"id"`
	Files []File `db:"files"`
}

func (d *DB) GetDocumentVersion(ctx context.Context, documentID string, version int64) (Document, error) {
	var doc Document
	err := d.dbx.GetContext(ctx, &doc, "SELECT * FROM documents WHERE id = $1 AND version = $2", documentID, version)
	return doc, err
}

func (d *DB) GetDocumentVersions(ctx context.Context, documentID string, withContent bool) ([]Document, error) {
	var (
		docs      []Document
		sqlString string
	)
	if withContent {
		sqlString = "SELECT id, version, content, language FROM documents where id = $1 ORDER BY version DESC"
	} else {
		sqlString = "SELECT id, version FROM documents where id = $1 ORDER BY version DESC"
	}
	err := d.dbx.SelectContext(ctx, &docs, sqlString, documentID)
	return docs, err
}

func (d *DB) GetVersionCount(ctx context.Context, documentID string) (int, error) {
	var count int
	err := d.dbx.GetContext(ctx, &count, "SELECT COUNT(*) FROM documents WHERE id = $1", documentID)
	return count, err
}

func (d *DB) CreateDocument(ctx context.Context, content string, language string) (Document, error) {
	return d.createDocument(ctx, content, language, 0)
}

func (d *DB) createDocument(ctx context.Context, content string, language string, try int) (Document, error) {
	if try >= 10 {
		return Document{}, errors.New("failed to create document because of duplicate key after 10 tries")
	}
	now := time.Now().Unix()
	doc := Document{
		ID:       d.randomString(8),
		Content:  content,
		Language: language,
		Version:  now,
	}
	_, err := d.dbx.NamedExecContext(ctx, "INSERT INTO documents (id, version, content, language) VALUES (:id, :version, :content, :language) RETURNING *", doc)

	if err != nil {
		var (
			sqliteErr *sqlite.Error
			pgErr     *pgconn.PgError
		)
		if errors.As(err, &sqliteErr) || errors.As(err, &pgErr) {
			if (sqliteErr != nil && sqliteErr.Code() == 1555) || (pgErr != nil && pgErr.Code == "23505") {
				return d.createDocument(ctx, content, language, try+1)
			}
		}
	}

	return doc, err
}

func (d *DB) UpdateDocument(ctx context.Context, documentID string, content string, language string) (Document, error) {
	doc := Document{
		ID:       documentID,
		Version:  time.Now().Unix(),
		Content:  content,
		Language: language,
	}
	res, err := d.dbx.NamedExecContext(ctx, "INSERT INTO documents (id, version, content, language) VALUES (:id, :version, :content, :language)", doc)
	if err != nil {
		return Document{}, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return Document{}, err
	}
	if rows == 0 {
		return Document{}, sql.ErrNoRows
	}

	return doc, nil
}

func (d *DB) DeleteDocument(ctx context.Context, documentID string) (Document, error) {
	var document Document
	if err := d.dbx.GetContext(ctx, &document, "DELETE FROM documents WHERE id = $1 RETURNING *", documentID); err != nil {
		return Document{}, err
	}

	return document, nil
}

func (d *DB) DeleteDocumentByVersion(ctx context.Context, documentID string, version int64) (Document, error) {
	var document Document
	if err := d.dbx.GetContext(ctx, "DELETE FROM documents WHERE id = $1 AND version = $2 returning *", documentID, version); err != nil {
		return Document{}, err
	}

	return document, nil
}

func (d *DB) DeleteExpiredDocuments(ctx context.Context, expireAfter time.Duration) error {
	_, err := d.dbx.ExecContext(ctx, "DELETE FROM documents WHERE version < $1", time.Now().Add(expireAfter).Unix())
	return err
}
