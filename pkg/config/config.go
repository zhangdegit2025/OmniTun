package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	DatabaseURL         string     `mapstructure:"database_url"`
	ValkeyURL           string     `mapstructure:"valkey_url"`
	NATSURL             string     `mapstructure:"nats_url"`
	ClickHouseURL       string     `mapstructure:"clickhouse_url"`
	TrafficLogging      bool       `mapstructure:"traffic_logging"`
	S3Endpoint          string     `mapstructure:"s3_endpoint"`
	LogLevel            string     `mapstructure:"log_level"`
	ServerPort          int        `mapstructure:"server_port"`
	MetricsPort         int        `mapstructure:"metrics_port"`
	RelayPort           int        `mapstructure:"relay_port"`
	StripeSecret        string     `mapstructure:"stripe_secret"`
	StripeWebhookSecret string     `mapstructure:"stripe_webhook_secret"`
	AdminAuthSecret     string     `mapstructure:"admin_auth_secret"`
	AdminPort           int        `mapstructure:"admin_port"`
	SCIMToken           string     `mapstructure:"scim_token"`
	Auth                AuthConfig `mapstructure:"auth"`
}

type AuthConfig struct {
	JWTSecret          string `mapstructure:"jwt_secret"`
	JWTPublicKey       string `mapstructure:"jwt_public_key"`
	JWTPrivateKey      string `mapstructure:"jwt_private_key"`
	TokenExpiry        int    `mapstructure:"token_expiry"`
	GitHubClientID     string `mapstructure:"github_client_id"`
	GitHubClientSecret string `mapstructure:"github_client_secret"`
	GitHubRedirectURL  string `mapstructure:"github_redirect_url"`
	GoogleClientID     string `mapstructure:"google_client_id"`
	GoogleClientSecret string `mapstructure:"google_client_secret"`
	GoogleRedirectURL  string `mapstructure:"google_redirect_url"`
	OIDCClientID       string `mapstructure:"oidc_client_id"`
	OIDCClientSecret   string `mapstructure:"oidc_client_secret"`
	OIDCDiscoveryURL   string `mapstructure:"oidc_discovery_url"`
	OIDCRedirectURL    string `mapstructure:"oidc_redirect_url"`
	SAMLEnabled        bool   `mapstructure:"saml_enabled"`
	SAMLEntityID       string `mapstructure:"saml_entity_id"`
	SAMLACSURL         string `mapstructure:"saml_acs_url"`
	SAMLMetadataURL    string `mapstructure:"saml_metadata_url"`
	SAMLCertFile       string `mapstructure:"saml_cert_file"`
	SAMLKeyFile        string `mapstructure:"saml_key_file"`
}

func Load(configPath string) (*Config, error) {
	if _, err := os.Stat(configPath); err != nil {
		return nil, fmt.Errorf("config file not found at %s: %w", configPath, err)
	}

	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

func MustLoad(configPath string) *Config {
	cfg, err := Load(configPath)
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}
	return cfg
}

func (c *Config) Validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("database_url is required")
	}
	if c.ServerPort <= 0 || c.ServerPort > 65535 {
		return fmt.Errorf("server_port must be between 1 and 65535, got %d", c.ServerPort)
	}
	if c.RelayPort <= 0 || c.RelayPort > 65535 {
		return fmt.Errorf("relay_port must be between 1 and 65535, got %d", c.RelayPort)
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	if c.Auth.TokenExpiry <= 0 {
		c.Auth.TokenExpiry = 86400
	}
	return nil
}
