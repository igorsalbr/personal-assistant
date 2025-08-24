-- Drop triggers
DROP TRIGGER IF EXISTS trigger_memory_chunks_text_search ON memory_chunks;
DROP TRIGGER IF EXISTS trigger_users_updated_at ON users;
DROP TRIGGER IF EXISTS trigger_agents_updated_at ON agents;

-- Drop functions
DROP FUNCTION IF EXISTS update_memory_chunks_text_search();
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tables in reverse order
DROP TABLE IF EXISTS llm_providers;
DROP TABLE IF EXISTS external_services;
DROP TABLE IF EXISTS agents;
DROP TABLE IF EXISTS memory_chunks;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS users;

-- Drop extensions if no other dependencies
DROP EXTENSION IF EXISTS vector;
DROP EXTENSION IF EXISTS "uuid-ossp";