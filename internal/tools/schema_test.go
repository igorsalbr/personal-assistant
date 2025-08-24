package tools_test

import (
	"context"
	"testing"

	"personal-assistant/internal/domain"
	"personal-assistant/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockTool implements domain.Tool for testing
type MockTool struct {
	mock.Mock
}

func (m *MockTool) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockTool) Schema() *domain.JSONSchema {
	args := m.Called()
	return args.Get(0).(*domain.JSONSchema)
}

func (m *MockTool) Invoke(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	args := m.Called(ctx, input)
	return args.Get(0), args.Error(1)
}

func TestRegistry(t *testing.T) {
	t.Run("creates new registry", func(t *testing.T) {
		registry := tools.NewRegistry()
		assert.NotNil(t, registry)
	})

	t.Run("registers tool", func(t *testing.T) {
		registry := tools.NewRegistry()
		mockTool := new(MockTool)
		
		mockTool.On("Name").Return("test-tool")
		mockTool.On("Schema").Return(&domain.JSONSchema{
			Type: "object",
			Properties: map[string]domain.JSONSchemaProperty{
				"param1": {
					Type:        "string",
					Description: "Test parameter",
				},
			},
		})

		err := registry.RegisterTool(mockTool)
		assert.NoError(t, err)
		mockTool.AssertExpectations(t)
	})

	t.Run("gets registered tool", func(t *testing.T) {
		registry := tools.NewRegistry()
		mockTool := new(MockTool)
		
		mockTool.On("Name").Return("test-tool")
		mockTool.On("Schema").Return(&domain.JSONSchema{Type: "object"})

		err := registry.RegisterTool(mockTool)
		assert.NoError(t, err)

		tool, err := registry.GetTool("test-tool")
		assert.NoError(t, err)
		assert.Equal(t, mockTool, tool)
	})

	t.Run("returns error for non-existent tool", func(t *testing.T) {
		registry := tools.NewRegistry()
		
		tool, err := registry.GetTool("non-existent")
		assert.Error(t, err)
		assert.Nil(t, tool)
		assert.Contains(t, err.Error(), "tool non-existent not found")
	})

	t.Run("lists registered tools", func(t *testing.T) {
		registry := tools.NewRegistry()
		
		mockTool1 := new(MockTool)
		mockTool1.On("Name").Return("tool1")
		mockTool1.On("Schema").Return(&domain.JSONSchema{Type: "object"})
		
		mockTool2 := new(MockTool)
		mockTool2.On("Name").Return("tool2")
		mockTool2.On("Schema").Return(&domain.JSONSchema{Type: "object"})

		err1 := registry.RegisterTool(mockTool1)
		err2 := registry.RegisterTool(mockTool2)
		
		assert.NoError(t, err1)
		assert.NoError(t, err2)

		tools := registry.ListTools()
		assert.Len(t, tools, 2)
	})

	t.Run("prevents duplicate tool registration", func(t *testing.T) {
		registry := tools.NewRegistry()
		mockTool := new(MockTool)
		
		mockTool.On("Name").Return("duplicate-tool")
		mockTool.On("Schema").Return(&domain.JSONSchema{Type: "object"})

		err1 := registry.RegisterTool(mockTool)
		assert.NoError(t, err1)
		
		err2 := registry.RegisterTool(mockTool)
		assert.Error(t, err2)
		assert.Contains(t, err2.Error(), "already registered")
	})
}

func TestToolInvocation(t *testing.T) {
	t.Run("invokes tool successfully", func(t *testing.T) {
		registry := tools.NewRegistry()
		mockTool := new(MockTool)
		ctx := context.Background()
		
		mockTool.On("Name").Return("test-tool")
		mockTool.On("Schema").Return(&domain.JSONSchema{Type: "object"})
		
		input := map[string]interface{}{"key": "value"}
		expectedResult := map[string]interface{}{"result": "success"}
		
		mockTool.On("Invoke", ctx, input).Return(expectedResult, nil)

		err := registry.RegisterTool(mockTool)
		assert.NoError(t, err)

		result, err := registry.InvokeTool(ctx, "test-tool", input)
		assert.NoError(t, err)
		assert.Equal(t, expectedResult, result)
		mockTool.AssertExpectations(t)
	})
}

func TestToolCallExecution(t *testing.T) {
	t.Run("executes tool call", func(t *testing.T) {
		registry := tools.NewRegistry()
		mockTool := new(MockTool)
		ctx := context.Background()
		
		mockTool.On("Name").Return("test-tool")
		mockTool.On("Schema").Return(&domain.JSONSchema{Type: "object"})
		
		expectedResult := "tool executed"
		mockTool.On("Invoke", ctx, mock.AnythingOfType("map[string]interface {}")).Return(expectedResult, nil)

		err := registry.RegisterTool(mockTool)
		assert.NoError(t, err)

		toolCall := &domain.ToolCall{
			ID:   "call-123",
			Type: "function",
			Function: &domain.FunctionCall{
				Name:      "test-tool",
				Arguments: []byte(`{"param": "value"}`),
			},
		}

		result, err := registry.ExecuteToolCall(ctx, toolCall)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "test-tool", result.ToolName)
		assert.True(t, result.Success)
		assert.Equal(t, expectedResult, result.Result)
		mockTool.AssertExpectations(t)
	})

	t.Run("handles missing function in tool call", func(t *testing.T) {
		registry := tools.NewRegistry()
		ctx := context.Background()

		toolCall := &domain.ToolCall{
			ID:   "call-123",
			Type: "function",
			// Function is nil
		}

		result, err := registry.ExecuteToolCall(ctx, toolCall)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "missing function information")
	})

	t.Run("handles invalid JSON arguments", func(t *testing.T) {
		registry := tools.NewRegistry()
		ctx := context.Background()

		toolCall := &domain.ToolCall{
			ID:   "call-123",
			Type: "function",
			Function: &domain.FunctionCall{
				Name:      "test-tool",
				Arguments: []byte(`{invalid json`),
			},
		}

		result, err := registry.ExecuteToolCall(ctx, toolCall)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "failed to parse arguments")
	})
}

func TestConvertToLLMTools(t *testing.T) {
	t.Run("converts tools to LLM format", func(t *testing.T) {
		registry := tools.NewRegistry()
		mockTool := new(MockTool)
		
		mockTool.On("Name").Return("test-tool")
		mockTool.On("Schema").Return(&domain.JSONSchema{
			Type: "object",
			Properties: map[string]domain.JSONSchemaProperty{
				"location": {
					Type:        "string",
					Description: "City name",
				},
			},
			Required: []string{"location"},
		})

		err := registry.RegisterTool(mockTool)
		assert.NoError(t, err)

		llmTools := registry.ConvertToLLMTools()
		assert.Len(t, llmTools, 1)
		
		tool := llmTools[0]
		assert.Equal(t, "function", tool.Type)
		assert.Equal(t, "test-tool", tool.Function.Name)
		assert.NotNil(t, tool.Function.Parameters)
	})
}