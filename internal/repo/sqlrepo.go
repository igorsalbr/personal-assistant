package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"personal-assistant/internal/domain"
	"personal-assistant/internal/log"
)

// PostgresRepository implements the Repository interface using PostgreSQL
type PostgresRepository struct {
	db     *pgxpool.Pool
	logger *log.Logger
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(db *pgxpool.Pool, logger *log.Logger) *PostgresRepository {
	return &PostgresRepository{
		db:     db,
		logger: logger,
	}
}

// Ping checks database connectivity
func (r *PostgresRepository) Ping(ctx context.Context) error {
	return r.db.Ping(ctx)
}

// Close closes the database connection
func (r *PostgresRepository) Close() error {
	r.db.Close()
	return nil
}

// CreateUser creates a new user
func (r *PostgresRepository) CreateUser(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO users (id, tenant_id, phone, profile, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	
	profileJSON, err := json.Marshal(user.Profile)
	if err != nil {
		return fmt.Errorf("failed to marshal profile: %w", err)
	}

	_, err = r.db.Exec(ctx, query, user.ID, user.TenantID, user.Phone, profileJSON, user.CreatedAt, user.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	r.logger.WithContext(ctx).Debug().
		Str("user_id", user.ID.String()).
		Str("tenant_id", user.TenantID).
		Str("phone", user.Phone).
		Msg("user created")

	return nil
}

// GetUser retrieves a user by tenant and phone
func (r *PostgresRepository) GetUser(ctx context.Context, tenantID, phone string) (*domain.User, error) {
	query := `
		SELECT id, tenant_id, phone, profile, created_at, updated_at
		FROM users
		WHERE tenant_id = $1 AND phone = $2
	`

	var user domain.User
	var profileJSON []byte

	err := r.db.QueryRow(ctx, query, tenantID, phone).Scan(
		&user.ID, &user.TenantID, &user.Phone, &profileJSON, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if err := json.Unmarshal(profileJSON, &user.Profile); err != nil {
		return nil, fmt.Errorf("failed to unmarshal profile: %w", err)
	}

	return &user, nil
}

// GetUserByID retrieves a user by ID
func (r *PostgresRepository) GetUserByID(ctx context.Context, tenantID string, userID uuid.UUID) (*domain.User, error) {
	query := `
		SELECT id, tenant_id, phone, profile, created_at, updated_at
		FROM users
		WHERE tenant_id = $1 AND id = $2
	`

	var user domain.User
	var profileJSON []byte

	err := r.db.QueryRow(ctx, query, tenantID, userID).Scan(
		&user.ID, &user.TenantID, &user.Phone, &profileJSON, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}

	if err := json.Unmarshal(profileJSON, &user.Profile); err != nil {
		return nil, fmt.Errorf("failed to unmarshal profile: %w", err)
	}

	return &user, nil
}

// UpdateUser updates an existing user
func (r *PostgresRepository) UpdateUser(ctx context.Context, user *domain.User) error {
	query := `
		UPDATE users 
		SET profile = $1, updated_at = $2
		WHERE tenant_id = $3 AND id = $4
	`

	profileJSON, err := json.Marshal(user.Profile)
	if err != nil {
		return fmt.Errorf("failed to marshal profile: %w", err)
	}

	user.UpdatedAt = time.Now().UTC()
	_, err = r.db.Exec(ctx, query, profileJSON, user.UpdatedAt, user.TenantID, user.ID)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

// CreateMessage creates a new message
func (r *PostgresRepository) CreateMessage(ctx context.Context, message *domain.Message) error {
	query := `
		INSERT INTO messages (id, tenant_id, user_id, message_id, direction, text, timestamp, token_usage, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	var tokenUsageJSON []byte
	if message.TokenUsage != nil {
		var err error
		tokenUsageJSON, err = json.Marshal(message.TokenUsage)
		if err != nil {
			return fmt.Errorf("failed to marshal token usage: %w", err)
		}
	}

	metadataJSON, err := json.Marshal(message.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	_, err = r.db.Exec(ctx, query, 
		message.ID, message.TenantID, message.UserID, message.MessageID,
		message.Direction, message.Text, message.Timestamp, 
		tokenUsageJSON, metadataJSON, message.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}

	r.logger.WithContext(ctx).Debug().
		Str("message_id", message.ID.String()).
		Str("tenant_id", message.TenantID).
		Str("direction", message.Direction).
		Msg("message created")

	return nil
}

// GetMessages retrieves messages for a user
func (r *PostgresRepository) GetMessages(ctx context.Context, tenantID string, userID uuid.UUID, limit int) ([]domain.Message, error) {
	query := `
		SELECT id, tenant_id, user_id, message_id, direction, text, timestamp, token_usage, metadata, created_at
		FROM messages
		WHERE tenant_id = $1 AND user_id = $2
		ORDER BY timestamp DESC
		LIMIT $3
	`

	rows, err := r.db.Query(ctx, query, tenantID, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	var messages []domain.Message
	for rows.Next() {
		var message domain.Message
		var tokenUsageJSON, metadataJSON []byte

		err := rows.Scan(
			&message.ID, &message.TenantID, &message.UserID, &message.MessageID,
			&message.Direction, &message.Text, &message.Timestamp,
			&tokenUsageJSON, &metadataJSON, &message.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}

		if len(tokenUsageJSON) > 0 {
			if err := json.Unmarshal(tokenUsageJSON, &message.TokenUsage); err != nil {
				return nil, fmt.Errorf("failed to unmarshal token usage: %w", err)
			}
		}

		if err := json.Unmarshal(metadataJSON, &message.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		messages = append(messages, message)
	}

	return messages, rows.Err()
}

// GetMessageByID retrieves a message by its Infobip message ID
func (r *PostgresRepository) GetMessageByID(ctx context.Context, tenantID string, messageID string) (*domain.Message, error) {
	query := `
		SELECT id, tenant_id, user_id, message_id, direction, text, timestamp, token_usage, metadata, created_at
		FROM messages
		WHERE tenant_id = $1 AND message_id = $2
	`

	var message domain.Message
	var tokenUsageJSON, metadataJSON []byte

	err := r.db.QueryRow(ctx, query, tenantID, messageID).Scan(
		&message.ID, &message.TenantID, &message.UserID, &message.MessageID,
		&message.Direction, &message.Text, &message.Timestamp,
		&tokenUsageJSON, &metadataJSON, &message.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	if len(tokenUsageJSON) > 0 {
		if err := json.Unmarshal(tokenUsageJSON, &message.TokenUsage); err != nil {
			return nil, fmt.Errorf("failed to unmarshal token usage: %w", err)
		}
	}

	if err := json.Unmarshal(metadataJSON, &message.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &message, nil
}

// GetAgents retrieves all agents
func (r *PostgresRepository) GetAgents(ctx context.Context) ([]domain.AgentConfig, error) {
	query := `
		SELECT id, name, version, allowed_tenants, config, enabled, created_at, updated_at
		FROM agents
		ORDER BY name
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query agents: %w", err)
	}
	defer rows.Close()

	var agents []domain.AgentConfig
	for rows.Next() {
		var agent domain.AgentConfig
		var allowedTenants []string
		var configJSON []byte

		err := rows.Scan(
			&agent.ID, &agent.Name, &agent.Version, &allowedTenants,
			&configJSON, &agent.Enabled, &agent.CreatedAt, &agent.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent: %w", err)
		}

		agent.AllowedTenants = allowedTenants

		if err := json.Unmarshal(configJSON, &agent.Config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal agent config: %w", err)
		}

		agents = append(agents, agent)
	}

	return agents, rows.Err()
}

// GetAgentByName retrieves an agent by name
func (r *PostgresRepository) GetAgentByName(ctx context.Context, name string) (*domain.AgentConfig, error) {
	query := `
		SELECT id, name, version, allowed_tenants, config, enabled, created_at, updated_at
		FROM agents
		WHERE name = $1
	`

	var agent domain.AgentConfig
	var allowedTenants []string
	var configJSON []byte

	err := r.db.QueryRow(ctx, query, name).Scan(
		&agent.ID, &agent.Name, &agent.Version, &allowedTenants,
		&configJSON, &agent.Enabled, &agent.CreatedAt, &agent.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	agent.AllowedTenants = allowedTenants

	if err := json.Unmarshal(configJSON, &agent.Config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal agent config: %w", err)
	}

	return &agent, nil
}

// CreateAgent creates a new agent
func (r *PostgresRepository) CreateAgent(ctx context.Context, agent *domain.AgentConfig) error {
	query := `
		INSERT INTO agents (id, name, version, allowed_tenants, config, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	configJSON, err := json.Marshal(agent.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal agent config: %w", err)
	}

	_, err = r.db.Exec(ctx, query,
		agent.ID, agent.Name, agent.Version, agent.AllowedTenants,
		configJSON, agent.Enabled, agent.CreatedAt, agent.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	return nil
}

// UpdateAgent updates an existing agent
func (r *PostgresRepository) UpdateAgent(ctx context.Context, agent *domain.AgentConfig) error {
	query := `
		UPDATE agents 
		SET version = $1, allowed_tenants = $2, config = $3, enabled = $4, updated_at = $5
		WHERE id = $6
	`

	configJSON, err := json.Marshal(agent.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal agent config: %w", err)
	}

	agent.UpdatedAt = time.Now().UTC()
	_, err = r.db.Exec(ctx, query,
		agent.Version, agent.AllowedTenants, configJSON,
		agent.Enabled, agent.UpdatedAt, agent.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update agent: %w", err)
	}

	return nil
}

// GetExternalServices retrieves all external services for a tenant
func (r *PostgresRepository) GetExternalServices(ctx context.Context, tenantID string) ([]domain.ExternalService, error) {
	query := `
		SELECT id, tenant_id, name, base_url, auth, config, created_at, updated_at
		FROM external_services
		WHERE tenant_id = $1
		ORDER BY name
	`

	rows, err := r.db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to query external services: %w", err)
	}
	defer rows.Close()

	var services []domain.ExternalService
	for rows.Next() {
		var service domain.ExternalService
		var authJSON, configJSON []byte

		err := rows.Scan(
			&service.ID, &service.TenantID, &service.Name, &service.BaseURL,
			&authJSON, &configJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan external service: %w", err)
		}

		if err := json.Unmarshal(authJSON, &service.Auth); err != nil {
			return nil, fmt.Errorf("failed to unmarshal auth: %w", err)
		}

		if err := json.Unmarshal(configJSON, &service.Config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config: %w", err)
		}

		services = append(services, service)
	}

	return services, rows.Err()
}

// GetExternalService retrieves a specific external service
func (r *PostgresRepository) GetExternalService(ctx context.Context, tenantID, name string) (*domain.ExternalService, error) {
	query := `
		SELECT id, tenant_id, name, base_url, auth, config, created_at, updated_at
		FROM external_services
		WHERE tenant_id = $1 AND name = $2
	`

	var service domain.ExternalService
	var authJSON, configJSON []byte

	err := r.db.QueryRow(ctx, query, tenantID, name).Scan(
		&service.ID, &service.TenantID, &service.Name, &service.BaseURL,
		&authJSON, &configJSON,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get external service: %w", err)
	}

	if err := json.Unmarshal(authJSON, &service.Auth); err != nil {
		return nil, fmt.Errorf("failed to unmarshal auth: %w", err)
	}

	if err := json.Unmarshal(configJSON, &service.Config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &service, nil
}

// CreateExternalService creates a new external service
func (r *PostgresRepository) CreateExternalService(ctx context.Context, service *domain.ExternalService) error {
	query := `
		INSERT INTO external_services (id, tenant_id, name, base_url, auth, config, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
	`

	authJSON, err := json.Marshal(service.Auth)
	if err != nil {
		return fmt.Errorf("failed to marshal auth: %w", err)
	}

	configJSON, err := json.Marshal(service.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	_, err = r.db.Exec(ctx, query,
		service.ID, service.TenantID, service.Name, service.BaseURL,
		authJSON, configJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to create external service: %w", err)
	}

	return nil
}

// UpdateExternalService updates an existing external service
func (r *PostgresRepository) UpdateExternalService(ctx context.Context, service *domain.ExternalService) error {
	query := `
		UPDATE external_services 
		SET base_url = $1, auth = $2, config = $3, updated_at = NOW()
		WHERE tenant_id = $4 AND id = $5
	`

	authJSON, err := json.Marshal(service.Auth)
	if err != nil {
		return fmt.Errorf("failed to marshal auth: %w", err)
	}

	configJSON, err := json.Marshal(service.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	_, err = r.db.Exec(ctx, query,
		service.BaseURL, authJSON, configJSON, service.TenantID, service.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update external service: %w", err)
	}

	return nil
}

// GetLLMProviders retrieves all LLM providers for a tenant
func (r *PostgresRepository) GetLLMProviders(ctx context.Context, tenantID string) ([]domain.LLMProviderConfig, error) {
	query := `
		SELECT id, tenant_id, provider, name, api_key, base_url, model_chat, model_embed, 
		       config, is_default, enabled, created_at, updated_at
		FROM llm_providers
		WHERE tenant_id = $1
		ORDER BY is_default DESC, name
	`

	rows, err := r.db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to query LLM providers: %w", err)
	}
	defer rows.Close()

	var providers []domain.LLMProviderConfig
	for rows.Next() {
		var provider domain.LLMProviderConfig
		var configJSON []byte
		var baseURL *string
		var modelEmbed *string

		err := rows.Scan(
			&provider.ID, &provider.TenantID, &provider.Provider, &provider.Name,
			&provider.APIKey, &baseURL, &provider.ModelChat, &modelEmbed,
			&configJSON, &provider.IsDefault, &provider.Enabled,
			&provider.CreatedAt, &provider.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan LLM provider: %w", err)
		}

		if baseURL != nil {
			provider.BaseURL = *baseURL
		}
		if modelEmbed != nil {
			provider.ModelEmbed = *modelEmbed
		}

		if err := json.Unmarshal(configJSON, &provider.Config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal LLM provider config: %w", err)
		}

		providers = append(providers, provider)
	}

	return providers, rows.Err()
}

// GetLLMProvider retrieves a specific LLM provider
func (r *PostgresRepository) GetLLMProvider(ctx context.Context, tenantID, name string) (*domain.LLMProviderConfig, error) {
	query := `
		SELECT id, tenant_id, provider, name, api_key, base_url, model_chat, model_embed,
		       config, is_default, enabled, created_at, updated_at
		FROM llm_providers
		WHERE tenant_id = $1 AND name = $2
	`

	var provider domain.LLMProviderConfig
	var configJSON []byte
	var baseURL *string
	var modelEmbed *string

	err := r.db.QueryRow(ctx, query, tenantID, name).Scan(
		&provider.ID, &provider.TenantID, &provider.Provider, &provider.Name,
		&provider.APIKey, &baseURL, &provider.ModelChat, &modelEmbed,
		&configJSON, &provider.IsDefault, &provider.Enabled,
		&provider.CreatedAt, &provider.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get LLM provider: %w", err)
	}

	if baseURL != nil {
		provider.BaseURL = *baseURL
	}
	if modelEmbed != nil {
		provider.ModelEmbed = *modelEmbed
	}

	if err := json.Unmarshal(configJSON, &provider.Config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal LLM provider config: %w", err)
	}

	return &provider, nil
}

// GetDefaultLLMProvider retrieves the default LLM provider for a tenant
func (r *PostgresRepository) GetDefaultLLMProvider(ctx context.Context, tenantID string) (*domain.LLMProviderConfig, error) {
	query := `
		SELECT id, tenant_id, provider, name, api_key, base_url, model_chat, model_embed,
		       config, is_default, enabled, created_at, updated_at
		FROM llm_providers
		WHERE tenant_id = $1 AND is_default = TRUE AND enabled = TRUE
		LIMIT 1
	`

	var provider domain.LLMProviderConfig
	var configJSON []byte
	var baseURL *string
	var modelEmbed *string

	err := r.db.QueryRow(ctx, query, tenantID).Scan(
		&provider.ID, &provider.TenantID, &provider.Provider, &provider.Name,
		&provider.APIKey, &baseURL, &provider.ModelChat, &modelEmbed,
		&configJSON, &provider.IsDefault, &provider.Enabled,
		&provider.CreatedAt, &provider.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get default LLM provider: %w", err)
	}

	if baseURL != nil {
		provider.BaseURL = *baseURL
	}
	if modelEmbed != nil {
		provider.ModelEmbed = *modelEmbed
	}

	if err := json.Unmarshal(configJSON, &provider.Config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal LLM provider config: %w", err)
	}

	return &provider, nil
}

// CreateLLMProvider creates a new LLM provider
func (r *PostgresRepository) CreateLLMProvider(ctx context.Context, provider *domain.LLMProviderConfig) error {
	// If this is being set as default, unset other defaults first
	if provider.IsDefault {
		_, err := r.db.Exec(ctx,
			`UPDATE llm_providers SET is_default = FALSE WHERE tenant_id = $1 AND is_default = TRUE`,
			provider.TenantID)
		if err != nil {
			return fmt.Errorf("failed to unset existing default LLM provider: %w", err)
		}
	}

	query := `
		INSERT INTO llm_providers (
			id, tenant_id, provider, name, api_key, base_url, model_chat, model_embed,
			config, is_default, enabled, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	configJSON, err := json.Marshal(provider.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal LLM provider config: %w", err)
	}

	var baseURL *string
	if provider.BaseURL != "" {
		baseURL = &provider.BaseURL
	}

	var modelEmbed *string
	if provider.ModelEmbed != "" {
		modelEmbed = &provider.ModelEmbed
	}

	_, err = r.db.Exec(ctx, query,
		provider.ID, provider.TenantID, provider.Provider, provider.Name,
		provider.APIKey, baseURL, provider.ModelChat, modelEmbed,
		configJSON, provider.IsDefault, provider.Enabled,
		provider.CreatedAt, provider.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create LLM provider: %w", err)
	}

	r.logger.WithContext(ctx).Debug().
		Str("provider_id", provider.ID.String()).
		Str("tenant_id", provider.TenantID).
		Str("provider_type", provider.Provider).
		Str("name", provider.Name).
		Msg("LLM provider created")

	return nil
}

// UpdateLLMProvider updates an existing LLM provider
func (r *PostgresRepository) UpdateLLMProvider(ctx context.Context, provider *domain.LLMProviderConfig) error {
	// If this is being set as default, unset other defaults first
	if provider.IsDefault {
		_, err := r.db.Exec(ctx,
			`UPDATE llm_providers SET is_default = FALSE WHERE tenant_id = $1 AND is_default = TRUE AND id != $2`,
			provider.TenantID, provider.ID)
		if err != nil {
			return fmt.Errorf("failed to unset existing default LLM provider: %w", err)
		}
	}

	query := `
		UPDATE llm_providers 
		SET provider = $1, api_key = $2, base_url = $3, model_chat = $4, model_embed = $5,
		    config = $6, is_default = $7, enabled = $8, updated_at = $9
		WHERE tenant_id = $10 AND id = $11
	`

	configJSON, err := json.Marshal(provider.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal LLM provider config: %w", err)
	}

	var baseURL *string
	if provider.BaseURL != "" {
		baseURL = &provider.BaseURL
	}

	var modelEmbed *string
	if provider.ModelEmbed != "" {
		modelEmbed = &provider.ModelEmbed
	}

	provider.UpdatedAt = time.Now().UTC()
	_, err = r.db.Exec(ctx, query,
		provider.Provider, provider.APIKey, baseURL, provider.ModelChat, modelEmbed,
		configJSON, provider.IsDefault, provider.Enabled, provider.UpdatedAt,
		provider.TenantID, provider.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update LLM provider: %w", err)
	}

	return nil
}

// GetTenantsConfig retrieves all tenant configurations
func (r *PostgresRepository) GetTenantsConfig(ctx context.Context) ([]domain.TenantConfig, error) {
	query := `
		SELECT id, tenant_id, waba_number, embedding_model, vector_store, 
		       enabled_agents, config, metadata, enabled, created_at, updated_at
		FROM tenants_config
		WHERE enabled = true
		ORDER BY tenant_id
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tenant configs: %w", err)
	}
	defer rows.Close()

	var configs []domain.TenantConfig
	for rows.Next() {
		var config domain.TenantConfig
		var configJSON, metadataJSON []byte

		err := rows.Scan(
			&config.ID, &config.TenantID, &config.WABANumber, &config.EmbeddingModel, &config.VectorStore,
			&config.EnabledAgents, &configJSON, &metadataJSON, &config.Enabled,
			&config.CreatedAt, &config.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tenant config: %w", err)
		}

		if err := json.Unmarshal(configJSON, &config.Config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &config.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		configs = append(configs, config)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate tenant configs: %w", err)
	}

	return configs, nil
}

// GetTenantConfig retrieves a specific tenant configuration by tenant ID
func (r *PostgresRepository) GetTenantConfig(ctx context.Context, tenantID string) (*domain.TenantConfig, error) {
	query := `
		SELECT id, tenant_id, waba_number, embedding_model, vector_store,
		       enabled_agents, config, metadata, enabled, created_at, updated_at
		FROM tenants_config
		WHERE tenant_id = $1
	`

	var config domain.TenantConfig
	var configJSON, metadataJSON []byte

	err := r.db.QueryRow(ctx, query, tenantID).Scan(
		&config.ID, &config.TenantID, &config.WABANumber, &config.EmbeddingModel, &config.VectorStore,
		&config.EnabledAgents, &configJSON, &metadataJSON, &config.Enabled,
		&config.CreatedAt, &config.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get tenant config: %w", err)
	}

	if err := json.Unmarshal(configJSON, &config.Config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &config.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &config, nil
}

// GetTenantConfigByWABA retrieves a tenant configuration by WABA number
func (r *PostgresRepository) GetTenantConfigByWABA(ctx context.Context, wabaNumber string) (*domain.TenantConfig, error) {
	query := `
		SELECT id, tenant_id, waba_number, embedding_model, vector_store,
		       enabled_agents, config, metadata, enabled, created_at, updated_at
		FROM tenants_config
		WHERE waba_number = $1 AND enabled = true
	`

	var config domain.TenantConfig
	var configJSON, metadataJSON []byte

	err := r.db.QueryRow(ctx, query, wabaNumber).Scan(
		&config.ID, &config.TenantID, &config.WABANumber, &config.EmbeddingModel, &config.VectorStore,
		&config.EnabledAgents, &configJSON, &metadataJSON, &config.Enabled,
		&config.CreatedAt, &config.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get tenant config by WABA: %w", err)
	}

	if err := json.Unmarshal(configJSON, &config.Config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &config.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &config, nil
}

// CreateTenantConfig creates a new tenant configuration
func (r *PostgresRepository) CreateTenantConfig(ctx context.Context, config *domain.TenantConfig) error {
	query := `
		INSERT INTO tenants_config (id, tenant_id, waba_number, embedding_model, vector_store,
		                           enabled_agents, config, metadata, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	configJSON, err := json.Marshal(config.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	metadataJSON, err := json.Marshal(config.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	config.CreatedAt = time.Now().UTC()
	config.UpdatedAt = config.CreatedAt

	_, err = r.db.Exec(ctx, query,
		config.ID, config.TenantID, config.WABANumber, config.EmbeddingModel, config.VectorStore,
		config.EnabledAgents, configJSON, metadataJSON, config.Enabled,
		config.CreatedAt, config.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create tenant config: %w", err)
	}

	r.logger.WithContext(ctx).Debug().
		Str("tenant_id", config.TenantID).
		Str("waba_number", config.WABANumber).
		Msg("tenant config created")

	return nil
}

// UpdateTenantConfig updates an existing tenant configuration
func (r *PostgresRepository) UpdateTenantConfig(ctx context.Context, config *domain.TenantConfig) error {
	query := `
		UPDATE tenants_config 
		SET waba_number = $1, embedding_model = $2, vector_store = $3,
		    enabled_agents = $4, config = $5, metadata = $6, enabled = $7, updated_at = $8
		WHERE tenant_id = $9
	`

	configJSON, err := json.Marshal(config.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	metadataJSON, err := json.Marshal(config.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	config.UpdatedAt = time.Now().UTC()
	_, err = r.db.Exec(ctx, query,
		config.WABANumber, config.EmbeddingModel, config.VectorStore,
		config.EnabledAgents, configJSON, metadataJSON, config.Enabled, config.UpdatedAt,
		config.TenantID,
	)
	if err != nil {
		return fmt.Errorf("failed to update tenant config: %w", err)
	}

	return nil
}

// GetSystemConfig retrieves a system configuration value
func (r *PostgresRepository) GetSystemConfig(ctx context.Context, key string) (*domain.SystemConfig, error) {
	query := `
		SELECT id, key, value, description, created_at, updated_at
		FROM system_config
		WHERE key = $1
	`

	var config domain.SystemConfig
	var valueJSON []byte

	err := r.db.QueryRow(ctx, query, key).Scan(
		&config.ID, &config.Key, &valueJSON, &config.Description,
		&config.CreatedAt, &config.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get system config: %w", err)
	}

	if err := json.Unmarshal(valueJSON, &config.Value); err != nil {
		return nil, fmt.Errorf("failed to unmarshal value: %w", err)
	}

	return &config, nil
}

// SetSystemConfig creates or updates a system configuration
func (r *PostgresRepository) SetSystemConfig(ctx context.Context, key string, value interface{}, description string) error {
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	query := `
		INSERT INTO system_config (id, key, value, description, created_at, updated_at)
		VALUES (uuid_generate_v4(), $1, $2, $3, NOW(), NOW())
		ON CONFLICT (key)
		DO UPDATE SET value = EXCLUDED.value, description = EXCLUDED.description, updated_at = NOW()
	`

	_, err = r.db.Exec(ctx, query, key, valueJSON, description)
	if err != nil {
		return fmt.Errorf("failed to set system config: %w", err)
	}

	return nil
}