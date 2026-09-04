package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type OpenAIProvider struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewOpenAIProvider(apiKey, model string, timeout time.Duration) *OpenAIProvider {
	return &OpenAIProvider{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (p *OpenAIProvider) Name() string { return p.model }

type openAIRequest struct {
	Model       string              `json:"model"`
	Temperature float64             `json:"temperature,omitempty"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Messages    []map[string]string `json:"messages"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (p *OpenAIProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResult, error) {
	if p.apiKey == "" {
		return CompletionResult{}, fmt.Errorf("OPENAI_API_KEY is empty")
	}
	body := openAIRequest{
		Model:       p.model,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Messages: []map[string]string{
			{"role": "system", "content": req.System},
			{"role": "user", "content": req.User},
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return CompletionResult{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return CompletionResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return CompletionResult{}, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return CompletionResult{}, err
	}
	var parsed openAIResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return CompletionResult{}, err
	}
	if parsed.Error != nil {
		return CompletionResult{}, fmt.Errorf("openai: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return CompletionResult{}, fmt.Errorf("openai: empty choices (%s)", truncate(string(data), 300))
	}
	return CompletionResult{Text: parsed.Choices[0].Message.Content, Model: p.model}, nil
}
