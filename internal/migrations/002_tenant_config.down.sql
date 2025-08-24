-- Rollback migration for tenant configuration tables

-- Drop RLS policies
DROP POLICY IF EXISTS tenant_config_isolation ON tenants_config;
DROP POLICY IF EXISTS users_tenant_isolation ON users;
DROP POLICY IF EXISTS messages_tenant_isolation ON messages;
DROP POLICY IF EXISTS memory_chunks_tenant_isolation ON memory_chunks;
DROP POLICY IF EXISTS external_services_tenant_isolation ON external_services;
DROP POLICY IF EXISTS llm_providers_tenant_isolation ON llm_providers;

-- Disable Row Level Security
ALTER TABLE IF EXISTS tenants_config DISABLE ROW LEVEL SECURITY;
ALTER TABLE users DISABLE ROW LEVEL SECURITY;
ALTER TABLE messages DISABLE ROW LEVEL SECURITY;
ALTER TABLE memory_chunks DISABLE ROW LEVEL SECURITY;
ALTER TABLE external_services DISABLE ROW LEVEL SECURITY;
ALTER TABLE llm_providers DISABLE ROW LEVEL SECURITY;

-- Drop functions
DROP FUNCTION IF EXISTS set_tenant_context(TEXT);
DROP FUNCTION IF EXISTS clear_tenant_context();
DROP FUNCTION IF EXISTS get_current_tenant();

-- Drop triggers
DROP TRIGGER IF EXISTS trigger_tenants_config_updated_at ON tenants_config;
DROP TRIGGER IF EXISTS trigger_system_config_updated_at ON system_config;

-- Drop indexes
DROP INDEX IF EXISTS idx_tenants_config_tenant_id;
DROP INDEX IF EXISTS idx_tenants_config_waba_number;
DROP INDEX IF EXISTS idx_tenants_config_enabled;
DROP INDEX IF EXISTS idx_tenants_config_config;
DROP INDEX IF EXISTS idx_tenants_config_metadata;
DROP INDEX IF EXISTS idx_system_config_key;
DROP INDEX IF EXISTS idx_system_config_value;

-- Drop tables
DROP TABLE IF EXISTS system_config;
DROP TABLE IF EXISTS tenants_config;