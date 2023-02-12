package main

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jackc/pgx/v5/tracelog"
	"log"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"modernc.org/sqlite"
	_ "modernc.org/sqlite"
)

var (
	//go:embed schema.sql
	schema string

	chars = []rune("abcdefghijklmnopqrstuvwxyz0123456789")
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func NewDatabase(ctx context.Context, cfg Config) (*Database, error) {
	var (
		driverName     string
		dataSourceName string
	)
	switch cfg.Database.Type {
	case "postgres":
		driverName = "pgx"
		pgCfg, err := pgx.ParseConfig(cfg.Database.PostgresDataSourceName())
		if err != nil {
			return nil, err
		}

		if cfg.DevMode {
			pgCfg.Tracer = &tracelog.TraceLog{
				Logger: tracelog.LoggerFunc(func(ctx context.Context, level tracelog.LogLevel, msg string, data map[string]any) {
					log.Println(msg, data)
				}),
				LogLevel: tracelog.LogLevelDebug,
			}
			dataSourceName = stdlib.RegisterConnConfig(pgCfg)
		}
	case "sqlite":
		driverName = "sqlite"
		dataSourceName = cfg.Database.Path
	default:
		return nil, errors.New("invalid database type, must be one of: postgres, sqlite")
	}
	dbx, err := sqlx.ConnectContext(ctx, driverName, dataSourceName)
	if err != nil {
		return nil, err
	}

	// execute schema
	if _, err = dbx.ExecContext(ctx, schema); err != nil {
		return nil, err
	}

	cleanupContext, cancel := context.WithCancel(context.Background())
	db := &Database{
		DB:            dbx,
		cleanupCancel: cancel,
	}

	go db.cleanup(cleanupContext, cfg.CleanupInterval, cfg.ExpireAfter)

	return db, nil
}

type Document struct {
	ID          string `db:"id"`
	Version     int64  `db:"version"`
	Content     string `db:"content"`
	Language    string `db:"language"`
	UpdateToken string `db:"update_token"`
}

type Database struct {
	*sqlx.DB
	cleanupCancel context.CancelFunc
}

func (d *Database) Close() error {
	return d.Close()
}

func (d *Database) GetDocument(ctx context.Context, id string) (Document, error) {
	var doc Document
	err := d.GetContext(ctx, &doc, "SELECT * FROM documents WHERE id = $1", id)
	return doc, err
}

func (d *Database) GetDocumentVersions(ctx context.Context, id string, withContent bool) ([]Document, error) {
	var docs []Document
	var sqlString string
	if withContent {
		sqlString = "SELECT id, version, content, language FROM documents where id = $1 ORDER BY version DESC"
	} else {
		sqlString = "SELECT id, version FROM documents where id = $1 ORDER BY version DESC"
	}
	err := d.SelectContext(ctx, &docs, sqlString, id)
	return docs, err
}

func (d *Database) GetDocumentByVersion(ctx context.Context, id string, version int64) (Document, error) {
	var doc Document
	err := d.GetContext(ctx, &doc, "SELECT * FROM documents WHERE id = $1 AND version = $2", id, version)
	return doc, err
}

func (d *Database) DeleteDocumentByVersion(ctx context.Context, version int64, documentID string) error {
	res, err := d.ExecContext(ctx, "DELETE FROM documents WHERE version = $1 AND id = $2", version, documentID)
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

func (d *Database) CreateDocument(ctx context.Context, content string, language string) (Document, error) {
	return d.createDocument(ctx, content, language, 0)
}

func (d *Database) createDocument(ctx context.Context, content string, language string, try int) (Document, error) {
	if try >= 10 {
		return Document{}, errors.New("failed to create document because of duplicate key after 10 tries")
	}
	now := time.Now().Unix()
	doc := Document{
		ID:          randomString(8),
		Content:     content,
		Language:    language,
		UpdateToken: randomString(32),
		Version:     now,
	}
	_, err := d.NamedExecContext(ctx, "INSERT INTO documents (id, version, content, language, update_token) VALUES (:id, :version, :content, :language, :update_token)", doc)

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

func (d *Database) UpdateDocument(ctx context.Context, id string, updateToken string, content string, language string) (Document, error) {
	doc := Document{
		ID:          id,
		Version:     time.Now().Unix(),
		Content:     content,
		Language:    language,
		UpdateToken: updateToken,
	}
	// TODO:this should do an insert with a new version now
	res, err := d.NamedExecContext(ctx, "INSERT INTO documents (id, version, content, language, update_token) VALUES (:id, :version, :content, :language, :update_token)", doc)
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

func (d *Database) DeleteDocument(ctx context.Context, id string, updateToken string) error {
	res, err := d.ExecContext(ctx, "DELETE FROM documents WHERE id = $1 AND update_token = $2", id, updateToken)
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

func (d *Database) DeleteExpiredDocuments(ctx context.Context, expireAfter time.Duration) error {
	_, err := d.ExecContext(ctx, "DELETE FROM documents WHERE updated_at < $1", time.Now().Add(expireAfter))
	return err
}

func (d *Database) cleanup(ctx context.Context, cleanUpInterval time.Duration, expireAfter time.Duration) {
	if expireAfter <= 0 {
		return
	}
	if cleanUpInterval <= 0 {
		cleanUpInterval = 10 * time.Minute
	}
	log.Println("Starting document cleanup...")
	ticker := time.NewTicker(cleanUpInterval)
	defer ticker.Stop()
	defer log.Println("document cleanup stopped")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := d.DeleteExpiredDocuments(ctx, expireAfter)
			if errors.Is(err, context.Canceled) {
				return
			}
			if err != nil {
				log.Println("failed to delete expired documents:", err)
			}
		}
	}
}

func randomString(length int) string {
	b := make([]rune, length)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}
