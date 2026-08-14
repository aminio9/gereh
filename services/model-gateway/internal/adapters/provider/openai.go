// Package provider contains provider adapters for upstream LLM services.
package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	modelgatewayv1 "github.com/aminio9/gereh/gen/go/gereh/model/gateway/v1"
	"github.com/aminio9/gereh/services/model-gateway/internal/domain"
)

// OpenAIAdapter handles OpenAI API communication and response normalization.
type OpenAIAdapter struct {
	httpClient *http.Client
	baseURL    string
}

// NewOpenAIAdapter creates a new OpenAI adapter with safe redirect policies.
func NewOpenAIAdapter(timeout time.Duration, baseURL string) *OpenAIAdapter {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &OpenAIAdapter{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// ProviderKey returns the provider identifier.
func (a *OpenAIAdapter) ProviderKey() string {
	return "openai"
}

type openAIChatRequest struct {
	Model          string           `json:"model"`
	Messages       []domain.Message `json:"messages"`
	Temperature    *float64         `json:"temperature,omitempty"`
	TopP           *float64         `json:"top_p,omitempty"`
	MaxTokens      *int64           `json:"max_tokens,omitempty"`
	Stream         bool             `json:"stream,omitempty"`
	StreamOptions  *streamOptions   `json:"stream_options,omitempty"`
	Stop           []string         `json:"stop,omitempty"`
	Tools          []openAITool     `json:"tools,omitempty"`
	ToolChoice     any              `json:"tool_choice,omitempty"`
	ResponseFormat any              `json:"response_format,omitempty"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type openAITool struct {
	Type     string         `json:"type"`
	Function openAIFunction `json:"function"`
}

type openAIFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type openAIChatResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Created int64  `json:"created"`
	Choices []struct {
		Index        int            `json:"index"`
		Message      domain.Message `json:"message"`
		FinishReason string         `json:"finish_reason"`
	} `json:"choices"`
	Usage *openAIUsage `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

type openAIUsage struct {
	PromptTokens        int64 `json:"prompt_tokens"`
	CompletionTokens    int64 `json:"completion_tokens"`
	TotalTokens         int64 `json:"total_tokens"`
	PromptTokensDetails *struct {
		CachedTokens int64 `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
	CompletionTokensDetails *struct {
		ReasoningTokens int64 `json:"reasoning_tokens"`
	} `json:"completion_tokens_details"`
}

type openAIStreamChunk struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Created int64  `json:"created"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role      string            `json:"role"`
			Content   string            `json:"content"`
			ToolCalls []domain.ToolCall `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *openAIUsage `json:"usage"`
}

// Complete executes a non-streaming OpenAI chat completion request.
func (a *OpenAIAdapter) Complete(
	ctx context.Context,
	route *modelgatewayv1.InferenceRoute,
	apiKey []byte,
	req domain.InferenceRequest,
) (domain.InferenceResult, error) {
	payload := openAIChatRequest{
		Model:          route.ProviderModelId,
		Messages:       req.Messages,
		Temperature:    req.Temperature,
		TopP:           req.TopP,
		MaxTokens:      req.MaxTokens,
		Stream:         false,
		Stop:           req.Stop,
		ToolChoice:     req.ToolChoice,
		ResponseFormat: req.ResponseFormat,
	}

	for _, t := range req.Tools {
		payload.Tools = append(payload.Tools, openAITool{
			Type: "function",
			Function: openAIFunction{
				Name:        t.Name,
				Description: t.Desc,
				Parameters:  t.Params,
			},
		})
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return domain.InferenceResult{}, fmt.Errorf("marshal openai payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		a.baseURL+"/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return domain.InferenceResult{}, fmt.Errorf("create openai request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+string(apiKey))

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return domain.InferenceResult{}, fmt.Errorf("do openai request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.InferenceResult{}, fmt.Errorf("read openai response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusTooManyRequests {
			return domain.InferenceResult{}, domain.ErrProviderRateLimit
		}
		return domain.InferenceResult{}, fmt.Errorf("%w: status %d", domain.ErrProviderCallFailed, resp.StatusCode)
	}

	var chatResp openAIChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return domain.InferenceResult{}, fmt.Errorf("unmarshal openai response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return domain.InferenceResult{}, errors.New("openai returned empty choices")
	}

	var usage domain.TokenUsage
	if chatResp.Usage != nil {
		usage.PromptTokens = chatResp.Usage.PromptTokens
		usage.CompletionTokens = chatResp.Usage.CompletionTokens
		usage.TotalTokens = chatResp.Usage.TotalTokens
		if chatResp.Usage.PromptTokensDetails != nil {
			usage.CachedPromptTokens = chatResp.Usage.PromptTokensDetails.CachedTokens
		}
		if chatResp.Usage.CompletionTokensDetails != nil {
			usage.ReasoningTokens = chatResp.Usage.CompletionTokensDetails.ReasoningTokens
		}
	}

	return domain.InferenceResult{
		ID:           chatResp.ID,
		Model:        chatResp.Model,
		Created:      chatResp.Created,
		Message:      chatResp.Choices[0].Message,
		FinishReason: chatResp.Choices[0].FinishReason,
		Usage:        usage,
	}, nil
}

// Stream executes a streaming OpenAI chat completion request.
func (a *OpenAIAdapter) Stream(
	ctx context.Context,
	route *modelgatewayv1.InferenceRoute,
	apiKey []byte,
	req domain.InferenceRequest,
	onChunk func(domain.StreamChunk) error,
) (domain.TokenUsage, error) {
	payload := openAIChatRequest{
		Model:       route.ProviderModelId,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
		Stream:      true,
		StreamOptions: &streamOptions{
			IncludeUsage: true,
		},
		Stop:           req.Stop,
		ToolChoice:     req.ToolChoice,
		ResponseFormat: req.ResponseFormat,
	}

	for _, t := range req.Tools {
		payload.Tools = append(payload.Tools, openAITool{
			Type: "function",
			Function: openAIFunction{
				Name:        t.Name,
				Description: t.Desc,
				Parameters:  t.Params,
			},
		})
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return domain.TokenUsage{}, fmt.Errorf("marshal openai payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		a.baseURL+"/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return domain.TokenUsage{}, fmt.Errorf("create openai stream request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+string(apiKey))
	httpReq.Header.Set("Accept", "text/event-stream")

	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return domain.TokenUsage{}, fmt.Errorf("do openai stream request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusTooManyRequests {
			return domain.TokenUsage{}, domain.ErrProviderRateLimit
		}
		return domain.TokenUsage{}, fmt.Errorf("%w: status %d", domain.ErrProviderCallFailed, resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)
	var finalUsage domain.TokenUsage

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return finalUsage, fmt.Errorf("read openai stream: %w", err)
		}

		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}

		data := bytes.TrimPrefix(line, []byte("data: "))
		if bytes.Equal(data, []byte("[DONE]")) {
			break
		}

		var chunk openAIStreamChunk
		if err := json.Unmarshal(data, &chunk); err != nil {
			continue
		}

		if chunk.Usage != nil {
			finalUsage.PromptTokens = chunk.Usage.PromptTokens
			finalUsage.CompletionTokens = chunk.Usage.CompletionTokens
			finalUsage.TotalTokens = chunk.Usage.TotalTokens
			if chunk.Usage.PromptTokensDetails != nil {
				finalUsage.CachedPromptTokens = chunk.Usage.PromptTokensDetails.CachedTokens
			}
			if chunk.Usage.CompletionTokensDetails != nil {
				finalUsage.ReasoningTokens = chunk.Usage.CompletionTokensDetails.ReasoningTokens
			}
		}

		var (
			deltaRole    string
			deltaContent string
			deltaTool    *domain.ToolCall
			finishReason *string
		)

		if len(chunk.Choices) > 0 {
			choice := chunk.Choices[0]
			deltaRole = choice.Delta.Role
			deltaContent = choice.Delta.Content
			finishReason = choice.FinishReason
			if len(choice.Delta.ToolCalls) > 0 {
				deltaTool = &choice.Delta.ToolCalls[0]
			}
		}

		outChunk := domain.StreamChunk{
			ID:           chunk.ID,
			Model:        chunk.Model,
			Created:      chunk.Created,
			DeltaRole:    deltaRole,
			DeltaContent: deltaContent,
			DeltaTool:    deltaTool,
			FinishReason: finishReason,
		}

		if err := onChunk(outChunk); err != nil {
			return finalUsage, err
		}
	}

	return finalUsage, nil
}
