package domain

import (
	"context"

	"github.com/google/uuid"
)

// LLMProvider defines the interface for LLM providers (OpenAI, Anthropic, etc.)
type LLMProvider interface {
	// Chat performs a chat completion with optional tool calls
	Chat(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error)
	
	// Embed generates embeddings for the given texts
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	
	// Name returns the provider name
	Name() string
}

// VectorStore defines the interface for vector storage backends
type VectorStore interface {
	// Upsert inserts or updates memory items
	Upsert(ctx context.Context, tenantID string, userID uuid.UUID, items []MemoryItem) ([]uuid.UUID, error)
	
	// Search performs a similarity search
	Search(ctx context.Context, tenantID string, userID uuid.UUID, queryEmbedding []float32, opts *SearchOptions) ([]MemoryHit, error)
	
	// GetByID retrieves a memory item by ID
	GetByID(ctx context.Context, tenantID string, userID uuid.UUID, id uuid.UUID) (*MemoryChunk, error)
	
	// UpdateByID updates a memory item by ID
	UpdateByID(ctx context.Context, tenantID string, userID uuid.UUID, id uuid.UUID, updates map[string]interface{}) error
	
	// DeleteByID deletes a memory item by ID
	DeleteByID(ctx context.Context, tenantID string, userID uuid.UUID, id uuid.UUID) error
	
	// Close closes the vector store connection
	Close() error
}

// Repository defines the interface for SQL database operations
type Repository interface {
	// User operations
	CreateUser(ctx context.Context, user *User) error
	GetUser(ctx context.Context, tenantID, phone string) (*User, error)
	GetUserByID(ctx context.Context, tenantID string, userID uuid.UUID) (*User, error)
	UpdateUser(ctx context.Context, user *User) error
	
	// Message operations
	CreateMessage(ctx context.Context, message *Message) error
	GetMessages(ctx context.Context, tenantID string, userID uuid.UUID, limit int) ([]Message, error)
	GetMessageByID(ctx context.Context, tenantID string, messageID string) (*Message, error)
	
	// Agent operations
	GetAgents(ctx context.Context) ([]AgentConfig, error)
	GetAgentByName(ctx context.Context, name string) (*AgentConfig, error)
	CreateAgent(ctx context.Context, agent *AgentConfig) error
	UpdateAgent(ctx context.Context, agent *AgentConfig) error
	
	// External service operations
	GetExternalServices(ctx context.Context, tenantID string) ([]ExternalService, error)
	GetExternalService(ctx context.Context, tenantID, name string) (*ExternalService, error)
	CreateExternalService(ctx context.Context, service *ExternalService) error
	UpdateExternalService(ctx context.Context, service *ExternalService) error
	
	// LLM provider operations
	GetLLMProviders(ctx context.Context, tenantID string) ([]LLMProviderConfig, error)
	GetLLMProvider(ctx context.Context, tenantID, name string) (*LLMProviderConfig, error)
	GetDefaultLLMProvider(ctx context.Context, tenantID string) (*LLMProviderConfig, error)
	CreateLLMProvider(ctx context.Context, config *LLMProviderConfig) error
	UpdateLLMProvider(ctx context.Context, config *LLMProviderConfig) error
	
	// Utility operations
	Ping(ctx context.Context) error
	Close() error
}

// Agent defines the interface for all agents
type Agent interface {
	// Name returns the agent name
	Name() string
	
	// AllowedTenants returns the list of tenant IDs this agent can serve
	AllowedTenants() []string
	
	// CanHandle checks if the agent can handle a specific intent
	CanHandle(intent string) bool
	
	// Handle processes a request and returns a response
	Handle(ctx context.Context, req *AgentRequest) (*AgentResponse, error)
}

// Tool defines the interface for MCP-like tools
type Tool interface {
	// Name returns the tool name
	Name() string
	
	// Schema returns the JSON schema for tool parameters
	Schema() *JSONSchema
	
	// Invoke executes the tool with the given input
	Invoke(ctx context.Context, input map[string]interface{}) (interface{}, error)
}

// Orchestrator defines the interface for the main orchestrator
type Orchestrator interface {
	// Route processes a message and determines the appropriate actions
	Route(ctx context.Context, tenant *Tenant, user *User, message *Message) (*AgentResponse, error)
}

// TenantManager defines the interface for managing tenants
type TenantManager interface {
	// GetTenant retrieves tenant configuration by WABA number
	GetTenant(wabaNumber string) (*Tenant, error)
	
	// GetTenantByID retrieves tenant configuration by tenant ID
	GetTenantByID(tenantID string) (*Tenant, error)
	
	// ListTenants returns all configured tenants
	ListTenants() ([]Tenant, error)
	
	// IsAgentEnabled checks if an agent is enabled for a tenant
	IsAgentEnabled(tenantID, agentName string) bool
	
	// GetRepository returns a repository instance for the tenant
	GetRepository(tenantID string) (Repository, error)
	
	// GetVectorStore returns a vector store instance for the tenant
	GetVectorStore(tenantID string) (VectorStore, error)
	
	// GetLLMProvider returns an LLM provider instance for the tenant
	GetLLMProvider(tenantID string) (LLMProvider, error)
}

// InfobipClient defines the interface for Infobip API client
type InfobipClient interface {
	// SendText sends a text message via WhatsApp
	SendText(ctx context.Context, from, to, text string, messageIDRef ...string) (*InfobipMessage, error)
	
	// SendMessage sends a structured message
	SendMessage(ctx context.Context, message *InfobipMessage) (*InfobipMessage, error)
}

// RAGPipeline defines the interface for RAG operations
type RAGPipeline interface {
	// StoreMemory stores a memory item with embedding
	StoreMemory(ctx context.Context, tenantID string, userID uuid.UUID, item *MemoryItem) (*uuid.UUID, error)
	
	// SearchMemory searches for relevant memories
	SearchMemory(ctx context.Context, tenantID string, userID uuid.UUID, query string, opts *SearchOptions) ([]MemoryHit, error)
	
	// GetContext builds context from search results
	GetContext(ctx context.Context, memories []MemoryHit, maxTokens int) string
}

// MessageProcessor defines the interface for processing incoming messages
type MessageProcessor interface {
	// ProcessIncoming processes an incoming webhook message
	ProcessIncoming(ctx context.Context, webhookMsg *InfobipWebhookMessage) error
	
	// ProcessMessage processes a single message
	ProcessMessage(ctx context.Context, tenant *Tenant, user *User, message *Message) error
}

// ToolRegistry defines the interface for managing tools
type ToolRegistry interface {
	// RegisterTool registers a new tool
	RegisterTool(tool Tool) error
	
	// GetTool retrieves a tool by name
	GetTool(name string) (Tool, error)
	
	// ListTools returns all registered tools
	ListTools() []Tool
	
	// GetToolsForTenant returns tools available for a specific tenant
	GetToolsForTenant(tenantID string) []Tool
	
	// ExecuteToolCall executes a tool call from LLM
	ExecuteToolCall(ctx context.Context, toolCall *ToolCall) (*ToolInvocationResult, error)
}

// AgentRegistry defines the interface for managing agents
type AgentRegistry interface {
	// RegisterAgent registers a new agent
	RegisterAgent(agent Agent) error
	
	// GetAgent retrieves an agent by name
	GetAgent(name string) (Agent, error)
	
	// ListAgents returns all registered agents
	ListAgents() []Agent
	
	// GetAgentsForTenant returns agents available for a specific tenant
	GetAgentsForTenant(tenantID string) []Agent
}