package infobip

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"personal-assistant/internal/config"
	"personal-assistant/internal/domain"
	"personal-assistant/internal/log"
)

// Client implements the InfobipClient interface
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	logger     *log.Logger
}

// NewClient creates a new Infobip client
func NewClient(cfg *config.InfobipConfig, logger *log.Logger) *Client {
	return &Client{
		baseURL: cfg.BaseURL,
		apiKey:  cfg.APIKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// SendText sends a text message via WhatsApp
func (c *Client) SendText(ctx context.Context, from, to, text string, messageIDRef ...string) (*domain.InfobipMessage, error) {
	message := &domain.InfobipMessage{
		From: from,
		To:   to,
		Content: domain.InfobipMessageContent{
			Text: text,
		},
	}
	
	// Set callback data if message ID reference provided
	if len(messageIDRef) > 0 && messageIDRef[0] != "" {
		message.CallbackData = messageIDRef[0]
	}
	
	return c.SendMessage(ctx, message)
}

// SendMessage sends a structured message
func (c *Client) SendMessage(ctx context.Context, message *domain.InfobipMessage) (*domain.InfobipMessage, error) {
	start := time.Now()
	
	// Prepare request
	url := fmt.Sprintf("%s/whatsapp/1/message/text", c.baseURL)
	
	// Create request body
	requestBody := map[string]interface{}{
		"from": message.From,
		"to":   message.To,
		"content": map[string]interface{}{
			"text": message.Content.Text,
		},
	}
	
	if message.CallbackData != "" {
		requestBody["callbackData"] = message.CallbackData
	}
	
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}
	
	c.logger.WithContext(ctx).Debug().
		Str("from", message.From).
		Str("to", message.To).
		Str("text", log.SanitizeText(message.Content.Text)).
		Msg("sending WhatsApp message")
	
	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Authorization", fmt.Sprintf("App %s", c.apiKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	
	// Make the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.WithContext(ctx).Error().
			Err(err).
			Dur("duration", time.Since(start)).
			Msg("Infobip API request failed")
		return nil, fmt.Errorf("Infobip API request failed: %w", err)
	}
	defer resp.Body.Close()
	
	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	c.logger.LogAPICall("infobip", "POST", url, resp.StatusCode, time.Since(start))
	
	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		c.logger.WithContext(ctx).Error().
			Int("status_code", resp.StatusCode).
			Str("response_body", string(bodyBytes)).
			Msg("Infobip API error response")
		return nil, fmt.Errorf("Infobip API error: %d - %s", resp.StatusCode, string(bodyBytes))
	}
	
	// Parse response
	var response InfobipSendResponse
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	
	// Check if we have messages in the response
	if len(response.Messages) == 0 {
		return nil, fmt.Errorf("no messages in Infobip response")
	}
	
	// Get the first message result
	msgResult := response.Messages[0]
	
	// Create response message
	responseMsg := &domain.InfobipMessage{
		From:      message.From,
		To:        message.To,
		MessageID: msgResult.MessageID,
		Content:   message.Content,
	}
	
	c.logger.WithContext(ctx).Debug().
		Str("message_id", msgResult.MessageID).
		Str("status", msgResult.Status.Name).
		Dur("duration", time.Since(start)).
		Msg("WhatsApp message sent successfully")
	
	return responseMsg, nil
}

// InfobipSendResponse represents the response from Infobip send API
type InfobipSendResponse struct {
	Messages []InfobipMessageResult `json:"messages"`
}

// InfobipMessageResult represents a single message result
type InfobipMessageResult struct {
	To        string                 `json:"to"`
	MessageID string                 `json:"messageId"`
	Status    InfobipMessageStatus   `json:"status"`
}

// InfobipMessageStatus represents the status of a message
type InfobipMessageStatus struct {
	GroupID     int    `json:"groupId"`
	GroupName   string `json:"groupName"`
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// RetryableClient wraps the Infobip client with retry logic
type RetryableClient struct {
	client     *Client
	maxRetries int
	baseDelay  time.Duration
	logger     *log.Logger
}

// NewRetryableClient creates a new retryable Infobip client
func NewRetryableClient(cfg *config.InfobipConfig, logger *log.Logger, maxRetries int, baseDelay time.Duration) *RetryableClient {
	return &RetryableClient{
		client:     NewClient(cfg, logger),
		maxRetries: maxRetries,
		baseDelay:  baseDelay,
		logger:     logger,
	}
}

// SendText sends a text message with retry logic
func (rc *RetryableClient) SendText(ctx context.Context, from, to, text string, messageIDRef ...string) (*domain.InfobipMessage, error) {
	var lastErr error
	
	for attempt := 0; attempt <= rc.maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			delay := rc.baseDelay * time.Duration(1<<(attempt-1))
			rc.logger.WithContext(ctx).Warn().
				Int("attempt", attempt).
				Dur("delay", delay).
				Err(lastErr).
				Msg("retrying Infobip request after delay")
			
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
				// Continue with retry
			}
		}
		
		result, err := rc.client.SendText(ctx, from, to, text, messageIDRef...)
		if err == nil {
			if attempt > 0 {
				rc.logger.WithContext(ctx).Info().
					Int("attempt", attempt+1).
					Msg("Infobip request succeeded after retry")
			}
			return result, nil
		}
		
		lastErr = err
		
		// Check if error is retryable
		if !rc.isRetryableError(err) {
			break
		}
	}
	
	rc.logger.WithContext(ctx).Error().
		Int("max_retries", rc.maxRetries).
		Err(lastErr).
		Msg("Infobip request failed after all retries")
	
	return nil, fmt.Errorf("failed after %d retries: %w", rc.maxRetries, lastErr)
}

// SendMessage sends a structured message with retry logic
func (rc *RetryableClient) SendMessage(ctx context.Context, message *domain.InfobipMessage) (*domain.InfobipMessage, error) {
	return rc.SendText(ctx, message.From, message.To, message.Content.Text, message.CallbackData)
}

// isRetryableError determines if an error should trigger a retry
func (rc *RetryableClient) isRetryableError(err error) bool {
	// For simplicity, retry on all errors except context cancellation
	// In production, you might want to be more specific about which errors to retry
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}
	
	// Could also check for specific HTTP status codes
	// e.g., don't retry on 4xx errors except 429 (rate limit)
	
	return true
}

// Webhook verification utilities
func VerifyWebhookToken(receivedToken, expectedToken string) bool {
	if expectedToken == "" {
		return true // Skip verification if no token configured
	}
	return receivedToken == expectedToken
}

// ValidateWebhookSignature validates the webhook signature (if implemented by Infobip)
func ValidateWebhookSignature(payload []byte, signature, secret string) bool {
	// Infobip doesn't typically use HMAC signatures like some other providers
	// This is a placeholder for custom signature validation if needed
	// In practice, you might rely on IP whitelisting or the verification token
	return true
}