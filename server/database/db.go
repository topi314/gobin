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
	"github.com/topi314/gomigrate"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/semconv/v1.25.0"
	_ "modernc.org/sqlite"

	"github.com/topi314/gobin/v2/internal/timex"
)

var chars = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

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

var _ gomigrate.Queryer = (*DB)(nil)

func New(ctx context.Context, cfg Config) (*DB, error) {
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

	return &DB{
		DB:   dbx,
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}, nil
}

type DB struct {
	*sqlx.DB
	rand *rand.Rand
}

func (d *DB) randomString(length int) string {
	b := make([]rune, length)
	for i := range b {
		b[i] = chars[d.rand.Intn(len(chars))]
	}
	return string(b)
}
