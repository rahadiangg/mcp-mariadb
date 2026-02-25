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

// Gemini client constants
const (
	geminiDefaultModel = "text-embedding-004"
	geminiEndpoint     = "https://generativelanguage.googleapis.com/v1/models"
)

// geminiModelDimensions maps Gemini model names to their dimensions
var geminiModelDimensions = map[string]int{
	"text-embedding-004": 768,
}

// GeminiClient implements embedding generation using Google's Gemini API
type GeminiClient struct {
	apiKey string
	client *http.Client
	logger *zap.Logger
}

// NewGeminiClient creates a new Gemini embedding client
func NewGeminiClient(apiKey string, logger *zap.Logger) (*GeminiClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	return &GeminiClient{
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		logger: logger,
	}, nil
}

// geminiEmbeddingRequest is the request body for Gemini embeddings API
type geminiEmbeddingRequest struct {
	Content struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"content"`
}

// geminiEmbeddingResponse is the response from Gemini embeddings API
type geminiEmbeddingResponse struct {
	Embedding struct {
		Values []float64 `json:"values"`
	} `json:"embedding"`
}

// Embed generates embeddings for the given texts using Gemini's API
func (c *GeminiClient) Embed(ctx context.Context, texts []string, model string) ([][]float64, error) {
	if model == "" {
		model = geminiDefaultModel
	}

	// Validate model
	if _, ok := geminiModelDimensions[model]; !ok {
		c.logger.Warn("Unknown Gemini model, using default", zap.String("model", model))
		model = geminiDefaultModel
	}

	// Gemini requires separate requests per text (batch not supported in current API)
	embeddings := make([][]float64, len(texts))

	for i, text := range texts {
		url := fmt.Sprintf("%s/%s:embedContent?key=%s", geminiEndpoint, model, c.apiKey)

		reqBody := geminiEmbeddingRequest{}
		reqBody.Content.Parts = []struct {
			Text string `json:"text"`
		}{{Text: text}}

		reqJSON, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request for text %d: %w", i, err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request for text %d: %w", i, err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Body = io.NopCloser(strings.NewReader(string(reqJSON)))

		resp, err := c.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to execute request for text %d: %w", i, err)
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read response for text %d: %w", i, err)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("Gemini API error for text %d (status %d): %s", i, resp.StatusCode, string(respBody))
		}

		var result geminiEmbeddingResponse
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("failed to parse response for text %d: %w", i, err)
		}

		embeddings[i] = result.Embedding.Values
	}

	c.logger.Debug("Gemini embeddings generated",
		zap.Int("count", len(embeddings)),
		zap.Int("dimension", len(embeddings[0])),
	)

	return embeddings, nil
}

// GetDimension returns the embedding dimension for the given model
func (c *GeminiClient) GetDimension(ctx context.Context, model string) (int, error) {
	if model == "" {
		model = geminiDefaultModel
	}

	dim, ok := geminiModelDimensions[model]
	if !ok {
		return 0, fmt.Errorf("unknown dimension for Gemini model '%s'", model)
	}

	return dim, nil
}
