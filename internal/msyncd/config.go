// Package msyncd implements the MinusSync server daemon.
package msyncd

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Version is the server version.
const Version = "1.0.0"

// RepositoryConfig holds configuration for a single hosted repository.
type RepositoryConfig struct {
	Name           string `yaml:"name"`
	Path           string `yaml:"path"`
	ReadOnly       bool   `yaml:"read_only"`
	MaxSizeGB      int    `yaml:"max_size_gb"`
	AllowForcePush bool   `yaml:"allow_force_push"`
	AnonymousRead  bool   `yaml:"anonymous_read"`
}

// AuthToken holds a user/token pair for authentication.
type AuthToken struct {
	User  string `yaml:"user"`
	Token string `yaml:"token"`
}

// ListenConfig holds listen address and TLS settings.
type ListenConfig struct {
	Address string   `yaml:"address"`
	Port    int      `yaml:"port"`
	TLS     TLSConfig `yaml:"tls"`
}

// TLSConfig holds TLS settings.
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	Method string      `yaml:"method"`
	Tokens []AuthToken `yaml:"tokens"`
}

// SearchConfig holds search settings.
type SearchConfig struct {
	Enabled        bool `yaml:"enabled"`
	MaxIndexSizeMB int  `yaml:"max_index_size_mb"`
}

// GCConfig holds garbage collection settings.
type GCConfig struct {
	IntervalHours int `yaml:"interval_hours"`
	KeepPackDays  int `yaml:"keep_pack_days"`
}

// LogConfig holds logging settings.
type LogConfig struct {
	Level      string `yaml:"level"`
	File       string `yaml:"file"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxBackups int    `yaml:"max_backups"`
}

// Config holds the server configuration.
type Config struct {
	Listen       ListenConfig        `yaml:"listen"`
	Auth         AuthConfig          `yaml:"auth"`
	Repositories []RepositoryConfig  `yaml:"repositories"`

	Search   SearchConfig `yaml:"search"`
	GC       GCConfig     `yaml:"gc"`
	Logging  LogConfig    `yaml:"logging"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	cfg := &Config{}
	cfg.Listen.Address = "0.0.0.0"
	cfg.Listen.Port = 65530
	cfg.Auth.Method = "none"
	cfg.Search.Enabled = true
	cfg.Search.MaxIndexSizeMB = 512
	cfg.GC.IntervalHours = 24
	cfg.GC.KeepPackDays = 7
	cfg.Logging.Level = "info"
	return cfg
}

// LoadConfig reads a YAML config file.
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Use defaults if config doesn't exist
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return cfg, nil
}
