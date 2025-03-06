package server

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/topi314/gobin/v3/internal/timex"
	"github.com/topi314/gobin/v3/server/database"
)

func LoadConfig(cfgPath string) (Config, error) {
	file, err := os.Open(cfgPath)
	if err != nil {
		return Config{}, fmt.Errorf("failed to open config file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	cfg := defaultConfig()
	if err = toml.NewDecoder(file).Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("failed to decode config file: %w", err)
	}

	return cfg, nil
}

func defaultConfig() Config {
	return Config{
		Debug:            false,
		DevMode:          false,
		ListenAddr:       ":80",
		HTTPTimeout:      timex.Duration(30 * time.Second),
		JWTSecret:        "",
		MaxDocumentSize:  0,
		MaxHighlightSize: 0,
		CustomStyles:     "",
		DefaultStyle:     "onedark",
		Database: database.Config{
			Type:            database.TypeSQLite,
			Debug:           false,
			ExpireAfter:     0,
			CleanupInterval: timex.Duration(time.Minute),
			Path:            "gobin.db",
			Host:            "localhost",
			Port:            5432,
			Username:        "gobin",
			Password:        "",
			Database:        "gobin",
			SSLMode:         "disable",
		},
		Log: LogConfig{
			Level:     slog.LevelInfo,
			Format:    LogFormatText,
			AddSource: false,
			NoColor:   false,
		},
		RateLimit: RateLimitConfig{
			Enabled:   false,
			Requests:  10,
			Duration:  timex.Duration(time.Minute),
			Whitelist: []string{"127.0.0.1"},
			Blacklist: nil,
		},
		Preview: PreviewConfig{
			Enabled:      false,
			InkscapePath: "inkscape",
			MaxLines:     0,
			DPI:          120,
			CacheSize:    1024,
			CacheTTL:     timex.Duration(time.Hour),
		},
		Otel: OtelConfig{
			Enabled:    false,
			InstanceID: "1",
			Trace: TraceConfig{
				Enabled:  false,
				Endpoint: "localhost:4318",
				Insecure: false,
			},
			Metrics: MetricsConfig{
				Enabled:    false,
				ListenAddr: ":8080",
			},
		},
		Webhook: WebhookConfig{
			Timeout:       timex.Duration(10 * time.Second),
			MaxTries:      3,
			Backoff:       timex.Duration(time.Second),
			BackoffFactor: 2,
			MaxBackoff:    timex.Duration(5 * time.Minute),
		},
	}
}

type Config struct {
	Debug            bool            `toml:"debug"`
	DevMode          bool            `toml:"dev_mode"`
	ListenAddr       string          `toml:"listen_addr"`
	HTTPTimeout      timex.Duration  `toml:"http_timeout"`
	JWTSecret        string          `toml:"jwt_secret"`
	MaxDocumentSize  int64           `toml:"max_document_size"`
	MaxHighlightSize int             `toml:"max_highlight_size"`
	CustomStyles     string          `toml:"custom_styles"`
	DefaultStyle     string          `toml:"default_style"`
	Log              LogConfig       `toml:"log"`
	Database         database.Config `toml:"database"`
	RateLimit        RateLimitConfig `toml:"rate_limit"`
	Preview          PreviewConfig   `toml:"preview"`
	Otel             OtelConfig      `toml:"otel"`
	Webhook          WebhookConfig   `toml:"webhook"`
}

func (c Config) String() string {
	return fmt.Sprintf("Debug: %t\nDevMode: %t\nListenAddr: %s\nHTTPTimeout: %s\nJWTSecret: %s\nMaxDocumentSize: %d\nMaxHighlightSize: %d\nCustomStyles: %s\nDefaultStyle: %s\nLog: %s\nDatabase: %s\nRateLimit: %s\nPreview: %s\nOtel: %s\nWebhook: %s",
		c.Debug,
		c.DevMode,
		c.ListenAddr,
		time.Duration(c.HTTPTimeout),
		strings.Repeat("*", len(c.JWTSecret)),
		c.MaxDocumentSize,
		c.MaxHighlightSize,
		c.CustomStyles,
		c.DefaultStyle,
		c.Log,
		c.Database,
		c.RateLimit,
		c.Preview,
		c.Otel,
		c.Webhook,
	)
}

type LogFormat string

const (
	LogFormatJSON LogFormat = "json"
	LogFormatText LogFormat = "text"
)

type LogConfig struct {
	Level     slog.Level `toml:"level"`
	Format    LogFormat  `toml:"format"`
	AddSource bool       `toml:"add_source"`
	NoColor   bool       `toml:"no_color"`
}

func (c LogConfig) String() string {
	return fmt.Sprintf("\n Level: %s\n Format: %s\n AddSource: %t\n NoColor: %t",
		c.Level,
		c.Format,
		c.AddSource,
		c.NoColor,
	)
}

type RateLimitConfig struct {
	Enabled   bool           `toml:"enabled"`
	Requests  int            `toml:"requests"`
	Duration  timex.Duration `toml:"duration"`
	Whitelist []string       `toml:"whitelist"`
	Blacklist []string       `toml:"blacklist"`
}

func (c RateLimitConfig) String() string {
	return fmt.Sprintf("\n Enabled: %t\n Requests: %d\n Duration: %s\n Whitelist: %v\n Blacklist: %v",
		c.Enabled,
		c.Requests,
		time.Duration(c.Duration),
		c.Whitelist,
		c.Blacklist,
	)
}

type PreviewConfig struct {
	Enabled      bool           `toml:"enabled"`
	InkscapePath string         `toml:"inkscape_path"`
	MaxLines     int            `toml:"max_lines"`
	DPI          int            `toml:"dpi"`
	CacheSize    int            `toml:"cache_size"`
	CacheTTL     timex.Duration `toml:"cache_ttl"`
}

func (c PreviewConfig) String() string {
	return fmt.Sprintf("\n Enabled: %t\n InkscapePath: %s\n MaxLines: %d\n DPI: %d\n CacheSize: %d\n CacheTTL: %s",
		c.Enabled,
		c.InkscapePath,
		c.MaxLines,
		c.DPI,
		c.CacheSize,
		time.Duration(c.CacheTTL),
	)
}

type OtelConfig struct {
	Enabled    bool          `toml:"enabled"`
	InstanceID string        `toml:"instance_id"`
	Trace      TraceConfig   `toml:"trace"`
	Metrics    MetricsConfig `toml:"metrics"`
}

func (c OtelConfig) String() string {
	return fmt.Sprintf("\n Enabled: %t\n InstanceID: %s\n Trace: %s\n Metrics: %s",
		c.Enabled,
		c.InstanceID,
		c.Trace,
		c.Metrics,
	)
}

type TraceConfig struct {
	Enabled  bool   `toml:"enabled"`
	Endpoint string `toml:"endpoint"`
	Insecure bool   `toml:"insecure"`
}

func (c TraceConfig) String() string {
	return fmt.Sprintf("\n  Enabled: %t\n  Endpoint: %s\n  Insecure: %t",
		c.Enabled,
		c.Endpoint,
		c.Insecure,
	)
}

type MetricsConfig struct {
	Enabled    bool   `toml:"enabled"`
	ListenAddr string `toml:"listen_addr"`
}

func (c MetricsConfig) String() string {
	return fmt.Sprintf("\n  Enabled: %t\n  ListenAddr: %s",
		c.Enabled,
		c.ListenAddr,
	)
}

type WebhookConfig struct {
	Enabled       bool           `toml:"enabled"`
	Timeout       timex.Duration `toml:"timeout"`
	MaxTries      int            `toml:"max_tries"`
	Backoff       timex.Duration `toml:"backoff"`
	BackoffFactor float64        `toml:"backoff_factor"`
	MaxBackoff    timex.Duration `toml:"max_backoff"`
}

func (c WebhookConfig) String() string {
	return fmt.Sprintf("\n Enabled: %t\n Timeout: %s\n MaxTries: %d\n Backoff: %s\n BackoffFactor: %f\n MaxBackoff: %s",
		c.Enabled,
		time.Duration(c.Timeout),
		c.MaxTries,
		time.Duration(c.Backoff),
		c.BackoffFactor,
		time.Duration(c.MaxBackoff),
	)
}
