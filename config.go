package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	ListenAddr      string         `json:"listen_addr"`
	Database        DatabaseConfig `json:"database"`
	ExpireAfter     time.Duration  `json:"expire_after"`
	CleanupInterval time.Duration  `json:"clean_up_interval"`
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
	return fmt.Sprintf("\n ListenAddr: %s,\n Database: %s,\n ExpireAfter: %s,\n CleanupInterval: %s\n", c.ListenAddr, c.Database, c.ExpireAfter, c.CleanupInterval)
}

type DatabaseConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Database string `json:"database"`
	SSLMode  string `json:"ssl_mode"`
}

func (c DatabaseConfig) String() string {
	return fmt.Sprintf("\n  Host: %s,\n  Port: %d,\n  Username: %s,\n  Password: %s,\n  Database: %s,\n  SSLMode: %s", c.Host, c.Port, c.Username, strings.Repeat("*", len(c.Password)), c.Database, c.SSLMode)
}

func (c DatabaseConfig) DataSourceName() string {
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
