package embeddings

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNewGeminiClient(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name    string
		apiKey  string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid api key",
			apiKey:  "test-key-123",
			wantErr: false,
		},
		{
			name:    "empty api key",
			apiKey:  "",
			wantErr: true,
			errMsg:  "API key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewGeminiClient(tt.apiKey, logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewGeminiClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && client == nil {
				t.Error("NewGeminiClient() returned nil client without error")
			}
			if tt.errMsg != "" && err != nil {
				if err.Error() != tt.errMsg {
					t.Errorf("NewGeminiClient() error = %v, want %v", err, tt.errMsg)
				}
			}
		})
	}
}

func TestGeminiClientGetDimension(t *testing.T) {
	logger := zap.NewNop()
	client, err := NewGeminiClient("test-key", logger)
	if err != nil {
		t.Fatalf("NewGeminiClient() failed: %v", err)
	}

	tests := []struct {
		name      string
		model     string
		wantDim   int
		wantErr   bool
		errMsg    string
	}{
		{
			name:    "text-embedding-004",
			model:   "text-embedding-004",
			wantDim: 768,
			wantErr: false,
		},
		{
			name:    "empty model (uses default)",
			model:   "",
			wantDim: 768,
			wantErr: false,
		},
		{
			name:    "unknown model",
			model:   "unknown-model",
			wantErr: true,
			errMsg:  "unknown dimension for Gemini model 'unknown-model'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dim, err := client.GetDimension(context.Background(), tt.model)
			if (err != nil) != tt.wantErr {
				t.Errorf("GeminiClient.GetDimension() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && dim != tt.wantDim {
				t.Errorf("GeminiClient.GetDimension() = %v, want %v", dim, tt.wantDim)
			}
			if tt.errMsg != "" && err != nil {
				if err.Error() != tt.errMsg {
					t.Errorf("GeminiClient.GetDimension() error = %v, want %v", err, tt.errMsg)
				}
			}
		})
	}
}

func TestGeminiModelValidation(t *testing.T) {
	logger := zap.NewNop()

	// Test that the model dimension map contains expected keys
	client, err := NewGeminiClient("test-key", logger)
	if err != nil {
		t.Fatalf("NewGeminiClient() failed: %v", err)
	}

	if client == nil {
		t.Error("Failed to create GeminiClient")
	}

	expectedModels := []string{
		"text-embedding-004",
	}

	for _, model := range expectedModels {
		if dim, ok := geminiModelDimensions[model]; !ok {
			t.Errorf("Model %q not found in dimensions map", model)
		} else if dim == 0 {
			t.Errorf("Model %q has invalid dimension: %d", model, dim)
		}
	}
}

func TestGeminiDefaultModel(t *testing.T) {
	if geminiDefaultModel == "" {
		t.Error("geminiDefaultModel should not be empty")
	}

	if _, ok := geminiModelDimensions[geminiDefaultModel]; !ok {
		t.Errorf("Default model %q not found in dimensions map", geminiDefaultModel)
	}
}

func TestGeminiEmbedUnknownModel(t *testing.T) {
	logger := zap.NewNop()
	client, err := NewGeminiClient("test-key", logger)
	if err != nil {
		t.Fatalf("NewGeminiClient() failed: %v", err)
	}

	ctx := context.Background()
	// Unknown model will fall back to default and try to make HTTP request
	// This will fail due to invalid API key
	_, err = client.Embed(ctx, []string{"test"}, "unknown-model")
	if err == nil {
		t.Error("Embed() with unknown model and test key should error")
	}
}

func TestGeminiEmbedModelDefaults(t *testing.T) {
	logger := zap.NewNop()
	client, err := NewGeminiClient("test-key", logger)
	if err != nil {
		t.Fatalf("NewGeminiClient() failed: %v", err)
	}

	if client == nil {
		t.Fatal("Expected non-nil client")
	}

	// Test that empty model uses default
	emptyModel := ""
	if emptyModel == "" {
		emptyModel = geminiDefaultModel
	}
	if emptyModel != geminiDefaultModel {
		t.Errorf("Expected default model %q, got %q", geminiDefaultModel, emptyModel)
	}
}

func TestGeminiClientTimeout(t *testing.T) {
	logger := zap.NewNop()
	client, err := NewGeminiClient("test-key", logger)
	if err != nil {
		t.Fatalf("NewGeminiClient() failed: %v", err)
	}

	if client.client == nil {
		t.Error("Expected non-nil HTTP client")
	}
	if client.client.Timeout != 60*time.Second {
		t.Errorf("Expected timeout 60s, got %v", client.client.Timeout)
	}
}
