# Unified Database Architecture

## Overview

Implementação de uma arquitetura de banco unificado com isolamento por tenant usando Row-Level Security (RLS) do PostgreSQL.

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

A arquitetura resolve completamente o "exagero" dos múltiplos DBs mantendo isolamento perfeito! 🚀