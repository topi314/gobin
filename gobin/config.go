package gobin

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/topi314/gobin/v2/gobin/database"
)

type Config struct {
	Log              LogConfig        `cfg:"log"`
	Debug            bool             `cfg:"debug"`
	DevMode          bool             `cfg:"dev_mode"`
	ListenAddr       string           `cfg:"listen_addr"`
	HTTPTimeout      time.Duration    `cfg:"http_timeout"`
	Database         database.Config  `cfg:"database"`
	MaxDocumentSize  int              `cfg:"max_document_size"`
	MaxHighlightSize int              `cfg:"max_highlight_size"`
	RateLimit        *RateLimitConfig `cfg:"rate_limit"`
	JWTSecret        string           `cfg:"jwt_secret"`
	Preview          *PreviewConfig   `cfg:"preview"`
	Otel             *OtelConfig      `cfg:"otel"`
	Webhook          *WebhookConfig   `cfg:"webhook"`
	CustomStyles     string           `cfg:"custom_styles"`
	DefaultStyle     string           `cfg:"default_style"`
}

func (c Config) String() string {
	return fmt.Sprintf("\n Log: %s\n Debug: %t\n DevMode: %t\n ListenAddr: %s\n HTTPTimeout: %s\n Database: %s\n MaxDocumentSize: %d\n MaxHighlightSize: %d\n RateLimit: %s\n JWTSecret: %s\n Preview: %s\n Otel: %s\n Webhook: %s\n CustomStyles: %s\n DefaultStyle: %s\n",
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
		c.Webhook,
		c.CustomStyles,
		c.DefaultStyle,
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

type WebhookConfig struct {
	Timeout       time.Duration `cfg:"timeout"`
	MaxTries      int           `cfg:"max_tries"`
	Backoff       time.Duration `cfg:"backoff"`
	BackoffFactor float64       `cfg:"backoff_factor"`
	MaxBackoff    time.Duration `cfg:"max_backoff"`
}

func (c WebhookConfig) String() string {
	return fmt.Sprintf("\n  Timeout: %s\n  MaxTries: %d\n  Backoff: %s\n  BackoffFactor: %f\n  MaxBackoff: %s",
		c.Timeout,
		c.MaxTries,
		c.Backoff,
		c.BackoffFactor,
		c.MaxBackoff,
	)
}
