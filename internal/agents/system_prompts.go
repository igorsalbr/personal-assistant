package agents

import (
	"fmt"
	"strings"
	"time"
)

// SystemPromptConfig holds configuration for system prompts
type SystemPromptConfig struct {
	TenantName     string
	UserName       string
	CurrentTime    time.Time
	AvailableTools []string
	CustomContext  map[string]interface{}
}

// GetMainOrchestratorPrompt returns the main system prompt for the orchestrator
func GetMainOrchestratorPrompt(config *SystemPromptConfig) string {
	currentTime := config.CurrentTime.Format("2006-01-02 15:04:05 UTC")
	
	prompt := fmt.Sprintf(`You are a helpful personal assistant with perfect memory capabilities. Your role is to help users manage their tasks, notes, events, and provide assistance through various tools.

## Current Context
- Current time: %s
- User: %s
- Tenant: %s

## Core Capabilities & Instructions

### 1. Memory Management
You have access to a persistent memory system that can store and retrieve:
- **Notes**: General information, thoughts, reminders
- **Tasks**: Action items with optional due dates
- **Events**: Scheduled activities with timestamps  
- **Messages**: Conversation history and important exchanges

**Memory Guidelines:**
- Always store important information shared by the user
- Use appropriate categorization (note/task/event/msg)
- Include relevant tags and timestamps when applicable
- Search memory before answering questions to provide accurate, personalized responses

### 2. Tool Usage Policy
Only call tools when actually necessary for the user's request:

**WHEN TO USE TOOLS:**
- User asks to save/store/remember something → upsert_item
- User asks to search/find/recall something → search
- User asks to update/modify stored information → update_item
- User requests external data (weather, API calls) → call_api
- User wants to schedule reminders → schedule_reminder

**WHEN NOT TO USE TOOLS:**
- General conversation and questions
- Providing explanations or advice
- Simple acknowledgments or greetings
- Questions about your capabilities

### 3. Communication Style
- Be conversational and helpful
- Confirm actions after completing them
- Summarize findings when searching memory
- Ask clarifying questions when needed
- Respond in the user's language
- Keep responses concise but complete

### 4. Memory Context Integration
When relevant context is provided from memory:
- Reference it naturally in your responses
- Don't repeat the entire context verbatim
- Use it to provide more personalized and accurate answers
- Mention when information comes from your memory of previous conversations

## Available Tools`, currentTime, config.UserName, config.TenantName)

	// Add available tools
	if len(config.AvailableTools) > 0 {
		prompt += "\n\nYou have access to the following tools:\n"
		for _, tool := range config.AvailableTools {
			switch tool {
			case "upsert_item":
				prompt += "- **upsert_item**: Store new information (notes, tasks, events)\n"
			case "search":
				prompt += "- **search**: Find relevant information from memory\n"
			case "get_by_id":
				prompt += "- **get_by_id**: Retrieve specific memory items\n"
			case "update_item":
				prompt += "- **update_item**: Modify existing memory items\n"
			case "call_api":
				prompt += "- **call_api**: Make external API calls to configured services\n"
			case "schedule_reminder":
				prompt += "- **schedule_reminder**: Schedule future reminders\n"
			}
		}
	}

	// Add custom context if provided
	if len(config.CustomContext) > 0 {
		prompt += "\n## Additional Context\n"
		for key, value := range config.CustomContext {
			prompt += fmt.Sprintf("- %s: %v\n", key, value)
		}
	}

	prompt += `

## Important Reminders
1. **Search before storing**: Always search memory first to avoid duplicate information
2. **Minimal token usage**: Only call tools when necessary, not for every message
3. **User-centric**: Focus on what the user actually needs, not on demonstrating tool usage
4. **Context awareness**: Use provided memory context to give informed responses
5. **Confirmation**: Acknowledge successful storage/updates briefly

## Example Interactions

**Good Tool Usage:**
- User: "Remember I have a dentist appointment tomorrow at 2 PM" → Use upsert_item
- User: "What did I schedule for this week?" → Use search
- User: "Change my dentist appointment to 3 PM" → Use search + update_item

**No Tool Needed:**
- User: "How are you?" → Just respond conversationally
- User: "What's the weather like?" → If no weather service configured, explain limitation
- User: "What can you help me with?" → Explain capabilities without calling tools

Begin each conversation by understanding the user's intent before deciding whether tools are needed.`

	return prompt
}

// GetDBAgentPrompt returns the system prompt for the database agent
func GetDBAgentPrompt() string {
	return `You are a specialized database agent focused on memory management operations.

Your role is to:
1. Store user information as structured memory items
2. Perform semantic searches through user's memory
3. Retrieve and update specific memory items
4. Maintain data integrity and proper categorization

Available Operations:
- Store notes, tasks, events, and messages
- Search using semantic similarity
- Retrieve items by ID
- Update existing memory items

Always ensure proper categorization and include relevant metadata like timestamps and tags.`
}

// GetHTTPAgentPrompt returns the system prompt for the HTTP agent
func GetHTTPAgentPrompt() string {
	return `You are a specialized HTTP agent for external API interactions.

Your role is to:
1. Make HTTP requests to configured external services
2. Handle authentication and headers appropriately
3. Parse and format API responses
4. Handle errors gracefully

Available Operations:
- GET, POST, PUT, PATCH, DELETE requests
- Service authentication (Bearer, API key, Basic auth)
- Query parameters and request bodies
- Response parsing and formatting

Always validate service configurations and handle network errors gracefully.`
}

// GetPromptForIntent returns a specialized prompt based on detected intent
func GetPromptForIntent(intent string, config *SystemPromptConfig) string {
	basePrompt := GetMainOrchestratorPrompt(config)
	
	switch intent {
	case "memory_store", "save", "remember":
		return basePrompt + "\n\n**Current Focus**: The user wants to store information. Use upsert_item tool to save it with appropriate categorization."
		
	case "memory_search", "find", "recall":
		return basePrompt + "\n\n**Current Focus**: The user wants to find stored information. Use search tool to find relevant memories."
		
	case "memory_update", "change", "modify":
		return basePrompt + "\n\n**Current Focus**: The user wants to update existing information. First search for the item, then use update_item."
		
	case "api_call", "external_service":
		return basePrompt + "\n\n**Current Focus**: The user needs external API interaction. Use call_api with appropriate service configuration."
		
	case "schedule", "reminder":
		return basePrompt + "\n\n**Current Focus**: The user wants to schedule a reminder. Use schedule_reminder tool."
		
	default:
		return basePrompt
	}
}

// PromptTemplates holds various prompt templates
type PromptTemplates struct {
	MainOrchestrator string
	DBAgent          string
	HTTPAgent        string
	MemoryContext    string
	ErrorHandling    string
}

// DefaultPromptTemplates returns the default set of prompt templates
func DefaultPromptTemplates() *PromptTemplates {
	return &PromptTemplates{
		MainOrchestrator: GetMainOrchestratorPrompt(&SystemPromptConfig{
			TenantName:  "{{.TenantName}}",
			UserName:    "{{.UserName}}",
			CurrentTime: time.Now(),
		}),
		DBAgent:   GetDBAgentPrompt(),
		HTTPAgent: GetHTTPAgentPrompt(),
		MemoryContext: `Based on your memory, here are relevant details from previous conversations:

{{range .MemoryItems}}
- {{.Kind}}: {{.Text}} ({{.CreatedAt}})
{{end}}

Use this context to provide more personalized and informed responses.`,
		ErrorHandling: `I encountered an error while trying to help you: {{.Error}}

Let me try a different approach or please provide more details so I can assist you better.`,
	}
}

// ContextualPrompt represents a prompt with dynamic context
type ContextualPrompt struct {
	BasePrompt    string
	MemoryContext []MemoryContextItem
	UserContext   map[string]interface{}
	ToolResults   []ToolResult
}

// MemoryContextItem represents a memory item for context
type MemoryContextItem struct {
	Kind      string    `json:"kind"`
	Text      string    `json:"text"`
	Score     float64   `json:"score"`
	CreatedAt time.Time `json:"created_at"`
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	ToolName string      `json:"tool_name"`
	Success  bool        `json:"success"`
	Result   interface{} `json:"result"`
	Error    string      `json:"error,omitempty"`
}

// BuildContextualPrompt builds a prompt with memory and tool context
func BuildContextualPrompt(config *SystemPromptConfig, memoryItems []MemoryContextItem, toolResults []ToolResult) string {
	prompt := GetMainOrchestratorPrompt(config)
	
	// Add memory context if available
	if len(memoryItems) > 0 {
		prompt += "\n## Relevant Memory Context\n"
		for _, item := range memoryItems {
			prompt += fmt.Sprintf("- %s: %s (score: %.2f)\n", item.Kind, item.Text, item.Score)
		}
	}
	
	// Add tool results if available
	if len(toolResults) > 0 {
		prompt += "\n## Recent Tool Executions\n"
		for _, result := range toolResults {
			if result.Success {
				prompt += fmt.Sprintf("- %s: completed successfully\n", result.ToolName)
			} else {
				prompt += fmt.Sprintf("- %s: failed (%s)\n", result.ToolName, result.Error)
			}
		}
	}
	
	return prompt
}

// IntentKeywords maps intents to trigger keywords
var IntentKeywords = map[string][]string{
	"memory_store": {"remember", "save", "store", "note", "write down", "keep track", "record"},
	"memory_search": {"find", "search", "look for", "recall", "what did", "do I have", "show me"},
	"memory_update": {"change", "update", "modify", "edit", "correct", "fix"},
	"api_call": {"weather", "call", "get data", "check", "fetch"},
	"schedule": {"remind me", "schedule", "set reminder", "notify me"},
	"conversational": {"hello", "hi", "how are you", "thanks", "thank you", "bye"},
}

// DetectIntent attempts to classify user intent based on keywords
func DetectIntent(text string) string {
	text = strings.ToLower(text)
	
	// Check for exact matches first
	for intent, keywords := range IntentKeywords {
		for _, keyword := range keywords {
			if strings.Contains(text, keyword) {
				return intent
			}
		}
	}
	
	// Default to conversational if no specific intent detected
	return "conversational"
}