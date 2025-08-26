package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"

	"personal-assistant/internal/domain"
	"personal-assistant/internal/log"
)

// BedrockProvider implements the LLMProvider interface for AWS Bedrock
type BedrockProvider struct {
	client     *bedrockruntime.Client
	config     *domain.LLMProviderConfig
	logger     *log.Logger
	chatModel  string
	embedModel string
}

// NewBedrockProvider creates a new AWS Bedrock provider
func NewBedrockProvider(config *domain.LLMProviderConfig, logger *log.Logger) (*BedrockProvider, error) {
	// Load AWS config with default credentials chain
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO(),
		awsconfig.WithRegion("us-west-2"), // Default region, can be overridden by AWS_REGION env var
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create Bedrock runtime client
	client := bedrockruntime.NewFromConfig(cfg)

	// Use configured models or defaults
	chatModel := config.ModelChat
	if chatModel == "" {
		chatModel = "openai.gpt-oss-120b-1:0"
	}

	embedModel := config.ModelEmbed
	if embedModel == "" {
		embedModel = "amazon.titan-embed-text-v1"
	}

	return &BedrockProvider{
		client:     client,
		config:     config,
		logger:     logger,
		chatModel:  chatModel,
		embedModel: embedModel,
	}, nil
}

// Name returns the provider name
func (p *BedrockProvider) Name() string {
	return fmt.Sprintf("bedrock-%s", p.config.Name)
}

// Chat performs a chat completion using AWS Bedrock
func (p *BedrockProvider) Chat(ctx context.Context, req *domain.ChatCompletionRequest) (*domain.ChatCompletionResponse, error) {
	start := time.Now()

	p.logger.WithContext(ctx).Debug().
		Str("model", p.chatModel).
		Int("messages", len(req.Messages)).
		Int("max_tokens", req.MaxTokens).
		Msg("making AWS Bedrock chat completion request")

	// Convert domain request to Bedrock format
	body, err := p.convertChatRequestToBedrock(req)
	if err != nil {
		return nil, fmt.Errorf("failed to convert chat request: %w", err)
	}

	// Make the on-demand request
	result, err := p.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(p.chatModel),
		Body:        body,
		ContentType: aws.String("application/json"),
		Accept:      aws.String("application/json"),
	})
	if err != nil {
		p.logger.WithContext(ctx).Error().
			Err(err).
			Dur("duration", time.Since(start)).
			Msg("AWS Bedrock chat completion failed")
		return nil, fmt.Errorf("AWS Bedrock chat completion failed: %w", err)
	}

	// Convert response
	domainResp, err := p.convertBedrockChatResponse(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to convert Bedrock response: %w", err)
	}

	p.logger.WithContext(ctx).Debug().
		Dur("duration", time.Since(start)).
		Msg("AWS Bedrock chat completion successful")

	return domainResp, nil
}

// Embed generates embeddings using AWS Bedrock
func (p *BedrockProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	start := time.Now()

	p.logger.WithContext(ctx).Debug().
		Str("model", p.embedModel).
		Int("texts", len(texts)).
		Msg("making AWS Bedrock embedding request")

	var allEmbeddings [][]float32

	// Process texts individually for Titan embed model
	for _, text := range texts {
		body, err := json.Marshal(map[string]interface{}{
			"inputText": text,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to marshal embedding request: %w", err)
		}

		result, err := p.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
			ModelId:     aws.String(p.embedModel),
			Body:        body,
			ContentType: aws.String("application/json"),
			Accept:      aws.String("application/json"),
		})
		if err != nil {
			return nil, fmt.Errorf("AWS Bedrock embedding failed: %w", err)
		}

		var response struct {
			Embedding []float32 `json:"embedding"`
		}

		if err := json.Unmarshal(result.Body, &response); err != nil {
			return nil, fmt.Errorf("failed to unmarshal embedding response: %w", err)
		}

		allEmbeddings = append(allEmbeddings, response.Embedding)
	}

	p.logger.WithContext(ctx).Debug().
		Dur("duration", time.Since(start)).
		Int("embeddings", len(allEmbeddings)).
		Msg("AWS Bedrock embedding successful")

	return allEmbeddings, nil
}

// convertChatRequestToBedrock converts domain request to Bedrock Claude format
func (p *BedrockProvider) convertChatRequestToBedrock(req *domain.ChatCompletionRequest) ([]byte, error) {
	// Convert messages to Claude format
	messages := make([]map[string]interface{}, 0, len(req.Messages))

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			// Skip system messages for now, or handle them specially
			continue
		}

		bedrockMsg := map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		}

		messages = append(messages, bedrockMsg)
	}

	// Build request body for Claude
	body := map[string]interface{}{
		"messages":          messages,
		"max_tokens":        req.MaxTokens,
		"temperature":       req.Temperature,
		"anthropic_version": "bedrock-2023-05-31",
	}

	// Add system message if present
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			body["system"] = msg.Content
			break
		}
	}

	return json.Marshal(body)
}

// convertBedrockChatResponse converts Bedrock response to domain response
func (p *BedrockProvider) convertBedrockChatResponse(body []byte) (*domain.ChatCompletionResponse, error) {
	var response struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Bedrock response: %w", err)
	}

	// Extract text content
	var content string
	for _, c := range response.Content {
		if c.Type == "text" {
			content = c.Text
			break
		}
	}

	return &domain.ChatCompletionResponse{
		Choices: []domain.Choice{
			{
				Index: 0,
				Message: &domain.ChatMessage{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: "stop",
			},
		},
		Usage: &domain.TokenUsage{
			PromptTokens:     response.Usage.InputTokens,
			CompletionTokens: response.Usage.OutputTokens,
			TotalTokens:      response.Usage.InputTokens + response.Usage.OutputTokens,
		},
	}, nil
}
