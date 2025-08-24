package processor

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"personal-assistant/internal/agents"
	"personal-assistant/internal/agents/builtin"
	"personal-assistant/internal/domain"
	"personal-assistant/internal/log"
	"personal-assistant/internal/rag"
)

// MessageProcessor processes incoming WhatsApp messages
type MessageProcessor struct {
	tenantManager domain.TenantManager
	infobipClient domain.InfobipClient
	toolRegistry  domain.ToolRegistry
	logger        *log.Logger
}

// NewMessageProcessor creates a new message processor
func NewMessageProcessor(
	tenantManager domain.TenantManager,
	infobipClient domain.InfobipClient,
	toolRegistry domain.ToolRegistry,
	logger *log.Logger,
) *MessageProcessor {
	return &MessageProcessor{
		tenantManager: tenantManager,
		infobipClient: infobipClient,
		toolRegistry:  toolRegistry,
		logger:        logger,
	}
}

// ProcessIncoming processes an incoming webhook message
func (p *MessageProcessor) ProcessIncoming(ctx context.Context, webhookMsg *domain.InfobipWebhookMessage) error {
	if len(webhookMsg.Results) == 0 {
		return fmt.Errorf("no results in webhook message")
	}

	// Process each result
	for _, result := range webhookMsg.Results {
		if err := p.processWebhookResult(ctx, &result); err != nil {
			p.logger.WithContext(ctx).Error().
				Err(err).
				Str("message_id", result.MessageID).
				Msg("failed to process webhook result")
			// Continue processing other results
			continue
		}
	}

	return nil
}

// processWebhookResult processes a single webhook result
func (p *MessageProcessor) processWebhookResult(ctx context.Context, result *domain.InfobipWebhookResult) error {
	// Extract message text
	if result.Message.Type != "TEXT" || result.Message.Text.Text == "" {
		p.logger.WithContext(ctx).Debug().
			Str("message_type", result.Message.Type).
			Msg("skipping non-text message")
		return nil
	}

	// Get tenant by WABA number (the 'to' field in incoming messages)
	tenant, err := p.tenantManager.GetTenant(result.To)
	if err != nil {
		return fmt.Errorf("failed to get tenant for WABA number %s: %w", result.To, err)
	}

	// Get or create user
	repo, err := p.tenantManager.GetRepository(tenant.ID)
	if err != nil {
		return fmt.Errorf("failed to get repository for tenant %s: %w", tenant.ID, err)
	}

	user, err := p.getOrCreateUser(ctx, repo, tenant.ID, result.From, result.Contact.Name)
	if err != nil {
		return fmt.Errorf("failed to get or create user: %w", err)
	}

	// Create message record
	message := &domain.Message{
		ID:        uuid.New(),
		TenantID:  tenant.ID,
		UserID:    user.ID,
		MessageID: result.MessageID,
		Direction: "inbound",
		Text:      result.Message.Text.Text,
		Timestamp: result.ReceivedAt,
		Metadata: map[string]interface{}{
			"integration_type": result.IntegrationType,
			"contact_name":     result.Contact.Name,
		},
		CreatedAt: time.Now().UTC(),
	}

	if err := repo.CreateMessage(ctx, message); err != nil {
		return fmt.Errorf("failed to store incoming message: %w", err)
	}

	// Process the message
	return p.ProcessMessage(ctx, tenant, user, message)
}

// ProcessMessage processes a single message
func (p *MessageProcessor) ProcessMessage(ctx context.Context, tenant *domain.Tenant, user *domain.User, message *domain.Message) error {
	start := time.Now()

	// Add tenant and user context
	ctx = context.WithValue(ctx, log.TenantIDKey, tenant.ID)
	ctx = context.WithValue(ctx, log.UserIDKey, user.ID.String())

	logger := p.logger.WithContext(ctx).WithTenant(tenant.ID).WithUser(user.ID.String())

	logger.Info().
		Str("message_text", log.SanitizeText(message.Text)).
		Msg("processing message")

	// Get LLM provider for tenant
	llmProvider, err := p.tenantManager.GetLLMProvider(tenant.ID)
	if err != nil {
		return fmt.Errorf("failed to get LLM provider: %w", err)
	}

	// Get vector store for tenant
	vectorStore, err := p.tenantManager.GetVectorStore(tenant.ID)
	if err != nil {
		return fmt.Errorf("failed to get vector store: %w", err)
	}

	// Get repository for tenant
	repo, err := p.tenantManager.GetRepository(tenant.ID)
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}

	// Create RAG pipeline
	ragPipeline := rag.NewPipeline(llmProvider, vectorStore, repo, logger, rag.DefaultPipelineConfig())

	// Initialize tools for this tenant
	if err := p.initializeToolsForTenant(ctx, tenant.ID, vectorStore, llmProvider, repo, logger); err != nil {
		return fmt.Errorf("failed to initialize tools: %w", err)
	}

	// Create orchestrator
	orchestrator := agents.NewMainOrchestrator(
		llmProvider,
		ragPipeline,
		p.toolRegistry,
		nil, // Agent registry can be nil for now
		logger,
		agents.DefaultOrchestratorConfig(),
	)

	// Process message through orchestrator
	response, err := orchestrator.Route(ctx, tenant, user, message)
	if err != nil {
		return fmt.Errorf("orchestrator failed to process message: %w", err)
	}

	// Send response back to user
	if response.Text != "" {
		err = p.sendResponse(ctx, tenant, user, message, response.Text)
		if err != nil {
			return fmt.Errorf("failed to send response: %w", err)
		}
	}

	// Store outbound message
	outboundMessage := &domain.Message{
		ID:        uuid.New(),
		TenantID:  tenant.ID,
		UserID:    user.ID,
		MessageID: fmt.Sprintf("out_%d", time.Now().UnixNano()),
		Direction: "outbound",
		Text:      response.Text,
		Timestamp: time.Now().UTC(),
		Metadata:  response.Metadata,
		CreatedAt: time.Now().UTC(),
	}

	if err := repo.CreateMessage(ctx, outboundMessage); err != nil {
		logger.Warn().Err(err).Msg("failed to store outbound message")
		// Don't fail the whole process for this
	}

	logger.Info().
		Dur("duration", time.Since(start)).
		Bool("response_sent", response.Text != "").
		Msg("message processing completed")

	return nil
}

// getOrCreateUser gets an existing user or creates a new one
func (p *MessageProcessor) getOrCreateUser(ctx context.Context, repo domain.Repository, tenantID, phone, contactName string) (*domain.User, error) {
	// Try to get existing user
	user, err := repo.GetUser(ctx, tenantID, phone)
	if err != nil {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	if user != nil {
		return user, nil
	}

	// Create new user
	user = &domain.User{
		ID:       uuid.New(),
		TenantID: tenantID,
		Phone:    phone,
		Profile: map[string]interface{}{
			"name":       contactName,
			"created_at": time.Now().UTC().Format(time.RFC3339),
		},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := repo.CreateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	p.logger.WithContext(ctx).Info().
		Str("user_id", user.ID.String()).
		Str("phone", phone).
		Str("tenant_id", tenantID).
		Msg("new user created")

	return user, nil
}

// sendResponse sends a response message back to the user
func (p *MessageProcessor) sendResponse(ctx context.Context, tenant *domain.Tenant, user *domain.User, originalMessage *domain.Message, responseText string) error {
	_, err := p.infobipClient.SendText(ctx, tenant.WABANumber, user.Phone, responseText, originalMessage.MessageID)
	if err != nil {
		return fmt.Errorf("failed to send response via Infobip: %w", err)
	}

	p.logger.WithContext(ctx).Debug().
		Str("to", user.Phone).
		Str("text", log.SanitizeText(responseText)).
		Msg("response sent successfully")

	return nil
}

// initializeToolsForTenant initializes tools for a specific tenant
func (p *MessageProcessor) initializeToolsForTenant(ctx context.Context, tenantID string, vectorStore domain.VectorStore, llmProvider domain.LLMProvider, repo domain.Repository, logger *log.Logger) error {
	// Register DB tools
	if err := p.toolRegistry.RegisterTool(builtin.NewDBUpsertTool(vectorStore, llmProvider, logger)); err != nil {
		logger.Warn().Err(err).Msg("failed to register upsert tool")
	}

	if err := p.toolRegistry.RegisterTool(builtin.NewDBSearchTool(vectorStore, llmProvider, logger)); err != nil {
		logger.Warn().Err(err).Msg("failed to register search tool")
	}

	if err := p.toolRegistry.RegisterTool(builtin.NewDBGetByIDTool(vectorStore, logger)); err != nil {
		logger.Warn().Err(err).Msg("failed to register get_by_id tool")
	}

	if err := p.toolRegistry.RegisterTool(builtin.NewDBUpdateItemTool(vectorStore, llmProvider, logger)); err != nil {
		logger.Warn().Err(err).Msg("failed to register update_item tool")
	}

	// Register HTTP tools
	if err := p.toolRegistry.RegisterTool(builtin.NewHTTPCallTool(repo, logger)); err != nil {
		logger.Warn().Err(err).Msg("failed to register call_api tool")
	}

	if err := p.toolRegistry.RegisterTool(builtin.NewReminderScheduleTool(logger)); err != nil {
		logger.Warn().Err(err).Msg("failed to register schedule_reminder tool")
	}

	// Register example weather tool (optional)
	if err := p.toolRegistry.RegisterTool(builtin.NewWeatherTool("demo_api_key", logger)); err != nil {
		logger.Warn().Err(err).Msg("failed to register weather tool")
	}

	logger.Debug().
		Str("tenant_id", tenantID).
		Int("tools_registered", len(p.toolRegistry.ListTools())).
		Msg("tools initialized for tenant")

	return nil
}

// Close closes the message processor and its resources
func (p *MessageProcessor) Close() error {
	p.logger.Info().Msg("message processor shutting down")
	return nil
}
