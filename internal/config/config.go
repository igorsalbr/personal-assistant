package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/kelseyhightower/envconfig"
	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	// Server configuration
	Port    string `envconfig:"APP_PORT" default:"8080"`
	BaseURL string `envconfig:"APP_BASE_URL" default:"http://localhost:8080"`

	// Logging
	LogLevel string `envconfig:"LOG_LEVEL" default:"info"`

	// LLM configuration
	LLM LLMConfig

	// Vector store configuration
	VectorBackend string `envconfig:"VECTOR_BACKEND" default:"pgvector"` // pgvector, qdrant, sql_fallback

	// Database
	DatabaseURL string `envconfig:"DATABASE_URL_DEFAULT" required:"true"`

	// Tenants
	TenantsConfigPath string `envconfig:"TENANTS_CONFIG_PATH" default:"tenants.yaml"`

	// Infobip
	Infobip InfobipConfig

	// Webhook
	WebhookVerifyToken string `envconfig:"WEBHOOK_VERIFY_TOKEN" required:"true"`

	// RAG configuration
	RAG RAGConfig

	// Token limits
	MaxTokensReply      int `envconfig:"MAX_TOKENS_REPLY" default:"500"`
	SummarizeThreshold int `envconfig:"SUMMARIZE_THRESHOLD" default:"10000"`
}

// LLMConfig holds LLM provider configuration
type LLMConfig struct {
	Provider   string `envconfig:"LLM_PROVIDER" default:"openai"` // openai, anthropic, mock
	APIKey     string `envconfig:"LLM_API_KEY" required:"true"`
	ModelChat  string `envconfig:"LLM_MODEL_CHAT" default:"gpt-3.5-turbo"`
	ModelEmbed string `envconfig:"LLM_MODEL_EMBED" default:"text-embedding-ada-002"`
}

// InfobipConfig holds Infobip configuration
type InfobipConfig struct {
	BaseURL     string   `envconfig:"INFOBIP_BASE_URL" default:"https://api.infobip.com"`
	APIKey      string   `envconfig:"INFOBIP_API_KEY" required:"true"`
	WABANumbers []string `envconfig:"INFOBIP_WABA_NUMBERS" default:""`
}

// RAGConfig holds RAG-specific configuration
type RAGConfig struct {
	TopK     int     `envconfig:"RAG_TOP_K" default:"5"`
	MinScore float64 `envconfig:"RAG_MIN_SCORE" default:"0.7"`
}

// TenantConfig represents a single tenant configuration
type TenantConfig struct {
	TenantID       string            `yaml:"tenant_id"`
	WABANumber     string            `yaml:"waba_number"`
	DBDSN          string            `yaml:"db_dsn"`
	EmbeddingModel string            `yaml:"embedding_model"`
	VectorStore    string            `yaml:"vector_store"`
	EnabledAgents  []string          `yaml:"enabled_agents"`
	Config         map[string]any    `yaml:"config,omitempty"`
	Metadata       map[string]string `yaml:"metadata,omitempty"`
}

// TenantsConfig holds all tenant configurations
type TenantsConfig struct {
	Tenants []TenantConfig `yaml:"tenants"`
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	var cfg Config
	err := envconfig.Process("", &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Parse comma-separated WABA numbers
	if wabaNumbers := os.Getenv("INFOBIP_WABA_NUMBERS"); wabaNumbers != "" {
		cfg.Infobip.WABANumbers = strings.Split(wabaNumbers, ",")
		for i, num := range cfg.Infobip.WABANumbers {
			cfg.Infobip.WABANumbers[i] = strings.TrimSpace(num)
		}
	}

	return &cfg, nil
}

// LoadTenants loads tenant configurations from the specified file
func (c *Config) LoadTenants() (*TenantsConfig, error) {
	data, err := os.ReadFile(c.TenantsConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read tenants config file: %w", err)
	}

	var tenants TenantsConfig
	if err := yaml.Unmarshal(data, &tenants); err != nil {
		return nil, fmt.Errorf("failed to parse tenants config: %w", err)
	}

	return &tenants, nil
}

// GetEnvOrDefault gets an environment variable or returns a default value
func GetEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetEnvBool gets an environment variable as a boolean
func GetEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

// GetEnvInt gets an environment variable as an integer
func GetEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}