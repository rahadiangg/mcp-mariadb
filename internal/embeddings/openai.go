package embeddings

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// OpenAI client constants
const (
	openaiDefaultModel = "text-embedding-3-small"
	openaiEndpoint     = "https://api.openai.com/v1/embeddings"
)

// openaiModelDimensions maps OpenAI model names to their dimensions
var openaiModelDimensions = map[string]int{
	"text-embedding-3-small": 1536,
	"text-embedding-3-large": 3072,
	"text-embedding-ada-002": 1536,
}

// OpenAIClient implements embedding generation using OpenAI's API
type OpenAIClient struct {
	apiKey string
	client *http.Client
	logger *zap.Logger
}

// NewOpenAIClient creates a new OpenAI embedding client
func NewOpenAIClient(apiKey string, logger *zap.Logger) (*OpenAIClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	return &OpenAIClient{
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		logger: logger,
	}, nil
}

// openAIEmbeddingRequest is the request body for OpenAI embeddings API
type openAIEmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// openAIEmbeddingResponse is the response from OpenAI embeddings API
type openAIEmbeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// Embed generates embeddings for the given texts using OpenAI's API
func (c *OpenAIClient) Embed(ctx context.Context, texts []string, model string) ([][]float64, error) {
	if model == "" {
		model = openaiDefaultModel
	}

	// Validate model
	if _, ok := openaiModelDimensions[model]; !ok {
		c.logger.Warn("Unknown OpenAI model, using default", zap.String("model", model))
		model = openaiDefaultModel
	}

	// Build request
	reqBody := openAIEmbeddingRequest{
		Model: model,
		Input: texts,
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", openaiEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	// Use a custom reader to avoid loading entire body in memory
	// (for production, you'd want to handle this more carefully)
	req.Body = io.NopCloser(strings.NewReader(string(reqJSON)))

	// Execute request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var result openAIEmbeddingResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract embeddings
	embeddings := make([][]float64, len(texts))
	for _, item := range result.Data {
		if item.Index >= 0 && item.Index < len(embeddings) {
			embeddings[item.Index] = item.Embedding
		}
	}

	c.logger.Debug("OpenAI embeddings generated",
		zap.Int("count", len(embeddings)),
		zap.Int("dimension", len(embeddings[0])),
	)

	return embeddings, nil
}

// GetDimension returns the embedding dimension for the given model
func (c *OpenAIClient) GetDimension(ctx context.Context, model string) (int, error) {
	if model == "" {
		model = openaiDefaultModel
	}

	dim, ok := openaiModelDimensions[model]
	if !ok {
		return 0, fmt.Errorf("unknown dimension for OpenAI model '%s'", model)
	}

	return dim, nil
}
