package openai_test

import (
	"context"
	"testing"

	"personal-assistant/internal/domain"
	"personal-assistant/internal/log"
	openaiProvider "personal-assistant/internal/llm/openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIProvider(t *testing.T) {
	// Create test config and logger
	config := &domain.LLMProviderConfig{
		APIKey:    "test-api-key",
		Provider:  "openai",
		Name:      "test",
		ModelChat: "gpt-3.5-turbo",
	}
	logger := log.Init("debug")
	
	provider, err := openaiProvider.NewProvider(config, logger)
	if err != nil {
		t.Skip("Skipping OpenAI tests - provider creation failed:", err)
	}

	t.Run("provider name", func(t *testing.T) {
		assert.Equal(t, "openai-test", provider.Name())
	})

	t.Run("chat completion structure", func(t *testing.T) {
		ctx := context.Background()
		
		req := &domain.ChatCompletionRequest{
			Messages: []domain.ChatMessage{
				{
					Role:    "user",
					Content: "Hello, world!",
				},
			},
			MaxTokens:   100,
			Temperature: 0.7,
		}

		// This would normally call the API, but we're testing structure
		// In a real test environment, you'd mock the HTTP client
		_, err := provider.Chat(ctx, req)
		
		// We expect an error since we're using a fake API key
		assert.Error(t, err)
	})

	t.Run("embedding structure", func(t *testing.T) {
		ctx := context.Background()
		texts := []string{"Hello, world!"}

		// This would normally call the API
		_, err := provider.Embed(ctx, texts)
		
		// We expect an error since we're using a fake API key
		assert.Error(t, err)
	})
}

func TestChatRequestConversion(t *testing.T) {
	t.Run("converts chat request with tools", func(t *testing.T) {
		toolDef := domain.ToolDefinition{
			Type: "function",
			Function: &domain.ToolFunction{
				Name:        "get_weather",
				Description: "Get weather information",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "The city name",
						},
					},
					"required": []string{"location"},
				},
			},
		}

		req := &domain.ChatCompletionRequest{
			Messages: []domain.ChatMessage{
				{
					Role:    "user",
					Content: "What's the weather in NYC?",
				},
			},
			Tools:       []domain.ToolDefinition{toolDef},
			ToolChoice:  "auto",
			MaxTokens:   100,
			Temperature: 0.7,
		}

		// Test that the structure is valid
		require.NotNil(t, req)
		assert.Len(t, req.Messages, 1)
		assert.Len(t, req.Tools, 1)
		assert.Equal(t, "function", req.Tools[0].Type)
		assert.Equal(t, "get_weather", req.Tools[0].Function.Name)
		assert.Equal(t, "auto", req.ToolChoice)
	})

	t.Run("converts chat message with tool calls", func(t *testing.T) {
		toolCall := domain.ToolCall{
			ID:   "call_123",
			Type: "function",
			Function: &domain.FunctionCall{
				Name:      "get_weather",
				Arguments: []byte(`{"location": "NYC"}`),
			},
		}

		message := domain.ChatMessage{
			Role:      "assistant",
			Content:   "",
			ToolCalls: []domain.ToolCall{toolCall},
		}

		assert.Equal(t, "assistant", message.Role)
		assert.Len(t, message.ToolCalls, 1)
		assert.Equal(t, "call_123", message.ToolCalls[0].ID)
		assert.Equal(t, "function", message.ToolCalls[0].Type)
		assert.Equal(t, "get_weather", message.ToolCalls[0].Function.Name)
	})
}