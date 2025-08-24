-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    phone VARCHAR(50) NOT NULL,
    profile JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(tenant_id, phone)
);

-- Create messages table
CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    message_id VARCHAR(255) NOT NULL,
    direction VARCHAR(20) NOT NULL CHECK (direction IN ('inbound', 'outbound')),
    text TEXT NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    token_usage JSONB,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(tenant_id, message_id)
);

-- Create memory_chunks table with vector column
CREATE TABLE memory_chunks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind VARCHAR(50) NOT NULL CHECK (kind IN ('note', 'event', 'task', 'msg')),
    text TEXT NOT NULL,
    embedding vector(1536), -- OpenAI ada-002 embedding size
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create agents table
CREATE TABLE agents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) UNIQUE NOT NULL,
    version VARCHAR(50) NOT NULL DEFAULT '1.0.0',
    allowed_tenants TEXT[] DEFAULT ARRAY[]::TEXT[],
    config JSONB DEFAULT '{}',
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create external_services table
CREATE TABLE external_services (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    base_url TEXT NOT NULL,
    auth JSONB DEFAULT '{}',
    config JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(tenant_id, name)
);

-- Create llm_providers table
CREATE TABLE llm_providers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    provider VARCHAR(50) NOT NULL CHECK (provider IN ('openai', 'deepseek', 'anthropic', 'mock')),
    name VARCHAR(255) NOT NULL,
    api_key TEXT NOT NULL,
    base_url TEXT,
    model_chat VARCHAR(255) NOT NULL,
    model_embed VARCHAR(255),
    config JSONB DEFAULT '{}',
    is_default BOOLEAN DEFAULT FALSE,
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(tenant_id, name)
);

-- Create indexes for better performance
CREATE INDEX idx_users_tenant_phone ON users(tenant_id, phone);
CREATE INDEX idx_messages_tenant_user ON messages(tenant_id, user_id);
CREATE INDEX idx_messages_timestamp ON messages(timestamp DESC);
CREATE INDEX idx_memory_chunks_tenant_user ON memory_chunks(tenant_id, user_id);
CREATE INDEX idx_memory_chunks_kind ON memory_chunks(kind);
CREATE INDEX idx_memory_chunks_created ON memory_chunks(created_at DESC);

-- Create vector similarity index using IVFFlat
CREATE INDEX idx_memory_chunks_embedding ON memory_chunks 
USING ivfflat (embedding vector_cosine_ops) 
WITH (lists = 100);

-- Create GIN indexes for JSONB metadata searches
CREATE INDEX idx_memory_chunks_metadata ON memory_chunks USING GIN (metadata);
CREATE INDEX idx_messages_metadata ON messages USING GIN (metadata);
CREATE INDEX idx_users_profile ON users USING GIN (profile);
CREATE INDEX idx_external_services_tenant ON external_services(tenant_id);
CREATE INDEX idx_llm_providers_tenant ON llm_providers(tenant_id);
CREATE INDEX idx_llm_providers_tenant_default ON llm_providers(tenant_id, is_default) WHERE is_default = TRUE;

-- Add text search capabilities as fallback
ALTER TABLE memory_chunks ADD COLUMN text_search tsvector;
CREATE INDEX idx_memory_chunks_text_search ON memory_chunks USING GIN (text_search);

-- Create trigger to automatically update text_search column
CREATE OR REPLACE FUNCTION update_memory_chunks_text_search()
RETURNS TRIGGER AS $$
BEGIN
    NEW.text_search = to_tsvector('english', COALESCE(NEW.text, ''));
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_memory_chunks_text_search
    BEFORE INSERT OR UPDATE ON memory_chunks
    FOR EACH ROW
    EXECUTE FUNCTION update_memory_chunks_text_search();

-- Create trigger to update updated_at timestamps
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trigger_agents_updated_at
    BEFORE UPDATE ON agents
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trigger_external_services_updated_at
    BEFORE UPDATE ON external_services
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trigger_llm_providers_updated_at
    BEFORE UPDATE ON llm_providers
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Insert default agents
INSERT INTO agents (name, version, allowed_tenants, config, enabled) VALUES
('db_agent', '1.0.0', ARRAY[]::TEXT[], '{"description": "Database operations agent for managing user memory"}', TRUE),
('http_agent', '1.0.0', ARRAY[]::TEXT[], '{"description": "HTTP client agent for external API calls"}', TRUE),
('orchestrator', '1.0.0', ARRAY[]::TEXT[], '{"description": "Main orchestrator agent"}', TRUE);