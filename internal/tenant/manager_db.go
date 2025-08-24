package tenant

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"personal-assistant/internal/config"
	"personal-assistant/internal/domain"
	"personal-assistant/internal/llm"
	"personal-assistant/internal/log"
	repoImpl "personal-assistant/internal/repo"
	"personal-assistant/internal/rag/vectorstore"
)

// DatabaseManager implements the TenantManager interface using database-first configuration
type DatabaseManager struct {
	config   *config.Config
	logger   *log.Logger
	globalDB *pgxpool.Pool
	globalRepo domain.Repository

	// Caches
	tenants      map[string]*domain.Tenant       // WABA number -> Tenant
	tenantsByID  map[string]*domain.Tenant       // Tenant ID -> Tenant
	repositories map[string]domain.Repository    // Tenant ID -> Repository
	vectorStores map[string]domain.VectorStore   // Tenant ID -> VectorStore
	llmProviders map[string]*llm.ProviderManager // Tenant ID -> LLM Provider Manager

	mutex sync.RWMutex
}

// newDatabaseManager creates a new database-first tenant manager
func newDatabaseManager(cfg *config.Config, logger *log.Logger) (*DatabaseManager, error) {
	// Connect to the central database
	db, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to central database: %w", err)
	}

	// Test connection
	if err := db.Ping(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping central database: %w", err)
	}

	// Create global repository
	globalRepo := repoImpl.NewPostgresRepository(db, logger)

	manager := &DatabaseManager{
		config:       cfg,
		logger:       logger,
		globalDB:     db,
		globalRepo:   globalRepo,
		tenants:      make(map[string]*domain.Tenant),
		tenantsByID:  make(map[string]*domain.Tenant),
		repositories: make(map[string]domain.Repository),
		vectorStores: make(map[string]domain.VectorStore),
		llmProviders: make(map[string]*llm.ProviderManager),
	}

	// Initialize tenants from database
	if err := manager.initializeTenants(); err != nil {
		return nil, fmt.Errorf("failed to initialize tenants: %w", err)
	}

	logger.Info().Int("tenants_loaded", len(manager.tenants)).Msg("database tenant manager initialized")

	return manager, nil
}

// GetTenant retrieves tenant configuration by WABA number
func (m *DatabaseManager) GetTenant(wabaNumber string) (*domain.Tenant, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	tenant, exists := m.tenants[wabaNumber]
	if !exists {
		return nil, fmt.Errorf("tenant not found for WABA number: %s", wabaNumber)
	}

	return tenant, nil
}

// GetTenantByID retrieves tenant configuration by tenant ID
func (m *DatabaseManager) GetTenantByID(tenantID string) (*domain.Tenant, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	tenant, exists := m.tenantsByID[tenantID]
	if !exists {
		return nil, fmt.Errorf("tenant not found for ID: %s", tenantID)
	}

	return tenant, nil
}

// ListTenants returns all configured tenants
func (m *DatabaseManager) ListTenants() ([]domain.Tenant, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	tenants := make([]domain.Tenant, 0, len(m.tenantsByID))
	for _, tenant := range m.tenantsByID {
		tenants = append(tenants, *tenant)
	}

	return tenants, nil
}

// IsAgentEnabled checks if an agent is enabled for a tenant
func (m *DatabaseManager) IsAgentEnabled(tenantID, agentName string) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	tenant, exists := m.tenantsByID[tenantID]
	if !exists {
		return false
	}

	for _, enabledAgent := range tenant.EnabledAgents {
		if enabledAgent == agentName {
			return true
		}
	}

	return false
}

// GetRepository returns a repository instance for the tenant (uses global DB with RLS)
func (m *DatabaseManager) GetRepository(tenantID string) (domain.Repository, error) {
	m.mutex.RLock()
	repo, exists := m.repositories[tenantID]
	m.mutex.RUnlock()

	if exists {
		return repo, nil
	}

	// Create repository if it doesn't exist
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check again in case another goroutine created it
	if repo, exists := m.repositories[tenantID]; exists {
		return repo, nil
	}

	_, exists = m.tenantsByID[tenantID]
	if !exists {
		return nil, fmt.Errorf("tenant not found: %s", tenantID)
	}

	// Create a tenant-specific repository wrapper that sets RLS context
	repo = &TenantRepository{
		repo:     m.globalRepo,
		tenantID: tenantID,
		db:       m.globalDB,
		logger:   m.logger.WithTenant(tenantID),
	}

	m.repositories[tenantID] = repo

	m.logger.Info().Str("tenant_id", tenantID).Msg("tenant repository created")

	return repo, nil
}

// GetVectorStore returns a vector store instance for the tenant
func (m *DatabaseManager) GetVectorStore(tenantID string) (domain.VectorStore, error) {
	m.mutex.RLock()
	store, exists := m.vectorStores[tenantID]
	m.mutex.RUnlock()

	if exists {
		return store, nil
	}

	// Create vector store if it doesn't exist
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check again in case another goroutine created it
	if store, exists := m.vectorStores[tenantID]; exists {
		return store, nil
	}

	tenant, exists := m.tenantsByID[tenantID]
	if !exists {
		return nil, fmt.Errorf("tenant not found: %s", tenantID)
	}

	// Create vector store based on tenant configuration
	factory := vectorstore.NewFactory()
	storeType := vectorstore.GetVectorStoreType(tenant.VectorStore)

	config := map[string]interface{}{
		"db_url": m.config.DatabaseURL, // Use central DB
		"logger": m.logger.WithTenant(tenantID),
	}

	store, err := factory.Create(storeType, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create vector store for tenant %s: %w", tenantID, err)
	}

	m.vectorStores[tenantID] = store

	m.logger.Info().
		Str("tenant_id", tenantID).
		Str("vector_store_type", string(storeType)).
		Msg("vector store created for tenant")

	return store, nil
}

// GetLLMProvider returns the default LLM provider for a tenant
func (m *DatabaseManager) GetLLMProvider(tenantID string) (domain.LLMProvider, error) {
	m.mutex.RLock()
	providerManager, exists := m.llmProviders[tenantID]
	m.mutex.RUnlock()

	if !exists {
		// Create provider manager if it doesn't exist
		m.mutex.Lock()
		providerManager = llm.NewProviderManager(m.logger.WithTenant(tenantID))
		m.llmProviders[tenantID] = providerManager
		m.mutex.Unlock()
	}

	// Get default LLM provider config for tenant from database
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Set tenant context for RLS
	if err := m.setTenantContext(ctx, tenantID); err != nil {
		return nil, fmt.Errorf("failed to set tenant context: %w", err)
	}

	providerConfig, err := m.globalRepo.GetDefaultLLMProvider(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get default LLM provider config: %w", err)
	}

	if providerConfig == nil {
		return nil, fmt.Errorf("no default LLM provider configured for tenant: %s", tenantID)
	}

	// Get or create the provider
	provider, err := providerManager.GetProvider(tenantID, providerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get LLM provider: %w", err)
	}

	return provider, nil
}

// initializeTenants initializes all tenants from database
func (m *DatabaseManager) initializeTenants() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get all tenant configurations from database
	tenantConfigs, err := m.globalRepo.GetTenantsConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load tenant configurations from database: %w", err)
	}

	for _, tenantConfig := range tenantConfigs {
		tenant := m.convertConfigToTenant(&tenantConfig)

		// Store by WABA number and tenant ID
		m.tenants[tenant.WABANumber] = tenant
		m.tenantsByID[tenant.ID] = tenant

		m.logger.Debug().
			Str("tenant_id", tenant.ID).
			Str("waba_number", tenant.WABANumber).
			Msg("tenant initialized from database")
	}

	return nil
}

// convertConfigToTenant converts a domain.TenantConfig to domain.Tenant
func (m *DatabaseManager) convertConfigToTenant(config *domain.TenantConfig) *domain.Tenant {
	// Convert map[string]any to map[string]string for metadata
	metadata := make(map[string]string)
	if config.Metadata != nil {
		for k, v := range config.Metadata {
			if str, ok := v.(string); ok {
				metadata[k] = str
			} else {
				metadata[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	return &domain.Tenant{
		ID:             config.TenantID,
		WABANumber:     config.WABANumber,
		DBDSN:          m.config.DatabaseURL, // Use central DB for all tenants
		EmbeddingModel: config.EmbeddingModel,
		VectorStore:    config.VectorStore,
		EnabledAgents:  config.EnabledAgents,
		Config:         config.Config,
		Metadata:       metadata,
	}
}

// ReloadTenants reloads tenant configurations from database
func (m *DatabaseManager) ReloadTenants() error {
	m.logger.Info().Msg("reloading tenant configurations from database")

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Clear existing tenants
	oldTenants := m.tenants
	oldTenantsById := m.tenantsByID

	m.tenants = make(map[string]*domain.Tenant)
	m.tenantsByID = make(map[string]*domain.Tenant)

	// Reinitialize tenants from database
	if err := m.initializeTenants(); err != nil {
		// Restore old state on error
		m.tenants = oldTenants
		m.tenantsByID = oldTenantsById
		return fmt.Errorf("failed to reinitialize tenants from database: %w", err)
	}

	// Clean up resources for removed tenants
	m.cleanupRemovedTenants(oldTenantsById)

	m.logger.Info().Int("tenants_loaded", len(m.tenants)).Msg("tenant configurations reloaded from database")

	return nil
}

// cleanupRemovedTenants cleans up resources for tenants that were removed
func (m *DatabaseManager) cleanupRemovedTenants(oldTenants map[string]*domain.Tenant) {
	for tenantID := range oldTenants {
		if _, exists := m.tenantsByID[tenantID]; !exists {
			// Tenant was removed, clean up resources
			m.logger.Info().Str("tenant_id", tenantID).Msg("cleaning up removed tenant")

			// Close repository connection
			if repo, exists := m.repositories[tenantID]; exists {
				repo.Close()
				delete(m.repositories, tenantID)
			}

			// Close vector store
			if store, exists := m.vectorStores[tenantID]; exists {
				store.Close()
				delete(m.vectorStores, tenantID)
			}

			// Clear LLM provider cache
			if providerManager, exists := m.llmProviders[tenantID]; exists {
				providerManager.ClearTenant(tenantID)
				delete(m.llmProviders, tenantID)
			}
		}
	}
}

// setTenantContext sets the tenant context for RLS
func (m *DatabaseManager) setTenantContext(ctx context.Context, tenantID string) error {
	_, err := m.globalDB.Exec(ctx, "SELECT set_tenant_context($1)", tenantID)
	return err
}

// Close closes all tenant resources
func (m *DatabaseManager) Close() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var errors []string

	// Close all repositories
	for tenantID, repo := range m.repositories {
		if err := repo.Close(); err != nil {
			errors = append(errors, fmt.Sprintf("failed to close repository for tenant %s: %v", tenantID, err))
		}
	}

	// Close all vector stores
	for tenantID, store := range m.vectorStores {
		if err := store.Close(); err != nil {
			errors = append(errors, fmt.Sprintf("failed to close vector store for tenant %s: %v", tenantID, err))
		}
	}

	// Close global database connection
	if m.globalDB != nil {
		m.globalDB.Close()
	}

	// Clear all caches
	m.repositories = make(map[string]domain.Repository)
	m.vectorStores = make(map[string]domain.VectorStore)
	m.llmProviders = make(map[string]*llm.ProviderManager)

	if len(errors) > 0 {
		return fmt.Errorf("errors occurred while closing tenant resources: %v", errors)
	}

	m.logger.Info().Msg("database tenant manager closed")
	return nil
}

// GetTenantStats returns statistics about tenant resource usage
func (m *DatabaseManager) GetTenantStats() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return map[string]interface{}{
		"total_tenants":       len(m.tenants),
		"active_repos":        len(m.repositories),
		"active_vectorstores": len(m.vectorStores),
		"active_llm_managers": len(m.llmProviders),
		"tenant_ids":          m.getTenantIDs(),
		"database_backend":    "centralized",
	}
}

// getTenantIDs returns a list of all tenant IDs
func (m *DatabaseManager) getTenantIDs() []string {
	ids := make([]string, 0, len(m.tenantsByID))
	for id := range m.tenantsByID {
		ids = append(ids, id)
	}
	return ids
}