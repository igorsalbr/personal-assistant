# Unified Database Architecture

## Overview

Implementação de uma arquitetura de banco unificado com isolamento por tenant usando Row-Level Security (RLS) do PostgreSQL.


1. memory_chunks - Core RAG Memory System

  Purpose: Stores user memories with vector embeddings for
  semantic search
  - id: UUID primary key
  - tenant_id: Multi-tenant isolation
  - user_id: References users table
  - kind: Type of memory ('note', 'event', 'task', 'msg')
  - text: The actual memory content
  - embedding: Vector embeddings (1536 dimensions for OpenAI
  ada-002)
  - metadata: JSONB for additional context
  - text_search: Auto-generated tsvector for full-text search
  fallback

  Usage:
  - Stores conversation history, user preferences, events, tasks
  - Enables semantic search using pgvector for retrieving relevant
   context
  - Has both vector similarity search and text search fallback
  - Used by pgvector.go:48-114 for upserting memories and
  pgvector.go:117-224 for similarity search

  2. users - User Management

  Purpose: Tracks WhatsApp users per tenant
  - id: UUID primary key
  - tenant_id: Tenant isolation
  - phone: WhatsApp phone number
  - profile: JSONB for user preferences/data
  - created_at/updated_at: Timestamps

  Usage: Maps WhatsApp phone numbers to internal user IDs for
  memory association

  3. messages - Conversation History

  Purpose: Stores all WhatsApp message exchanges
  - id: UUID primary key
  - tenant_id: Tenant isolation
  - user_id: Links to users table
  - message_id: External message ID from WhatsApp/Infobip
  - direction: 'inbound' or 'outbound'
  - text: Message content
  - timestamp: Message timestamp
  - token_usage: JSONB tracking LLM API usage
  - metadata: JSONB for additional context

  Usage: Full conversation history, token tracking, audit trail

  4. tenants_config - Multi-Tenant Configuration

  Purpose: Database-stored tenant configurations (replaces YAML
  files)
  - tenant_id: Unique tenant identifier
  - waba_number: WhatsApp Business number
  - embedding_model: Which embedding model to use
  - vector_store: 'pgvector' or 'sql_fallback'
  - enabled_agents: Array of enabled agent names
  - config: JSONB tenant-specific settings
  - enabled: Active status

  Usage: Manages different WhatsApp business accounts with
  isolated configurations

  5. system_config - Global Settings

  Purpose: System-wide configuration key-value store
  - key: Setting name (e.g., 'default_embedding_model')
  - value: JSONB setting value
  - description: Human-readable description

  Usage: Global defaults, feature flags, system-wide settings

  6. llm_providers - LLM Configuration per Tenant

  Purpose: Per-tenant LLM provider settings
  - tenant_id: Tenant isolation
  - provider: 'openai', 'deepseek', 'anthropic', 'mock'
  - name: Provider instance name
  - api_key: Encrypted API key
  - model_chat/model_embed: Model names
  - is_default: Default provider for tenant
  - config: JSONB provider-specific settings

  Usage: Allows different tenants to use different LLM providers
  and models

  7. agents - System Agent Registry

  Purpose: Available system agents and their configurations
  - name: Agent identifier ('db_agent', 'http_agent',
  'orchestrator')
  - version: Agent version
  - allowed_tenants: Array of tenant IDs that can use this agent
  - config: JSONB agent configuration
  - enabled: Active status

  Usage: Manages which agents are available and their permissions

  8. external_services - API Integration Configuration

  Purpose: Per-tenant external API configurations
  - tenant_id: Tenant isolation
  - name: Service name
  - base_url: API endpoint
  - auth: JSONB authentication config
  - config: JSONB service-specific settings

  Usage: Allows tenants to configure external APIs (weather,
  calendar, etc.)

  9. allowed_contacts - Security Access Control

  Purpose: WhatsApp contact whitelist per tenant
  - tenant_id: Tenant isolation
  - phone_number: Allowed WhatsApp number
  - contact_name: Display name
  - permissions: Array of allowed actions ('chat', 'schedule',
  'admin')
  - notes: Additional context

  Usage: Controls which WhatsApp numbers can interact with each
  tenant's bot

## Estrutura do Banco Único

### Schema Principal
```sql
-- Configurações dos tenants (substitui tenants.yaml)
tenants_config (
    tenant_id, waba_number, embedding_model, vector_store,
    enabled_agents, config, metadata, enabled
)

-- Configurações globais do sistema
system_config (
    key, value, description
)

-- Dados isolados por tenant com RLS
users (tenant_id, phone, profile, ...)
messages (tenant_id, user_id, text, ...)  
memory_chunks (tenant_id, user_id, text, embedding, ...)
llm_providers (tenant_id, provider, api_key, model_chat, ...)
external_services (tenant_id, name, base_url, auth, ...)
```

### Row-Level Security (RLS)
Cada tabela tenant-specific tem uma policy:
```sql
CREATE POLICY tenant_isolation ON users
    FOR ALL  
    USING (tenant_id = current_setting('app.current_tenant', true));
```

## Gerenciamento de Tenants

### Factory Pattern
```go
// Auto-seleciona entre YAML ou Database manager
tenantManager := tenant.NewTenantManager(cfg, logger)

// Ou força um específico:
dbManager := tenant.NewDatabaseManager(cfg, logger)    // DB-first
yamlManager := tenant.NewManager(cfg, logger)          // YAML-first
```

### Configuração via Environment
```bash
# Controla qual manager usar
TENANT_CONFIG_SOURCE=database  # Força database manager
TENANT_CONFIG_SOURCE=yaml      # Força YAML manager
# (omitir = auto-detect)

# Database central
DATABASE_URL_DEFAULT="postgres://user:pass@host/unified_db"
```

## Isolamento por Tenant

### Context Management  
Cada operação automaticamente seta o contexto do tenant:
```go
// TenantRepository wrapper
func (r *TenantRepository) GetUser(ctx, tenantID, phone) (*User, error) {
    r.setTenantContext(ctx)  // SET app.current_tenant = 'tenant_id'
    return r.repo.GetUser(ctx, tenantID, phone)  // RLS filtra automaticamente
}
```

### Agent MCP Isolation
- Cada agent só vê dados do seu tenant
- RLS garante isolamento automático
- Sem necessidade de WHERE tenant_id = ? no código

## Configuração de Tenants

### Via Database (Recomendado)
```sql
-- Adicionar novo tenant
INSERT INTO tenants_config (tenant_id, waba_number, config, ...) VALUES (...);

-- Configurar LLM provider para o tenant  
INSERT INTO llm_providers (tenant_id, provider, api_key, ...) VALUES (...);

-- Configurações globais
INSERT INTO system_config (key, value) VALUES ('default_model', '"gpt-4"');
```

### Via YAML (Fallback)
Mantém compatibilidade com `tenants.yaml` existente para transição gradual.

## Vantagens da Arquitetura

1. **Resource Efficiency**: 1 DB vs N DBs por tenant
2. **Simplified Operations**: 1 backup, 1 schema, 1 conexão pool
3. **Native Isolation**: PostgreSQL RLS é mais robusto que filtragem manual
4. **Centralized Config**: Todas configurações no banco
5. **Better Scalability**: Connection pooling mais eficiente

## Implementation Files

### Core Architecture
- `internal/tenant/factory.go` - Manager selection logic
- `internal/tenant/manager_db.go` - Database-first manager
- `internal/tenant/tenant_repository.go` - RLS wrapper
- `internal/domain/models.go` - New models (TenantConfig, SystemConfig)

### Database Schema
- `internal/migrations/002_tenant_config.up.sql` - Schema migration
- `internal/migrations/002_tenant_config.down.sql` - Rollback migration

### Repository Updates  
- `internal/repo/sqlrepo.go` - Added new tenant config methods
- `internal/domain/interfaces.go` - Added TenantManager.Close() method

## Usage

```go
// Initialize tenant manager (auto-select)
manager, err := tenant.NewTenantManager(cfg, logger)

// Get isolated repository for tenant
repo, err := manager.GetRepository("tenant_123")

// All operations automatically filtered by RLS
users, err := repo.GetMessages(ctx, "tenant_123", userID, 10)
// ^^ Only returns messages for tenant_123
```