-- Setup Bedrock LLM Provider
-- First, update the check constraint to allow bedrock
ALTER TABLE llm_providers DROP CONSTRAINT llm_providers_provider_check;
ALTER TABLE llm_providers ADD CONSTRAINT llm_providers_provider_check 
    CHECK (provider::text = ANY (ARRAY['openai'::character varying, 'deepseek'::character varying, 'anthropic'::character varying, 'bedrock'::character varying, 'mock'::character varying]::text[]));

-- Then insert the bedrock provider
INSERT INTO llm_providers (
    id,
    tenant_id,
    provider,
    name,
    api_key,
    base_url,
    model_chat,
    model_embed,
    config,
    is_default,
    enabled,
    created_at,
    updated_at
) VALUES (
    gen_random_uuid(),
    'test_tentant_001',  -- Replace with your actual tenant ID
    'bedrock',
    'bedrock-default',
    'ABSKQmVkcm9ja0FQSUtleS1rNWU0LWF0LTczMDMzNTE4Ndeepseek-chatDc5NDpuZmdTZ3VoNDdjbitydytlK0xLMmFBNUJXQ0Ixb3NnZ3lQb3h4MnlPaDJCb1dsQmpUUUNtWE9oZGFqOD0=',
    'https://bedrock-runtime.us-west-2.amazonaws.com',  -- or your preferred region
    'openai.gpt-oss-120b-1:0',
    'amazon.titan-embed-text-v1',
    '{"region": "us-west-2"}',
    true,  -- is_default
    true,  -- enabled
    NOW(),
    NOW()
);