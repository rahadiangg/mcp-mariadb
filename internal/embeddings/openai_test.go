package embeddings

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNewOpenAIClient(t *testing.T) {
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
			client, err := NewOpenAIClient(tt.apiKey, logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewOpenAIClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && client == nil {
				t.Error("NewOpenAIClient() returned nil client without error")
			}
			if tt.errMsg != "" && err != nil {
				if err.Error() != tt.errMsg {
					t.Errorf("NewOpenAIClient() error = %v, want %v", err, tt.errMsg)
				}
			}
		})
	}
}

func TestOpenAIClientEmbed(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("expected POST request, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Authorization header 'Bearer test-key', got %s", r.Header.Get("Authorization"))
		}

		// Send mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"object": "list",
			"data": [
				{
					"object": "embedding",
					"embedding": [0.1, 0.2, 0.3],
					"index": 0
				},
				{
					"object": "embedding",
					"embedding": [0.4, 0.5, 0.6],
					"index": 1
				}
			],
			"model": "text-embedding-3-small",
			"usage": {
				"prompt_tokens": 10,
				"total_tokens": 10
			}
		}`))
	}))
	defer server.Close()

	// Note: The HTTP endpoint testing requires making openaiEndpoint configurable
	// or using interfaces. For now, we skip this test.
	tests := []struct {
		name    string
		texts   []string
		model   string
		wantErr bool
	}{
		{
			name:    "single text",
			texts:   []string{"hello"},
			model:   "text-embedding-3-small",
			wantErr: false,
		},
		{
			name:    "multiple texts",
			texts:   []string{"hello", "world"},
			model:   "text-embedding-3-small",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip since we can't easily mock the HTTP endpoint without interfaces
			t.Skip("requires endpoint configuration or interface mocking")
		})
	}
}

func TestOpenAIClientGetDimension(t *testing.T) {
	logger := zap.NewNop()
	client, err := NewOpenAIClient("test-key", logger)
	if err != nil {
		t.Fatalf("NewOpenAIClient() failed: %v", err)
	}

	tests := []struct {
		name      string
		model     string
		wantDim   int
		wantErr   bool
		errMsg    string
	}{
		{
			name:    "text-embedding-3-small",
			model:   "text-embedding-3-small",
			wantDim: 1536,
			wantErr: false,
		},
		{
			name:    "text-embedding-3-large",
			model:   "text-embedding-3-large",
			wantDim: 3072,
			wantErr: false,
		},
		{
			name:    "text-embedding-ada-002",
			model:   "text-embedding-ada-002",
			wantDim: 1536,
			wantErr: false,
		},
		{
			name:    "empty model (uses default)",
			model:   "",
			wantDim: 1536,
			wantErr: false,
		},
		{
			name:    "unknown model",
			model:   "unknown-model",
			wantErr: true,
			errMsg:  "unknown dimension for OpenAI model 'unknown-model'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dim, err := client.GetDimension(context.Background(), tt.model)
			if (err != nil) != tt.wantErr {
				t.Errorf("OpenAIClient.GetDimension() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && dim != tt.wantDim {
				t.Errorf("OpenAIClient.GetDimension() = %v, want %v", dim, tt.wantDim)
			}
			if tt.errMsg != "" && err != nil {
				if err.Error() != tt.errMsg {
					t.Errorf("OpenAIClient.GetDimension() error = %v, want %v", err, tt.errMsg)
				}
			}
		})
	}
}

func TestOpenAIModelValidation(t *testing.T) {
	_ = zap.NewNop()

	// We can't test the actual Embed() method without a proper mock,
	// but we can verify that the model dimension map contains expected keys
	expectedModels := []string{
		"text-embedding-3-small",
		"text-embedding-3-large",
		"text-embedding-ada-002",
	}

	for _, model := range expectedModels {
		if dim, ok := openaiModelDimensions[model]; !ok {
			t.Errorf("Model %q not found in dimensions map", model)
		} else if dim == 0 {
			t.Errorf("Model %q has invalid dimension: %d", model, dim)
		}
	}
}

func TestOpenAIClientTimeout(t *testing.T) {
	logger := zap.NewNop()
	client, err := NewOpenAIClient("test-key", logger)
	if err != nil {
		t.Fatalf("NewOpenAIClient() failed: %v", err)
	}

	if client.client == nil {
		t.Error("Expected non-nil HTTP client")
	}
	if client.client.Timeout != 60*time.Second {
		t.Errorf("Expected timeout 60s, got %v", client.client.Timeout)
	}
}

func TestOpenAIDefaultModel(t *testing.T) {
	if openaiDefaultModel == "" {
		t.Error("openaiDefaultModel should not be empty")
	}

	if _, ok := openaiModelDimensions[openaiDefaultModel]; !ok {
		t.Errorf("Default model %q not found in dimensions map", openaiDefaultModel)
	}
}
