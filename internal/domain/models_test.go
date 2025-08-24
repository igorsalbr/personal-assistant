package domain_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"personal-assistant/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUser(t *testing.T) {
	t.Run("creates valid user", func(t *testing.T) {
		userID := uuid.New()
		user := &domain.User{
			ID:       userID,
			TenantID: "test-tenant",
			Phone:    "+1234567890",
			Profile:  map[string]interface{}{"name": "John Doe"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		assert.Equal(t, userID, user.ID)
		assert.Equal(t, "test-tenant", user.TenantID)
		assert.Equal(t, "+1234567890", user.Phone)
		assert.Equal(t, "John Doe", user.Profile["name"])
	})
}

func TestMessage(t *testing.T) {
	t.Run("creates valid message", func(t *testing.T) {
		messageID := uuid.New()
		userID := uuid.New()
		
		message := &domain.Message{
			ID:        messageID,
			TenantID:  "test-tenant",
			UserID:    userID,
			MessageID: "infobip-123",
			Direction: "inbound",
			Text:      "Hello world",
			Timestamp: time.Now(),
			TokenUsage: &domain.TokenUsage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
			Metadata:  map[string]interface{}{"source": "whatsapp"},
			CreatedAt: time.Now(),
		}

		assert.Equal(t, messageID, message.ID)
		assert.Equal(t, "test-tenant", message.TenantID)
		assert.Equal(t, userID, message.UserID)
		assert.Equal(t, "infobip-123", message.MessageID)
		assert.Equal(t, "inbound", message.Direction)
		assert.Equal(t, "Hello world", message.Text)
		assert.Equal(t, 30, message.TokenUsage.TotalTokens)
	})
}

func TestAgentConfig(t *testing.T) {
	t.Run("creates valid agent config", func(t *testing.T) {
		agentID := uuid.New()
		agent := &domain.AgentConfig{
			ID:             agentID,
			Name:           "test-agent",
			Version:        "1.0.0",
			AllowedTenants: []string{"tenant1", "tenant2"},
			Config:         map[string]any{"key": "value"},
			Enabled:        true,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		assert.Equal(t, agentID, agent.ID)
		assert.Equal(t, "test-agent", agent.Name)
		assert.Equal(t, "1.0.0", agent.Version)
		assert.Len(t, agent.AllowedTenants, 2)
		assert.True(t, agent.Enabled)
		assert.Equal(t, "value", agent.Config["key"])
	})
}

func TestChatMessage(t *testing.T) {
	t.Run("creates valid chat message", func(t *testing.T) {
		msg := &domain.ChatMessage{
			Role:    "user",
			Content: "Hello AI",
			Name:    "user1",
		}

		assert.Equal(t, "user", msg.Role)
		assert.Equal(t, "Hello AI", msg.Content)
		assert.Equal(t, "user1", msg.Name)
	})

	t.Run("creates message with tool calls", func(t *testing.T) {
		toolCall := domain.ToolCall{
			ID:   "call-123",
			Type: "function",
			Function: &domain.FunctionCall{
				Name:      "get_weather",
				Arguments: []byte(`{"location":"NYC"}`),
			},
		}

		msg := &domain.ChatMessage{
			Role:      "assistant",
			ToolCalls: []domain.ToolCall{toolCall},
		}

		assert.Equal(t, "assistant", msg.Role)
		assert.Len(t, msg.ToolCalls, 1)
		assert.Equal(t, "call-123", msg.ToolCalls[0].ID)
		assert.Equal(t, "get_weather", msg.ToolCalls[0].Function.Name)
	})
}

func TestToolDefinition(t *testing.T) {
	t.Run("creates valid tool definition", func(t *testing.T) {
		tool := &domain.ToolDefinition{
			Type: "function",
			Function: &domain.ToolFunction{
				Name:        "get_weather",
				Description: "Get current weather",
				Parameters:  map[string]interface{}{"type": "object"},
			},
		}

		assert.Equal(t, "function", tool.Type)
		require.NotNil(t, tool.Function)
		assert.Equal(t, "get_weather", tool.Function.Name)
		assert.Equal(t, "Get current weather", tool.Function.Description)
	})
}

func TestTenant(t *testing.T) {
	t.Run("creates valid tenant", func(t *testing.T) {
		tenant := &domain.Tenant{
			ID:             "tenant-123",
			WABANumber:     "+1234567890",
			DBDSN:          "postgres://user:pass@localhost/db",
			EmbeddingModel: "text-embedding-ada-002",
			VectorStore:    "pgvector",
			EnabledAgents:  []string{"agent1", "agent2"},
			Config:         map[string]any{"key": "value"},
			Metadata:       map[string]string{"env": "test"},
		}

		assert.Equal(t, "tenant-123", tenant.ID)
		assert.Equal(t, "+1234567890", tenant.WABANumber)
		assert.Equal(t, "pgvector", tenant.VectorStore)
		assert.Len(t, tenant.EnabledAgents, 2)
		assert.Equal(t, "test", tenant.Metadata["env"])
	})
}

func TestInfobipMessage(t *testing.T) {
	t.Run("creates valid infobip message", func(t *testing.T) {
		msg := &domain.InfobipMessage{
			From:      "+1234567890",
			To:        "+0987654321",
			MessageID: "msg-123",
			Content: domain.InfobipMessageContent{
				Text: "Hello from Infobip",
			},
			CallbackData: "callback-data",
		}

		assert.Equal(t, "+1234567890", msg.From)
		assert.Equal(t, "+0987654321", msg.To)
		assert.Equal(t, "msg-123", msg.MessageID)
		assert.Equal(t, "Hello from Infobip", msg.Content.Text)
		assert.Equal(t, "callback-data", msg.CallbackData)
	})
}