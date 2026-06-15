package config

import (
	"fmt"
	"os"

	"github.com/go-playground/validator/v10"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Env             string             `yaml:"env"          validate:"omitempty,oneof=dev prod"`
	Server          ServerConfig       `yaml:"server"       validate:"required"`
	Database        DatabaseConfig     `yaml:"database"     validate:"required"`
	Entitlements    EntitlementsConfig `yaml:"entitlements" validate:"required"`
	Auth            AuthConfig         `yaml:"auth"         validate:"required"`
	Providers       ProvidersConfig    `yaml:"providers"`
	Webhooks        OutgoingWebhooks   `yaml:"webhooks"`
	Log             LogConfig          `yaml:"log"`
	Metrics         MetricsConfig      `yaml:"metrics"`
	SyncPeriod      string             `yaml:"sync_period"`
	ReconcilePeriod string             `yaml:"reconcile_period"`
	StrictAccess    bool               `yaml:"strict_access"`
}

type ServerConfig struct {
	Host string `yaml:"host" validate:"omitempty,ip|hostname"`
	Port int    `yaml:"port" validate:"required,min=1,max=65535"`
}

type DatabaseConfig struct {
	Driver    string `yaml:"driver"    validate:"required,oneof=sqlite postgres"`
	DSN       string `yaml:"dsn"       validate:"required"`
	Namespace string `yaml:"namespace"`
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
	DefaultPlan     string          `yaml:"default_plan"`
	DefaultLanguage string          `yaml:"default_language"`
	Plans           []PlanConfig    `yaml:"plans"            validate:"required,min=1,dive"`
	Features        []FeatureConfig `yaml:"features"         validate:"required,min=1,dive"`
}

type PlanConfig struct {
	ID          string            `yaml:"id"          validate:"required"`
	Name        map[string]string `yaml:"name"        validate:"required,min=1"`
	Description map[string]string `yaml:"description"`
	Features    []string          `yaml:"features"    validate:"required,min=1"`
}

type FeatureConfig struct {
	ID          string            `yaml:"id"          validate:"required"`
	Name        map[string]string `yaml:"name"        validate:"required,min=1"`
	Description map[string]string `yaml:"description"`
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
	normalizeEntitlements(&cfg.Entitlements)

	validate := validator.New()
	if err := validate.Struct(&cfg); err != nil {
		return nil, fmt.Errorf("config: validation failed: %w", err)
	}

	if err := validateEntitlements(&cfg.Entitlements); err != nil {
		return nil, fmt.Errorf("config: validation failed: %w", err)
	}

	return &cfg, nil
}

// normalizeEntitlements canonicalizes the default language and every
// translation map key to its BCP-47 form (e.g. "en_us" -> "en-US"), so config
// keys match the tags negotiated from the Accept-Language header regardless of
// the casing or separator the author used.
func normalizeEntitlements(ent *EntitlementsConfig) {
	ent.DefaultLanguage = canonicalLocale(ent.DefaultLanguage)
	for i := range ent.Plans {
		ent.Plans[i].Name = canonicalLocaleKeys(ent.Plans[i].Name)
		ent.Plans[i].Description = canonicalLocaleKeys(ent.Plans[i].Description)
	}
	for i := range ent.Features {
		ent.Features[i].Name = canonicalLocaleKeys(ent.Features[i].Name)
		ent.Features[i].Description = canonicalLocaleKeys(ent.Features[i].Description)
	}
}

// canonicalLocaleKeys returns a copy of m with every key canonicalized.
func canonicalLocaleKeys(m map[string]string) map[string]string {
	if len(m) == 0 {
		return m
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[canonicalLocale(k)] = v
	}
	return out
}

// canonicalLocale returns the BCP-47 canonical form of a locale tag, or the
// input unchanged if it is not a parseable tag.
func canonicalLocale(s string) string {
	if t := language.Make(s); t != language.Und {
		return t.String()
	}
	return s
}

// validateEntitlements enforces that every translatable field provides the
// default language, guaranteeing the API can always resolve a value.
func validateEntitlements(ent *EntitlementsConfig) error {
	def := ent.DefaultLanguage
	check := func(kind, id, field string, m map[string]string) error {
		if len(m) == 0 {
			return nil // optional field (e.g. description) may be absent
		}
		if _, ok := m[def]; !ok {
			return fmt.Errorf("%s %q: %s is missing the default language %q", kind, id, field, def)
		}
		return nil
	}
	for _, p := range ent.Plans {
		if err := check("plan", p.ID, "name", p.Name); err != nil {
			return err
		}
		if err := check("plan", p.ID, "description", p.Description); err != nil {
			return err
		}
	}
	for _, f := range ent.Features {
		if err := check("feature", f.ID, "name", f.Name); err != nil {
			return err
		}
		if err := check("feature", f.ID, "description", f.Description); err != nil {
			return err
		}
	}
	return nil
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
	if cfg.Entitlements.DefaultLanguage == "" {
		cfg.Entitlements.DefaultLanguage = "en"
	}
}
