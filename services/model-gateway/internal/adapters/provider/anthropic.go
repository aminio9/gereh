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
	"github.com/google/uuid"
)

// AnthropicAdapter handles Anthropic Messages API communication and normalization.
type AnthropicAdapter struct {
	httpClient *http.Client
	baseURL    string
}

// NewAnthropicAdapter creates a new Anthropic adapter.
func NewAnthropicAdapter(timeout time.Duration, baseURL string) *AnthropicAdapter {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}
	return &AnthropicAdapter{
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
func (a *AnthropicAdapter) ProviderKey() string {
	return "anthropic"
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	System      string             `json:"system,omitempty"`
	MaxTokens   int64              `json:"max_tokens"`
	Temperature *float64           `json:"temperature,omitempty"`
	TopP        *float64           `json:"top_p,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
	StopSeqs    []string           `json:"stop_sequences,omitempty"`
}

type anthropicResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Model   string `json:"model"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens              int64 `json:"input_tokens"`
		OutputTokens             int64 `json:"output_tokens"`
		CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	} `json:"usage"`
}

// Complete executes a non-streaming Anthropic chat completion request.
func (a *AnthropicAdapter) Complete(
	ctx context.Context,
	route *modelgatewayv1.InferenceRoute,
	apiKey []byte,
	req domain.InferenceRequest,
) (domain.InferenceResult, error) {
	var system string
	var messages []anthropicMessage

	for _, m := range req.Messages {
		if m.Role == "system" {
			system = m.Content
		} else {
			messages = append(messages, anthropicMessage{
				Role:    m.Role,
				Content: m.Content,
			})
		}
	}

	maxTokens := int64(4096)
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		maxTokens = *req.MaxTokens
	}

	payload := anthropicRequest{
		Model:       route.ProviderModelId,
		Messages:    messages,
		System:      system,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      false,
		StopSeqs:    req.Stop,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return domain.InferenceResult{}, fmt.Errorf("marshal anthropic payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		a.baseURL+"/messages",
		bytes.NewReader(body),
	)
	if err != nil {
		return domain.InferenceResult{}, fmt.Errorf("create anthropic request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", string(apiKey))
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return domain.InferenceResult{}, fmt.Errorf("do anthropic request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.InferenceResult{}, fmt.Errorf("read anthropic response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusTooManyRequests {
			return domain.InferenceResult{}, domain.ErrProviderRateLimit
		}
		return domain.InferenceResult{}, fmt.Errorf("%w: status %d", domain.ErrProviderCallFailed, resp.StatusCode)
	}

	var antResp anthropicResponse
	if err := json.Unmarshal(respBody, &antResp); err != nil {
		return domain.InferenceResult{}, fmt.Errorf("unmarshal anthropic response: %w", err)
	}

	var content string
	for _, c := range antResp.Content {
		if c.Type == "text" {
			content += c.Text
		}
	}

	return domain.InferenceResult{
		ID:      antResp.ID,
		Model:   antResp.Model,
		Created: time.Now().Unix(),
		Message: domain.Message{
			Role:    "assistant",
			Content: content,
		},
		FinishReason: antResp.StopReason,
		Usage: domain.TokenUsage{
			PromptTokens:       antResp.Usage.InputTokens,
			CompletionTokens:   antResp.Usage.OutputTokens,
			TotalTokens:        antResp.Usage.InputTokens + antResp.Usage.OutputTokens,
			CachedPromptTokens: antResp.Usage.CacheReadInputTokens,
		},
	}, nil
}

// Stream executes a streaming Anthropic chat completion request.
func (a *AnthropicAdapter) Stream(
	ctx context.Context,
	route *modelgatewayv1.InferenceRoute,
	apiKey []byte,
	req domain.InferenceRequest,
	onChunk func(domain.StreamChunk) error,
) (domain.TokenUsage, error) {
	var system string
	var messages []anthropicMessage

	for _, m := range req.Messages {
		if m.Role == "system" {
			system = m.Content
		} else {
			messages = append(messages, anthropicMessage{
				Role:    m.Role,
				Content: m.Content,
			})
		}
	}

	maxTokens := int64(4096)
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		maxTokens = *req.MaxTokens
	}

	payload := anthropicRequest{
		Model:       route.ProviderModelId,
		Messages:    messages,
		System:      system,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      true,
		StopSeqs:    req.Stop,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return domain.TokenUsage{}, fmt.Errorf("marshal anthropic payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		a.baseURL+"/messages",
		bytes.NewReader(body),
	)
	if err != nil {
		return domain.TokenUsage{}, fmt.Errorf("create anthropic stream request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", string(apiKey))
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Accept", "text/event-stream")

	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return domain.TokenUsage{}, fmt.Errorf("do anthropic stream request: %w", err)
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
	chunkID := "chatcmpl-" + uuid.NewString()

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return finalUsage, fmt.Errorf("read anthropic stream: %w", err)
		}

		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}

		data := bytes.TrimPrefix(line, []byte("data: "))

		var event struct {
			Type    string `json:"type"`
			Message *struct {
				ID    string `json:"id"`
				Usage struct {
					InputTokens int64 `json:"input_tokens"`
				} `json:"usage"`
			} `json:"message"`
			Delta *struct {
				Type         string `json:"type"`
				Text         string `json:"text"`
				StopReason   string `json:"stop_reason"`
				StopSequence string `json:"stop_sequence"`
			} `json:"delta"`
			Usage *struct {
				OutputTokens int64 `json:"output_tokens"`
			} `json:"usage"`
		}

		if err := json.Unmarshal(data, &event); err != nil {
			continue
		}

		switch event.Type {
		case "message_start":
			if event.Message != nil {
				chunkID = event.Message.ID
				finalUsage.PromptTokens = event.Message.Usage.InputTokens
			}
		case "content_block_delta":
			if event.Delta != nil && event.Delta.Text != "" {
				chunk := domain.StreamChunk{
					ID:           chunkID,
					Model:        route.ProviderModelId,
					Created:      time.Now().Unix(),
					DeltaRole:    "assistant",
					DeltaContent: event.Delta.Text,
				}
				if err := onChunk(chunk); err != nil {
					return finalUsage, err
				}
			}
		case "message_delta":
			if event.Usage != nil {
				finalUsage.CompletionTokens = event.Usage.OutputTokens
				finalUsage.TotalTokens = finalUsage.PromptTokens + finalUsage.CompletionTokens
			}
			if event.Delta != nil && event.Delta.StopReason != "" {
				reason := event.Delta.StopReason
				chunk := domain.StreamChunk{
					ID:           chunkID,
					Model:        route.ProviderModelId,
					Created:      time.Now().Unix(),
					FinishReason: &reason,
				}
				if err := onChunk(chunk); err != nil {
					return finalUsage, err
				}
			}
		}
	}

	return finalUsage, nil
}
