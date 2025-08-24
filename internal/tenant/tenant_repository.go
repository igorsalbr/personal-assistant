package tenant

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"personal-assistant/internal/domain"
	"personal-assistant/internal/log"
)

// TenantRepository wraps a repository with tenant context management for RLS
type TenantRepository struct {
	repo     domain.Repository
	tenantID string
	db       *pgxpool.Pool
	logger   *log.Logger
}

// setTenantContext sets the tenant context for RLS before any operation
func (r *TenantRepository) setTenantContext(ctx context.Context) error {
	_, err := r.db.Exec(ctx, "SELECT set_tenant_context($1)", r.tenantID)
	if err != nil {
		r.logger.Error().Err(err).Str("tenant_id", r.tenantID).Msg("failed to set tenant context")
	}
	return err
}

// User operations
func (r *TenantRepository) CreateUser(ctx context.Context, user *domain.User) error {
	if err := r.setTenantContext(ctx); err != nil {
		return err
	}
	return r.repo.CreateUser(ctx, user)
}

func (r *TenantRepository) GetUser(ctx context.Context, tenantID, phone string) (*domain.User, error) {
	if err := r.setTenantContext(ctx); err != nil {
		return nil, err
	}
	return r.repo.GetUser(ctx, tenantID, phone)
}

func (r *TenantRepository) GetUserByID(ctx context.Context, tenantID string, userID uuid.UUID) (*domain.User, error) {
	if err := r.setTenantContext(ctx); err != nil {
		return nil, err
	}
	return r.repo.GetUserByID(ctx, tenantID, userID)
}

func (r *TenantRepository) UpdateUser(ctx context.Context, user *domain.User) error {
	if err := r.setTenantContext(ctx); err != nil {
		return err
	}
	return r.repo.UpdateUser(ctx, user)
}

// Message operations
func (r *TenantRepository) CreateMessage(ctx context.Context, message *domain.Message) error {
	if err := r.setTenantContext(ctx); err != nil {
		return err
	}
	return r.repo.CreateMessage(ctx, message)
}

func (r *TenantRepository) GetMessages(ctx context.Context, tenantID string, userID uuid.UUID, limit int) ([]domain.Message, error) {
	if err := r.setTenantContext(ctx); err != nil {
		return nil, err
	}
	return r.repo.GetMessages(ctx, tenantID, userID, limit)
}

func (r *TenantRepository) GetMessageByID(ctx context.Context, tenantID string, messageID string) (*domain.Message, error) {
	if err := r.setTenantContext(ctx); err != nil {
		return nil, err
	}
	return r.repo.GetMessageByID(ctx, tenantID, messageID)
}

// Agent operations (global - no tenant context needed)
func (r *TenantRepository) GetAgents(ctx context.Context) ([]domain.AgentConfig, error) {
	return r.repo.GetAgents(ctx)
}

func (r *TenantRepository) GetAgentByName(ctx context.Context, name string) (*domain.AgentConfig, error) {
	return r.repo.GetAgentByName(ctx, name)
}

func (r *TenantRepository) CreateAgent(ctx context.Context, agent *domain.AgentConfig) error {
	return r.repo.CreateAgent(ctx, agent)
}

func (r *TenantRepository) UpdateAgent(ctx context.Context, agent *domain.AgentConfig) error {
	return r.repo.UpdateAgent(ctx, agent)
}

// External service operations
func (r *TenantRepository) GetExternalServices(ctx context.Context, tenantID string) ([]domain.ExternalService, error) {
	if err := r.setTenantContext(ctx); err != nil {
		return nil, err
	}
	return r.repo.GetExternalServices(ctx, tenantID)
}

func (r *TenantRepository) GetExternalService(ctx context.Context, tenantID, name string) (*domain.ExternalService, error) {
	if err := r.setTenantContext(ctx); err != nil {
		return nil, err
	}
	return r.repo.GetExternalService(ctx, tenantID, name)
}

func (r *TenantRepository) CreateExternalService(ctx context.Context, service *domain.ExternalService) error {
	if err := r.setTenantContext(ctx); err != nil {
		return err
	}
	return r.repo.CreateExternalService(ctx, service)
}

func (r *TenantRepository) UpdateExternalService(ctx context.Context, service *domain.ExternalService) error {
	if err := r.setTenantContext(ctx); err != nil {
		return err
	}
	return r.repo.UpdateExternalService(ctx, service)
}

// LLM provider operations
func (r *TenantRepository) GetLLMProviders(ctx context.Context, tenantID string) ([]domain.LLMProviderConfig, error) {
	if err := r.setTenantContext(ctx); err != nil {
		return nil, err
	}
	return r.repo.GetLLMProviders(ctx, tenantID)
}

func (r *TenantRepository) GetLLMProvider(ctx context.Context, tenantID, name string) (*domain.LLMProviderConfig, error) {
	if err := r.setTenantContext(ctx); err != nil {
		return nil, err
	}
	return r.repo.GetLLMProvider(ctx, tenantID, name)
}

func (r *TenantRepository) GetDefaultLLMProvider(ctx context.Context, tenantID string) (*domain.LLMProviderConfig, error) {
	if err := r.setTenantContext(ctx); err != nil {
		return nil, err
	}
	return r.repo.GetDefaultLLMProvider(ctx, tenantID)
}

func (r *TenantRepository) CreateLLMProvider(ctx context.Context, config *domain.LLMProviderConfig) error {
	if err := r.setTenantContext(ctx); err != nil {
		return err
	}
	return r.repo.CreateLLMProvider(ctx, config)
}

func (r *TenantRepository) UpdateLLMProvider(ctx context.Context, config *domain.LLMProviderConfig) error {
	if err := r.setTenantContext(ctx); err != nil {
		return err
	}
	return r.repo.UpdateLLMProvider(ctx, config)
}

// Tenant configuration operations (admin level - clear context first)
func (r *TenantRepository) GetTenantsConfig(ctx context.Context) ([]domain.TenantConfig, error) {
	// Clear tenant context for admin operations
	_, _ = r.db.Exec(ctx, "SELECT clear_tenant_context()")
	return r.repo.GetTenantsConfig(ctx)
}

func (r *TenantRepository) GetTenantConfig(ctx context.Context, tenantID string) (*domain.TenantConfig, error) {
	// Clear tenant context for admin operations
	_, _ = r.db.Exec(ctx, "SELECT clear_tenant_context()")
	return r.repo.GetTenantConfig(ctx, tenantID)
}

func (r *TenantRepository) GetTenantConfigByWABA(ctx context.Context, wabaNumber string) (*domain.TenantConfig, error) {
	// Clear tenant context for admin operations
	_, _ = r.db.Exec(ctx, "SELECT clear_tenant_context()")
	return r.repo.GetTenantConfigByWABA(ctx, wabaNumber)
}

func (r *TenantRepository) CreateTenantConfig(ctx context.Context, config *domain.TenantConfig) error {
	// Clear tenant context for admin operations
	_, _ = r.db.Exec(ctx, "SELECT clear_tenant_context()")
	return r.repo.CreateTenantConfig(ctx, config)
}

func (r *TenantRepository) UpdateTenantConfig(ctx context.Context, config *domain.TenantConfig) error {
	// Clear tenant context for admin operations
	_, _ = r.db.Exec(ctx, "SELECT clear_tenant_context()")
	return r.repo.UpdateTenantConfig(ctx, config)
}

// System configuration operations (global - no tenant context needed)
func (r *TenantRepository) GetSystemConfig(ctx context.Context, key string) (*domain.SystemConfig, error) {
	return r.repo.GetSystemConfig(ctx, key)
}

func (r *TenantRepository) SetSystemConfig(ctx context.Context, key string, value interface{}, description string) error {
	return r.repo.SetSystemConfig(ctx, key, value, description)
}

// Utility operations
func (r *TenantRepository) Ping(ctx context.Context) error {
	return r.repo.Ping(ctx)
}

func (r *TenantRepository) Close() error {
	// Don't close the underlying repo since it's shared
	// Just clear the tenant context
	ctx := context.Background()
	_, _ = r.db.Exec(ctx, "SELECT clear_tenant_context()")
	return nil
}