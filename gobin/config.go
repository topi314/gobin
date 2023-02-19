package gobin

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	DevMode         bool           `json:"dev_mode"`
	ListenAddr      string         `json:"listen_addr"`
	Database        DatabaseConfig `json:"database"`
	MaxDocumentSize int            `json:"max_document_size"`
	ExpireAfter     time.Duration  `json:"expire_after"`
	CleanupInterval time.Duration  `json:"cleanup_interval"`
}

func (c *Config) UnmarshalJSON(data []byte) error {
	type config Config
	v := struct {
		ExpireAfter     string `json:"expire_after"`
		CleanupInterval string `json:"cleanup_interval"`
		*config
	}{
		config: (*config)(c),
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	d1, err := time.ParseDuration(v.ExpireAfter)
	if err != nil {
		return err
	}
	c.ExpireAfter = d1

	d2, err := time.ParseDuration(v.CleanupInterval)
	if err != nil {
		return err
	}
	c.CleanupInterval = d2
	return nil
}

func (c Config) String() string {
	return fmt.Sprintf("\n ListenAddr: %s,\n DB: %s,\n ExpireAfter: %s,\n CleanupInterval: %s\n", c.ListenAddr, c.Database, c.ExpireAfter, c.CleanupInterval)
}

type DatabaseConfig struct {
	Type string `json:"type"`

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

func (c DatabaseConfig) String() string {
	str := fmt.Sprintf("\n  Type: %s,\n  ", c.Type)
	switch c.Type {
	case "postgres":
		str += fmt.Sprintf("Host: %s,\n  Port: %d,\n  Username: %s,\n  Password: %s,\n  DB: %s,\n  SSLMode: %s", c.Host, c.Port, c.Username, strings.Repeat("*", len(c.Password)), c.Database, c.SSLMode)
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
