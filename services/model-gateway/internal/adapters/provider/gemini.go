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

// GeminiAdapter handles Google Gemini API communication and normalization.
type GeminiAdapter struct {
	httpClient *http.Client
	baseURL    string
}

// NewGeminiAdapter creates a new Gemini adapter.
func NewGeminiAdapter(timeout time.Duration, baseURL string) *GeminiAdapter {
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}
	return &GeminiAdapter{
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
func (a *GeminiAdapter) ProviderKey() string {
	return "google"
}

type geminiPart struct {
	Text string `json:"text,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiRequest struct {
	Contents          []geminiContent `json:"contents"`
	GenerationConfig  *geminiConfig   `json:"generationConfig,omitempty"`
	SystemInstruction *geminiContent  `json:"systemInstruction,omitempty"`
}

type geminiConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
	MaxOutputTokens *int64   `json:"maxOutputTokens,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
			Role  string       `json:"role"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata *struct {
		PromptTokenCount        int64 `json:"promptTokenCount"`
		CandidatesTokenCount    int64 `json:"candidatesTokenCount"`
		TotalTokenCount         int64 `json:"totalTokenCount"`
		CachedContentTokenCount int64 `json:"cachedContentTokenCount"`
	} `json:"usageMetadata"`
}

// Complete executes a non-streaming Gemini generateContent request.
func (a *GeminiAdapter) Complete(
	ctx context.Context,
	route *modelgatewayv1.InferenceRoute,
	apiKey []byte,
	req domain.InferenceRequest,
) (domain.InferenceResult, error) {
	var (
		systemInstruction *geminiContent
		contents          []geminiContent
	)

	for _, m := range req.Messages {
		if m.Role == "system" {
			systemInstruction = &geminiContent{
				Parts: []geminiPart{{Text: m.Content}},
			}
		} else {
			role := "user"
			if m.Role == "assistant" {
				role = "model"
			}
			contents = append(contents, geminiContent{
				Role:  role,
				Parts: []geminiPart{{Text: m.Content}},
			})
		}
	}

	payload := geminiRequest{
		Contents:          contents,
		SystemInstruction: systemInstruction,
		GenerationConfig: &geminiConfig{
			Temperature:     req.Temperature,
			TopP:            req.TopP,
			MaxOutputTokens: req.MaxTokens,
			StopSequences:   req.Stop,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return domain.InferenceResult{}, fmt.Errorf("marshal gemini payload: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", a.baseURL, route.ProviderModelId, string(apiKey))
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		url,
		bytes.NewReader(body),
	)
	if err != nil {
		return domain.InferenceResult{}, fmt.Errorf("create gemini request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return domain.InferenceResult{}, fmt.Errorf("do gemini request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.InferenceResult{}, fmt.Errorf("read gemini response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusTooManyRequests {
			return domain.InferenceResult{}, domain.ErrProviderRateLimit
		}
		return domain.InferenceResult{}, fmt.Errorf("%w: status %d", domain.ErrProviderCallFailed, resp.StatusCode)
	}

	var gemResp geminiResponse
	if err := json.Unmarshal(respBody, &gemResp); err != nil {
		return domain.InferenceResult{}, fmt.Errorf("unmarshal gemini response: %w", err)
	}

	if len(gemResp.Candidates) == 0 {
		return domain.InferenceResult{}, errors.New("gemini returned no candidates")
	}

	candidate := gemResp.Candidates[0]
	var text string
	for _, p := range candidate.Content.Parts {
		text += p.Text
	}

	var usage domain.TokenUsage
	if gemResp.UsageMetadata != nil {
		usage.PromptTokens = gemResp.UsageMetadata.PromptTokenCount
		usage.CompletionTokens = gemResp.UsageMetadata.CandidatesTokenCount
		usage.TotalTokens = gemResp.UsageMetadata.TotalTokenCount
		usage.CachedPromptTokens = gemResp.UsageMetadata.CachedContentTokenCount
	}

	return domain.InferenceResult{
		ID:      "chatcmpl-" + uuid.NewString(),
		Model:   route.ProviderModelId,
		Created: time.Now().Unix(),
		Message: domain.Message{
			Role:    "assistant",
			Content: text,
		},
		FinishReason: strings.ToLower(candidate.FinishReason),
		Usage:        usage,
	}, nil
}

// Stream executes a streaming Gemini streamGenerateContent request.
func (a *GeminiAdapter) Stream(
	ctx context.Context,
	route *modelgatewayv1.InferenceRoute,
	apiKey []byte,
	req domain.InferenceRequest,
	onChunk func(domain.StreamChunk) error,
) (domain.TokenUsage, error) {
	var (
		systemInstruction *geminiContent
		contents          []geminiContent
	)

	for _, m := range req.Messages {
		if m.Role == "system" {
			systemInstruction = &geminiContent{
				Parts: []geminiPart{{Text: m.Content}},
			}
		} else {
			role := "user"
			if m.Role == "assistant" {
				role = "model"
			}
			contents = append(contents, geminiContent{
				Role:  role,
				Parts: []geminiPart{{Text: m.Content}},
			})
		}
	}

	payload := geminiRequest{
		Contents:          contents,
		SystemInstruction: systemInstruction,
		GenerationConfig: &geminiConfig{
			Temperature:     req.Temperature,
			TopP:            req.TopP,
			MaxOutputTokens: req.MaxTokens,
			StopSequences:   req.Stop,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return domain.TokenUsage{}, fmt.Errorf("marshal gemini payload: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse&key=%s", a.baseURL, route.ProviderModelId, string(apiKey))
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		url,
		bytes.NewReader(body),
	)
	if err != nil {
		return domain.TokenUsage{}, fmt.Errorf("create gemini stream request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return domain.TokenUsage{}, fmt.Errorf("do gemini stream request: %w", err)
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
			return finalUsage, fmt.Errorf("read gemini stream: %w", err)
		}

		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}

		data := bytes.TrimPrefix(line, []byte("data: "))
		var gemResp geminiResponse
		if err := json.Unmarshal(data, &gemResp); err != nil {
			continue
		}

		if gemResp.UsageMetadata != nil {
			finalUsage.PromptTokens = gemResp.UsageMetadata.PromptTokenCount
			finalUsage.CompletionTokens = gemResp.UsageMetadata.CandidatesTokenCount
			finalUsage.TotalTokens = gemResp.UsageMetadata.TotalTokenCount
			finalUsage.CachedPromptTokens = gemResp.UsageMetadata.CachedContentTokenCount
		}

		if len(gemResp.Candidates) > 0 {
			candidate := gemResp.Candidates[0]
			var deltaText string
			for _, p := range candidate.Content.Parts {
				deltaText += p.Text
			}

			var finishReason *string
			if candidate.FinishReason != "" && candidate.FinishReason != "FINISH_REASON_UNSPECIFIED" {
				r := strings.ToLower(candidate.FinishReason)
				finishReason = &r
			}

			chunk := domain.StreamChunk{
				ID:           chunkID,
				Model:        route.ProviderModelId,
				Created:      time.Now().Unix(),
				DeltaRole:    "assistant",
				DeltaContent: deltaText,
				FinishReason: finishReason,
			}

			if err := onChunk(chunk); err != nil {
				return finalUsage, err
			}
		}
	}

	return finalUsage, nil
}
