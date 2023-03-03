package gobin

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	DevMode         bool             `json:"dev_mode"`
	Debug           bool             `json:"debug"`
	ListenAddr      string           `json:"listen_addr"`
	Database        DatabaseConfig   `json:"database"`
	MaxDocumentSize int              `json:"max_document_size"`
	RateLimit       *RateLimitConfig `json:"rate_limit"`
}

func (c Config) String() string {
	return fmt.Sprintf("\n DevMode: %t\n Debug: %t\n ListenAddr: %s\n Database: %s\n MaxDocumentSize: %d\n Rate Limit: %s\n", c.DevMode, c.Debug, c.ListenAddr, c.Database, c.MaxDocumentSize, c.RateLimit)
}

type DatabaseConfig struct {
	Type            string        `json:"type"`
	Debug           bool          `json:"debug"`
	ExpireAfter     time.Duration `json:"expire_after"`
	CleanupInterval time.Duration `json:"cleanup_interval"`

	// SQLite
	Path string `json:"path"`

	// PostgreSQL
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Database string `json:"database"`
	SSLMode  string `json:"ssl_mode"`
}

func (c *DatabaseConfig) UnmarshalJSON(data []byte) error {
	type config DatabaseConfig
	v := struct {
		ExpireAfter     string `json:"expire_after"`
		CleanupInterval string `json:"cleanup_interval"`
		*config
	}{
		config: (*config)(c),
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}
	if v.ExpireAfter != "" {
		d1, err := time.ParseDuration(v.ExpireAfter)
		if err != nil {
			return fmt.Errorf("failed to parse expire after duration: %w", err)
		}
		c.ExpireAfter = d1
	}

	if v.CleanupInterval != "" {
		d2, err := time.ParseDuration(v.CleanupInterval)
		if err != nil {
			return fmt.Errorf("failed to parse cleanup interval duration: %w", err)
		}
		c.CleanupInterval = d2
	}
	return nil
}

func (c DatabaseConfig) String() string {
	str := fmt.Sprintf("\n  Type: %s\n  Debug: %t\n  ExpireAfter: %s\n  CleanupInterval: %s\n  ", c.Type, c.Debug, c.ExpireAfter, c.CleanupInterval)
	switch c.Type {
	case "postgres":
		str += fmt.Sprintf("Host: %s\n  Port: %d\n  Username: %s\n  Password: %s\n  Database: %s\n  SSLMode: %s", c.Host, c.Port, c.Username, strings.Repeat("*", len(c.Password)), c.Database, c.SSLMode)
	case "sqlite":
		str += fmt.Sprintf("Path: %s", c.Path)
	default:
		str += "Invalid database type!"
	}
	return str
}

func (c DatabaseConfig) PostgresDataSourceName() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s", c.Host, c.Port, c.Username, c.Password, c.Database, c.SSLMode)
}

type RateLimitConfig struct {
	Requests int           `json:"requests"`
	Duration time.Duration `json:"duration"`
}

func (c *RateLimitConfig) UnmarshalJSON(data []byte) error {
	var v struct {
		Requests int    `json:"requests"`
		Duration string `json:"duration"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("failed to unmarshal rate limit config: %w", err)
	}
	if v.Duration != "" {
		duration, err := time.ParseDuration(v.Duration)
		if err != nil {
			return fmt.Errorf("failed to parse rate limit duration: %w", err)
		}
		c.Duration = duration
	}
	c.Requests = v.Requests

	return nil
}

func (c RateLimitConfig) String() string {
	return fmt.Sprintf("\n  Requests: %d\n  Duration: %s", c.Requests, c.Duration)
}

func LoadConfig(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer file.Close()

	var cfg Config
	if err = json.NewDecoder(file).Decode(&cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}
