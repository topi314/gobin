package main

import (
	"context"
	"errors"
	"log"
	"math/rand"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

var chars = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

func init() {
	rand.Seed(time.Now().UnixNano())
}

type Document struct {
	ID          string    `db:"id"`
	Content     string    `db:"content"`
	UpdateToken string    `db:"update_token"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

type UpdateDocument struct {
	Document
	PreviousUpdateToken string `db:"previous_update_token"`
}

func NewDatabase(cfg Config) (*Database, error) {
	dbx, err := sqlx.Connect("pgx", cfg.Database.DataSourceName())
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	db := &Database{
		DB:     dbx,
		cancel: cancel,
	}

	go db.cleanup(ctx, cfg.CleanupInterval, cfg.ExpireAfter)

	return db, nil
}

type Database struct {
	*sqlx.DB
	cancel context.CancelFunc
}

func (d *Database) Close() error {
	return d.Close()
}

func (d *Database) GetDocument(ctx context.Context, id string) (Document, error) {
	var doc Document
	err := d.GetContext(ctx, &doc, "SELECT * FROM documents WHERE id = $1", id)
	return doc, err
}

func (d *Database) CreateDocument(ctx context.Context, content string) (Document, error) {
	now := time.Now()
	doc := Document{
		ID:          randomString(8),
		Content:     content,
		UpdateToken: randomString(32),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	_, err := d.NamedExecContext(ctx, "INSERT INTO documents (id, content, update_token, created_at, updated_at) VALUES (:id, :content, :update_token, :created_at, :updated_at)", doc)
	return doc, err
}

func (d *Database) UpdateDocument(ctx context.Context, id string, updateToken string, content string) (Document, error) {
	doc := UpdateDocument{
		Document: Document{
			ID:          id,
			Content:     content,
			UpdateToken: randomString(32),
			UpdatedAt:   time.Now(),
		},
		PreviousUpdateToken: updateToken,
	}
	_, err := d.NamedExecContext(ctx, "UPDATE documents SET content = :content, update_token = :update_token, updated_at = :updated_at WHERE id = :id AND update_token = :previous_update_token", doc)
	return doc.Document, err
}

func (d *Database) DeleteDocument(ctx context.Context, id string, updateToken string) error {
	_, err := d.ExecContext(ctx, "DELETE FROM documents WHERE id = $1 AND update_token = $2", id, updateToken)
	return err
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
