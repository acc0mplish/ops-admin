package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App      AppConfig      `yaml:"app"`
	DB       DBConfig       `yaml:"db"`
	Security SecurityConfig `yaml:"security"`
	SSL      SSLConfig      `yaml:"ssl"`
}

type SecurityConfig struct {
	CredentialKey string `yaml:"credential-key"`
}

type SSLConfig struct {
	ACMEEmail             string `yaml:"acme-email"`
	ProductionCA          string `yaml:"production-ca"`
	StagingCA             string `yaml:"staging-ca"`
	DNSPollingSeconds     int    `yaml:"dns-polling-seconds"`
	DNSPropagationSeconds int    `yaml:"dns-propagation-seconds"`
	ExpiryWarningDays     int    `yaml:"expiry-warning-days"`
}

type AppConfig struct {
	Name string `yaml:"name"`
	Port string `yaml:"port"`
	Mode string `yaml:"mode"`
}

type DBConfig struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
	LogMode  bool   `yaml:"log-mode"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	cfg.Security.CredentialKey = strings.TrimSpace(cfg.Security.CredentialKey)
	if cfg.Security.CredentialKey != "" && len([]byte(cfg.Security.CredentialKey)) < 32 {
		return nil, fmt.Errorf("security.credential-key must contain at least 32 bytes")
	}
	// A temporary port is useful for local verification without interfering with
	// an already running development server.
	if port := os.Getenv("OPS_ADMIN_PORT"); port != "" {
		cfg.App.Port = port
	}
	return &cfg, nil
}
