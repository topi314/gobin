package gobin

import (
	"fmt"
	"log/slog"
	"strings"
	"time"
)

type Config struct {
	Log              LogConfig        `cfg:"log"`
	Debug            bool             `cfg:"debug"`
	DevMode          bool             `cfg:"dev_mode"`
	ListenAddr       string           `cfg:"listen_addr"`
	HTTPTimeout      time.Duration    `cfg:"http_timeout"`
	Database         DatabaseConfig   `cfg:"database"`
	MaxDocumentSize  int              `cfg:"max_document_size"`
	MaxHighlightSize int              `cfg:"max_highlight_size"`
	RateLimit        *RateLimitConfig `cfg:"rate_limit"`
	JWTSecret        string           `cfg:"jwt_secret"`
	Preview          *PreviewConfig   `cfg:"preview"`
	Otel             *OtelConfig      `cfg:"otel"`
}

func (c Config) String() string {
	return fmt.Sprintf("\n Log: %s\n Debug: %t\n DevMode: %t\n ListenAddr: %s\n HTTPTimeout: %s\n Database: %s\n MaxDocumentSize: %d\n MaxHighlightSize: %d\n RateLimit: %s\n JWTSecret: %s\n Preview: %s\n Otel: %s\n",
		c.Log,
		c.Debug,
		c.DevMode,
		c.ListenAddr,
		c.HTTPTimeout,
		c.Database,
		c.MaxDocumentSize,
		c.MaxHighlightSize,
		c.RateLimit, strings.Repeat("*", len(c.JWTSecret)),
		c.Preview,
		c.Otel,
	)
}

type LogConfig struct {
	Level     slog.Level `cfg:"level"`
	Format    string     `cfg:"format"`
	AddSource bool       `cfg:"add_source"`
	NoColor   bool       `cfg:"no_color"`
}

func (c LogConfig) String() string {
	return fmt.Sprintf("\n  Level: %s\n  Format: %s\n  AddSource: %t\n  NoColor: %t\n",
		c.Level,
		c.Format,
		c.AddSource,
		c.NoColor,
	)
}

type DatabaseConfig struct {
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

func (c DatabaseConfig) String() string {
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

func (c DatabaseConfig) PostgresDataSourceName() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host,
		c.Port,
		c.Username,
		c.Password,
		c.Database,
		c.SSLMode,
	)
}

type RateLimitConfig struct {
	Requests  int           `cfg:"requests"`
	Duration  time.Duration `cfg:"duration"`
	Whitelist []string      `cfg:"whitelist"`
	Blacklist []string      `cfg:"blacklist"`
}

func (c RateLimitConfig) String() string {
	return fmt.Sprintf("\n  Requests: %d\n  Duration: %s\n  Whitelist: %v\n  Blacklist: %v",
		c.Requests,
		c.Duration,
		c.Whitelist,
		c.Blacklist,
	)
}

type PreviewConfig struct {
	InkscapePath string        `cfg:"inkscape_path"`
	MaxLines     int           `cfg:"max_lines"`
	DPI          int           `cfg:"dpi"`
	CacheSize    int           `cfg:"cache_size"`
	CacheTTL     time.Duration `cfg:"cache_ttl"`
}

func (c PreviewConfig) String() string {
	return fmt.Sprintf("\n  InkscapePath: %s\n  MaxLines: %d\n  DPI: %d\n  CacheSize: %d\n  CacheTTL: %s",
		c.InkscapePath,
		c.MaxLines,
		c.DPI,
		c.CacheSize,
		c.CacheTTL,
	)
}

type OtelConfig struct {
	InstanceID string         `cfg:"instance_id"`
	Trace      *TraceConfig   `cfg:"trace"`
	Metrics    *MetricsConfig `cfg:"metrics"`
}

func (c OtelConfig) String() string {
	return fmt.Sprintf("\n  InstanceID: %s\n  Trace: %s\n  Metrics: %s",
		c.InstanceID,
		c.Trace,
		c.Metrics,
	)
}

type TraceConfig struct {
	Endpoint string `cfg:"endpoint"`
	Insecure bool   `cfg:"insecure"`
}

func (c TraceConfig) String() string {
	return fmt.Sprintf("\n   Endpoint: %s\n   Insecure: %t",
		c.Endpoint,
		c.Insecure,
	)
}

type MetricsConfig struct {
	ListenAddr string `cfg:"listen_addr"`
}

func (c MetricsConfig) String() string {
	return fmt.Sprintf("\n   ListenAddr: %s",
		c.ListenAddr,
	)
}
