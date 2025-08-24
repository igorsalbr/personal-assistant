package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sashabaranov/go-openai"

	"personal-assistant/internal/domain"
	"personal-assistant/internal/log"
)

// DeepSeekProvider implements the LLMProvider interface for DeepSeek
// DeepSeek uses OpenAI-compatible API, so we can reuse the OpenAI client
type DeepSeekProvider struct {
	client     *openai.Client
	config     *domain.LLMProviderConfig
	logger     *log.Logger
	chatModel  string
	embedModel string
}

// ChatModel returns the chat model name
func (p *DeepSeekProvider) ChatModel() string {
	return p.chatModel
}

// EmbedModel returns the embedding model name
func (p *DeepSeekProvider) EmbedModel() string {
	return p.embedModel
}

// NewDeepSeekProvider creates a new DeepSeek provider
func NewDeepSeekProvider(config *domain.LLMProviderConfig, logger *log.Logger) (*DeepSeekProvider, error) {
	// Configure client with DeepSeek base URL
	clientConfig := openai.DefaultConfig(config.APIKey)

	// Use custom base URL or default DeepSeek URL
	if config.BaseURL != "" {
		clientConfig.BaseURL = config.BaseURL
	} else {
		clientConfig.BaseURL = "https://api.deepseek.com/v1"
	}

	client := openai.NewClientWithConfig(clientConfig)

	// Use configured models or defaults
	chatModel := config.ModelChat
	if chatModel == "" {
		chatModel = "deepseek-chat"
	}

	embedModel := config.ModelEmbed
	if embedModel == "" {
		embedModel = "text-embedding-v1" // DeepSeek's embedding model
	}

	return &DeepSeekProvider{
		client:     client,
		config:     config,
		logger:     logger,
		chatModel:  chatModel,
		embedModel: embedModel,
	}, nil
}

// Name returns the provider name
func (p *DeepSeekProvider) Name() string {
	return fmt.Sprintf("deepseek-%s", p.config.Name)
}

// Chat performs a chat completion
func (p *DeepSeekProvider) Chat(ctx context.Context, req *domain.ChatCompletionRequest) (*domain.ChatCompletionResponse, error) {
	start := time.Now()

	// Convert domain request to OpenAI-compatible request
	openaiReq := p.convertChatRequest(req)

	p.logger.WithContext(ctx).Debug().
		Str("model", openaiReq.Model).
		Int("messages", len(openaiReq.Messages)).
		Int("max_tokens", openaiReq.MaxTokens).
		Msg("making DeepSeek chat completion request")

	// Make the request
	resp, err := p.client.CreateChatCompletion(ctx, openaiReq)
	if err != nil {
		p.logger.WithContext(ctx).Error().
			Err(err).
			Dur("duration", time.Since(start)).
			Msg("DeepSeek chat completion failed")
		return nil, fmt.Errorf("DeepSeek chat completion failed: %w", err)
	}

	// Convert response
	domainResp := p.convertChatResponse(&resp)

	// Log token usage
	if domainResp.Usage != nil {
		p.logger.LogTokenUsage("deepseek_chat",
			domainResp.Usage.PromptTokens,
			domainResp.Usage.CompletionTokens,
			domainResp.Usage.TotalTokens)
	}

	p.logger.WithContext(ctx).Debug().
		Str("finish_reason", domainResp.Choices[0].FinishReason).
		Dur("duration", time.Since(start)).
		Msg("DeepSeek chat completion successful")

	return domainResp, nil
}

// Embed generates embeddings for the given texts
func (p *DeepSeekProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	start := time.Now()

	p.logger.WithContext(ctx).Debug().
		Str("model", p.embedModel).
		Int("texts", len(texts)).
		Msg("making DeepSeek embedding request")

	// Make the request
	resp, err := p.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: texts,
		Model: openai.EmbeddingModel(p.embedModel),
	})
	if err != nil {
		p.logger.WithContext(ctx).Error().
			Err(err).
			Dur("duration", time.Since(start)).
			Msg("DeepSeek embedding failed")
		return nil, fmt.Errorf("DeepSeek embedding failed: %w", err)
	}

	// Extract embeddings
	embeddings := make([][]float32, len(resp.Data))
	for i, data := range resp.Data {
		embeddings[i] = data.Embedding
	}

	// Log token usage
	if resp.Usage.TotalTokens > 0 {
		p.logger.LogTokenUsage("deepseek_embed",
			resp.Usage.PromptTokens,
			0, // embeddings don't have completion tokens
			resp.Usage.TotalTokens)
	}

	p.logger.WithContext(ctx).Debug().
		Dur("duration", time.Since(start)).
		Int("embeddings", len(embeddings)).
		Msg("DeepSeek embedding successful")

	return embeddings, nil
}

// convertChatRequest converts domain request to OpenAI-compatible request
func (p *DeepSeekProvider) convertChatRequest(req *domain.ChatCompletionRequest) openai.ChatCompletionRequest {
	openaiReq := openai.ChatCompletionRequest{
		Model:       p.chatModel,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}

	// Convert messages
	for _, msg := range req.Messages {
		openaiMsg := openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}

		// Handle tool calls
		if len(msg.ToolCalls) > 0 {
			for _, toolCall := range msg.ToolCalls {
				openaiMsg.ToolCalls = append(openaiMsg.ToolCalls, openai.ToolCall{
					ID:   toolCall.ID,
					Type: openai.ToolType(toolCall.Type),
					Function: openai.FunctionCall{
						Name:      toolCall.Function.Name,
						Arguments: string(toolCall.Function.Arguments),
					},
				})
			}
		}

		// Handle tool call ID
		if msg.ToolCallID != "" {
			openaiMsg.ToolCallID = msg.ToolCallID
		}

		// Handle name
		if msg.Name != "" {
			openaiMsg.Name = msg.Name
		}

		openaiReq.Messages = append(openaiReq.Messages, openaiMsg)
	}

	// Convert tools
	if len(req.Tools) > 0 {
		for _, tool := range req.Tools {
			openaiTool := openai.Tool{
				Type: openai.ToolType(tool.Type),
			}

			if tool.Function != nil {
				openaiTool.Function = openai.FunctionDefinition{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
					Parameters:  tool.Function.Parameters,
				}
			}

			openaiReq.Tools = append(openaiReq.Tools, openaiTool)
		}

		// Set tool choice
		if req.ToolChoice != nil {
			openaiReq.ToolChoice = req.ToolChoice
		}
	}

	return openaiReq
}

// convertChatResponse converts OpenAI response to domain response
func (p *DeepSeekProvider) convertChatResponse(resp *openai.ChatCompletionResponse) *domain.ChatCompletionResponse {
	domainResp := &domain.ChatCompletionResponse{
		ID:      resp.ID,
		Object:  resp.Object,
		Created: resp.Created,
		Model:   resp.Model,
	}

	// Convert choices
	for _, choice := range resp.Choices {
		domainChoice := domain.Choice{
			Index:        choice.Index,
			FinishReason: string(choice.FinishReason),
		}

		if choice.Message.Content != "" || len(choice.Message.ToolCalls) > 0 {
			domainMsg := &domain.ChatMessage{
				Role:    choice.Message.Role,
				Content: choice.Message.Content,
			}

			// Convert tool calls
			for _, toolCall := range choice.Message.ToolCalls {
				domainToolCall := domain.ToolCall{
					ID:   toolCall.ID,
					Type: string(toolCall.Type),
				}

				if toolCall.Function.Name != "" {
					domainToolCall.Function = &domain.FunctionCall{
						Name:      toolCall.Function.Name,
						Arguments: json.RawMessage(toolCall.Function.Arguments),
					}
				}

				domainMsg.ToolCalls = append(domainMsg.ToolCalls, domainToolCall)
			}

			domainChoice.Message = domainMsg
		}

		domainResp.Choices = append(domainResp.Choices, domainChoice)
	}

	// Convert usage
	if resp.Usage.TotalTokens > 0 {
		domainResp.Usage = &domain.TokenUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	}

	return domainResp
}
