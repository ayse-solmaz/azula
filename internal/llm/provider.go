package llm

import "context"

type CompletionRequest struct {
	System      string
	User        string
	Temperature float64
	MaxTokens   int
}

type CompletionResult struct {
	Text  string
	Model string
}

type Provider interface {
	Name() string
	Complete(ctx context.Context, req CompletionRequest) (CompletionResult, error)
}
