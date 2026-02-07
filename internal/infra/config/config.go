package config

import (
	"fmt"
	"os"

	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Env          string             `yaml:"env"          validate:"omitempty,oneof=dev prod"`
	Server       ServerConfig       `yaml:"server"       validate:"required"`
	Database     DatabaseConfig     `yaml:"database"     validate:"required"`
	Entitlements EntitlementsConfig `yaml:"entitlements" validate:"required"`
	Auth         AuthConfig         `yaml:"auth"         validate:"required"`
	Providers    ProvidersConfig    `yaml:"providers"`
	Webhooks     OutgoingWebhooks   `yaml:"webhooks"`
	Log          LogConfig          `yaml:"log"`
	Metrics      MetricsConfig      `yaml:"metrics"`
	SyncPeriod   string             `yaml:"sync_period"`
}

type ServerConfig struct {
	Host string `yaml:"host" validate:"omitempty,ip|hostname"`
	Port int    `yaml:"port" validate:"required,min=1,max=65535"`
}

type DatabaseConfig struct {
	Driver string `yaml:"driver" validate:"required,oneof=sqlite postgres"`
	DSN    string `yaml:"dsn"    validate:"required"`
}

type AuthConfig struct {
	APIKey string `yaml:"api_key" validate:"required"`
}

type LogConfig struct {
	Level  string `yaml:"level"  validate:"omitempty,oneof=debug info warn error"`
	Format string `yaml:"format" validate:"omitempty,oneof=json text"`
}

type MetricsConfig struct {
	Enable    bool   `yaml:"enable"`
	GoMetrics bool   `yaml:"go_metrics"`
	Path      string `yaml:"path"       validate:"omitempty,startswith=/"`
}

type EntitlementsConfig struct {
	DefaultPlan string          `yaml:"default_plan"`
	Plans       []PlanConfig    `yaml:"plans"        validate:"required,min=1,dive"`
	Features    []FeatureConfig `yaml:"features"     validate:"required,min=1,dive"`
}

type PlanConfig struct {
	ID       string   `yaml:"id"       validate:"required"`
	Name     string   `yaml:"name"     validate:"required"`
	Features []string `yaml:"features" validate:"required,min=1"`
}

type FeatureConfig struct {
	ID          string `yaml:"id"          validate:"required"`
	Name        string `yaml:"name"        validate:"required"`
	Description string `yaml:"description"`
}

// ProvidersConfig groups all payment provider configurations
type ProvidersConfig struct {
	LemonSqueezy LemonSqueezyConfig `yaml:"lemonsqueezy"`
}

// LemonSqueezyConfig contains LemonSqueezy-specific settings
type LemonSqueezyConfig struct {
	APIKey   string                      `yaml:"api_key"  validate:"required"`
	Products []ProductMapping            `yaml:"products" validate:"dive"`
	Webhook  LemonSqueezyIncomingWebhook `yaml:"webhook"`
}

type ProductMapping struct {
	ProductID int    `yaml:"product_id" validate:"required"`
	PlanID    string `yaml:"plan_id"    validate:"required"`
}

// LemonSqueezyIncomingWebhook configures incoming webhook from LemonSqueezy
type LemonSqueezyIncomingWebhook struct {
	Secret string `yaml:"secret"`
}

// OutgoingWebhooks configures webhooks sent to external services
type OutgoingWebhooks struct {
	Endpoints []WebhookEndpoint `yaml:"endpoints" validate:"dive"`
}

// WebhookEndpoint defines a single outgoing webhook destination
type WebhookEndpoint struct {
	URL    string `yaml:"url"    validate:"required,url"`
	Secret string `yaml:"secret" validate:"required"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: failed to read file: %w", err)
	}

	// Expand environment variables in the config
	data = []byte(os.ExpandEnv(string(data)))

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: failed to parse yaml: %w", err)
	}

	applyDefaults(&cfg)

	validate := validator.New()
	if err := validate.Struct(&cfg); err != nil {
		return nil, fmt.Errorf("config: validation failed: %w", err)
	}

	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Log.Format == "" {
		cfg.Log.Format = "json"
	}
	if cfg.Env == "" {
		cfg.Env = "prod"
	}
	if cfg.Metrics.Path == "" {
		cfg.Metrics.Path = "/metrics"
	}
}
