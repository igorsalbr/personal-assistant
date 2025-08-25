-- Migration to add allowed contacts for tenant security
-- Each tenant can specify which WhatsApp numbers are allowed to interact with their bot

-- Create allowed_contacts table
CREATE TABLE allowed_contacts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL REFERENCES tenants_config(tenant_id) ON DELETE CASCADE,
    phone_number VARCHAR(50) NOT NULL,
    contact_name VARCHAR(255),
    permissions TEXT[] DEFAULT ARRAY['chat', 'schedule']::TEXT[], -- What this contact can do
    notes TEXT,
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Ensure unique phone number per tenant (same person can be in multiple tenants)
    UNIQUE(tenant_id, phone_number)
);

-- Create indexes for performance
CREATE INDEX idx_allowed_contacts_tenant_id ON allowed_contacts(tenant_id);
CREATE INDEX idx_allowed_contacts_phone_number ON allowed_contacts(phone_number);
CREATE INDEX idx_allowed_contacts_enabled ON allowed_contacts(enabled) WHERE enabled = TRUE;
CREATE INDEX idx_allowed_contacts_tenant_phone ON allowed_contacts(tenant_id, phone_number) WHERE enabled = TRUE;

-- Add trigger to update updated_at
CREATE TRIGGER trigger_allowed_contacts_updated_at
    BEFORE UPDATE ON allowed_contacts
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Enable Row Level Security
ALTER TABLE allowed_contacts ENABLE ROW LEVEL SECURITY;

-- Create RLS policy for tenant isolation
CREATE POLICY allowed_contacts_tenant_isolation ON allowed_contacts
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant', true));

-- Insert some example data (you can remove this in production)
-- This assumes you have a tenant with tenant_id 'demo_tenant'
INSERT INTO allowed_contacts (tenant_id, phone_number, contact_name, permissions, notes) VALUES 
(
    'test_tenant_001', 
    '+1234567890', 
    'Demo User', 
    ARRAY['chat', 'schedule', 'admin']::TEXT[], 
    'Main user with full permissions'
);

-- Add system config for security settings
INSERT INTO system_config (key, value, description) VALUES 
(
    'require_contact_allowlist', 
    'true', 
    'Whether to require senders to be in allowed_contacts table'
),
(
    'default_contact_permissions', 
    '["chat", "schedule"]', 
    'Default permissions for new allowed contacts'
),
(
    'unknown_sender_action', 
    '"ignore"', 
    'Action to take for unknown senders: ignore, notify_admin, or auto_add'
);