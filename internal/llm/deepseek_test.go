package llm_test

import (
	"context"
	"testing"

	"personal-assistant/internal/domain"
	"personal-assistant/internal/llm"
	"personal-assistant/internal/log"

	"github.com/stretchr/testify/assert"
)

func TestNewDeepSeekProvider_Defaults(t *testing.T) {
	config := &domain.LLMProviderConfig{
		APIKey: "dummy-key",
		Name:   "test",
	}
	logger := log.Init("info")
	provider, err := llm.NewDeepSeekProvider(config, logger)
	assert.NoError(t, err)
	assert.Equal(t, "deepseek-test", provider.Name())
	assert.Equal(t, "deepseek-chat", provider.ChatModel())
	assert.Equal(t, "text-embedding-v1", provider.EmbedModel())
}

func TestDeepSeekProvider_Embed_EmptyInput(t *testing.T) {
	config := &domain.LLMProviderConfig{APIKey: "dummy-key", Name: "test"}
	logger := log.Init("info")
	provider, _ := llm.NewDeepSeekProvider(config, logger)
	result, err := provider.Embed(context.Background(), []string{})
	assert.NoError(t, err)
	assert.Empty(t, result)
}
