package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"

	"personal-assistant/internal/domain"
	"personal-assistant/internal/log"
)

type BedrockProvider struct {
	client     *bedrockruntime.Client
	config     *domain.LLMProviderConfig
	logger     *log.Logger
	chatModel  string
	embedModel string
}

func NewBedrockProvider(config *domain.LLMProviderConfig, logger *log.Logger) (*BedrockProvider, error) {
	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	
	if accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY are required")
	}

	cfg := aws.Config{
		Region:      "us-west-2",
		Credentials: credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
	}

	client := bedrockruntime.NewFromConfig(cfg)

	chatModel := config.ModelChat
	if chatModel == "" {
		chatModel = "openai.gpt-oss-120b-1:0"
	}

	embedModel := config.ModelEmbed
	if embedModel == "" {
		embedModel = "amazon.titan-embed-text-v2:0"
	}

	return &BedrockProvider{
		client:     client,
		config:     config,
		logger:     logger,
		chatModel:  chatModel,
		embedModel: embedModel,
	}, nil
}

func (p *BedrockProvider) Name() string {
	return "bedrock"
}

func (p *BedrockProvider) Chat(ctx context.Context, req *domain.ChatCompletionRequest) (*domain.ChatCompletionResponse, error) {
	body, err := p.buildClaudeRequest(req)
	if err != nil {
		return nil, err
	}

	result, err := p.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(p.chatModel),
		Body:        body,
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return nil, fmt.Errorf("bedrock invoke failed: %w", err)
	}

	return p.parseClaudeResponse(result.Body)
}

func (p *BedrockProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	var embeddings [][]float32

	for _, text := range texts {
		body, _ := json.Marshal(map[string]string{"inputText": text})

		result, err := p.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
			ModelId:     aws.String(p.embedModel),
			Body:        body,
			ContentType: aws.String("application/json"),
		})
		if err != nil {
			return nil, err
		}

		var resp struct {
			Embedding []float32 `json:"embedding"`
		}
		json.Unmarshal(result.Body, &resp)
		embeddings = append(embeddings, resp.Embedding)
	}

	return embeddings, nil
}

func (p *BedrockProvider) buildClaudeRequest(req *domain.ChatCompletionRequest) ([]byte, error) {
	messages := []map[string]string{}
	var system string

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			system = msg.Content
		} else {
			messages = append(messages, map[string]string{
				"role":    msg.Role,
				"content": msg.Content,
			})
		}
	}

	body := map[string]interface{}{
		"messages":          messages,
		"max_tokens":        req.MaxTokens,
		"anthropic_version": "bedrock-2023-05-31",
	}

	if system != "" {
		body["system"] = system
	}

	return json.Marshal(body)
}

func (p *BedrockProvider) parseClaudeResponse(body []byte) (*domain.ChatCompletionResponse, error) {
	var resp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	content := ""
	if len(resp.Content) > 0 {
		content = resp.Content[0].Text
	}

	return &domain.ChatCompletionResponse{
		Choices: []domain.Choice{{
			Message: &domain.ChatMessage{
				Role:    "assistant",
				Content: content,
			},
		}},
		Usage: &domain.TokenUsage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}, nil
}
