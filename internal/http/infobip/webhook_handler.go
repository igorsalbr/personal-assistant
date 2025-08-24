package infobip

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"personal-assistant/internal/config"
	"personal-assistant/internal/domain"
	"personal-assistant/internal/infobip"
	"personal-assistant/internal/log"
)

// WebhookHandler handles incoming Infobip webhook requests
type WebhookHandler struct {
	processor    domain.MessageProcessor
	config       *config.Config
	logger       *log.Logger
	seenMessages map[string]time.Time // Simple deduplication cache
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(processor domain.MessageProcessor, config *config.Config, logger *log.Logger) *WebhookHandler {
	return &WebhookHandler{
		processor:    processor,
		config:       config,
		logger:       logger,
		seenMessages: make(map[string]time.Time),
	}
}

// HandleIncoming handles incoming message webhooks
func (h *WebhookHandler) HandleIncoming(c echo.Context) error {
	ctx := c.Request().Context()
	requestID := c.Response().Header().Get(echo.HeaderXRequestID)
	
	// Add request ID to context for logging
	ctx = context.WithValue(ctx, log.RequestIDKey, requestID)
	logger := h.logger.WithContext(ctx)
	
	// Verify webhook token if configured
	if h.config.WebhookVerifyToken != "" {
		token := c.Request().Header.Get("X-Hub-Signature")
		if !infobip.VerifyWebhookToken(token, h.config.WebhookVerifyToken) {
			logger.Warn().
				Str("received_token", token).
				Msg("webhook token verification failed")
			return c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "unauthorized",
			})
		}
	}
	
	// Read request body
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		logger.Error().Err(err).Msg("failed to read webhook body")
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "failed to read request body",
		})
	}
	
	logger.Debug().
		Str("body", string(body)).
		Msg("received webhook request")
	
	// Parse webhook message
	var webhookMsg domain.InfobipWebhookMessage
	if err := json.Unmarshal(body, &webhookMsg); err != nil {
		logger.Error().
			Err(err).
			Str("body", string(body)).
			Msg("failed to parse webhook message")
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid JSON format",
		})
	}
	
	// Validate webhook structure
	if len(webhookMsg.Results) == 0 {
		logger.Warn().Msg("webhook message has no results")
		return c.JSON(http.StatusOK, map[string]string{
			"status": "no_results",
		})
	}
	
	// Process each result
	for _, result := range webhookMsg.Results {
		if err := h.processWebhookResult(ctx, &result); err != nil {
			logger.Error().
				Err(err).
				Str("message_id", result.MessageID).
				Str("from", result.From).
				Msg("failed to process webhook result")
			
			// Continue processing other results even if one fails
			continue
		}
	}
	
	logger.Info().
		Int("results_count", len(webhookMsg.Results)).
		Msg("webhook processed successfully")
	
	// Return success response
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":           "processed",
		"results_count":    len(webhookMsg.Results),
		"processed_at":     time.Now().UTC().Format(time.RFC3339),
		"request_id":       requestID,
	})
}

// processWebhookResult processes a single webhook result
func (h *WebhookHandler) processWebhookResult(ctx context.Context, result *domain.InfobipWebhookResult) error {
	logger := h.logger.WithContext(ctx)
	
	// Extract message text
	messageText := ""
	if result.Message.Type == "TEXT" && result.Message.Text.Text != "" {
		messageText = result.Message.Text.Text
		logger.Info().Str("text", messageText).Msg("Processing text message")
	} else {
		logger.Debug().
			Str("message_type", result.Message.Type).
			Msg("skipping non-text message")
		return nil // Skip non-text messages for now
	}
	
	// Check for message deduplication
	if h.isDuplicateMessage(result.MessageID) {
		logger.Debug().
			Str("message_id", result.MessageID).
			Msg("skipping duplicate message")
		return nil
	}
	
	// Mark message as seen
	h.markMessageSeen(result.MessageID)
	
	// Create webhook message for processing
	webhookMsg := &domain.InfobipWebhookMessage{
		Results: []domain.InfobipWebhookResult{*result},
	}
	
	// Log message processing start
	start := time.Now()
	logger.LogMessageProcessing(
		result.MessageID,
		result.From,
		result.To,
		result.Message.Type,
		0, // Duration will be logged separately
	)
	
	// Process the message
	err := h.processor.ProcessIncoming(ctx, webhookMsg)
	duration := time.Since(start)
	
	if err != nil {
		logger.Error().
			Err(err).
			Str("message_id", result.MessageID).
			Str("from", result.From).
			Str("to", result.To).
			Dur("duration", duration).
			Msg("message processing failed")
		return fmt.Errorf("failed to process message %s: %w", result.MessageID, err)
	}
	
	logger.Info().
		Str("message_id", result.MessageID).
		Str("from", result.From).
		Str("to", result.To).
		Dur("duration", duration).
		Msg("message processed successfully")
	
	return nil
}

// isDuplicateMessage checks if a message has already been processed recently
func (h *WebhookHandler) isDuplicateMessage(messageID string) bool {
	if lastSeen, exists := h.seenMessages[messageID]; exists {
		// Consider it duplicate if seen within the last 5 minutes
		if time.Since(lastSeen) < 5*time.Minute {
			return true
		}
	}
	return false
}

// markMessageSeen marks a message as seen and cleans up old entries
func (h *WebhookHandler) markMessageSeen(messageID string) {
	h.seenMessages[messageID] = time.Now()
	
	// Clean up old entries (keep only last hour)
	cutoff := time.Now().Add(-1 * time.Hour)
	for id, timestamp := range h.seenMessages {
		if timestamp.Before(cutoff) {
			delete(h.seenMessages, id)
		}
	}
}

// HandleStatus handles message status webhooks (delivery receipts, etc.)
func (h *WebhookHandler) HandleStatus(c echo.Context) error {
	ctx := c.Request().Context()
	requestID := c.Response().Header().Get(echo.HeaderXRequestID)
	
	// Add request ID to context for logging
	ctx = context.WithValue(ctx, log.RequestIDKey, requestID)
	logger := h.logger.WithContext(ctx)
	
	// Read request body
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		logger.Error().Err(err).Msg("failed to read status webhook body")
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "failed to read request body",
		})
	}
	
	logger.Debug().
		Str("body", string(body)).
		Msg("received status webhook")
	
	// For now, just log the status update
	// In a production system, you might want to update message status in database
	var statusUpdate map[string]interface{}
	if err := json.Unmarshal(body, &statusUpdate); err != nil {
		logger.Error().
			Err(err).
			Str("body", string(body)).
			Msg("failed to parse status webhook")
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid JSON format",
		})
	}
	
	logger.Info().
		Interface("status_update", statusUpdate).
		Msg("received message status update")
	
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":       "received",
		"processed_at": time.Now().UTC().Format(time.RFC3339),
		"request_id":   requestID,
	})
}

// Health check endpoint specific to Infobip webhooks
func (h *WebhookHandler) HandleHealth(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":    "healthy",
		"service":   "infobip-webhook",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   "1.0.0",
	})
}