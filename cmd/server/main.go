package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"personal-assistant/internal/config"
	"personal-assistant/internal/http/contacts"
	"personal-assistant/internal/http/infobip"
	infobipClient "personal-assistant/internal/infobip"
	"personal-assistant/internal/log"
	"personal-assistant/internal/processor"
	"personal-assistant/internal/tenant"
	"personal-assistant/internal/tools"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logging
	logger := log.Init(cfg.LogLevel)
	log.SetGlobalLogger(logger)

	logger.Info().Msg("Starting WhatsApp LLM Bot server")

	// Create Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())
	e.Use(middleware.RequestID())

	// Add request logging middleware
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			
			// Add request ID to context
			ctx := context.WithValue(c.Request().Context(), log.RequestIDKey, c.Response().Header().Get(echo.HeaderXRequestID))
			c.SetRequest(c.Request().WithContext(ctx))

			err := next(c)
			
			// Log request
			duration := time.Since(start)
			logger.WithContext(ctx).Info().
				Str("method", c.Request().Method).
				Str("uri", c.Request().RequestURI).
				Str("remote_ip", c.RealIP()).
				Int("status", c.Response().Status).
				Dur("duration", duration).
				Msg("http request")

			return err
		}
	})

	// Initialize tenant manager
	tenantManager, err := tenant.NewTenantManager(cfg, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize tenant manager")
	}
	defer tenantManager.Close()

	// Initialize Infobip client
	infobipCli := infobipClient.NewRetryableClient(&cfg.Infobip, logger, 3, 1*time.Second)

	// Initialize tool registry
	toolRegistry := tools.NewRegistry()

	// Initialize message processor
	messageProcessor := processor.NewMessageProcessor(tenantManager, infobipCli, toolRegistry, logger)

	// Initialize webhook handler
	webhookHandler := infobip.NewWebhookHandler(messageProcessor, cfg, logger)

	// Initialize contacts handler
	contactsHandler := contacts.NewContactsHandler(tenantManager, logger)

	// Health check endpoint
	e.GET("/health", healthCheck)

	// Webhook endpoints
	e.POST("/webhooks/infobip", webhookHandler.HandleIncoming)
	e.POST("/webhooks/infobip/status", webhookHandler.HandleStatus)
	e.GET("/webhooks/infobip/health", webhookHandler.HandleHealth)

	// Contacts management API endpoints
	api := e.Group("/api/v1")
	api.GET("/tenants/:tenant_id/contacts", contactsHandler.ListContacts)
	api.GET("/tenants/:tenant_id/contacts/:phone_number", contactsHandler.GetContact)
	api.POST("/tenants/:tenant_id/contacts", contactsHandler.CreateContact)
	api.PUT("/tenants/:tenant_id/contacts/:phone_number", contactsHandler.UpdateContact)
	api.DELETE("/tenants/:tenant_id/contacts/:contact_id", contactsHandler.DeleteContact)
	api.GET("/tenants/:tenant_id/contacts/check", contactsHandler.CheckContact)

	// Start server in a goroutine
	go func() {
		address := fmt.Sprintf(":%s", cfg.Port)
		logger.Info().Str("address", address).Msg("Server starting")
		
		if err := e.Start(address); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("Server forced to shutdown")
		os.Exit(1)
	}

	logger.Info().Msg("Server stopped")
}

// healthCheck returns the health status of the service
func healthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   "1.0.0",
		"service":   "whatsapp-llm-bot",
	})
}