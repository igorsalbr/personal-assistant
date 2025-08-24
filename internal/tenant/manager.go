package tenant

import (
	"context"
	"crypto/sha256"
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

// Manager implements the TenantManager interface
type Manager struct {
	config        *config.Config
	tenantsConfig *config.TenantsConfig
	logger        *log.Logger

	// Caches
	tenants      map[string]*domain.Tenant       // WABA number -> Tenant
	tenantsByID  map[string]*domain.Tenant       // Tenant ID -> Tenant
	repositories map[string]domain.Repository    // Tenant ID -> Repository
	vectorStores map[string]domain.VectorStore   // Tenant ID -> VectorStore
	llmProviders map[string]*llm.ProviderManager // Tenant ID -> LLM Provider Manager

	mutex sync.RWMutex
}

// NewManager creates a new tenant manager
func NewManager(cfg *config.Config, logger *log.Logger) (*Manager, error) {
	// Load tenant configurations
	tenantsConfig, err := cfg.LoadTenants()
	if err != nil {
		return nil, fmt.Errorf("failed to load tenant configurations: %w", err)
	}

	manager := &Manager{
		config:        cfg,
		tenantsConfig: tenantsConfig,
		logger:        logger,
		tenants:       make(map[string]*domain.Tenant),
		tenantsByID:   make(map[string]*domain.Tenant),
		repositories:  make(map[string]domain.Repository),
		vectorStores:  make(map[string]domain.VectorStore),
		llmProviders:  make(map[string]*llm.ProviderManager),
	}

	// Initialize tenants
	if err := manager.initializeTenants(); err != nil {
		return nil, fmt.Errorf("failed to initialize tenants: %w", err)
	}

	logger.Info().Int("tenants_loaded", len(manager.tenants)).Msg("tenant manager initialized")

	return manager, nil
}

// GetTenant retrieves tenant configuration by WABA number
func (m *Manager) GetTenant(wabaNumber string) (*domain.Tenant, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	tenant, exists := m.tenants[wabaNumber]
	if !exists {
		return nil, fmt.Errorf("tenant not found for WABA number: %s", wabaNumber)
	}

	return tenant, nil
}

// GetTenantByID retrieves tenant configuration by tenant ID
func (m *Manager) GetTenantByID(tenantID string) (*domain.Tenant, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	tenant, exists := m.tenantsByID[tenantID]
	if !exists {
		return nil, fmt.Errorf("tenant not found for ID: %s", tenantID)
	}

	return tenant, nil
}

// ListTenants returns all configured tenants
func (m *Manager) ListTenants() ([]domain.Tenant, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	tenants := make([]domain.Tenant, 0, len(m.tenantsByID))
	for _, tenant := range m.tenantsByID {
		tenants = append(tenants, *tenant)
	}

	return tenants, nil
}

// IsAgentEnabled checks if an agent is enabled for a tenant
func (m *Manager) IsAgentEnabled(tenantID, agentName string) bool {
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

// GetRepository returns a repository instance for the tenant
func (m *Manager) GetRepository(tenantID string) (domain.Repository, error) {
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

	tenant, exists := m.tenantsByID[tenantID]
	if !exists {
		return nil, fmt.Errorf("tenant not found: %s", tenantID)
	}

	// Create database connection
	db, err := pgxpool.New(context.Background(), tenant.DBDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to create database connection for tenant %s: %w", tenantID, err)
	}

	// Test connection
	if err := db.Ping(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database for tenant %s: %w", tenantID, err)
	}

	// Create repository
	repo = repoImpl.NewPostgresRepository(db, m.logger.WithTenant(tenantID))
	m.repositories[tenantID] = repo

	m.logger.Info().Str("tenant_id", tenantID).Msg("repository created for tenant")

	return repo, nil
}

// GetVectorStore returns a vector store instance for the tenant
func (m *Manager) GetVectorStore(tenantID string) (domain.VectorStore, error) {
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
		"db_url": tenant.DBDSN,
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
func (m *Manager) GetLLMProvider(tenantID string) (domain.LLMProvider, error) {
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

	// Get repository to fetch LLM provider config
	repo, err := m.GetRepository(tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}

	// Get default LLM provider config for tenant
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	providerConfig, err := repo.GetDefaultLLMProvider(ctx, tenantID)
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

// initializeTenants initializes all tenants from configuration
func (m *Manager) initializeTenants() error {
	for _, tenantConfig := range m.tenantsConfig.Tenants {
		tenant, err := m.convertConfigToTenant(&tenantConfig)
		if err != nil {
			return fmt.Errorf("failed to convert config for tenant %s: %w", tenantConfig.TenantID, err)
		}

		// Store by WABA number and tenant ID
		m.tenants[tenant.WABANumber] = tenant
		m.tenantsByID[tenant.ID] = tenant

		m.logger.Debug().
			Str("tenant_id", tenant.ID).
			Str("waba_number", tenant.WABANumber).
			Msg("tenant initialized")
	}

	return nil
}

// convertConfigToTenant converts a config.TenantConfig to domain.Tenant
func (m *Manager) convertConfigToTenant(config *config.TenantConfig) (*domain.Tenant, error) {
	// Generate tenant ID if not provided
	tenantID := config.TenantID
	if tenantID == "" {
		tenantID = m.generateTenantID(config.WABANumber)
	}

	// Validate required fields
	if config.WABANumber == "" {
		return nil, fmt.Errorf("WABA number is required")
	}

	if config.DBDSN == "" {
		return nil, fmt.Errorf("database DSN is required")
	}

	// Set defaults
	embeddingModel := config.EmbeddingModel
	if embeddingModel == "" {
		embeddingModel = "text-embedding-ada-002"
	}

	vectorStore := config.VectorStore
	if vectorStore == "" {
		vectorStore = "pgvector"
	}

	enabledAgents := config.EnabledAgents
	if len(enabledAgents) == 0 {
		enabledAgents = []string{"db_agent", "http_agent", "orchestrator"}
	}

	return &domain.Tenant{
		ID:             tenantID,
		WABANumber:     config.WABANumber,
		DBDSN:          config.DBDSN,
		EmbeddingModel: embeddingModel,
		VectorStore:    vectorStore,
		EnabledAgents:  enabledAgents,
		Config:         config.Config,
		Metadata:       config.Metadata,
	}, nil
}

// generateTenantID generates a tenant ID based on WABA number
func (m *Manager) generateTenantID(wabaNumber string) string {
	// Create a hash of the WABA number for consistent tenant ID
	hash := sha256.Sum256([]byte(wabaNumber))
	return fmt.Sprintf("tenant_%x", hash[:8]) // Use first 8 bytes of hash
}

// ReloadTenants reloads tenant configurations from file
func (m *Manager) ReloadTenants() error {
	m.logger.Info().Msg("reloading tenant configurations")

	// Load new configuration
	tenantsConfig, err := m.config.LoadTenants()
	if err != nil {
		return fmt.Errorf("failed to load tenant configurations: %w", err)
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Clear existing tenants
	oldTenants := m.tenants
	oldTenantsById := m.tenantsByID

	m.tenants = make(map[string]*domain.Tenant)
	m.tenantsByID = make(map[string]*domain.Tenant)
	m.tenantsConfig = tenantsConfig

	// Reinitialize tenants
	if err := m.initializeTenants(); err != nil {
		// Restore old state on error
		m.tenants = oldTenants
		m.tenantsByID = oldTenantsById
		return fmt.Errorf("failed to reinitialize tenants: %w", err)
	}

	// Clean up resources for removed tenants
	m.cleanupRemovedTenants(oldTenantsById)

	m.logger.Info().Int("tenants_loaded", len(m.tenants)).Msg("tenant configurations reloaded")

	return nil
}

// cleanupRemovedTenants cleans up resources for tenants that were removed
func (m *Manager) cleanupRemovedTenants(oldTenants map[string]*domain.Tenant) {
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

// Close closes all tenant resources
func (m *Manager) Close() error {
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

	// Clear all caches
	m.repositories = make(map[string]domain.Repository)
	m.vectorStores = make(map[string]domain.VectorStore)
	m.llmProviders = make(map[string]*llm.ProviderManager)

	if len(errors) > 0 {
		return fmt.Errorf("errors occurred while closing tenant resources: %v", errors)
	}

	m.logger.Info().Msg("tenant manager closed")
	return nil
}

// GetTenantStats returns statistics about tenant resource usage
func (m *Manager) GetTenantStats() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return map[string]interface{}{
		"total_tenants":       len(m.tenants),
		"active_repos":        len(m.repositories),
		"active_vectorstores": len(m.vectorStores),
		"active_llm_managers": len(m.llmProviders),
		"tenant_ids":          m.getTenantIDs(),
	}
}

// getTenantIDs returns a list of all tenant IDs
func (m *Manager) getTenantIDs() []string {
	ids := make([]string, 0, len(m.tenantsByID))
	for id := range m.tenantsByID {
		ids = append(ids, id)
	}
	return ids
}

// ValidateConfig validates tenant configuration
func ValidateConfig(config *config.TenantConfig) error {
	if config.WABANumber == "" {
		return fmt.Errorf("WABA number is required")
	}

	if config.DBDSN == "" {
		return fmt.Errorf("database DSN is required")
	}

	// Validate vector store type
	validVectorStores := []string{"pgvector", "sql_fallback"}
	if config.VectorStore != "" {
		valid := false
		for _, vs := range validVectorStores {
			if config.VectorStore == vs {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid vector store type: %s. Valid options: %v", config.VectorStore, validVectorStores)
		}
	}

	return nil
}
