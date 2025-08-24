package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"

	"personal-assistant/internal/domain"
	"personal-assistant/internal/log"
)

// MockProvider implements the LLMProvider interface for testing
type MockProvider struct {
	config *domain.LLMProviderConfig
	logger *log.Logger
}

// NewMockProvider creates a new mock provider
func NewMockProvider(config *domain.LLMProviderConfig, logger *log.Logger) (*MockProvider, error) {
	return &MockProvider{
		config: config,
		logger: logger,
	}, nil
}

// Name returns the provider name
func (p *MockProvider) Name() string {
	return fmt.Sprintf("mock-%s", p.config.Name)
}

// Chat performs a mock chat completion
func (p *MockProvider) Chat(ctx context.Context, req *domain.ChatCompletionRequest) (*domain.ChatCompletionResponse, error) {
	p.logger.WithContext(ctx).Debug().
		Int("messages", len(req.Messages)).
		Msg("mock chat completion request")

	// Simulate some processing time
	time.Sleep(100 * time.Millisecond)

	// Generate mock response based on the last message
	var responseText string
	var toolCalls []domain.ToolCall

	if len(req.Messages) > 0 {
		lastMsg := req.Messages[len(req.Messages)-1]

		// Check if this looks like a tool-requiring request
		if p.shouldUseTool(lastMsg.Content) && len(req.Tools) > 0 {
			// Generate a tool call response
			tool := req.Tools[0] // Use first available tool
			toolCalls = []domain.ToolCall{
				{
					ID:   fmt.Sprintf("call_%s", uuid.New().String()[:8]),
					Type: "function",
					Function: &domain.FunctionCall{
						Name:      tool.Function.Name,
						Arguments: json.RawMessage(`{"query": "mock query", "top_k": 5}`),
					},
				},
			}
		} else {
			// Generate a text response
			responseText = p.generateMockResponse(lastMsg.Content)
		}
	} else {
		responseText = "Hello! I'm a mock LLM provider. How can I help you?"
	}

	// Create mock response
	resp := &domain.ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-mock-%s", uuid.New().String()[:8]),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   p.config.ModelChat,
		Choices: []domain.Choice{
			{
				Index:        0,
				FinishReason: "stop",
				Message: &domain.ChatMessage{
					Role:      "assistant",
					Content:   responseText,
					ToolCalls: toolCalls,
				},
			},
		},
		Usage: &domain.TokenUsage{
			PromptTokens:     p.estimateTokens(req.Messages),
			CompletionTokens: p.estimateTokens([]domain.ChatMessage{{Content: responseText}}),
			TotalTokens:      0, // Will be calculated
		},
	}

	resp.Usage.TotalTokens = resp.Usage.PromptTokens + resp.Usage.CompletionTokens

	p.logger.WithContext(ctx).Debug().
		Str("response", responseText).
		Int("tool_calls", len(toolCalls)).
		Msg("mock chat completion response")

	return resp, nil
}

// Embed generates mock embeddings
func (p *MockProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	p.logger.WithContext(ctx).Debug().
		Int("texts", len(texts)).
		Msg("mock embedding request")

	// Simulate some processing time
	time.Sleep(50 * time.Millisecond)

	// Generate deterministic but pseudo-random embeddings
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		// Use text hash as seed for consistent embeddings
		seed := int64(p.hashString(text))
		r := rand.New(rand.NewSource(seed))

		// Generate 1536-dimensional embedding (same as OpenAI)
		embedding := make([]float32, 1536)
		for j := 0; j < 1536; j++ {
			embedding[j] = r.Float32()*2 - 1 // Range [-1, 1]
		}

		// Normalize the vector
		embedding = p.normalize(embedding)
		embeddings[i] = embedding
	}

	p.logger.WithContext(ctx).Debug().
		Int("embeddings", len(embeddings)).
		Msg("mock embedding response")

	return embeddings, nil
}

// shouldUseTool determines if a message should trigger a tool call
func (p *MockProvider) shouldUseTool(content string) bool {
	keywords := []string{
		"search", "find", "look", "get", "retrieve",
		"remember", "recall", "note", "save", "store",
		"task", "todo", "event", "schedule", "reminder",
		"call", "api", "http", "service",
	}

	content = strings.ToLower(content)
	for _, keyword := range keywords {
		if strings.Contains(content, keyword) {
			return true
		}
	}
	return false
}

// generateMockResponse creates a contextual mock response
func (p *MockProvider) generateMockResponse(input string) string {
	responses := []string{
		"I understand your request. As a mock LLM, I'm simulating a helpful response to your message about: %s",
		"Thank you for your message. I'm processing your request regarding: %s",
		"I'm a mock assistant. Based on your input about %s, here's my simulated response.",
		"Mock LLM response: I've received your message and I'm providing a helpful reply about: %s",
	}

	// Select response based on input length
	index := len(input) % len(responses)

	// Truncate input for display if too long
	displayInput := input
	if len(displayInput) > 50 {
		displayInput = displayInput[:47] + "..."
	}

	return fmt.Sprintf(responses[index], displayInput)
}

// estimateTokens provides a rough token count estimation
func (p *MockProvider) estimateTokens(messages []domain.ChatMessage) int {
	totalChars := 0
	for _, msg := range messages {
		totalChars += len(msg.Content)
		totalChars += len(msg.Role) + 10 // Account for role and formatting
	}
	// Rough approximation: 1 token ≈ 4 characters
	return (totalChars + 3) / 4
}

// hashString creates a simple hash of a string
func (p *MockProvider) hashString(s string) uint32 {
	hash := uint32(0)
	for _, c := range s {
		hash = hash*31 + uint32(c)
	}
	return hash
}

// normalize normalizes a vector to unit length
func (p *MockProvider) normalize(vec []float32) []float32 {
	var norm float32
	for _, v := range vec {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))

	if norm == 0 {
		return vec
	}

	normalized := make([]float32, len(vec))
	for i, v := range vec {
		normalized[i] = v / norm
	}
	return normalized
}
