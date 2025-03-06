package database

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"math/rand"
	"strings"
	"time"

	"github.com/XSAM/otelsql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jackc/pgx/v5/tracelog"
	"github.com/jmoiron/sqlx"
	"github.com/topi314/gomigrate"
	"github.com/topi314/gomigrate/drivers/postgres"
	"github.com/topi314/gomigrate/drivers/sqlite"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/semconv/v1.25.0"
	_ "modernc.org/sqlite"

	"github.com/topi314/gobin/v3/internal/timex"
)

var (
	chars = []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	r     = rand.New(rand.NewSource(time.Now().UnixNano()))
)

type Type string

const (
	TypePostgres Type = "postgres"
	TypeSQLite   Type = "sqlite"
)

type Config struct {
	Type            Type           `toml:"type"`
	Debug           bool           `toml:"debug"`
	ExpireAfter     timex.Duration `toml:"expire_after"`
	CleanupInterval timex.Duration `toml:"cleanup_interval"`

	// SQLite
	Path string `toml:"path"`

	// PostgreSQL
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	Username string `toml:"username"`
	Password string `toml:"password"`
	Database string `toml:"database"`
	SSLMode  string `toml:"ssl_mode"`
}

func (c Config) String() string {
	str := fmt.Sprintf("\n  Type: %s\n  Debug: %t\n  ExpireAfter: %s\n  CleanupInterval: %s\n  ",
		c.Type,
		c.Debug,
		time.Duration(c.ExpireAfter),
		time.Duration(c.CleanupInterval),
	)
	switch c.Type {
	case TypePostgres:
		str += fmt.Sprintf("Host: %s\n  Port: %d\n  Username: %s\n  Password: %s\n  Database: %s\n  SSLMode: %s",
			c.Host,
			c.Port,
			c.Username,
			strings.Repeat("*", len(c.Password)),
			c.Database,
			c.SSLMode,
		)
	case TypeSQLite:
		str += fmt.Sprintf("Path: %s", c.Path)
	default:
		str += "Invalid database type!"
	}
	return str
}

func (c Config) PostgresDataSourceName() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host,
		c.Port,
		c.Username,
		c.Password,
		c.Database,
		c.SSLMode,
	)
}

func New(ctx context.Context, cfg Config, migrations fs.FS) (DB, error) {
	var (
		driverName      string
		dataSourceName  string
		dbSystem        attribute.KeyValue
		migrationDriver gomigrate.NewDriver
	)
	switch cfg.Type {
	case TypePostgres:
		driverName = "pgx"
		dbSystem = semconv.DBSystemPostgreSQL
		pgCfg, err := pgx.ParseConfig(cfg.PostgresDataSourceName())
		if err != nil {
			return nil, err
		}
		migrationDriver = postgres.New

		if cfg.Debug {
			pgCfg.Tracer = &tracelog.TraceLog{
				Logger: tracelog.LoggerFunc(func(ctx context.Context, level tracelog.LogLevel, msg string, data map[string]any) {
					args := make([]any, 0, len(data))
					for k, v := range data {
						args = append(args, slog.Any(k, v))
					}
					slog.DebugContext(ctx, msg, slog.Group("data", args...))
				}),
				LogLevel: tracelog.LogLevelDebug,
			}
		}
		dataSourceName = stdlib.RegisterConnConfig(pgCfg)
	case TypeSQLite:
		driverName = "sqliteDB"
		dbSystem = semconv.DBSystemSqlite
		dataSourceName = cfg.Path
		migrationDriver = sqlite.New
	default:
		return nil, errors.New("invalid database type, must be one of: postgresDB, sqliteDB")
	}

	sqlDB, err := otelsql.Open(driverName, dataSourceName,
		otelsql.WithAttributes(dbSystem),
		otelsql.WithSQLCommenter(true),
		otelsql.WithAttributesGetter(func(ctx context.Context, method otelsql.Method, query string, args []driver.NamedValue) []attribute.KeyValue {
			return []attribute.KeyValue{
				semconv.DBOperationKey.String(string(method)),
				semconv.DBStatementKey.String(query),
			}
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err = otelsql.RegisterDBStatsMetrics(sqlDB, otelsql.WithAttributes(dbSystem)); err != nil {
		return nil, fmt.Errorf("failed to register database stats metrics: %w", err)
	}

	dbx := sqlx.NewDb(sqlDB, driverName)
	if err = dbx.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if err = gomigrate.Migrate(ctx, dbx, migrationDriver, migrations, gomigrate.WithDirectory("server/migrations")); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	switch cfg.Type {
	case TypePostgres:
		return newPostgresDB(dbx), nil
	case TypeSQLite:
		return newSQLiteDB(dbx), nil
	default:
		return nil, errors.New("invalid database type, must be one of: postgresDB, sqliteDB")
	}
}

type DB interface {
	gomigrate.Queryer

	GetDocument(ctx context.Context, documentID string) ([]File, error)
	GetDocumentVersion(ctx context.Context, documentID string, documentVersion int64) ([]File, error)
	GetVersionCount(ctx context.Context, documentID string) (int, error)
	GetDocumentVersions(ctx context.Context, documentID string) ([]int64, error)
	GetDocumentVersionsWithFiles(ctx context.Context, documentID string, withContent bool) (map[int64][]File, error)
	CreateDocument(ctx context.Context, files []File) (*string, *int64, error)
	UpdateDocument(ctx context.Context, documentID string, files []File) (*int64, error)
	DeleteDocument(ctx context.Context, documentID string) (*Document, error)
	DeleteDocumentVersion(ctx context.Context, documentID string, documentVersion int64) (*Document, error)
	DeleteDocumentVersions(ctx context.Context, documentID string) error
	DeleteExpiredDocuments(ctx context.Context, expireAfter time.Duration) ([]Document, error)

	GetDocumentFile(ctx context.Context, documentID string, fileName string) (*File, error)
	GetDocumentFileVersion(ctx context.Context, documentID string, documentVersion int64, fileName string) (*File, error)
	DeleteDocumentFile(ctx context.Context, documentID string, fileName string) error
	DeleteDocumentVersionFile(ctx context.Context, documentID string, documentVersion int64, fileName string) error

	GetWebhook(ctx context.Context, documentID string, webhookID string, secret string) (*Webhook, error)
	GetWebhooksByDocumentID(ctx context.Context, documentID string) ([]Webhook, error)
	GetAndDeleteWebhooksByDocumentID(ctx context.Context, documentID string) ([]Webhook, error)
	CreateWebhook(ctx context.Context, documentID string, url string, secret string, events []string) (*Webhook, error)
	UpdateWebhook(ctx context.Context, documentID string, webhookID string, secret string, newURL string, newSecret string, newEvents []string) (*Webhook, error)
	DeleteWebhook(ctx context.Context, documentID string, webhookID string, secret string) error

	Close() error
}

func randomString(length int) string {
	b := make([]rune, length)
	for i := range b {
		b[i] = chars[r.Intn(len(chars))]
	}
	return string(b)
}
