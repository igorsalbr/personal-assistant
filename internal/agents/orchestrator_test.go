package agents_test

import (
	"context"
	"personal-assistant/internal/domain"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAgent implements domain.Agent for testing
type MockAgent struct {
	NameValue string
	Tenants   []string
}

func (a *MockAgent) Handle(_ context.Context, _ *domain.AgentRequest) (*domain.AgentResponse, error) {
	return &domain.AgentResponse{Text: "mock"}, nil
}

func (a *MockAgent) CanHandle(_ string) bool {
	return true
}

func (a *MockAgent) Name() string {
	return a.NameValue
}

func (a *MockAgent) AllowedTenants() []string {
	return a.Tenants
}

type MockAgentRegistry struct {
	mock.Mock
	agents map[string]domain.Agent
}

func (m *MockAgentRegistry) RegisterAgent(agent domain.Agent) error {
	if m.agents == nil {
		m.agents = make(map[string]domain.Agent)
	}
	m.agents[agent.Name()] = agent
	return nil
}

func (m *MockAgentRegistry) GetAgent(name string) (domain.Agent, error) {
	if m.agents == nil {
		return nil, assert.AnError
	}
	agent, ok := m.agents[name]
	if !ok {
		return nil, assert.AnError
	}
	return agent, nil
}

func (m *MockAgentRegistry) ListAgents() []domain.Agent                        { return nil }
func (m *MockAgentRegistry) GetAgentsForTenant(tenantID string) []domain.Agent { return nil }

func TestAgentRegistry_RegisterAndGetAgent(t *testing.T) {
	reg := &MockAgentRegistry{}
	agent := &MockAgent{NameValue: "test-agent"}
	err := reg.RegisterAgent(agent)
	assert.NoError(t, err)
	got, err := reg.GetAgent("test-agent")
	assert.NoError(t, err)
	assert.Equal(t, agent, got)
}

func TestAgentRegistry_GetAgent_NotFound(t *testing.T) {
	reg := &MockAgentRegistry{}
	_, err := reg.GetAgent("missing-agent")
	assert.Error(t, err)
}
