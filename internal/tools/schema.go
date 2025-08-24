package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"personal-assistant/internal/domain"
)

// Registry manages all available tools
type Registry struct {
	tools   map[string]domain.Tool
	schemas map[string]*domain.JSONSchema
}

// NewRegistry creates a new tool registry
func NewRegistry() *Registry {
	return &Registry{
		tools:   make(map[string]domain.Tool),
		schemas: make(map[string]*domain.JSONSchema),
	}
}

// RegisterTool registers a new tool
func (r *Registry) RegisterTool(tool domain.Tool) error {
	name := tool.Name()
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool %s already registered", name)
	}
	
	r.tools[name] = tool
	r.schemas[name] = tool.Schema()
	return nil
}

// GetTool retrieves a tool by name
func (r *Registry) GetTool(name string) (domain.Tool, error) {
	tool, exists := r.tools[name]
	if !exists {
		return nil, fmt.Errorf("tool %s not found", name)
	}
	return tool, nil
}

// ListTools returns all registered tools
func (r *Registry) ListTools() []domain.Tool {
	tools := make([]domain.Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// GetToolsForTenant returns tools available for a specific tenant
func (r *Registry) GetToolsForTenant(tenantID string) []domain.Tool {
	// For now, return all tools. In production, you might want to
	// filter based on tenant permissions or enabled tools
	return r.ListTools()
}

// GetSchema returns the JSON schema for a tool
func (r *Registry) GetSchema(toolName string) (*domain.JSONSchema, error) {
	schema, exists := r.schemas[toolName]
	if !exists {
		return nil, fmt.Errorf("schema for tool %s not found", toolName)
	}
	return schema, nil
}

// ValidateInput validates tool input against its schema
func (r *Registry) ValidateInput(toolName string, input map[string]interface{}) error {
	schema, err := r.GetSchema(toolName)
	if err != nil {
		return err
	}
	
	// Basic validation - check required fields
	if schema.Required != nil {
		for _, requiredField := range schema.Required {
			if _, exists := input[requiredField]; !exists {
				return fmt.Errorf("required field '%s' is missing", requiredField)
			}
		}
	}
	
	// Additional validation could be implemented here
	// For now, we'll do basic type checking for properties
	if schema.Properties != nil {
		for fieldName, fieldSchema := range schema.Properties {
			if value, exists := input[fieldName]; exists {
				if err := validateFieldType(fieldName, value, fieldSchema); err != nil {
					return err
				}
			}
		}
	}
	
	return nil
}

// validateFieldType performs basic type validation
func validateFieldType(fieldName string, value interface{}, schema domain.JSONSchemaProperty) error {
	switch schema.Type {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("field '%s' must be a string", fieldName)
		}
	case "integer":
		switch value.(type) {
		case int, int32, int64, float64:
			// Accept various numeric types
		default:
			return fmt.Errorf("field '%s' must be an integer", fieldName)
		}
	case "number":
		switch value.(type) {
		case int, int32, int64, float32, float64:
			// Accept various numeric types
		default:
			return fmt.Errorf("field '%s' must be a number", fieldName)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("field '%s' must be a boolean", fieldName)
		}
	case "array":
		if _, ok := value.([]interface{}); !ok {
			return fmt.Errorf("field '%s' must be an array", fieldName)
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("field '%s' must be an object", fieldName)
		}
	}
	
	// Validate enum values if specified
	if len(schema.Enum) > 0 {
		valueStr, ok := value.(string)
		if !ok {
			return fmt.Errorf("field '%s' with enum constraint must be a string", fieldName)
		}
		
		for _, enumValue := range schema.Enum {
			if enumValue == valueStr {
				return nil
			}
		}
		return fmt.Errorf("field '%s' must be one of: %v", fieldName, schema.Enum)
	}
	
	return nil
}

// InvokeTool invokes a tool with the given input
func (r *Registry) InvokeTool(ctx context.Context, toolName string, input map[string]interface{}) (interface{}, error) {
	// Validate input
	if err := r.ValidateInput(toolName, input); err != nil {
		return nil, fmt.Errorf("validation failed for tool %s: %w", toolName, err)
	}
	
	// Get tool
	tool, err := r.GetTool(toolName)
	if err != nil {
		return nil, err
	}
	
	// Invoke tool
	return tool.Invoke(ctx, input)
}

// ConvertToLLMTools converts registered tools to LLM tool format
func (r *Registry) ConvertToLLMTools() []domain.ToolDefinition {
	llmTools := make([]domain.ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		llmTool := domain.ToolDefinition{
			Type: "function",
			Function: &domain.ToolFunction{
				Name:        tool.Name(),
				Description: r.getToolDescription(tool),
				Parameters:  r.convertSchemaToParameters(tool.Schema()),
			},
		}
		llmTools = append(llmTools, llmTool)
	}
	return llmTools
}

// getToolDescription extracts description from tool schema or provides default
func (r *Registry) getToolDescription(tool domain.Tool) string {
	// Try to get description from schema
	schema := tool.Schema()
	if desc, exists := schema.Properties["description"]; exists {
		if descStr := desc.Description; descStr != "" {
			return descStr
		}
	}
	
	// Default description based on tool name
	return fmt.Sprintf("Tool: %s", tool.Name())
}

// convertSchemaToParameters converts JSONSchema to LLM parameters format
func (r *Registry) convertSchemaToParameters(schema *domain.JSONSchema) map[string]interface{} {
	params := map[string]interface{}{
		"type": "object",
	}
	
	if len(schema.Properties) > 0 {
		properties := make(map[string]interface{})
		for propName, propSchema := range schema.Properties {
			properties[propName] = r.convertPropertySchema(propSchema)
		}
		params["properties"] = properties
	}
	
	if len(schema.Required) > 0 {
		params["required"] = schema.Required
	}
	
	return params
}

// convertPropertySchema converts a property schema to LLM format
func (r *Registry) convertPropertySchema(prop domain.JSONSchemaProperty) map[string]interface{} {
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
		result["items"] = r.convertPropertySchema(*prop.Items)
	}
	
	if len(prop.Properties) > 0 {
		properties := make(map[string]interface{})
		for propName, propSchema := range prop.Properties {
			properties[propName] = r.convertPropertySchema(propSchema)
		}
		result["properties"] = properties
	}
	
	return result
}


// ExecuteToolCall executes a tool call from LLM
func (r *Registry) ExecuteToolCall(ctx context.Context, toolCall *domain.ToolCall) (*domain.ToolInvocationResult, error) {
	if toolCall.Function == nil {
		return &domain.ToolInvocationResult{
			Success: false,
			Error:   "tool call missing function information",
		}, nil
	}
	
	// Parse function arguments
	var args map[string]interface{}
	if err := json.Unmarshal(toolCall.Function.Arguments, &args); err != nil {
		return &domain.ToolInvocationResult{
			ToolName: toolCall.Function.Name,
			Success:  false,
			Error:    fmt.Sprintf("failed to parse arguments: %v", err),
		}, nil
	}
	
	// Invoke the tool
	result, err := r.InvokeTool(ctx, toolCall.Function.Name, args)
	if err != nil {
		return &domain.ToolInvocationResult{
			ToolName: toolCall.Function.Name,
			Success:  false,
			Error:    err.Error(),
		}, nil
	}
	
	return &domain.ToolInvocationResult{
		ToolName: toolCall.Function.Name,
		Success:  true,
		Result:   result,
	}, nil
}

// DefaultRegistry is the global tool registry instance
var DefaultRegistry = NewRegistry()

// RegisterTool registers a tool in the default registry
func RegisterTool(tool domain.Tool) error {
	return DefaultRegistry.RegisterTool(tool)
}

// GetTool retrieves a tool from the default registry
func GetTool(name string) (domain.Tool, error) {
	return DefaultRegistry.GetTool(name)
}

// InvokeTool invokes a tool from the default registry
func InvokeTool(ctx context.Context, toolName string, input map[string]interface{}) (interface{}, error) {
	return DefaultRegistry.InvokeTool(ctx, toolName, input)
}

// ExecuteToolCall executes a tool call from the default registry
func ExecuteToolCall(ctx context.Context, toolCall *domain.ToolCall) (*domain.ToolInvocationResult, error) {
	return DefaultRegistry.ExecuteToolCall(ctx, toolCall)
}