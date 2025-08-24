-- Migration to add tenant configuration tables
-- This replaces the YAML-based tenant configuration with database tables

-- Create tenants configuration table
CREATE TABLE tenants_config (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) UNIQUE NOT NULL,
    waba_number VARCHAR(50) UNIQUE NOT NULL,
    embedding_model VARCHAR(255) NOT NULL DEFAULT 'text-embedding-ada-002',
    vector_store VARCHAR(50) NOT NULL DEFAULT 'pgvector' 
        CHECK (vector_store IN ('pgvector', 'sql_fallback')),
    enabled_agents TEXT[] DEFAULT ARRAY['db_agent', 'http_agent', 'orchestrator']::TEXT[],
    config JSONB DEFAULT '{}',
    metadata JSONB DEFAULT '{}',
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create system configuration table for global settings
CREATE TABLE system_config (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    key VARCHAR(255) UNIQUE NOT NULL,
    value JSONB NOT NULL,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Add tenant_id columns to existing tables for better isolation tracking
-- (Most tables already have tenant_id, this ensures consistency)

-- Create indexes for performance
CREATE INDEX idx_tenants_config_tenant_id ON tenants_config(tenant_id);
CREATE INDEX idx_tenants_config_waba_number ON tenants_config(waba_number);
CREATE INDEX idx_tenants_config_enabled ON tenants_config(enabled) WHERE enabled = TRUE;
CREATE INDEX idx_system_config_key ON system_config(key);

-- Create GIN indexes for JSONB columns
CREATE INDEX idx_tenants_config_config ON tenants_config USING GIN (config);
CREATE INDEX idx_tenants_config_metadata ON tenants_config USING GIN (metadata);
CREATE INDEX idx_system_config_value ON system_config USING GIN (value);

-- Add trigger to update updated_at for tenants_config
CREATE TRIGGER trigger_tenants_config_updated_at
    BEFORE UPDATE ON tenants_config
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Add trigger to update updated_at for system_config
CREATE TRIGGER trigger_system_config_updated_at
    BEFORE UPDATE ON system_config
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Enable Row Level Security on tenant-specific tables
ALTER TABLE tenants_config ENABLE ROW LEVEL SECURITY;
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE memory_chunks ENABLE ROW LEVEL SECURITY;
ALTER TABLE external_services ENABLE ROW LEVEL SECURITY;
ALTER TABLE llm_providers ENABLE ROW LEVEL SECURITY;

-- Create RLS policies for tenant isolation
-- Tenants can only access their own data

-- Policy for tenants_config - each tenant can only see their own config
CREATE POLICY tenant_config_isolation ON tenants_config
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant', true));

-- Policy for users - tenant isolation
CREATE POLICY users_tenant_isolation ON users
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant', true));

-- Policy for messages - tenant isolation
CREATE POLICY messages_tenant_isolation ON messages
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant', true));

-- Policy for memory_chunks - tenant isolation
CREATE POLICY memory_chunks_tenant_isolation ON memory_chunks
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant', true));

-- Policy for external_services - tenant isolation
CREATE POLICY external_services_tenant_isolation ON external_services
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant', true));

-- Policy for llm_providers - tenant isolation
CREATE POLICY llm_providers_tenant_isolation ON llm_providers
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant', true));

-- Insert default system configuration
INSERT INTO system_config (key, value, description) VALUES 
(
    'database_migration_version', 
    '"002"', 
    'Current database migration version'
),
(
    'tenant_isolation_enabled', 
    'true', 
    'Whether tenant isolation via RLS is enabled'
),
(
    'default_embedding_model', 
    '"text-embedding-ada-002"', 
    'Default embedding model for new tenants'
),
(
    'supported_vector_stores', 
    '["pgvector", "sql_fallback"]', 
    'List of supported vector store types'
),
(
    'available_agents', 
    '["db_agent", "http_agent", "orchestrator"]', 
    'List of available system agents'
);

-- Create function to set tenant context for RLS
CREATE OR REPLACE FUNCTION set_tenant_context(tenant_id_param TEXT)
RETURNS void AS $$
BEGIN
    PERFORM set_config('app.current_tenant', tenant_id_param, false);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Create function to clear tenant context
CREATE OR REPLACE FUNCTION clear_tenant_context()
RETURNS void AS $$
BEGIN
    PERFORM set_config('app.current_tenant', '', false);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Create function to get current tenant context
CREATE OR REPLACE FUNCTION get_current_tenant()
RETURNS TEXT AS $$
BEGIN
    RETURN current_setting('app.current_tenant', true);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;