package builtin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"personal-assistant/internal/domain"
	"personal-assistant/internal/log"
)

// HTTPAgent handles external HTTP API calls
type HTTPAgent struct {
	name           string
	allowedTenants []string
	httpClient     *http.Client
	repository     domain.Repository
	logger         *log.Logger
}

// NewHTTPAgent creates a new HTTP agent
func NewHTTPAgent(repository domain.Repository, logger *log.Logger, allowedTenants []string) *HTTPAgent {
	return &HTTPAgent{
		name:           "http_agent",
		allowedTenants: allowedTenants,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		repository: repository,
		logger:     logger,
	}
}

// Name returns the agent name
func (a *HTTPAgent) Name() string {
	return a.name
}

// AllowedTenants returns the list of tenant IDs this agent can serve
func (a *HTTPAgent) AllowedTenants() []string {
	return a.allowedTenants
}

// CanHandle checks if the agent can handle a specific intent
func (a *HTTPAgent) CanHandle(intent string) bool {
	httpIntents := []string{
		"api_call", "http_request", "external_call", "web_request",
		"get", "post", "put", "patch", "delete",
		"weather", "calendar", "crm", "external_service",
	}
	
	for _, httpIntent := range httpIntents {
		if intent == httpIntent {
			return true
		}
	}
	return false
}

// Handle processes a request and returns a response
func (a *HTTPAgent) Handle(ctx context.Context, req *domain.AgentRequest) (*domain.AgentResponse, error) {
	a.logger.WithContext(ctx).Debug().
		Str("tenant_id", req.TenantID).
		Str("user_id", req.UserID.String()).
		Str("text", req.Text).
		Msg("HTTP agent handling request")
	
	return &domain.AgentResponse{
		Text: "I can help you make HTTP API calls to external services. Available tool: call_api.",
		Metadata: map[string]interface{}{
			"available_tools": []string{"call_api"},
			"agent_type":      "http_client",
		},
	}, nil
}

// HTTPCallTool handles external HTTP API calls
type HTTPCallTool struct {
	httpClient *http.Client
	repository domain.Repository
	logger     *log.Logger
}

// NewHTTPCallTool creates a new HTTP call tool
func NewHTTPCallTool(repository domain.Repository, logger *log.Logger) *HTTPCallTool {
	return &HTTPCallTool{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		repository: repository,
		logger:     logger,
	}
}

// Name returns the tool name
func (t *HTTPCallTool) Name() string {
	return "call_api"
}

// Schema returns the JSON schema for the tool parameters
func (t *HTTPCallTool) Schema() *domain.JSONSchema {
	return &domain.JSONSchema{
		Type: "object",
		Properties: map[string]domain.JSONSchemaProperty{
			"service_name": {
				Type:        "string",
				Description: "Name of the external service configured for this tenant",
			},
			"method": {
				Type:        "string",
				Description: "HTTP method to use",
				Enum:        []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
			},
			"path": {
				Type:        "string",
				Description: "API endpoint path (relative to service base URL)",
			},
			"headers": {
				Type:        "object",
				Description: "Additional headers to send (optional)",
			},
			"query": {
				Type:        "object",
				Description: "Query parameters (optional)",
			},
			"body": {
				Type:        "object",
				Description: "Request body for POST/PUT/PATCH (optional)",
			},
		},
		Required: []string{"service_name", "method", "path"},
	}
}

// Invoke executes the tool with the given input
func (t *HTTPCallTool) Invoke(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	start := time.Now()
	
	// Extract parameters
	serviceName, _ := input["service_name"].(string)
	method, _ := input["method"].(string)
	path, _ := input["path"].(string)
	headersMap, _ := input["headers"].(map[string]interface{})
	queryMap, _ := input["query"].(map[string]interface{})
	bodyMap, _ := input["body"].(map[string]interface{})
	
	// Extract tenant info from context
	tenantID, _ := ctx.Value(log.TenantIDKey).(string)
	if tenantID == "" {
		return nil, fmt.Errorf("missing tenant_id in context")
	}
	
	// Get external service configuration
	service, err := t.repository.GetExternalService(ctx, tenantID, serviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get service configuration: %w", err)
	}
	if service == nil {
		return nil, fmt.Errorf("service '%s' not configured for tenant", serviceName)
	}
	
	// Build the full URL
	baseURL := service.BaseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}
	fullURL := baseURL + path
	
	// Add query parameters
	if len(queryMap) > 0 {
		u, err := url.Parse(fullURL)
		if err != nil {
			return nil, fmt.Errorf("invalid URL: %w", err)
		}
		
		q := u.Query()
		for key, value := range queryMap {
			q.Add(key, fmt.Sprintf("%v", value))
		}
		u.RawQuery = q.Encode()
		fullURL = u.String()
	}
	
	// Prepare request body
	var reqBody io.Reader
	var bodyBytes []byte
	if len(bodyMap) > 0 && (method == "POST" || method == "PUT" || method == "PATCH") {
		bodyBytes, err = json.Marshal(bodyMap)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(bodyBytes)
	}
	
	t.logger.WithContext(ctx).Debug().
		Str("service", serviceName).
		Str("method", method).
		Str("url", fullURL).
		Msg("making HTTP API call")
	
	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	
	// Set default headers
	if len(bodyBytes) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "whatsapp-llm-bot/1.0")
	
	// Add custom headers
	for key, value := range headersMap {
		req.Header.Set(key, fmt.Sprintf("%v", value))
	}
	
	// Add authentication from service config
	if authConfig, ok := service.Auth["type"]; ok {
		switch authConfig {
		case "bearer":
			if token, ok := service.Auth["token"].(string); ok {
				req.Header.Set("Authorization", "Bearer "+token)
			}
		case "api_key":
			if apiKey, ok := service.Auth["api_key"].(string); ok {
				if header, ok := service.Auth["header"].(string); ok {
					req.Header.Set(header, apiKey)
				} else {
					req.Header.Set("X-API-Key", apiKey)
				}
			}
		case "basic":
			if username, ok := service.Auth["username"].(string); ok {
				if password, ok := service.Auth["password"].(string); ok {
					req.SetBasicAuth(username, password)
				}
			}
		}
	}
	
	// Make the request
	resp, err := t.httpClient.Do(req)
	if err != nil {
		t.logger.WithContext(ctx).Error().
			Err(err).
			Str("service", serviceName).
			Str("url", fullURL).
			Dur("duration", time.Since(start)).
			Msg("HTTP API call failed")
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()
	
	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	duration := time.Since(start)
	t.logger.LogAPICall(serviceName, method, fullURL, resp.StatusCode, duration)
	
	// Parse response headers
	responseHeaders := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			responseHeaders[key] = values[0]
		}
	}
	
	// Try to parse response body as JSON
	var responseBody interface{}
	if len(respBody) > 0 {
		contentType := resp.Header.Get("Content-Type")
		if strings.Contains(contentType, "application/json") {
			if err := json.Unmarshal(respBody, &responseBody); err != nil {
				// If JSON parsing fails, return as string
				responseBody = string(respBody)
			}
		} else {
			responseBody = string(respBody)
		}
	}
	
	result := map[string]interface{}{
		"status":   resp.StatusCode,
		"headers":  responseHeaders,
		"body":     responseBody,
		"duration": duration.Milliseconds(),
		"request": map[string]interface{}{
			"method":  method,
			"url":     fullURL,
			"service": serviceName,
		},
	}
	
	// Log successful call
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		t.logger.WithContext(ctx).Debug().
			Str("service", serviceName).
			Int("status", resp.StatusCode).
			Dur("duration", duration).
			Msg("HTTP API call successful")
	} else {
		t.logger.WithContext(ctx).Warn().
			Str("service", serviceName).
			Int("status", resp.StatusCode).
			Str("response", string(respBody)).
			Dur("duration", duration).
			Msg("HTTP API call returned error status")
	}
	
	return result, nil
}

// ReminderScheduleTool schedules reminders (mock implementation)
type ReminderScheduleTool struct {
	logger *log.Logger
}

// NewReminderScheduleTool creates a new reminder scheduling tool
func NewReminderScheduleTool(logger *log.Logger) *ReminderScheduleTool {
	return &ReminderScheduleTool{
		logger: logger,
	}
}

// Name returns the tool name
func (t *ReminderScheduleTool) Name() string {
	return "schedule_reminder"
}

// Schema returns the JSON schema for the tool parameters
func (t *ReminderScheduleTool) Schema() *domain.JSONSchema {
	return &domain.JSONSchema{
		Type: "object",
		Properties: map[string]domain.JSONSchemaProperty{
			"item_id": {
				Type:        "string",
				Description: "UUID of the memory item to remind about",
			},
			"when": {
				Type:        "string",
				Description: "ISO8601 timestamp when to send the reminder",
			},
			"channel": {
				Type:        "string",
				Description: "Channel to send reminder through",
				Enum:        []string{"whatsapp", "email", "sms"},
			},
		},
		Required: []string{"item_id", "when", "channel"},
	}
}

// Invoke executes the tool with the given input
func (t *ReminderScheduleTool) Invoke(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	// Extract parameters
	itemID, _ := input["item_id"].(string)
	whenStr, _ := input["when"].(string)
	channel, _ := input["channel"].(string)
	
	// Validate timestamp
	scheduledTime, err := time.Parse(time.RFC3339, whenStr)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp format, use ISO8601: %w", err)
	}
	
	// Check if time is in the future
	if scheduledTime.Before(time.Now()) {
		return nil, fmt.Errorf("scheduled time must be in the future")
	}
	
	// Extract tenant and user info from context for logging
	tenantID, _ := ctx.Value(log.TenantIDKey).(string)
	userIDStr, _ := ctx.Value(log.UserIDKey).(string)
	
	t.logger.WithContext(ctx).Info().
		Str("tenant_id", tenantID).
		Str("user_id", userIDStr).
		Str("item_id", itemID).
		Str("scheduled_at", scheduledTime.Format(time.RFC3339)).
		Str("channel", channel).
		Msg("reminder scheduled (mock implementation)")
	
	// This is a mock implementation
	// In production, you would:
	// 1. Store the reminder in a database
	// 2. Set up a background job/cron to check for due reminders
	// 3. Send notifications through the specified channel
	
	return map[string]interface{}{
		"scheduled_at": scheduledTime.Format(time.RFC3339),
		"item_id":      itemID,
		"channel":      channel,
		"status":       "scheduled",
		"reminder_id":  fmt.Sprintf("reminder_%d", time.Now().Unix()),
		"note":         "This is a mock implementation. In production, actual reminder scheduling would be implemented.",
	}, nil
}

// WeatherTool is an example external service tool
type WeatherTool struct {
	httpClient *http.Client
	apiKey     string
	logger     *log.Logger
}

// NewWeatherTool creates a new weather tool
func NewWeatherTool(apiKey string, logger *log.Logger) *WeatherTool {
	return &WeatherTool{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		apiKey: apiKey,
		logger: logger,
	}
}

// Name returns the tool name
func (t *WeatherTool) Name() string {
	return "get_weather"
}

// Schema returns the JSON schema for the tool parameters
func (t *WeatherTool) Schema() *domain.JSONSchema {
	return &domain.JSONSchema{
		Type: "object",
		Properties: map[string]domain.JSONSchemaProperty{
			"location": {
				Type:        "string",
				Description: "City name or coordinates (lat,lon) to get weather for",
			},
			"units": {
				Type:        "string",
				Description: "Temperature units",
				Enum:        []string{"metric", "imperial", "standard"},
			},
		},
		Required: []string{"location"},
	}
}

// Invoke executes the tool with the given input
func (t *WeatherTool) Invoke(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	location, _ := input["location"].(string)
	units := "metric"
	if u, ok := input["units"].(string); ok {
		units = u
	}
	
	// This is a mock implementation
	// In production, you would call a real weather API like OpenWeatherMap
	
	t.logger.WithContext(ctx).Debug().
		Str("location", location).
		Str("units", units).
		Msg("getting weather information (mock)")
	
	// Mock weather data
	return map[string]interface{}{
		"location":    location,
		"temperature": 22.5,
		"feels_like":  24.1,
		"humidity":    65,
		"description": "partly cloudy",
		"wind_speed":  3.2,
		"units":       units,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"note":        "This is mock weather data. In production, integrate with a real weather API.",
	}, nil
}