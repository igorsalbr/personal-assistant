package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
)

// User represents a WhatsApp user
type User struct {
	ID        uuid.UUID              `json:"id" db:"id"`
	TenantID  string                 `json:"tenant_id" db:"tenant_id"`
	Phone     string                 `json:"phone" db:"phone"`
	Profile   map[string]interface{} `json:"profile" db:"profile"`
	CreatedAt time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt time.Time              `json:"updated_at" db:"updated_at"`
}

// Message represents a WhatsApp message
type Message struct {
	ID         uuid.UUID              `json:"id" db:"id"`
	TenantID   string                 `json:"tenant_id" db:"tenant_id"`
	UserID     uuid.UUID              `json:"user_id" db:"user_id"`
	MessageID  string                 `json:"message_id" db:"message_id"` // Infobip message ID
	Direction  string                 `json:"direction" db:"direction"`   // inbound, outbound
	Text       string                 `json:"text" db:"text"`
	Timestamp  time.Time              `json:"timestamp" db:"timestamp"`
	TokenUsage *TokenUsage            `json:"token_usage" db:"token_usage"`
	Metadata   map[string]interface{} `json:"metadata" db:"metadata"`
	CreatedAt  time.Time              `json:"created_at" db:"created_at"`
}

// MemoryChunk represents a piece of user memory for RAG
type MemoryChunk struct {
	ID        uuid.UUID              `json:"id" db:"id"`
	TenantID  string                 `json:"tenant_id" db:"tenant_id"`
	UserID    uuid.UUID              `json:"user_id" db:"user_id"`
	Kind      string                 `json:"kind" db:"kind"` // note, event, task, msg
	Text      string                 `json:"text" db:"text"`
	Embedding pgvector.Vector        `json:"-" db:"embedding"`
	Metadata  map[string]interface{} `json:"metadata" db:"metadata"`
	CreatedAt time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt time.Time              `json:"updated_at" db:"updated_at"`
}

// AgentConfig represents an agent configuration stored in database
type AgentConfig struct {
	ID             uuid.UUID         `json:"id" db:"id"`
	Name           string            `json:"name" db:"name"`
	Version        string            `json:"version" db:"version"`
	AllowedTenants []string          `json:"allowed_tenants" db:"allowed_tenants"`
	Config         map[string]any    `json:"config" db:"config"`
	Enabled        bool              `json:"enabled" db:"enabled"`
	CreatedAt      time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at" db:"updated_at"`
}

// ExternalService represents an external service configuration
type ExternalService struct {
	ID        uuid.UUID              `json:"id" db:"id"`
	TenantID  string                 `json:"tenant_id" db:"tenant_id"`
	Name      string                 `json:"name" db:"name"`
	BaseURL   string                 `json:"base_url" db:"base_url"`
	Auth      map[string]interface{} `json:"auth" db:"auth"`
	Config    map[string]interface{} `json:"config" db:"config"`
	CreatedAt time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt time.Time              `json:"updated_at" db:"updated_at"`
}

// LLMProviderConfig represents LLM provider configuration stored in database
type LLMProviderConfig struct {
	ID          uuid.UUID              `json:"id" db:"id"`
	TenantID    string                 `json:"tenant_id" db:"tenant_id"`
	Provider    string                 `json:"provider" db:"provider"` // openai, deepseek, anthropic
	Name        string                 `json:"name" db:"name"`         // friendly name
	APIKey      string                 `json:"api_key" db:"api_key"`
	BaseURL     string                 `json:"base_url" db:"base_url,omitempty"`
	ModelChat   string                 `json:"model_chat" db:"model_chat"`
	ModelEmbed  string                 `json:"model_embed" db:"model_embed,omitempty"`
	Config      map[string]interface{} `json:"config" db:"config"`
	IsDefault   bool                   `json:"is_default" db:"is_default"`
	Enabled     bool                   `json:"enabled" db:"enabled"`
	CreatedAt   time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at" db:"updated_at"`
}

// TokenUsage represents token usage information
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// MemoryItem represents an item to be stored in memory
type MemoryItem struct {
	Kind     string                 `json:"kind"`
	Text     string                 `json:"text"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// MemoryHit represents a search result from vector store
type MemoryHit struct {
	ID       uuid.UUID              `json:"id"`
	Kind     string                 `json:"kind"`
	Text     string                 `json:"text"`
	Score    float64                `json:"score"`
	Metadata map[string]interface{} `json:"metadata"`
}

// ToolCall represents a tool call request
type ToolCall struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Function *FunctionCall   `json:"function,omitempty"`
}

// FunctionCall represents a function call within a tool call
type FunctionCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolResponse represents a tool execution response
type ToolResponse struct {
	ID      string      `json:"id"`
	Success bool        `json:"success"`
	Result  interface{} `json:"result,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// AgentRequest represents a request to an agent
type AgentRequest struct {
	TenantID    string                 `json:"tenant_id"`
	UserID      uuid.UUID              `json:"user_id"`
	MessageID   string                 `json:"message_id"`
	Text        string                 `json:"text"`
	Context     []MemoryHit            `json:"context,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	RequestID   string                 `json:"request_id"`
}

// AgentResponse represents a response from an agent
type AgentResponse struct {
	Text      string         `json:"text,omitempty"`
	ToolCalls []ToolCall     `json:"tool_calls,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Error     string         `json:"error,omitempty"`
}

// ChatMessage represents a chat message for LLM communication
type ChatMessage struct {
	Role         string      `json:"role"` // system, user, assistant, tool
	Content      string      `json:"content,omitempty"`
	ToolCalls    []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID   string      `json:"tool_call_id,omitempty"`
	Name         string      `json:"name,omitempty"`
}

// ChatCompletionRequest represents a chat completion request
type ChatCompletionRequest struct {
	Messages    []ChatMessage    `json:"messages"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	ToolChoice  interface{}      `json:"tool_choice,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Temperature float32          `json:"temperature,omitempty"`
}

// ChatCompletionResponse represents a chat completion response
type ChatCompletionResponse struct {
	ID      string            `json:"id"`
	Object  string            `json:"object"`
	Created int64             `json:"created"`
	Model   string            `json:"model"`
	Choices []Choice          `json:"choices"`
	Usage   *TokenUsage       `json:"usage,omitempty"`
}

// Choice represents a choice in chat completion response
type Choice struct {
	Index        int          `json:"index"`
	Message      *ChatMessage `json:"message,omitempty"`
	Delta        *ChatMessage `json:"delta,omitempty"`
	FinishReason string       `json:"finish_reason,omitempty"`
}

// ToolDefinition represents a tool definition for chat completions
type ToolDefinition struct {
	Type     string       `json:"type"`
	Function *ToolFunction `json:"function,omitempty"`
}

// ToolFunction represents a tool function definition
type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// EmbeddingRequest represents an embedding request
type EmbeddingRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model,omitempty"`
}

// EmbeddingResponse represents an embedding response
type EmbeddingResponse struct {
	Object string      `json:"object"`
	Data   []Embedding `json:"data"`
	Model  string      `json:"model"`
	Usage  *TokenUsage `json:"usage,omitempty"`
}

// Embedding represents a single embedding
type Embedding struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

// InfobipMessage represents an Infobip WhatsApp message
type InfobipMessage struct {
	From      string                 `json:"from"`
	To        string                 `json:"to"`
	MessageID string                 `json:"messageId,omitempty"`
	Content   InfobipMessageContent  `json:"content"`
	CallbackData string              `json:"callbackData,omitempty"`
}

// InfobipMessageContent represents the content of an Infobip message
type InfobipMessageContent struct {
	Text string `json:"text"`
}

// InfobipWebhookMessage represents an incoming webhook message from Infobip
type InfobipWebhookMessage struct {
	Results []InfobipWebhookResult `json:"results"`
}

// InfobipWebhookResult represents a single result in the webhook
type InfobipWebhookResult struct {
	MessageID string                      `json:"messageId"`
	From      string                      `json:"from"`
	To        string                      `json:"to"`
	IntegrationType string                `json:"integrationType"`
	ReceivedAt      time.Time             `json:"receivedAt"`
	Message         InfobipIncomingMessage `json:"message"`
	Contact         InfobipContact        `json:"contact"`
	Price           InfobipPrice          `json:"price,omitempty"`
}

// InfobipIncomingMessage represents an incoming message content
type InfobipIncomingMessage struct {
	Type string                `json:"type"`
	Text InfobipIncomingText   `json:"text,omitempty"`
}

// InfobipIncomingText represents text content
type InfobipIncomingText struct {
	Text string `json:"text"`
}

// InfobipContact represents contact information
type InfobipContact struct {
	Name string `json:"name,omitempty"`
}

// InfobipPrice represents pricing information
type InfobipPrice struct {
	PricePerMessage float64 `json:"pricePerMessage"`
	Currency        string  `json:"currency"`
}

// JSONSchema represents a JSON schema for tool parameters
type JSONSchema struct {
	Type       string                        `json:"type"`
	Properties map[string]JSONSchemaProperty `json:"properties,omitempty"`
	Required   []string                      `json:"required,omitempty"`
}

// JSONSchemaProperty represents a property in a JSON schema
type JSONSchemaProperty struct {
	Type        string                           `json:"type"`
	Description string                           `json:"description,omitempty"`
	Enum        []string                         `json:"enum,omitempty"`
	Items       *JSONSchemaProperty              `json:"items,omitempty"`
	Properties  map[string]JSONSchemaProperty    `json:"properties,omitempty"`
}

// Tenant represents a tenant configuration
type Tenant struct {
	ID             string            `json:"id"`
	WABANumber     string            `json:"waba_number"`
	DBDSN          string            `json:"db_dsn"`
	EmbeddingModel string            `json:"embedding_model"`
	VectorStore    string            `json:"vector_store"`
	EnabledAgents  []string          `json:"enabled_agents"`
	Config         map[string]any    `json:"config"`
	Metadata       map[string]string `json:"metadata"`
}

// SearchFilter represents filters for memory search
type SearchFilter struct {
	Kinds []string          `json:"kinds,omitempty"`
	Tags  []string          `json:"tags,omitempty"`
	Meta  map[string]string `json:"meta,omitempty"`
}

// SearchOptions represents options for memory search
type SearchOptions struct {
	TopK     int           `json:"top_k,omitempty"`
	MinScore float64       `json:"min_score,omitempty"`
	Filter   *SearchFilter `json:"filter,omitempty"`
}

// ToolInvocationResult represents the result of a tool invocation
type ToolInvocationResult struct {
	ToolName string      `json:"tool_name"`
	Success  bool        `json:"success"`
	Result   interface{} `json:"result,omitempty"`
	Error    string      `json:"error,omitempty"`
}