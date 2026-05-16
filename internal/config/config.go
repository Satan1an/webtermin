package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	DataDir  string         `yaml:"data_dir"`
	Security SecurityConfig `yaml:"security"`
	Terminal TerminalConfig `yaml:"terminal"`
	Logging  LoggingConfig  `yaml:"logging"`
}

type ServerConfig struct {
	Listen       string `yaml:"listen"`
	TLSCert      string `yaml:"tls_cert"`
	TLSKey       string `yaml:"tls_key"`
	ExternalHost string `yaml:"external_host"`
}

type SecurityConfig struct {
	SessionTTLMin     int  `yaml:"session_ttl_min"`
	PTYIdleTimeoutMin int  `yaml:"pty_idle_timeout_min"`
	Require2FA        bool `yaml:"require_2fa"`
	MaxLoginAttempts  int  `yaml:"max_login_attempts"`
	LockoutMin        int  `yaml:"lockout_min"`
}

type TerminalConfig struct {
	DefaultShell string `yaml:"default_shell"`
}

type LoggingConfig struct {
	Level string `yaml:"level"`
}

func (s SecurityConfig) SessionTTL() time.Duration {
	return time.Duration(s.SessionTTLMin) * time.Minute
}

func (s SecurityConfig) PTYIdleTimeout() time.Duration {
	return time.Duration(s.PTYIdleTimeoutMin) * time.Minute
}

func (s SecurityConfig) Lockout() time.Duration {
	return time.Duration(s.LockoutMin) * time.Minute
}

func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Listen: "0.0.0.0:8443",
		},
		DataDir: "./data",
		Security: SecurityConfig{
			SessionTTLMin:     240,
			PTYIdleTimeoutMin: 30,
			Require2FA:        false,
			MaxLoginAttempts:  5,
			LockoutMin:        15,
		},
		Logging: LoggingConfig{Level: "info"},
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.DataDir == "" {
		cfg.DataDir = "./data"
	}
	abs, err := filepath.Abs(cfg.DataDir)
	if err == nil {
		cfg.DataDir = abs
	}
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create data_dir: %w", err)
	}
	return cfg, nil
}
