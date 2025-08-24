# Quick Setup Guide: DeepSeek + Local PostgreSQL

This guide shows how to quickly set up the WhatsApp bot for testing with DeepSeek LLM and local PostgreSQL (no vector database needed).

## Prerequisites

- PostgreSQL running locally
- DeepSeek API key
- WhatsApp Business API number (we'll use +1234567890 as example)

## Step 1: Environment Configuration

Create your `.env` file:

```bash
# Server Configuration
APP_PORT=8080
APP_BASE_URL=http://localhost:8080
LOG_LEVEL=debug

# DeepSeek LLM Configuration
LLM_PROVIDER=deepseek
LLM_API_KEY=your_deepseek_api_key_here
LLM_MODEL_CHAT=deepseek-chat
LLM_MODEL_EMBED=deepseek-embed

# Vector Store Configuration - Use SQL fallback (no pgvector needed)
VECTOR_BACKEND=sql_fallback

# Database Configuration
DATABASE_URL_DEFAULT=postgres://user:password@localhost/whatsapp_bot_test?sslmode=disable

# Infobip Configuration
INFOBIP_BASE_URL=https://api.infobip.com
INFOBIP_API_KEY=your_infobip_api_key_here

# Webhook Security
WEBHOOK_VERIFY_TOKEN=your_webhook_verify_token_here

# RAG Configuration
RAG_TOP_K=5
RAG_MIN_SCORE=0.7

# Token Limits
MAX_TOKENS_REPLY=500
SUMMARIZE_THRESHOLD=10000
```

## Step 2: Database Setup

### Create Database
```sql
CREATE DATABASE whatsapp_bot_test;
```

### Run Migrations
```bash
# Apply all migrations
migrate -path internal/migrations -database "postgres://user:password@localhost/whatsapp_bot_test?sslmode=disable" up
```

Or manually run each migration:
```bash
psql whatsapp_bot_test < internal/migrations/001_init.up.sql
psql whatsapp_bot_test < internal/migrations/002_tenant_config.up.sql  
psql whatsapp_bot_test < internal/migrations/003_allowed_contacts.up.sql
```

## Step 3: Database Configuration

### Insert Tenant Configuration
```sql
-- Insert your tenant configuration
INSERT INTO tenants_config (
    id, 
    tenant_id, 
    waba_number, 
    embedding_model, 
    vector_store, 
    enabled_agents,
    config,
    metadata,
    enabled
) VALUES (
    uuid_generate_v4(),
    'test_tenant_001',
    '+1234567890',  -- Your WhatsApp Business number
    'deepseek-embed',
    'sql_fallback',
    ARRAY['db_agent', 'http_agent', 'orchestrator']::TEXT[],
    '{"max_tokens": 500, "temperature": 0.7}',
    '{"environment": "test", "setup_date": "2024-01-01"}',
    true
);
```

### Insert LLM Provider Configuration
```sql
-- Insert DeepSeek provider configuration
INSERT INTO llm_providers (
    id,
    tenant_id,
    name,
    provider,
    api_key,
    base_url,
    model_chat,
    model_embed,
    config,
    is_default,
    enabled
) VALUES (
    uuid_generate_v4(),
    'test_tenant_001',
    'deepseek_default',
    'deepseek',
    'your_deepseek_api_key_here',
    'https://api.deepseek.com',
    'deepseek-chat',
    'deepseek-embed',
    '{"max_tokens": 500, "temperature": 0.7}',
    true,
    true
);
```

### Add Allowed Contacts (Your Phone Numbers)
```sql
-- Add yourself as an allowed contact
INSERT INTO allowed_contacts (
    id,
    tenant_id,
    phone_number,
    contact_name,
    permissions,
    notes,
    enabled
) VALUES (
    uuid_generate_v4(),
    'test_tenant_001',
    '+1987654321',  -- Replace with YOUR actual WhatsApp number
    'Test User (You)',
    ARRAY['chat', 'schedule', 'admin']::TEXT[],
    'Main test user with full permissions',
    true
);

-- Add additional test contacts if needed
INSERT INTO allowed_contacts (
    id,
    tenant_id,
    phone_number,
    contact_name,
    permissions,
    notes,
    enabled
) VALUES (
    uuid_generate_v4(),
    'test_tenant_001',
    '+1555123456',  -- Another test number
    'Test Contact 2',
    ARRAY['chat']::TEXT[],
    'Limited permissions test contact',
    true
);
```

## Step 4: System Configuration
```sql
-- Update system configuration for testing
UPDATE system_config 
SET value = '"test_tenant_001"' 
WHERE key = 'default_tenant_id';

-- Ensure contact allowlist is enabled
UPDATE system_config 
SET value = 'true' 
WHERE key = 'require_contact_allowlist';
```

## Step 5: Test the Setup

### Start the Server
```bash
go run cmd/server/main.go
```

### Verify Health
```bash
curl http://localhost:8080/health
```

### Test Contact Authorization
```bash
# Check if your number is allowed
curl "http://localhost:8080/api/v1/tenants/test_tenant_001/contacts/check?phone_number=%2B1987654321"

# Should return: {"tenant_id": "test_tenant_001", "phone_number": "+1987654321", "is_allowed": true}
```

### Test Webhook (Simulate Incoming Message)
```bash
curl -X POST http://localhost:8080/webhooks/infobip \
  -H "Content-Type: application/json" \
  -H "X-Hub-Signature: your_webhook_verify_token_here" \
  -d '{
    "results": [{
      "messageId": "test-msg-123",
      "from": "+1987654321",
      "to": "+1234567890",
      "integrationType": "WHATSAPP",
      "receivedAt": "2024-01-01T12:00:00Z",
      "message": {
        "type": "TEXT",
        "text": {
          "text": "Hello bot, what time is it?"
        }
      },
      "contact": {
        "name": "Test User"
      }
    }]
  }'
```

## Step 6: Verify Database Records

### Check Messages Table
```sql
-- Should see your test message
SELECT * FROM messages WHERE tenant_id = 'test_tenant_001';
```

### Check Users Table  
```sql
-- Should see user created from your phone number
SELECT * FROM users WHERE tenant_id = 'test_tenant_001';
```

### Check Logs
Look for these log messages:
- ✅ `"message processed successfully"`
- ⚠️ `"unauthorized sender attempted to contact bot"` (for unauthorized numbers)

## Testing Security

### Test Unauthorized Contact
```bash
curl -X POST http://localhost:8080/webhooks/infobip \
  -H "Content-Type: application/json" \
  -d '{
    "results": [{
      "messageId": "test-unauthorized-123",
      "from": "+1999999999",  
      "to": "+1234567890",
      "message": {
        "type": "TEXT",
        "text": {"text": "Hello from unauthorized number"}
      },
      "contact": {"name": "Unknown User"}
    }]
  }'
```

This should be silently ignored and logged as unauthorized.

## Minimal Test Without Vector DB

Since you're using `sql_fallback` for vector store, the bot will work without pgvector extension. It will:

1. ✅ Process incoming messages
2. ✅ Validate sender authorization  
3. ✅ Store messages in PostgreSQL
4. ✅ Call DeepSeek for responses
5. ✅ Send replies via Infobip
6. ⚠️ Skip vector/embedding storage (memory will be basic)

## Quick Database Reset

To reset your test database:
```sql
-- Clear all data
TRUNCATE messages, users, allowed_contacts, llm_providers, tenants_config RESTART IDENTITY CASCADE;

-- Then re-run the INSERT statements above
```

## Environment Variables Summary

**Required for DeepSeek + Local PostgreSQL:**
- `LLM_PROVIDER=deepseek`
- `LLM_API_KEY=your_deepseek_key`  
- `VECTOR_BACKEND=sql_fallback`
- `DATABASE_URL_DEFAULT=postgres://...`
- `INFOBIP_API_KEY=your_infobip_key`

**Your WhatsApp Numbers:**
- Business number (receives messages): `+1234567890` 
- Your personal number (sends messages): `+1987654321`

The bot will only respond to messages FROM `+1987654321` TO `+1234567890`.