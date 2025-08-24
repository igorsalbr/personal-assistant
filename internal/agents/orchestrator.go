package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"personal-assistant/internal/domain"
	"personal-assistant/internal/log"
)

// MainOrchestrator is the primary agent that coordinates all operations
type MainOrchestrator struct {
	llmProvider   domain.LLMProvider
	ragPipeline   domain.RAGPipeline
	toolRegistry  domain.ToolRegistry
	agentRegistry domain.AgentRegistry
	logger        *log.Logger
	config        *OrchestratorConfig
}

// OrchestratorConfig holds configuration for the orchestrator
type OrchestratorConfig struct {
	MaxTokens        int
	Temperature      float32
	MaxToolCalls     int
	EnableRAG        bool
	RAGTopK          int
	RAGMinScore      float64
	ContextTokens    int
	SummarizeEnabled bool
}

// NewMainOrchestrator creates a new main orchestrator
func NewMainOrchestrator(
	llmProvider domain.LLMProvider,
	ragPipeline domain.RAGPipeline,
	toolRegistry domain.ToolRegistry,
	agentRegistry domain.AgentRegistry,
	logger *log.Logger,
	config *OrchestratorConfig,
) *MainOrchestrator {
	if config == nil {
		config = &OrchestratorConfig{
			MaxTokens:        500,
			Temperature:      0.7,
			MaxToolCalls:     3,
			EnableRAG:        true,
			RAGTopK:          5,
			RAGMinScore:      0.7,
			ContextTokens:    2000,
			SummarizeEnabled: true,
		}
	}
	
	return &MainOrchestrator{
		llmProvider:   llmProvider,
		ragPipeline:   ragPipeline,
		toolRegistry:  toolRegistry,
		agentRegistry: agentRegistry,
		logger:        logger,
		config:        config,
	}
}

// Route processes a message and determines the appropriate actions
func (o *MainOrchestrator) Route(ctx context.Context, tenant *domain.Tenant, user *domain.User, message *domain.Message) (*domain.AgentResponse, error) {
	start := time.Now()
	
	// Add tenant and user context to the context
	ctx = context.WithValue(ctx, log.TenantIDKey, tenant.ID)
	ctx = context.WithValue(ctx, log.UserIDKey, user.ID.String())
	
	logger := o.logger.WithContext(ctx).WithTenant(tenant.ID).WithUser(user.ID.String())
	
	logger.Debug().
		Str("message_text", log.SanitizeText(message.Text)).
		Msg("orchestrator processing message")
	
	// Detect intent
	intent := DetectIntent(message.Text)
	logger.Debug().Str("detected_intent", intent).Msg("intent detected")
	
	// Get RAG context if enabled
	var memoryContext []MemoryContextItem
	if o.config.EnableRAG && o.ragPipeline != nil {
		ragHits, err := o.ragPipeline.SearchMemory(ctx, tenant.ID, user.ID, message.Text, &domain.SearchOptions{
			TopK:     o.config.RAGTopK,
			MinScore: o.config.RAGMinScore,
		})
		if err != nil {
			logger.Warn().Err(err).Msg("RAG search failed, continuing without context")
		} else {
			// Convert to memory context items
			for _, hit := range ragHits {
				memoryContext = append(memoryContext, MemoryContextItem{
					Kind:      hit.Kind,
					Text:      hit.Text,
					Score:     hit.Score,
					CreatedAt: time.Now(), // Would be from hit metadata in real implementation
				})
			}
			logger.Debug().Int("memory_items", len(memoryContext)).Msg("RAG context loaded")
		}
	}
	
	// Build system prompt with context
	promptConfig := &SystemPromptConfig{
		TenantName:     tenant.ID, // Could be a friendly name from config
		UserName:       user.Phone, // Could be from user profile
		CurrentTime:    time.Now(),
		AvailableTools: o.getAvailableToolNames(tenant.ID),
	}
	
	systemPrompt := BuildContextualPrompt(promptConfig, memoryContext, nil)
	
	// Check if this requires tool gating (simple conversational message)
	if !o.requiresTools(message.Text, intent) {
		response, err := o.handleConversationalMessage(ctx, systemPrompt, message.Text, memoryContext)
		if err != nil {
			return nil, fmt.Errorf("failed to handle conversational message: %w", err)
		}
		
		logger.Info().
			Dur("duration", time.Since(start)).
			Bool("used_tools", false).
			Msg("message processed conversationally")
		
		return response, nil
	}
	
	// Handle with tools
	response, err := o.handleWithTools(ctx, systemPrompt, message.Text, tenant, user)
	if err != nil {
		return nil, fmt.Errorf("failed to handle message with tools: %w", err)
	}
	
	logger.Info().
		Dur("duration", time.Since(start)).
		Bool("used_tools", true).
		Msg("message processed with tools")
	
	return response, nil
}

// requiresTools determines if a message needs tool invocation
func (o *MainOrchestrator) requiresTools(text, intent string) bool {
	// Skip tools for purely conversational intents
	if intent == "conversational" {
		return false
	}
	
	// Skip tools for simple greetings and acknowledgments
	lowerText := strings.ToLower(text)
	conversationalPhrases := []string{
		"hello", "hi", "hey", "good morning", "good afternoon", "good evening",
		"how are you", "what's up", "thanks", "thank you", "bye", "goodbye",
		"ok", "okay", "sure", "alright", "got it", "understood",
	}
	
	for _, phrase := range conversationalPhrases {
		if strings.Contains(lowerText, phrase) && len(strings.Fields(text)) <= 3 {
			return false
		}
	}
	
	// Require tools for specific intents
	toolRequiredIntents := []string{
		"memory_store", "memory_search", "memory_update", 
		"api_call", "schedule",
	}
	
	for _, toolIntent := range toolRequiredIntents {
		if intent == toolIntent {
			return true
		}
	}
	
	// Default to requiring tools for longer, complex messages
	return len(strings.Fields(text)) > 8
}

// handleConversationalMessage handles simple conversational messages without tools
func (o *MainOrchestrator) handleConversationalMessage(ctx context.Context, systemPrompt, userMessage string, memoryContext []MemoryContextItem) (*domain.AgentResponse, error) {
	messages := []domain.ChatMessage{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: userMessage,
		},
	}
	
	// Add memory context if available
	if len(memoryContext) > 0 {
		contextMsg := "Here's relevant context from your memory:\n"
		for _, item := range memoryContext {
			contextMsg += fmt.Sprintf("- %s: %s\n", item.Kind, item.Text)
		}
		
		messages = append(messages, domain.ChatMessage{
			Role:    "system",
			Content: contextMsg,
		})
	}
	
	req := &domain.ChatCompletionRequest{
		Messages:    messages,
		MaxTokens:   o.config.MaxTokens,
		Temperature: o.config.Temperature,
	}
	
	resp, err := o.llmProvider.Chat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("LLM chat failed: %w", err)
	}
	
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response choices from LLM")
	}
	
	return &domain.AgentResponse{
		Text: resp.Choices[0].Message.Content,
		Metadata: map[string]interface{}{
			"type":           "conversational",
			"memory_context": len(memoryContext),
			"token_usage":    resp.Usage,
		},
	}, nil
}

// handleWithTools handles messages that require tool invocation
func (o *MainOrchestrator) handleWithTools(ctx context.Context, systemPrompt, userMessage string, tenant *domain.Tenant, user *domain.User) (*domain.AgentResponse, error) {
	messages := []domain.ChatMessage{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: userMessage,
		},
	}
	
	// Get available tools for the tenant
	availableTools := o.toolRegistry.GetToolsForTenant(tenant.ID)
	llmTools := o.convertToolsForLLM(availableTools)
	
	var allToolResults []domain.ToolInvocationResult
	maxIterations := o.config.MaxToolCalls
	
	for iteration := 0; iteration < maxIterations; iteration++ {
		req := &domain.ChatCompletionRequest{
			Messages:    messages,
			Tools:       llmTools,
			ToolChoice:  "auto",
			MaxTokens:   o.config.MaxTokens,
			Temperature: o.config.Temperature,
		}
		
		resp, err := o.llmProvider.Chat(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("LLM chat failed on iteration %d: %w", iteration, err)
		}
		
		if len(resp.Choices) == 0 {
			return nil, fmt.Errorf("no response choices from LLM")
		}
		
		choice := resp.Choices[0]
		
		// Add assistant message to conversation
		messages = append(messages, *choice.Message)
		
		// Check if we have tool calls
		if len(choice.Message.ToolCalls) == 0 {
			// No more tool calls, return final response
			return &domain.AgentResponse{
				Text: choice.Message.Content,
				Metadata: map[string]interface{}{
					"type":         "tool_assisted",
					"iterations":   iteration + 1,
					"tool_results": allToolResults,
					"token_usage":  resp.Usage,
				},
			}, nil
		}
		
		// Execute tool calls
		for _, toolCall := range choice.Message.ToolCalls {
			result, err := o.toolRegistry.ExecuteToolCall(ctx, &toolCall)
			if err != nil {
				o.logger.WithContext(ctx).Error().
					Err(err).
					Str("tool_name", toolCall.Function.Name).
					Msg("tool execution failed")
				
				result = &domain.ToolInvocationResult{
					ToolName: toolCall.Function.Name,
					Success:  false,
					Error:    err.Error(),
				}
			}
			
			allToolResults = append(allToolResults, *result)
			
			// Add tool result to conversation
			var resultContent string
			if result.Success {
				if resultStr, ok := result.Result.(string); ok {
					resultContent = resultStr
				} else {
					resultBytes, _ := json.Marshal(result.Result)
					resultContent = string(resultBytes)
				}
			} else {
				resultContent = fmt.Sprintf("Error: %s", result.Error)
			}
			
			messages = append(messages, domain.ChatMessage{
				Role:       "tool",
				Content:    resultContent,
				ToolCallID: toolCall.ID,
				Name:       toolCall.Function.Name,
			})
		}
	}
	
	// If we've reached max iterations, return with partial results
	return &domain.AgentResponse{
		Text: "I was able to process your request partially, but reached the maximum number of tool calls allowed.",
		Metadata: map[string]interface{}{
			"type":            "tool_assisted_partial",
			"max_iterations":  maxIterations,
			"tool_results":    allToolResults,
			"warning":         "reached_max_tool_calls",
		},
	}, nil
}

// getAvailableToolNames returns the names of tools available for a tenant
func (o *MainOrchestrator) getAvailableToolNames(tenantID string) []string {
	tools := o.toolRegistry.GetToolsForTenant(tenantID)
	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.Name()
	}
	return names
}

// convertToolsForLLM converts domain tools to LLM tool format
func (o *MainOrchestrator) convertToolsForLLM(tools []domain.Tool) []domain.ToolDefinition {
	llmTools := make([]domain.ToolDefinition, len(tools))
	for i, tool := range tools {
		llmTools[i] = domain.ToolDefinition{
			Type: "function",
			Function: &domain.ToolFunction{
				Name:        tool.Name(),
				Description: o.getToolDescription(tool),
				Parameters:  o.convertSchemaToParameters(tool.Schema()),
			},
		}
	}
	return llmTools
}

// getToolDescription gets a description for a tool
func (o *MainOrchestrator) getToolDescription(tool domain.Tool) string {
	// This would ideally come from the tool itself
	// For now, provide descriptions based on tool names
	switch tool.Name() {
	case "upsert_item":
		return "Store or update information in memory (notes, tasks, events)"
	case "search":
		return "Search through stored memories using semantic similarity"
	case "get_by_id":
		return "Retrieve a specific memory item by its ID"
	case "update_item":
		return "Update an existing memory item"
	case "call_api":
		return "Make HTTP API calls to external services"
	case "schedule_reminder":
		return "Schedule future reminders"
	default:
		return fmt.Sprintf("Tool: %s", tool.Name())
	}
}

// convertSchemaToParameters converts a JSON schema to LLM parameters format
func (o *MainOrchestrator) convertSchemaToParameters(schema *domain.JSONSchema) map[string]interface{} {
	params := map[string]interface{}{
		"type": "object",
	}
	
	if len(schema.Properties) > 0 {
		properties := make(map[string]interface{})
		for propName, propSchema := range schema.Properties {
			properties[propName] = o.convertPropertySchema(propSchema)
		}
		params["properties"] = properties
	}
	
	if len(schema.Required) > 0 {
		params["required"] = schema.Required
	}
	
	return params
}

// convertPropertySchema converts a property schema to LLM format
func (o *MainOrchestrator) convertPropertySchema(prop domain.JSONSchemaProperty) map[string]interface{} {
	result := map[string]interface{}{
		"type": prop.Type,
	}
	
	if prop.Description != "" {
		result["description"] = prop.Description
	}
	
	if len(prop.Enum) > 0 {
		result["enum"] = prop.Enum
	}
	
	if prop.Items != nil {
		result["items"] = o.convertPropertySchema(*prop.Items)
	}
	
	if len(prop.Properties) > 0 {
		properties := make(map[string]interface{})
		for propName, propSchema := range prop.Properties {
			properties[propName] = o.convertPropertySchema(propSchema)
		}
		result["properties"] = properties
	}
	
	return result
}

// DefaultOrchestratorConfig returns the default orchestrator configuration
func DefaultOrchestratorConfig() *OrchestratorConfig {
	return &OrchestratorConfig{
		MaxTokens:        500,
		Temperature:      0.7,
		MaxToolCalls:     3,
		EnableRAG:        true,
		RAGTopK:          5,
		RAGMinScore:      0.7,
		ContextTokens:    2000,
		SummarizeEnabled: true,
	}
}