package repo_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"personal-assistant/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRepository implements domain.Repository for testing
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) CreateUser(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockRepository) GetUser(ctx context.Context, tenantID, phone string) (*domain.User, error) {
	args := m.Called(ctx, tenantID, phone)
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockRepository) GetUserByID(ctx context.Context, tenantID string, userID uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, tenantID, userID)
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockRepository) UpdateUser(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockRepository) CreateMessage(ctx context.Context, message *domain.Message) error {
	args := m.Called(ctx, message)
	return args.Error(0)
}

func (m *MockRepository) GetMessages(ctx context.Context, tenantID string, userID uuid.UUID, limit int) ([]domain.Message, error) {
	args := m.Called(ctx, tenantID, userID, limit)
	return args.Get(0).([]domain.Message), args.Error(1)
}

func (m *MockRepository) GetMessageByID(ctx context.Context, tenantID string, messageID string) (*domain.Message, error) {
	args := m.Called(ctx, tenantID, messageID)
	return args.Get(0).(*domain.Message), args.Error(1)
}

func (m *MockRepository) GetAgents(ctx context.Context) ([]domain.AgentConfig, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domain.AgentConfig), args.Error(1)
}

func (m *MockRepository) GetAgentByName(ctx context.Context, name string) (*domain.AgentConfig, error) {
	args := m.Called(ctx, name)
	return args.Get(0).(*domain.AgentConfig), args.Error(1)
}

func (m *MockRepository) CreateAgent(ctx context.Context, agent *domain.AgentConfig) error {
	args := m.Called(ctx, agent)
	return args.Error(0)
}

func (m *MockRepository) UpdateAgent(ctx context.Context, agent *domain.AgentConfig) error {
	args := m.Called(ctx, agent)
	return args.Error(0)
}

func (m *MockRepository) GetExternalServices(ctx context.Context, tenantID string) ([]domain.ExternalService, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).([]domain.ExternalService), args.Error(1)
}

func (m *MockRepository) GetExternalService(ctx context.Context, tenantID, name string) (*domain.ExternalService, error) {
	args := m.Called(ctx, tenantID, name)
	return args.Get(0).(*domain.ExternalService), args.Error(1)
}

func (m *MockRepository) CreateExternalService(ctx context.Context, service *domain.ExternalService) error {
	args := m.Called(ctx, service)
	return args.Error(0)
}

func (m *MockRepository) UpdateExternalService(ctx context.Context, service *domain.ExternalService) error {
	args := m.Called(ctx, service)
	return args.Error(0)
}

func (m *MockRepository) GetLLMProviders(ctx context.Context, tenantID string) ([]domain.LLMProviderConfig, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).([]domain.LLMProviderConfig), args.Error(1)
}

func (m *MockRepository) GetLLMProvider(ctx context.Context, tenantID, name string) (*domain.LLMProviderConfig, error) {
	args := m.Called(ctx, tenantID, name)
	return args.Get(0).(*domain.LLMProviderConfig), args.Error(1)
}

func (m *MockRepository) GetDefaultLLMProvider(ctx context.Context, tenantID string) (*domain.LLMProviderConfig, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).(*domain.LLMProviderConfig), args.Error(1)
}

func (m *MockRepository) CreateLLMProvider(ctx context.Context, config *domain.LLMProviderConfig) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

func (m *MockRepository) UpdateLLMProvider(ctx context.Context, config *domain.LLMProviderConfig) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

func (m *MockRepository) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockRepository) Close() error {
	args := m.Called()
	return args.Error(0)
}

// Tenant configuration operations
func (m *MockRepository) GetTenantsConfig(ctx context.Context) ([]domain.TenantConfig, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domain.TenantConfig), args.Error(1)
}

func (m *MockRepository) GetTenantConfig(ctx context.Context, tenantID string) (*domain.TenantConfig, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).(*domain.TenantConfig), args.Error(1)
}

func (m *MockRepository) GetTenantConfigByWABA(ctx context.Context, wabaNumber string) (*domain.TenantConfig, error) {
	args := m.Called(ctx, wabaNumber)
	return args.Get(0).(*domain.TenantConfig), args.Error(1)
}

func (m *MockRepository) CreateTenantConfig(ctx context.Context, config *domain.TenantConfig) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

func (m *MockRepository) UpdateTenantConfig(ctx context.Context, config *domain.TenantConfig) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

// System configuration operations
func (m *MockRepository) GetSystemConfig(ctx context.Context, key string) (*domain.SystemConfig, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(*domain.SystemConfig), args.Error(1)
}

func (m *MockRepository) SetSystemConfig(ctx context.Context, key string, value any, description string) error {
	args := m.Called(ctx, key, value, description)
	return args.Error(0)
}

// Allowed contacts operations
func (m *MockRepository) GetAllowedContacts(ctx context.Context, tenantID string) ([]domain.AllowedContact, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).([]domain.AllowedContact), args.Error(1)
}

func (m *MockRepository) GetAllowedContact(ctx context.Context, tenantID, phoneNumber string) (*domain.AllowedContact, error) {
	args := m.Called(ctx, tenantID, phoneNumber)
	return args.Get(0).(*domain.AllowedContact), args.Error(1)
}

func (m *MockRepository) CreateAllowedContact(ctx context.Context, contact *domain.AllowedContact) error {
	args := m.Called(ctx, contact)
	return args.Error(0)
}

func (m *MockRepository) UpdateAllowedContact(ctx context.Context, contact *domain.AllowedContact) error {
	args := m.Called(ctx, contact)
	return args.Error(0)
}

func (m *MockRepository) DeleteAllowedContact(ctx context.Context, tenantID string, contactID uuid.UUID) error {
	args := m.Called(ctx, tenantID, contactID)
	return args.Error(0)
}

func (m *MockRepository) IsContactAllowed(ctx context.Context, tenantID, phoneNumber string) (bool, error) {
	args := m.Called(ctx, tenantID, phoneNumber)
	return args.Bool(0), args.Error(1)
}

// Test Repository Interface Compliance
func TestRepositoryInterface(t *testing.T) {
	t.Run("mock repository implements interface", func(t *testing.T) {
		var _ domain.Repository = (*MockRepository)(nil)
	})
}

// Test User Operations
func TestUserOperations(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockRepository)

	t.Run("create user", func(t *testing.T) {
		user := &domain.User{
			ID:       uuid.New(),
			TenantID: "test-tenant",
			Phone:    "+1234567890",
			Profile:  map[string]any{"name": "John"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		mockRepo.On("CreateUser", ctx, user).Return(nil)

		err := mockRepo.CreateUser(ctx, user)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("get user by phone", func(t *testing.T) {
		expectedUser := &domain.User{
			ID:       uuid.New(),
			TenantID: "test-tenant",
			Phone:    "+1234567890",
		}

		mockRepo.On("GetUser", ctx, "test-tenant", "+1234567890").Return(expectedUser, nil)

		user, err := mockRepo.GetUser(ctx, "test-tenant", "+1234567890")
		assert.NoError(t, err)
		assert.Equal(t, expectedUser.Phone, user.Phone)
		mockRepo.AssertExpectations(t)
	})
}

// Test Message Operations
func TestMessageOperations(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockRepository)

	t.Run("create message", func(t *testing.T) {
		message := &domain.Message{
			ID:        uuid.New(),
			TenantID:  "test-tenant",
			UserID:    uuid.New(),
			MessageID: "infobip-123",
			Direction: "inbound",
			Text:      "Hello world",
			Timestamp: time.Now(),
			CreatedAt: time.Now(),
		}

		mockRepo.On("CreateMessage", ctx, message).Return(nil)

		err := mockRepo.CreateMessage(ctx, message)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("get messages", func(t *testing.T) {
		userID := uuid.New()
		expectedMessages := []domain.Message{
			{
				ID:        uuid.New(),
				TenantID:  "test-tenant",
				UserID:    userID,
				Text:      "Message 1",
				Direction: "inbound",
			},
			{
				ID:        uuid.New(),
				TenantID:  "test-tenant",
				UserID:    userID,
				Text:      "Message 2",
				Direction: "outbound",
			},
		}

		mockRepo.On("GetMessages", ctx, "test-tenant", userID, 10).Return(expectedMessages, nil)

		messages, err := mockRepo.GetMessages(ctx, "test-tenant", userID, 10)
		assert.NoError(t, err)
		assert.Len(t, messages, 2)
		assert.Equal(t, "Message 1", messages[0].Text)
		assert.Equal(t, "Message 2", messages[1].Text)
		mockRepo.AssertExpectations(t)
	})
}

// Test Agent Operations
func TestAgentOperations(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockRepository)

	t.Run("get agents", func(t *testing.T) {
		expectedAgents := []domain.AgentConfig{
			{
				ID:             uuid.New(),
				Name:           "agent1",
				Version:        "1.0.0",
				AllowedTenants: []string{"tenant1"},
				Enabled:        true,
			},
			{
				ID:             uuid.New(),
				Name:           "agent2",
				Version:        "1.0.0",
				AllowedTenants: []string{"tenant1", "tenant2"},
				Enabled:        false,
			},
		}

		mockRepo.On("GetAgents", ctx).Return(expectedAgents, nil)

		agents, err := mockRepo.GetAgents(ctx)
		assert.NoError(t, err)
		assert.Len(t, agents, 2)
		assert.Equal(t, "agent1", agents[0].Name)
		assert.True(t, agents[0].Enabled)
		assert.False(t, agents[1].Enabled)
		mockRepo.AssertExpectations(t)
	})

	t.Run("get agent by name", func(t *testing.T) {
		expectedAgent := &domain.AgentConfig{
			ID:      uuid.New(),
			Name:    "test-agent",
			Version: "1.0.0",
			Enabled: true,
		}

		mockRepo.On("GetAgentByName", ctx, "test-agent").Return(expectedAgent, nil)

		agent, err := mockRepo.GetAgentByName(ctx, "test-agent")
		assert.NoError(t, err)
		assert.Equal(t, "test-agent", agent.Name)
		assert.True(t, agent.Enabled)
		mockRepo.AssertExpectations(t)
	})
}

// Test Allowed Contacts Operations
func TestAllowedContactsOperations(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockRepository)

	t.Run("create allowed contact", func(t *testing.T) {
		contact := &domain.AllowedContact{
			ID:          uuid.New(),
			TenantID:    "test-tenant",
			PhoneNumber: "+1234567890",
			ContactName: "John Doe",
			Permissions: []string{"chat", "schedule"},
			Notes:       "Test user",
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		mockRepo.On("CreateAllowedContact", ctx, contact).Return(nil)

		err := mockRepo.CreateAllowedContact(ctx, contact)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("get allowed contacts", func(t *testing.T) {
		expectedContacts := []domain.AllowedContact{
			{
				ID:          uuid.New(),
				TenantID:    "test-tenant",
				PhoneNumber: "+1234567890",
				ContactName: "John Doe",
				Permissions: []string{"chat", "schedule"},
				Enabled:     true,
			},
			{
				ID:          uuid.New(),
				TenantID:    "test-tenant",
				PhoneNumber: "+0987654321",
				ContactName: "Jane Smith",
				Permissions: []string{"chat"},
				Enabled:     true,
			},
		}

		mockRepo.On("GetAllowedContacts", ctx, "test-tenant").Return(expectedContacts, nil)

		contacts, err := mockRepo.GetAllowedContacts(ctx, "test-tenant")
		assert.NoError(t, err)
		assert.Len(t, contacts, 2)
		assert.Equal(t, "+1234567890", contacts[0].PhoneNumber)
		assert.Equal(t, "John Doe", contacts[0].ContactName)
		assert.Contains(t, contacts[0].Permissions, "chat")
		assert.Contains(t, contacts[0].Permissions, "schedule")
		mockRepo.AssertExpectations(t)
	})

	t.Run("get allowed contact by phone", func(t *testing.T) {
		expectedContact := &domain.AllowedContact{
			ID:          uuid.New(),
			TenantID:    "test-tenant",
			PhoneNumber: "+1234567890",
			ContactName: "John Doe",
			Permissions: []string{"chat", "schedule", "admin"},
			Enabled:     true,
		}

		mockRepo.On("GetAllowedContact", ctx, "test-tenant", "+1234567890").Return(expectedContact, nil)

		contact, err := mockRepo.GetAllowedContact(ctx, "test-tenant", "+1234567890")
		assert.NoError(t, err)
		assert.Equal(t, "+1234567890", contact.PhoneNumber)
		assert.Equal(t, "John Doe", contact.ContactName)
		assert.Contains(t, contact.Permissions, "admin")
		assert.True(t, contact.Enabled)
		mockRepo.AssertExpectations(t)
	})

	t.Run("is contact allowed", func(t *testing.T) {
		// Test allowed contact
		mockRepo.On("IsContactAllowed", ctx, "test-tenant", "+1234567890").Return(true, nil)

		isAllowed, err := mockRepo.IsContactAllowed(ctx, "test-tenant", "+1234567890")
		assert.NoError(t, err)
		assert.True(t, isAllowed)

		// Test unauthorized contact
		mockRepo.On("IsContactAllowed", ctx, "test-tenant", "+9999999999").Return(false, nil)

		isAllowed, err = mockRepo.IsContactAllowed(ctx, "test-tenant", "+9999999999")
		assert.NoError(t, err)
		assert.False(t, isAllowed)

		mockRepo.AssertExpectations(t)
	})

	t.Run("update allowed contact", func(t *testing.T) {
		contact := &domain.AllowedContact{
			ID:          uuid.New(),
			TenantID:    "test-tenant",
			PhoneNumber: "+1234567890",
			ContactName: "John Smith", // Updated name
			Permissions: []string{"chat", "schedule", "admin"}, // Updated permissions
			Enabled:     true,
			UpdatedAt:   time.Now(),
		}

		mockRepo.On("UpdateAllowedContact", ctx, contact).Return(nil)

		err := mockRepo.UpdateAllowedContact(ctx, contact)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("delete allowed contact", func(t *testing.T) {
		contactID := uuid.New()
		mockRepo.On("DeleteAllowedContact", ctx, "test-tenant", contactID).Return(nil)

		err := mockRepo.DeleteAllowedContact(ctx, "test-tenant", contactID)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})
}