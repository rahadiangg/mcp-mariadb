package embeddings

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"
)

// mockEmbeddingClient is a test double for EmbeddingClient
type mockEmbeddingClient struct {
	embedFunc       func(ctx context.Context, texts []string, model string) ([][]float64, error)
	dimensionFunc   func(ctx context.Context, model string) (int, error)
}

func (m *mockEmbeddingClient) Embed(ctx context.Context, texts []string, model string) ([][]float64, error) {
	if m.embedFunc != nil {
		return m.embedFunc(ctx, texts, model)
	}
	// Default mock implementation
	result := make([][]float64, len(texts))
	for i := range result {
		result[i] = []float64{0.1, 0.2, 0.3}
	}
	return result, nil
}

func (m *mockEmbeddingClient) GetDimension(ctx context.Context, model string) (int, error) {
	if m.dimensionFunc != nil {
		return m.dimensionFunc(ctx, model)
	}
	return 3, nil
}

func TestNewService(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name    string
		provider string
		cfg      Config
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "empty provider",
			provider: "",
			cfg:      Config{},
			wantErr:  true,
			errMsg:   "unsupported embedding provider",
		},
		{
			name:     "unsupported provider",
			provider: "unknown",
			cfg:      Config{},
			wantErr:  true,
			errMsg:   "unsupported embedding provider",
		},
		{
			name:     "openai without key",
			provider: "openai",
			cfg:      Config{},
			wantErr:  true,
			errMsg:   "OpenAI API key is required",
		},
		{
			name:     "gemini without key",
			provider: "gemini",
			cfg:      Config{},
			wantErr:  true,
			errMsg:   "Gemini API key is required",
		},
		{
			name:     "openai with valid key",
			provider: "openai",
			cfg:      Config{
				OpenAIKey: "sk-test-key-12345",
			},
			wantErr: false,
		},
		{
			name:     "gemini with valid key",
			provider: "gemini",
			cfg:      Config{
				GeminiKey: "AIzaSyD-test-key",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := NewService(tt.provider, tt.cfg, logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if svc == nil {
					t.Error("NewService() returned nil service without error")
				}
				if svc.provider != tt.provider {
					t.Errorf("NewService() provider = %v, want %v", svc.provider, tt.provider)
				}
				if svc.client == nil {
					t.Error("NewService() client should not be nil")
				}
			}
			if err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("NewService() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestServiceEmbed(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name      string
		texts     []string
		model     string
		setupMock func(*mockEmbeddingClient)
		wantErr   bool
	}{
		{
			name:  "empty texts",
			texts: []string{},
			model: "test-model",
			setupMock: func(m *mockEmbeddingClient) {
				// No setup needed, should error before calling client
			},
			wantErr: true,
		},
		{
			name:  "nil texts",
			texts: nil,
			model: "test-model",
			setupMock: func(m *mockEmbeddingClient) {
				// No setup needed
			},
			wantErr: true,
		},
		{
			name:  "single text",
			texts: []string{"hello"},
			model: "test-model",
			setupMock: func(m *mockEmbeddingClient) {
				m.embedFunc = func(ctx context.Context, texts []string, model string) ([][]float64, error) {
					return [][]float64{{0.1, 0.2}}, nil
				}
			},
			wantErr: false,
		},
		{
			name:  "multiple texts",
			texts: []string{"hello", "world"},
			model: "test-model",
			setupMock: func(m *mockEmbeddingClient) {
				m.embedFunc = func(ctx context.Context, texts []string, model string) ([][]float64, error) {
					return [][]float64{{0.1, 0.2}, {0.3, 0.4}}, nil
				}
			},
			wantErr: false,
		},
		{
			name:  "client error",
			texts: []string{"test"},
			model: "test-model",
			setupMock: func(m *mockEmbeddingClient) {
				m.embedFunc = func(ctx context.Context, texts []string, model string) ([][]float64, error) {
					return nil, errors.New("API error")
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockEmbeddingClient{}
			if tt.setupMock != nil {
				tt.setupMock(mock)
			}

			s := &Service{
				provider: "test",
				client:   mock,
				logger:   logger,
			}

			got, err := s.Embed(context.Background(), tt.texts, tt.model)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service.Embed() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == nil {
				t.Error("Service.Embed() returned nil result without error")
			}
		})
	}
}

func TestServiceGetDimension(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name      string
		model     string
		dimension int
		setupMock func(*mockEmbeddingClient)
		wantErr   bool
	}{
		{
			name:     "valid dimension",
			model:    "test-model",
			dimension: 1536,
			setupMock: func(m *mockEmbeddingClient) {
				m.dimensionFunc = func(ctx context.Context, model string) (int, error) {
					return 1536, nil
				}
			},
			wantErr: false,
		},
		{
			name:  "client error",
			model: "test-model",
			setupMock: func(m *mockEmbeddingClient) {
				m.dimensionFunc = func(ctx context.Context, model string) (int, error) {
					return 0, errors.New("API error")
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockEmbeddingClient{}
			if tt.setupMock != nil {
				tt.setupMock(mock)
			}

			s := &Service{
				provider: "test",
				client:   mock,
				logger:   logger,
			}

			got, err := s.GetDimension(context.Background(), tt.model)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service.GetDimension() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.dimension {
				t.Errorf("Service.GetDimension() = %v, want %v", got, tt.dimension)
			}
		})
	}
}

// Helper function for string matching
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
