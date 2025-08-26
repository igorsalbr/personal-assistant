package llm

import (
	"fmt"

	"personal-assistant/internal/domain"
	"personal-assistant/internal/llm/openai"
	"personal-assistant/internal/log"
)

// ProviderType represents the type of LLM provider
type ProviderType string

const (
	// OpenAI provider
	OpenAI ProviderType = "openai"
	// DeepSeek provider
	DeepSeek ProviderType = "deepseek"
	// Anthropic provider
	Anthropic ProviderType = "anthropic"
	// Bedrock provider
	Bedrock ProviderType = "bedrock"
	// Mock provider for testing
	Mock ProviderType = "mock"
)

// Factory creates LLM provider instances
type Factory struct {
	logger *log.Logger
}

// NewFactory creates a new LLM provider factory
func NewFactory(logger *log.Logger) *Factory {
	return &Factory{
		logger: logger,
	}
}

// CreateProvider creates a new LLM provider instance based on configuration
func (f *Factory) CreateProvider(config *domain.LLMProviderConfig) (domain.LLMProvider, error) {
	providerType := GetProviderType(config.Provider)

	switch providerType {
	case OpenAI:
		return openai.NewProvider(config, f.logger)
	case DeepSeek:
		return NewDeepSeekProvider(config, f.logger)
	case Bedrock:
		return NewBedrockProvider(config, f.logger)
	case Mock:
		return NewMockProvider(config, f.logger)
	default:
		return nil, fmt.Errorf("unsupported LLM provider type: %s", config.Provider)
	}
}

// GetProviderType parses a string to ProviderType
func GetProviderType(s string) ProviderType {
	switch s {
	case "openai":
		return OpenAI
	case "deepseek":
		return DeepSeek
	case "anthropic":
		return Anthropic
	case "bedrock":
		return Bedrock
	case "mock":
		return Mock
	default:
		return OpenAI // default to OpenAI
	}
}

// ProviderManager manages multiple LLM providers for different tenants
type ProviderManager struct {
	providers map[string]domain.LLMProvider // key: tenantID_providerName
	factory   *Factory
	logger    *log.Logger
}

// NewProviderManager creates a new provider manager
func NewProviderManager(logger *log.Logger) *ProviderManager {
	return &ProviderManager{
		providers: make(map[string]domain.LLMProvider),
		factory:   NewFactory(logger),
		logger:    logger,
	}
}

// GetProvider gets or creates a provider for a tenant
func (pm *ProviderManager) GetProvider(tenantID string, config *domain.LLMProviderConfig) (domain.LLMProvider, error) {
	key := fmt.Sprintf("%s_%s", tenantID, config.Name)

	// Return existing provider if available
	if provider, exists := pm.providers[key]; exists {
		return provider, nil
	}

	// Create new provider
	provider, err := pm.factory.CreateProvider(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider %s for tenant %s: %w", config.Name, tenantID, err)
	}

	// Cache the provider
	pm.providers[key] = provider

	pm.logger.Info().
		Str("tenant_id", tenantID).
		Str("provider_name", config.Name).
		Str("provider_type", config.Provider).
		Msg("LLM provider created and cached")

	return provider, nil
}

// RemoveProvider removes a cached provider
func (pm *ProviderManager) RemoveProvider(tenantID, providerName string) {
	key := fmt.Sprintf("%s_%s", tenantID, providerName)
	delete(pm.providers, key)
}

// ClearTenant removes all providers for a tenant
func (pm *ProviderManager) ClearTenant(tenantID string) {
	for key := range pm.providers {
		if len(key) > len(tenantID) && key[:len(tenantID)+1] == tenantID+"_" {
			delete(pm.providers, key)
		}
	}
}

// GetDefaultModels returns default models for each provider type
func GetDefaultModels(providerType ProviderType) (chatModel, embedModel string) {
	switch providerType {
	case OpenAI:
		return "gpt-3.5-turbo", "text-embedding-ada-002"
	case DeepSeek:
		return "deepseek-chat", "text-embedding-v1"
	case Anthropic:
		return "claude-3-sonnet-20240229", ""
	case Bedrock:
		return "openai.gpt-oss-120b-1:0", "amazon.titan-embed-text-v2:0"
	case Mock:
		return "mock-chat", "mock-embed"
	default:
		return "gpt-3.5-turbo", "text-embedding-ada-002"
	}
}

// GetDefaultBaseURL returns the default base URL for each provider type
func GetDefaultBaseURL(providerType ProviderType) string {
	switch providerType {
	case OpenAI:
		return "https://api.openai.com/v1"
	case DeepSeek:
		return "https://api.deepseek.com/v1"
	case Anthropic:
		return "https://api.anthropic.com"
	case Bedrock:
		return "" // Uses AWS SDK, no base URL needed
	case Mock:
		return "http://localhost:8080/mock"
	default:
		return "https://api.openai.com/v1"
	}
}
