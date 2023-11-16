package database

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"time"

	"github.com/XSAM/otelsql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jackc/pgx/v5/tracelog"
	"github.com/jmoiron/sqlx"
	"github.com/topi314/tint"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

var chars = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

type Config struct {
	Type            string        `cfg:"type"`
	Debug           bool          `cfg:"debug"`
	ExpireAfter     time.Duration `cfg:"expire_after"`
	CleanupInterval time.Duration `cfg:"cleanup_interval"`

	// SQLite
	Path string `cfg:"path"`

	// PostgreSQL
	Host     string `cfg:"host"`
	Port     int    `cfg:"port"`
	Username string `cfg:"username"`
	Password string `cfg:"password"`
	Database string `cfg:"database"`
	SSLMode  string `cfg:"ssl_mode"`
}

func (c Config) String() string {
	str := fmt.Sprintf("\n  Type: %s\n  Debug: %t\n  ExpireAfter: %s\n  CleanupInterval: %s\n  ",
		c.Type,
		c.Debug,
		c.ExpireAfter,
		c.CleanupInterval,
	)
	switch c.Type {
	case "postgres":
		str += fmt.Sprintf("Host: %s\n  Port: %d\n  Username: %s\n  Password: %s\n  Database: %s\n  SSLMode: %s",
			c.Host,
			c.Port,
			c.Username,
			strings.Repeat("*", len(c.Password)),
			c.Database,
			c.SSLMode,
		)
	case "sqlite":
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

func New(ctx context.Context, cfg Config, schema string) (*DB, error) {
	var (
		driverName     string
		dataSourceName string
		dbSystem       attribute.KeyValue
	)
	switch cfg.Type {
	case "postgres":
		driverName = "pgx"
		dbSystem = semconv.DBSystemPostgreSQL
		pgCfg, err := pgx.ParseConfig(cfg.PostgresDataSourceName())
		if err != nil {
			return nil, err
		}

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
	case "sqlite":
		driverName = "sqlite"
		dbSystem = semconv.DBSystemSqlite
		dataSourceName = cfg.Path
	default:
		return nil, errors.New("invalid database type, must be one of: postgres, sqlite")
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
	// execute schema
	if _, err = dbx.ExecContext(ctx, schema); err != nil {
		return nil, fmt.Errorf("failed to execute schema: %w", err)
	}

	cleanupContext, cancel := context.WithCancel(context.Background())
	db := &DB{
		dbx:           dbx,
		cleanupCancel: cancel,
		rand:          rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	go db.cleanup(cleanupContext, cfg.CleanupInterval, cfg.ExpireAfter)

	return db, nil
}

type DB struct {
	dbx           *sqlx.DB
	cleanupCancel context.CancelFunc
	rand          *rand.Rand
	tracer        trace.Tracer
}

func (d *DB) Close() error {
	return d.dbx.Close()
}

func (d *DB) cleanup(ctx context.Context, cleanUpInterval time.Duration, expireAfter time.Duration) {
	if expireAfter <= 0 {
		return
	}
	if cleanUpInterval <= 0 {
		cleanUpInterval = 10 * time.Minute
	}
	slog.Info("Starting document cleanup...")
	ticker := time.NewTicker(cleanUpInterval)
	defer func() {
		ticker.Stop()
		slog.Info("document cleanup stopped")
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.doCleanup(expireAfter)
		}
	}
}

func (d *DB) doCleanup(expireAfter time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ctx, span := d.tracer.Start(ctx, "doCleanup")
	defer span.End()

	if err := d.DeleteExpiredDocuments(ctx, expireAfter); err != nil && !errors.Is(err, context.Canceled) {
		span.SetStatus(codes.Error, "failed to delete expired documents")
		span.RecordError(err)
		slog.ErrorContext(ctx, "failed to delete expired documents", tint.Err(err))
	}
}

func (d *DB) randomString(length int) string {
	b := make([]rune, length)
	for i := range b {
		b[i] = chars[d.rand.Intn(len(chars))]
	}
	return string(b)
}
