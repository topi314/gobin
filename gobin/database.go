package gobin

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"log"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jackc/pgx/v5/tracelog"
	"github.com/jmoiron/sqlx"
	"modernc.org/sqlite"
	_ "modernc.org/sqlite"
)

var chars = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

func init() {
	rand.Seed(time.Now().UnixNano())
}

func NewDB(ctx context.Context, cfg DatabaseConfig, schema string) (*DB, error) {
	var (
		driverName     string
		dataSourceName string
	)
	switch cfg.Type {
	case "postgres":
		driverName = "pgx"
		pgCfg, err := pgx.ParseConfig(cfg.PostgresDataSourceName())
		if err != nil {
			return nil, err
		}

		if cfg.Debug {
			pgCfg.Tracer = &tracelog.TraceLog{
				Logger: tracelog.LoggerFunc(func(ctx context.Context, level tracelog.LogLevel, msg string, data map[string]any) {
					log.Println(msg, data)
				}),
				LogLevel: tracelog.LogLevelDebug,
			}
		}
		dataSourceName = stdlib.RegisterConnConfig(pgCfg)
	case "sqlite":
		driverName = "sqlite"
		dataSourceName = cfg.Path
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
	db := &DB{
		dbx:           dbx,
		cleanupCancel: cancel,
	}

	go db.cleanup(cleanupContext, cfg.CleanupInterval, cfg.ExpireAfter)

	return db, nil
}

type Document struct {
	ID       string `db:"id"`
	Version  int64  `db:"version"`
	Content  string `db:"content"`
	Language string `db:"language"`
}

type DB struct {
	dbx           *sqlx.DB
	cleanupCancel context.CancelFunc
}

func (d *DB) Close() error {
	return d.dbx.Close()
}

func (d *DB) GetDocument(ctx context.Context, documentID string) (Document, error) {
	var doc Document
	err := d.dbx.GetContext(ctx, &doc, "SELECT * FROM documents WHERE id = $1 ORDER BY version DESC LIMIT 1", documentID)
	return doc, err
}

func (d *DB) GetDocumentVersion(ctx context.Context, documentID string, version int64) (Document, error) {
	var doc Document
	err := d.dbx.GetContext(ctx, &doc, "SELECT * FROM documents WHERE id = $1 AND version = $2", documentID, version)
	return doc, err
}

func (d *DB) GetDocumentVersions(ctx context.Context, documentID string, withContent bool) ([]Document, error) {
	var docs []Document
	var sqlString string
	if withContent {
		sqlString = "SELECT id, version, content, language FROM documents where id = $1 ORDER BY version DESC"
	} else {
		sqlString = "SELECT id, version FROM documents where id = $1 ORDER BY version DESC"
	}
	err := d.dbx.SelectContext(ctx, &docs, sqlString, documentID)
	return docs, err
}

func (d *DB) DeleteDocumentByVersion(ctx context.Context, documentID string, version int64) error {
	res, err := d.dbx.ExecContext(ctx, "DELETE FROM documents WHERE id = $1 AND version = $2", documentID, version)
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
		ID:       randomString(8),
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

func (d *DB) DeleteDocument(ctx context.Context, documentID string) error {
	res, err := d.dbx.ExecContext(ctx, "DELETE FROM documents WHERE id = $1", documentID)
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

func (d *DB) DeleteExpiredDocuments(ctx context.Context, expireAfter time.Duration) error {
	_, err := d.dbx.ExecContext(ctx, "DELETE FROM documents WHERE version < $1", time.Now().Add(expireAfter).Unix())
	return err
}

func (d *DB) cleanup(ctx context.Context, cleanUpInterval time.Duration, expireAfter time.Duration) {
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
