package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ayse-solmaz/azula/internal/config"
	"github.com/ayse-solmaz/azula/internal/domain"
)

type Router struct {
	cfg    config.Config
	client *http.Client
	mu     sync.Mutex
	busy   int
	slots  int
}

func NewRouter(cfg config.Config) *Router {
	return &Router{
		cfg:    cfg,
		client: &http.Client{Timeout: cfg.RequestTimeout},
		slots:  cfg.WorkerSlots,
	}
}

func (r *Router) Busy() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.busy
}

func (r *Router) Slots() int {
	return r.slots
}

func (r *Router) Acquire() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.busy >= r.slots {
		return domain.ErrBusy
	}
	r.busy++
	return nil
}

func (r *Router) Release() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.busy > 0 {
		r.busy--
	}
}

func (r *Router) CompleteJSON(ctx context.Context, cfg domain.ModelConfig, slot, system, user string) (string, error) {
	return r.completeJSON(ctx, cfg, slot, "", system, user)
}

func (r *Router) completeJSON(ctx context.Context, cfg domain.ModelConfig, slot, modelOverride, system, user string) (string, error) {
	provider, model := r.resolveSlot(ctx, cfg, slot, modelOverride)
	temp := cfg.Temperature
	if temp <= 0 {
		temp = 0.2
	}
	switch strings.ToLower(provider) {
	case "openai":
		return r.openai(ctx, model, system, user, temp, cfg.MaxTokens)
	default:
		return r.ollama(ctx, model, system, user, temp)
	}
}

func (r *Router) resolveSlot(ctx context.Context, cfg domain.ModelConfig, slot, modelOverride string) (provider, model string) {
	slot = strings.ToUpper(strings.TrimSpace(slot))
	switch slot {
	case "B":
		provider, model = cfg.ModelBProvider, cfg.ModelBName
		if modelOverride != "" {
			model = modelOverride
			break
		}
		if names, err := ListOllamaModels(ctx, r.cfg.OllamaBaseURL); err == nil {
			model = PickModelB(names, model)
		}
	case "C":
		provider = cfg.ModelCProvider
		model = cfg.ModelCName
		if provider == "" {
			provider = r.cfg.ModelCProvider
		}
		if model == "" {
			model = r.cfg.ModelCName
		}
		if provider == "" {
			provider = "openai"
		}
		if model == "" {
			model = "gpt-4o-mini"
		}
		if modelOverride != "" {
			model = modelOverride
		}
		if strings.EqualFold(provider, "openai") && r.cfg.OpenAIKey == "" {
			return r.resolveSlot(ctx, cfg, "A", "")
		}
	default:
		provider, model = cfg.ModelAProvider, cfg.ModelAName
		if modelOverride != "" {
			model = modelOverride
		}
	}
	if provider == "" {
		provider = "ollama"
	}
	return provider, model
}

func (r *Router) ollama(ctx context.Context, model, system, user string, temp float64) (string, error) {
	body := map[string]any{
		"model":  model,
		"stream": false,
		"options": map[string]any{
			"temperature": temp,
		},
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(r.cfg.OllamaBaseURL, "/")+"/api/chat", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("ollama: %s", strings.TrimSpace(string(b)))
	}
	var parsed struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		return "", err
	}
	return parsed.Message.Content, nil
}

func (r *Router) openai(ctx context.Context, model, system, user string, temp float64, maxTokens int) (string, error) {
	if r.cfg.OpenAIKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY not set")
	}
	if maxTokens <= 0 {
		maxTokens = 1200
	}
	body := map[string]any{
		"model":       model,
		"temperature": temp,
		"max_tokens":  maxTokens,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+r.cfg.OpenAIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("openai: %s", strings.TrimSpace(string(b)))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("openai: empty response")
	}
	return parsed.Choices[0].Message.Content, nil
}

func SleepStep() {
	time.Sleep(200 * time.Millisecond)
}
