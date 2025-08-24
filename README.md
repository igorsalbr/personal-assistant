# WhatsApp LLM Bot with Multi-Tenant Architecture

A production-ready WhatsApp bot built in Go that connects Large Language Models (OpenAI, DeepSeek) to WhatsApp via Infobip, featuring multi-tenancy, RAG (Retrieval-Augmented Generation), and MCP-style tools architecture.

## 🚀 Features

### Core Capabilities
- **Multi-Tenant Architecture**: Isolated databases and configurations per WhatsApp Business Account
- **LLM Integration**: Support for OpenAI, DeepSeek, and extensible provider system
- **RAG Memory System**: Persistent memory with vector similarity search using pgvector
- **MCP-Style Tools**: Modular tool system for database operations and external API calls
- **WhatsApp Integration**: Production-ready Infobip client with retry logic and error handling
- **Agent Architecture**: Orchestrator pattern with specialized agents (DB, HTTP, etc.)

### Advanced Features
- **Vector Search**: pgvector for semantic similarity with SQL fallback
- **Token Optimization**: Intelligent tool gating and context management
- **Database per Tenant**: Complete isolation with per-tenant configurations
- **Configurable LLM Providers**: Store provider configs in database, switch per tenant
- **Observability**: Structured logging, token usage tracking, and request tracing
- **Production Ready**: Docker support, migrations, health checks, graceful shutdown

## 📋 Prerequisites

- Go 1.22+
- PostgreSQL 14+ with pgvector extension
- Infobip WhatsApp Business API account
- OpenAI or DeepSeek API key

## 🛠️ Installation

### 1. Clone and Setup

```bash
git clone <repository-url>
cd whatsapp-llm-bot
cp .env.example .env
cp tenants.example.yaml tenants.yaml
```

### 2. Configure Environment Variables

Edit `.env` with your credentials:

```env
# LLM Configuration
LLM_PROVIDER=openai
LLM_API_KEY=your_openai_api_key_here

# For DeepSeek instead:
# LLM_PROVIDER=deepseek
# LLM_API_KEY=your_deepseek_api_key_here

# Database
DATABASE_URL_DEFAULT=postgres://user:password@localhost/whatsapp_bot?sslmode=disable

# Infobip
INFOBIP_API_KEY=your_infobip_api_key_here
INFOBIP_WABA_NUMBERS=+1234567890

# Webhook Security
WEBHOOK_VERIFY_TOKEN=your_secure_token_here
```

### 3. Setup PostgreSQL with pgvector

```sql
-- Install pgvector extension
CREATE EXTENSION vector;

-- Create your database
CREATE DATABASE whatsapp_bot_tenant1;
```

### 4. Configure Tenants

Edit `tenants.yaml`:

```yaml
tenants:
  - tenant_id: "my_business"
    waba_number: "+1234567890"
    db_dsn: "postgres://user:password@localhost/whatsapp_bot_tenant1?sslmode=disable"
    embedding_model: "text-embedding-ada-002"
    vector_store: "pgvector"
    enabled_agents: ["db_agent", "http_agent", "orchestrator"]
    config:
      business_name: "My Business"
      timezone: "America/New_York"
      rag_enabled: true
```

### 5. Build and Run

```bash
# Install dependencies
go mod tidy

# Run database migrations (will be created automatically on first run)
go run cmd/server/main.go
```

The server will start on port 8080 (configurable via `APP_PORT`).

## 🔧 Configuration

### Tenant Configuration

Each tenant represents a separate WhatsApp Business Account with isolated:
- Database connection
- LLM provider settings
- Enabled agents/tools
- Custom business logic

### LLM Providers

Support for multiple LLM providers:

**OpenAI:**
```env
LLM_PROVIDER=openai
LLM_API_KEY=sk-...
LLM_MODEL_CHAT=gpt-3.5-turbo
LLM_MODEL_EMBED=text-embedding-ada-002
```

**DeepSeek:**
```env
LLM_PROVIDER=deepseek
LLM_API_KEY=your_deepseek_key
LLM_MODEL_CHAT=deepseek-chat
LLM_MODEL_EMBED=text-embedding-v1
```

**Mock Provider (for testing):**
```env
LLM_PROVIDER=mock
LLM_API_KEY=mock_key
```

### Vector Stores

- **pgvector**: Full semantic search with vector similarity
- **sql_fallback**: Text search fallback when pgvector unavailable

## 🚀 Usage

### Setting up Infobip Webhook

1. Configure webhook URL in Infobip dashboard:
   ```
   https://yourdomain.com/webhooks/infobip
   ```

2. Set webhook verification token to match your `WEBHOOK_VERIFY_TOKEN`

### Example Interactions

**Storing Information:**
```
User: "Remember I have a dentist appointment tomorrow at 2 PM"
Bot: "I've saved your dentist appointment for tomorrow at 2 PM as an event."
```

**Searching Memory:**
```
User: "What do I have scheduled this week?"
Bot: "Based on your schedule, you have:
- Dentist appointment tomorrow at 2 PM
- Team meeting Friday at 10 AM"
```

**External API Calls:**
```
User: "What's the weather like?"
Bot: [Calls configured weather API] "Current weather: 22°C, partly cloudy"
```

## 🏗️ Architecture

### High-Level Architecture
```
WhatsApp → Infobip → Webhook → Tenant Manager → LLM + Tools → Response
                                      ↓
                              Database + Vector Store
```

### Key Components

1. **Tenant Manager**: Routes requests and manages per-tenant resources
2. **Orchestrator**: Main LLM agent that coordinates tool usage
3. **RAG Pipeline**: Embedding generation and similarity search
4. **Tool Registry**: MCP-style tools for database and API operations
5. **Message Processor**: Handles the full message lifecycle

### Database Schema

- `users`: WhatsApp users per tenant
- `messages`: Conversation history
- `memory_chunks`: RAG memory with vector embeddings
- `llm_providers`: Per-tenant LLM configurations
- `agents`: Available agents and their permissions

## 🛠️ Development

### Adding New Tools

Create a tool that implements the `domain.Tool` interface:

```go
type MyTool struct {}

func (t *MyTool) Name() string {
    return "my_tool"
}

func (t *MyTool) Schema() *domain.JSONSchema {
    return &domain.JSONSchema{
        Type: "object",
        Properties: map[string]domain.JSONSchemaProperty{
            "input": {Type: "string", Description: "Tool input"},
        },
        Required: []string{"input"},
    }
}

func (t *MyTool) Invoke(ctx context.Context, input map[string]interface{}) (interface{}, error) {
    // Tool logic here
    return map[string]string{"result": "success"}, nil
}
```

### Adding New Agents

Implement the `domain.Agent` interface:

```go
type MyAgent struct {}

func (a *MyAgent) Name() string { return "my_agent" }
func (a *MyAgent) AllowedTenants() []string { return []string{"tenant1"} }
func (a *MyAgent) CanHandle(intent string) bool { return intent == "my_intent" }
func (a *MyAgent) Handle(ctx context.Context, req *domain.AgentRequest) (*domain.AgentResponse, error) {
    // Agent logic here
}
```

### Testing

Run tests:
```bash
go test ./...
```

For integration testing with a mock LLM:
```bash
export LLM_PROVIDER=mock
go test ./tests/...
```

## 🚦 API Endpoints

- `GET /health` - Health check
- `POST /webhooks/infobip` - Incoming WhatsApp messages
- `POST /webhooks/infobip/status` - Message status updates
- `GET /webhooks/infobip/health` - Webhook health check

## 📊 Monitoring

The application provides structured logging with:
- Request tracing with correlation IDs
- Token usage tracking per LLM call
- Tool execution timing and success rates
- Per-tenant resource usage

Example log entry:
```json
{
  "level": "info",
  "tenant_id": "my_business",
  "user_id": "user123",
  "request_id": "req456",
  "message": "message processed successfully",
  "duration": "1.2s",
  "tools_used": ["upsert_item"],
  "token_usage": {"total": 150}
}
```

## 🔒 Security

- Per-tenant database isolation
- Webhook signature verification
- API key rotation support
- Input sanitization and validation
- Rate limiting ready (implement as middleware)

## 📈 Scalability

- Horizontal scaling ready
- Database connection pooling per tenant
- LLM provider caching and connection reuse
- Configurable resource limits per tenant
- Background job support for reminders/scheduled tasks

## 🐛 Troubleshooting

### Common Issues

**Database Connection Errors:**
- Verify PostgreSQL is running and accessible
- Check database DSN in tenant configuration
- Ensure pgvector extension is installed

**LLM API Errors:**
- Verify API key is correct and has sufficient quota
- Check network connectivity to provider
- Review rate limiting settings

**Webhook Not Receiving Messages:**
- Verify webhook URL is publicly accessible
- Check webhook verification token
- Review Infobip webhook configuration

**Memory/RAG Not Working:**
- Ensure embeddings are being generated (check logs)
- Verify vector store configuration
- Check RAG pipeline settings in tenant config

## 📝 License

[Add your license here]

## 🤝 Contributing

[Add contribution guidelines here]

## 🆘 Support

[Add support information here]

---

**Built with ❤️ using Go, PostgreSQL, pgvector, and modern LLM APIs**