-- Rollback migration for allowed contacts

-- Remove system config entries
DELETE FROM system_config 
WHERE key IN (
    'require_contact_allowlist', 
    'default_contact_permissions', 
    'unknown_sender_action'
);

-- Drop the allowed_contacts table (CASCADE will remove indexes, triggers, and policies)
DROP TABLE IF EXISTS allowed_contacts CASCADE;