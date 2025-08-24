package builtin

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"personal-assistant/internal/domain"
	"personal-assistant/internal/log"
)

// DBAgent handles database operations for user memory
type DBAgent struct {
	name           string
	allowedTenants []string
	vectorStore    domain.VectorStore
	llmProvider    domain.LLMProvider
	logger         *log.Logger
}

// NewDBAgent creates a new database agent
func NewDBAgent(vectorStore domain.VectorStore, llmProvider domain.LLMProvider, logger *log.Logger, allowedTenants []string) *DBAgent {
	return &DBAgent{
		name:           "db_agent",
		allowedTenants: allowedTenants,
		vectorStore:    vectorStore,
		llmProvider:    llmProvider,
		logger:         logger,
	}
}

// Name returns the agent name
func (a *DBAgent) Name() string {
	return a.name
}

// AllowedTenants returns the list of tenant IDs this agent can serve
func (a *DBAgent) AllowedTenants() []string {
	return a.allowedTenants
}

// CanHandle checks if the agent can handle a specific intent
func (a *DBAgent) CanHandle(intent string) bool {
	dbIntents := []string{
		"store_memory", "search_memory", "get_memory", "update_memory", "delete_memory",
		"upsert_item", "search", "get_by_id", "update_item",
		"note", "task", "event", "reminder", "memory",
	}
	
	for _, dbIntent := range dbIntents {
		if intent == dbIntent {
			return true
		}
	}
	return false
}

// Handle processes a request and returns a response
func (a *DBAgent) Handle(ctx context.Context, req *domain.AgentRequest) (*domain.AgentResponse, error) {
	a.logger.WithContext(ctx).Debug().
		Str("tenant_id", req.TenantID).
		Str("user_id", req.UserID.String()).
		Str("text", req.Text).
		Msg("DB agent handling request")
	
	// This agent works by providing tools to the LLM
	// Return available tools as part of the response
	return &domain.AgentResponse{
		Text: "I can help you manage your memory with database operations. Available tools: upsert_item, search, get_by_id, update_item.",
		Metadata: map[string]interface{}{
			"available_tools": []string{"upsert_item", "search", "get_by_id", "update_item"},
			"agent_type":      "database",
		},
	}, nil
}

// DBUpsertTool handles memory item creation/updates
type DBUpsertTool struct {
	vectorStore domain.VectorStore
	llmProvider domain.LLMProvider
	logger      *log.Logger
}

// NewDBUpsertTool creates a new upsert tool
func NewDBUpsertTool(vectorStore domain.VectorStore, llmProvider domain.LLMProvider, logger *log.Logger) *DBUpsertTool {
	return &DBUpsertTool{
		vectorStore: vectorStore,
		llmProvider: llmProvider,
		logger:      logger,
	}
}

// Name returns the tool name
func (t *DBUpsertTool) Name() string {
	return "upsert_item"
}

// Schema returns the JSON schema for the tool parameters
func (t *DBUpsertTool) Schema() *domain.JSONSchema {
	return &domain.JSONSchema{
		Type: "object",
		Properties: map[string]domain.JSONSchemaProperty{
			"kind": {
				Type:        "string",
				Description: "Type of memory item: note, event, task, or msg",
				Enum:        []string{"note", "event", "task", "msg"},
			},
			"text": {
				Type:        "string",
				Description: "The content text of the memory item",
			},
			"when": {
				Type:        "string",
				Description: "ISO8601 timestamp for events/tasks (optional)",
			},
			"tags": {
				Type:        "array",
				Description: "Tags to categorize the item (optional)",
				Items: &domain.JSONSchemaProperty{
					Type: "string",
				},
			},
		},
		Required: []string{"kind", "text"},
	}
}

// Invoke executes the tool with the given input
func (t *DBUpsertTool) Invoke(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	// Extract parameters
	kind, _ := input["kind"].(string)
	text, _ := input["text"].(string)
	whenStr, _ := input["when"].(string)
	tagsInterface, _ := input["tags"].([]interface{})
	
	// Extract tenant and user info from context
	tenantID, _ := ctx.Value(log.TenantIDKey).(string)
	userIDStr, _ := ctx.Value(log.UserIDKey).(string)
	
	if tenantID == "" || userIDStr == "" {
		return nil, fmt.Errorf("missing tenant_id or user_id in context")
	}
	
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid user_id format: %w", err)
	}
	
	// Process tags
	var tags []string
	if tagsInterface != nil {
		for _, tag := range tagsInterface {
			if tagStr, ok := tag.(string); ok {
				tags = append(tags, tagStr)
			}
		}
	}
	
	// Create metadata
	metadata := map[string]interface{}{
		"created_at": time.Now().UTC().Format(time.RFC3339),
	}
	
	if whenStr != "" {
		if _, err := time.Parse(time.RFC3339, whenStr); err != nil {
			return nil, fmt.Errorf("invalid timestamp format, use ISO8601: %w", err)
		}
		metadata["when"] = whenStr
	}
	
	if len(tags) > 0 {
		metadata["tags"] = tags
	}
	
	// Generate embedding for the text
	embeddings, err := t.llmProvider.Embed(ctx, []string{text})
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}
	
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings generated")
	}
	
	// Add embedding to metadata
	metadata["embedding"] = embeddings[0]
	
	// Create memory item
	item := domain.MemoryItem{
		Kind:     kind,
		Text:     text,
		Metadata: metadata,
	}
	
	// Store in vector store
	ids, err := t.vectorStore.Upsert(ctx, tenantID, userID, []domain.MemoryItem{item})
	if err != nil {
		return nil, fmt.Errorf("failed to store memory item: %w", err)
	}
	
	if len(ids) == 0 {
		return nil, fmt.Errorf("no items were stored")
	}
	
	t.logger.WithContext(ctx).Info().
		Str("tenant_id", tenantID).
		Str("user_id", userID.String()).
		Str("kind", kind).
		Str("id", ids[0].String()).
		Msg("memory item stored successfully")
	
	return map[string]interface{}{
		"id":     ids[0].String(),
		"status": "stored",
		"kind":   kind,
		"text":   text,
		"when":   whenStr,
		"tags":   tags,
	}, nil
}

// DBSearchTool handles memory searches
type DBSearchTool struct {
	vectorStore domain.VectorStore
	llmProvider domain.LLMProvider
	logger      *log.Logger
}

// NewDBSearchTool creates a new search tool
func NewDBSearchTool(vectorStore domain.VectorStore, llmProvider domain.LLMProvider, logger *log.Logger) *DBSearchTool {
	return &DBSearchTool{
		vectorStore: vectorStore,
		llmProvider: llmProvider,
		logger:      logger,
	}
}

// Name returns the tool name
func (t *DBSearchTool) Name() string {
	return "search"
}

// Schema returns the JSON schema for the tool parameters
func (t *DBSearchTool) Schema() *domain.JSONSchema {
	return &domain.JSONSchema{
		Type: "object",
		Properties: map[string]domain.JSONSchemaProperty{
			"query": {
				Type:        "string",
				Description: "Search query to find relevant memory items",
			},
			"top_k": {
				Type:        "integer",
				Description: "Number of results to return (default: 5, max: 20)",
			},
			"filter": {
				Type:        "object",
				Description: "Optional filters to apply",
				Properties: map[string]domain.JSONSchemaProperty{
					"kind": {
						Type:        "array",
						Description: "Filter by memory item types",
						Items: &domain.JSONSchemaProperty{
							Type: "string",
							Enum: []string{"note", "event", "task", "msg"},
						},
					},
					"tags": {
						Type:        "array",
						Description: "Filter by tags",
						Items: &domain.JSONSchemaProperty{
							Type: "string",
						},
					},
				},
			},
		},
		Required: []string{"query"},
	}
}

// Invoke executes the tool with the given input
func (t *DBSearchTool) Invoke(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	// Extract parameters
	query, _ := input["query"].(string)
	topK := 5
	if topKFloat, ok := input["top_k"].(float64); ok {
		topK = int(topKFloat)
		if topK > 20 {
			topK = 20
		}
	}
	
	// Extract tenant and user info from context
	tenantID, _ := ctx.Value(log.TenantIDKey).(string)
	userIDStr, _ := ctx.Value(log.UserIDKey).(string)
	
	if tenantID == "" || userIDStr == "" {
		return nil, fmt.Errorf("missing tenant_id or user_id in context")
	}
	
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid user_id format: %w", err)
	}
	
	// Generate embedding for the query
	embeddings, err := t.llmProvider.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}
	
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings generated for query")
	}
	
	// Parse filter options
	var filter *domain.SearchFilter
	if filterMap, ok := input["filter"].(map[string]interface{}); ok {
		filter = &domain.SearchFilter{}
		
		if kindArray, ok := filterMap["kind"].([]interface{}); ok {
			for _, k := range kindArray {
				if kindStr, ok := k.(string); ok {
					filter.Kinds = append(filter.Kinds, kindStr)
				}
			}
		}
		
		if tagArray, ok := filterMap["tags"].([]interface{}); ok {
			for _, tag := range tagArray {
				if tagStr, ok := tag.(string); ok {
					filter.Tags = append(filter.Tags, tagStr)
				}
			}
		}
	}
	
	// Create search options
	opts := &domain.SearchOptions{
		TopK:     topK,
		MinScore: 0.7, // Minimum similarity threshold
		Filter:   filter,
	}
	
	// Perform search
	hits, err := t.vectorStore.Search(ctx, tenantID, userID, embeddings[0], opts)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}
	
	t.logger.WithContext(ctx).Debug().
		Str("tenant_id", tenantID).
		Str("user_id", userID.String()).
		Str("query", query).
		Int("hits", len(hits)).
		Msg("memory search completed")
	
	// Format results
	items := make([]map[string]interface{}, len(hits))
	for i, hit := range hits {
		items[i] = map[string]interface{}{
			"id":       hit.ID.String(),
			"kind":     hit.Kind,
			"text":     hit.Text,
			"score":    hit.Score,
			"metadata": hit.Metadata,
		}
	}
	
	return map[string]interface{}{
		"items":       items,
		"query":       query,
		"total_found": len(hits),
		"search_options": map[string]interface{}{
			"top_k":     topK,
			"min_score": opts.MinScore,
			"filter":    filter,
		},
	}, nil
}

// DBGetByIDTool retrieves memory items by ID
type DBGetByIDTool struct {
	vectorStore domain.VectorStore
	logger      *log.Logger
}

// NewDBGetByIDTool creates a new get by ID tool
func NewDBGetByIDTool(vectorStore domain.VectorStore, logger *log.Logger) *DBGetByIDTool {
	return &DBGetByIDTool{
		vectorStore: vectorStore,
		logger:      logger,
	}
}

// Name returns the tool name
func (t *DBGetByIDTool) Name() string {
	return "get_by_id"
}

// Schema returns the JSON schema for the tool parameters
func (t *DBGetByIDTool) Schema() *domain.JSONSchema {
	return &domain.JSONSchema{
		Type: "object",
		Properties: map[string]domain.JSONSchemaProperty{
			"id": {
				Type:        "string",
				Description: "UUID of the memory item to retrieve",
			},
		},
		Required: []string{"id"},
	}
}

// Invoke executes the tool with the given input
func (t *DBGetByIDTool) Invoke(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	// Extract parameters
	idStr, _ := input["id"].(string)
	
	// Extract tenant and user info from context
	tenantID, _ := ctx.Value(log.TenantIDKey).(string)
	userIDStr, _ := ctx.Value(log.UserIDKey).(string)
	
	if tenantID == "" || userIDStr == "" {
		return nil, fmt.Errorf("missing tenant_id or user_id in context")
	}
	
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid user_id format: %w", err)
	}
	
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, fmt.Errorf("invalid id format: %w", err)
	}
	
	// Get the item
	item, err := t.vectorStore.GetByID(ctx, tenantID, userID, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get item: %w", err)
	}
	
	if item == nil {
		return map[string]interface{}{
			"found": false,
			"id":    idStr,
		}, nil
	}
	
	return map[string]interface{}{
		"found":      true,
		"id":         item.ID.String(),
		"kind":       item.Kind,
		"text":       item.Text,
		"metadata":   item.Metadata,
		"created_at": item.CreatedAt.Format(time.RFC3339),
		"updated_at": item.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// DBUpdateItemTool updates existing memory items
type DBUpdateItemTool struct {
	vectorStore domain.VectorStore
	llmProvider domain.LLMProvider
	logger      *log.Logger
}

// NewDBUpdateItemTool creates a new update item tool
func NewDBUpdateItemTool(vectorStore domain.VectorStore, llmProvider domain.LLMProvider, logger *log.Logger) *DBUpdateItemTool {
	return &DBUpdateItemTool{
		vectorStore: vectorStore,
		llmProvider: llmProvider,
		logger:      logger,
	}
}

// Name returns the tool name
func (t *DBUpdateItemTool) Name() string {
	return "update_item"
}

// Schema returns the JSON schema for the tool parameters
func (t *DBUpdateItemTool) Schema() *domain.JSONSchema {
	return &domain.JSONSchema{
		Type: "object",
		Properties: map[string]domain.JSONSchemaProperty{
			"id": {
				Type:        "string",
				Description: "UUID of the memory item to update",
			},
			"updates": {
				Type:        "object",
				Description: "Fields to update",
				Properties: map[string]domain.JSONSchemaProperty{
					"text": {
						Type:        "string",
						Description: "Update the text content",
					},
					"when": {
						Type:        "string",
						Description: "Update the timestamp (ISO8601)",
					},
					"tags": {
						Type:        "array",
						Description: "Update the tags",
						Items: &domain.JSONSchemaProperty{
							Type: "string",
						},
					},
				},
			},
		},
		Required: []string{"id", "updates"},
	}
}

// Invoke executes the tool with the given input
func (t *DBUpdateItemTool) Invoke(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	// Extract parameters
	idStr, _ := input["id"].(string)
	updatesMap, _ := input["updates"].(map[string]interface{})
	
	// Extract tenant and user info from context
	tenantID, _ := ctx.Value(log.TenantIDKey).(string)
	userIDStr, _ := ctx.Value(log.UserIDKey).(string)
	
	if tenantID == "" || userIDStr == "" {
		return nil, fmt.Errorf("missing tenant_id or user_id in context")
	}
	
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid user_id format: %w", err)
	}
	
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, fmt.Errorf("invalid id format: %w", err)
	}
	
	if len(updatesMap) == 0 {
		return nil, fmt.Errorf("no updates provided")
	}
	
	// Build updates map
	updates := make(map[string]interface{})
	
	// Handle text updates (requires re-embedding)
	if text, ok := updatesMap["text"].(string); ok {
		updates["text"] = text
		
		// Generate new embedding
		embeddings, err := t.llmProvider.Embed(ctx, []string{text})
		if err != nil {
			return nil, fmt.Errorf("failed to generate embedding for updated text: %w", err)
		}
		if len(embeddings) > 0 {
			updates["embedding"] = embeddings[0]
		}
	}
	
	// Handle metadata updates
	metadata := make(map[string]interface{})
	
	if whenStr, ok := updatesMap["when"].(string); ok {
		if _, err := time.Parse(time.RFC3339, whenStr); err != nil {
			return nil, fmt.Errorf("invalid timestamp format, use ISO8601: %w", err)
		}
		metadata["when"] = whenStr
	}
	
	if tagsInterface, ok := updatesMap["tags"].([]interface{}); ok {
		var tags []string
		for _, tag := range tagsInterface {
			if tagStr, ok := tag.(string); ok {
				tags = append(tags, tagStr)
			}
		}
		metadata["tags"] = tags
	}
	
	if len(metadata) > 0 {
		metadata["updated_at"] = time.Now().UTC().Format(time.RFC3339)
		updates["metadata"] = metadata
	}
	
	// Perform update
	err = t.vectorStore.UpdateByID(ctx, tenantID, userID, id, updates)
	if err != nil {
		return nil, fmt.Errorf("failed to update item: %w", err)
	}
	
	t.logger.WithContext(ctx).Info().
		Str("tenant_id", tenantID).
		Str("user_id", userID.String()).
		Str("id", idStr).
		Interface("updates", updates).
		Msg("memory item updated successfully")
	
	return map[string]interface{}{
		"ok":      true,
		"id":      idStr,
		"updates": updatesMap,
	}, nil
}