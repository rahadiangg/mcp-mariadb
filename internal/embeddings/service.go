package embeddings

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// Service provides embedding generation capabilities
type Service struct {
	provider string
	client   EmbeddingClient
	logger   *zap.Logger
}

// Config holds configuration for embedding service
type Config struct {
	OpenAIKey string
	GeminiKey string
}

// NewService creates a new embedding service
func NewService(provider string, cfg Config, logger *zap.Logger) (*Service, error) {
	s := &Service{
		provider: provider,
		logger:   logger,
	}

	switch provider {
	case "openai":
		if cfg.OpenAIKey == "" {
			return nil, fmt.Errorf("OpenAI API key is required for openai provider")
		}
		client, err := NewOpenAIClient(cfg.OpenAIKey, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create OpenAI client: %w", err)
		}
		s.client = client

	case "gemini":
		if cfg.GeminiKey == "" {
			return nil, fmt.Errorf("Gemini API key is required for gemini provider")
		}
		client, err := NewGeminiClient(cfg.GeminiKey, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create Gemini client: %w", err)
		}
		s.client = client

	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", provider)
	}

	logger.Info("Embedding service initialized", zap.String("provider", provider))
	return s, nil
}

// Embed generates embeddings for the given texts
func (s *Service) Embed(ctx context.Context, texts []string, model string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("texts must not be empty")
	}

	s.logger.Debug("Generating embeddings",
		zap.String("provider", s.provider),
		zap.String("model", model),
		zap.Int("count", len(texts)),
	)

	return s.client.Embed(ctx, texts, model)
}

// GetDimension returns the embedding dimension for the given model
func (s *Service) GetDimension(ctx context.Context, model string) (int, error) {
	s.logger.Debug("Getting embedding dimension",
		zap.String("provider", s.provider),
		zap.String("model", model),
	)

	return s.client.GetDimension(ctx, model)
}

// EmbeddingClient is the interface for embedding providers
type EmbeddingClient interface {
	Embed(ctx context.Context, texts []string, model string) ([][]float64, error)
	GetDimension(ctx context.Context, model string) (int, error)
}
