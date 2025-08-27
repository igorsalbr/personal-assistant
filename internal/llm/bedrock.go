package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrock"
	"github.com/aws/aws-sdk-go-v2/service/bedrock/types"

	"personal-assistant/internal/domain"
	"personal-assistant/internal/log"
)

type BedrockProvider struct {
	client     *bedrock.Client
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

	client := bedrock.NewFromConfig(cfg)

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
	system, messages := p.buildConverseRequest(req)

	input := &bedrock.ConverseInput{
		ModelId: aws.String(p.chatModel),
		Messages: messages,
		InferenceConfig: &types.InferenceConfiguration{
			MaxTokens:   aws.Int32(int32(req.MaxTokens)),
			Temperature: aws.Float32(req.Temperature),
		},
	}

	if len(system) > 0 {
		input.System = system
	}

	result, err := p.client.Converse(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("bedrock converse failed: %w", err)
	}

	return p.parseConverseResponse(result)
}

func (p *BedrockProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	var embeddings [][]float32

	for _, text := range texts {
		body, _ := json.Marshal(map[string]string{"inputText": text})

		result, err := p.client.InvokeModel(ctx, &bedrock.InvokeModelInput{
			ModelId: aws.String(p.embedModel),
			Body:    body,
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

func (p *BedrockProvider) buildConverseRequest(req *domain.ChatCompletionRequest) ([]types.SystemContentBlock, []types.Message) {
	var system []types.SystemContentBlock
	var messages []types.Message

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			system = append(system, &types.SystemContentBlockMemberText{
				Value: msg.Content,
			})
		} else {
			role := types.ConversationRoleUser
			if msg.Role == "assistant" {
				role = types.ConversationRoleAssistant
			}

			messages = append(messages, types.Message{
				Role: role,
				Content: []types.ContentBlock{
					&types.ContentBlockMemberText{
						Value: msg.Content,
					},
				},
			})
		}
	}

	return system, messages
}

func (p *BedrockProvider) parseConverseResponse(result *bedrock.ConverseOutput) (*domain.ChatCompletionResponse, error) {
	content := ""
	if result.Output != nil {
		if msg := result.Output.(*types.ConverseOutputMemberMessage); msg != nil {
			if len(msg.Value.Content) > 0 {
				if textBlock := msg.Value.Content[0].(*types.ContentBlockMemberText); textBlock != nil {
					content = textBlock.Value
				}
			}
		}
	}

	usage := &domain.TokenUsage{}
	if result.Usage != nil {
		usage.PromptTokens = int(aws.ToInt32(result.Usage.InputTokens))
		usage.CompletionTokens = int(aws.ToInt32(result.Usage.OutputTokens))
		usage.TotalTokens = int(aws.ToInt32(result.Usage.TotalTokens))
	}

	return &domain.ChatCompletionResponse{
		Choices: []domain.Choice{{
			Message: &domain.ChatMessage{
				Role:    "assistant",
				Content: content,
			},
		}},
		Usage: usage,
	}, nil
}
