package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type OllamaProvider struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

func NewOllamaProvider(baseURL, model string, timeout time.Duration) *OllamaProvider {
	return &OllamaProvider{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (p *OllamaProvider) Name() string { return p.model }

type ollamaChatRequest struct {
	Model    string              `json:"model"`
	Messages []map[string]string `json:"messages"`
	Stream   bool                `json:"stream"`
	Options  map[string]any      `json:"options,omitempty"`
}

type ollamaChatResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Error string `json:"error,omitempty"`
}

func (p *OllamaProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResult, error) {
	body := ollamaChatRequest{
		Model:  p.model,
		Stream: false,
		Messages: []map[string]string{
			{"role": "system", "content": req.System},
			{"role": "user", "content": req.User},
		},
		Options: map[string]any{},
	}
	if req.Temperature > 0 {
		body.Options["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 {
		body.Options["num_predict"] = req.MaxTokens
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return CompletionResult{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/chat", bytes.NewReader(raw))
	if err != nil {
		return CompletionResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return CompletionResult{}, fmt.Errorf("ollama %s: %w", p.model, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return CompletionResult{}, err
	}
	if resp.StatusCode >= 300 {
		return CompletionResult{}, fmt.Errorf("ollama %s HTTP %d: %s", p.model, resp.StatusCode, truncate(string(data), 400))
	}
	var parsed ollamaChatResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return CompletionResult{}, err
	}
	if parsed.Error != "" {
		return CompletionResult{}, fmt.Errorf("ollama: %s", parsed.Error)
	}
	return CompletionResult{Text: parsed.Message.Content, Model: p.model}, nil
}

type ollamaTags struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

func ListOllamaModels(ctx context.Context, baseURL string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var tags ollamaTags
	if err := json.Unmarshal(data, &tags); err != nil {
		return nil, err
	}
	var names []string
	for _, m := range tags.Models {
		names = append(names, m.Name)
	}
	return names, nil
}

func findOllamaModel(available []string, name string) string {
	if name == "" {
		return ""
	}
	for _, n := range available {
		if n == name || strings.HasPrefix(n, name+":") {
			return n
		}
	}
	return ""
}

func PickModelB(available []string, configured string) string {
	if configured != "" && configured != "qwen2.5:1.5b" {
		if hit := findOllamaModel(available, configured); hit != "" {
			return hit
		}
	}
	if hit := findOllamaModel(available, "azula-incident"); hit != "" {
		return hit
	}
	if hit := findOllamaModel(available, configured); hit != "" {
		return hit
	}
	if hit := findOllamaModel(available, "qwen2.5:1.5b"); hit != "" {
		return hit
	}
	if configured != "" {
		return configured
	}
	return "qwen2.5:1.5b"
}

type RuntimeStatus struct {
	Reachable          bool
	Models             []string
	IncidentModelReady bool
	AdapterOnDisk      bool
}

func AdapterOnDisk() bool {
	_, err := os.Stat(filepath.Join("adapters", "azula-incident", "merged-fp16", "model.safetensors"))
	return err == nil
}

func Probe(ctx context.Context, baseURL, modelB string) RuntimeStatus {
	st := RuntimeStatus{AdapterOnDisk: AdapterOnDisk(), Models: []string{}}
	names, err := ListOllamaModels(ctx, baseURL)
	if err != nil {
		return st
	}
	st.Reachable = true
	st.Models = names
	want := modelB
	if want == "" {
		want = "azula-incident"
	}
	st.IncidentModelReady = findOllamaModel(names, want) != "" || findOllamaModel(names, "azula-incident") != ""
	return st
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
