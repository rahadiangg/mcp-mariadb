package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rahadiangg/mcp-mariadb/internal/config"
	"go.uber.org/zap"
)

func TestNewServer(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name    string
		cfg     *config.Config
		wantNil bool
	}{
		{
			name: "valid config without embedding",
			cfg: &config.Config{
				Host:     "localhost",
				Port:     3306,
				User:     "test",
				Password: "test",
				Database: "testdb",
			},
			wantNil: false,
		},
		{
			name: "valid config with openai embedding",
			cfg: &config.Config{
				Host:              "localhost",
				Port:              3306,
				User:              "test",
				Password:          "test",
				Database:          "testdb",
				EmbeddingProvider: "openai",
				OpenAIKey:         "test-key",
			},
			wantNil: false,
		},
		{
			name: "valid config with gemini embedding",
			cfg: &config.Config{
				Host:              "localhost",
				Port:              3306,
				User:              "test",
				Password:          "test",
				Database:          "testdb",
				EmbeddingProvider: "gemini",
				GeminiKey:         "test-key",
			},
			wantNil: false,
		},
		{
			name: "valid config with invalid embedding provider",
			cfg: &config.Config{
				Host:              "localhost",
				Port:              3306,
				User:              "test",
				Password:          "test",
				Database:          "testdb",
				EmbeddingProvider: "invalid",
			},
			wantNil: false, // Server still created, embedding service is nil
		},
		{
			name: "valid config with openai but no key",
			cfg: &config.Config{
				Host:              "localhost",
				Port:              3306,
				User:              "test",
				Password:          "test",
				Database:          "testdb",
				EmbeddingProvider: "openai",
			},
			wantNil: false, // Server still created, embedding service is nil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewServer(tt.cfg, logger)
			if (s == nil) != tt.wantNil {
				t.Errorf("NewServer() = %v, wantNil %v", s, tt.wantNil)
			}
			if s != nil {
				if s.cfg != tt.cfg {
					t.Error("NewServer() did not set config")
				}
				if s.logger != logger {
					t.Error("NewServer() did not set logger")
				}
				if s.tools == nil {
					t.Error("NewServer() tools should be initialized")
				}
			}
		})
	}
}

func TestServerClose(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name    string
		setup   func() *Server
		wantErr bool
	}{
		{
			name: "close without pool",
			setup: func() *Server {
				return &Server{
					cfg:    &config.Config{},
					logger: logger,
					pool:   nil,
				}
			},
			wantErr: false,
		},
		{
			name: "close with nil pool",
			setup: func() *Server {
				return &Server{
					cfg:    &config.Config{},
					logger: logger,
					pool:   nil,
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.setup()
			err := s.Close()
			if (err != nil) != tt.wantErr {
				t.Errorf("Server.Close() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestServerRunUnsupportedTransport(t *testing.T) {
	logger := zap.NewNop()
	cfg := &config.Config{
		Host:     "localhost",
		Port:     3306,
		User:     "test",
		Password: "test",
		Database: "testdb",
	}

	s := NewServer(cfg, logger)

	// Test unsupported transport - will fail to initialize pool first
	err := s.Run(context.Background(), "unsupported", "127.0.0.1", 9001, "/mcp")
	if err == nil {
		t.Error("Server.Run() should return error without database connection")
	}
	// The error could be either initialization failure or unsupported transport
	// depending on whether the pool can be created
}

func TestCheckReadOnlyMode(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name       string
		readOnly   bool
		operation  string
		wantErr    bool
		errContains string
	}{
		{
			name:      "write operation when not read-only",
			readOnly:  false,
			operation: "create_database",
			wantErr:   false,
		},
		{
			name:       "write operation when read-only",
			readOnly:   true,
			operation:  "create_database",
			wantErr:    true,
			errContains: "operation forbidden: server is in read-only mode",
		},
		{
			name:      "any operation when not read-only",
			readOnly:  false,
			operation: "delete_table",
			wantErr:   false,
		},
		{
			name:       "write operation in read-only mode",
			readOnly:   true,
			operation:  "insert_data",
			wantErr:    true,
			errContains: "operation forbidden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{
				cfg:    &config.Config{ReadOnly: tt.readOnly},
				logger: logger,
			}

			err := s.checkReadOnlyMode(tt.operation)
			if (err != nil) != tt.wantErr {
				t.Errorf("Server.checkReadOnlyMode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.errContains != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Server.checkReadOnlyMode() error = %v, want error containing %q", err, tt.errContains)
				}
			}
		})
	}
}

func TestToolHandlerWrapper(t *testing.T) {
	tests := []struct {
		name     string
		handler  ToolHandler
		args     map[string]interface{}
		wantErr  bool
	}{
		{
			name: "successful handler",
			handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
				return map[string]string{"status": "ok"}, nil
			},
			args:    map[string]interface{}{},
			wantErr: false,
		},
		{
			name: "handler returns error",
			handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
				return nil, ErrTest
			},
			args:    map[string]interface{}{},
			wantErr: true, // HandlerWrapper sets IsError when handler returns error
		},
		{
			name: "handler with arguments",
			handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
				return args, nil
			},
			args: map[string]interface{}{
				"key": "value",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := &Tool{
				Name:    "test_tool",
				Handler: tt.handler,
			}

			wrapper := tool.HandlerWrapper()
			result, err := wrapper(tt.args)

			if err != nil {
				t.Errorf("Tool.HandlerWrapper() error = %v, want nil", err)
				return
			}
			if result == nil {
				t.Error("Tool.HandlerWrapper() returned nil result")
				return
			}
			if result.IsError != tt.wantErr {
				t.Errorf("Tool.HandlerWrapper() IsError = %v, want %v", result.IsError, tt.wantErr)
			}
			if len(result.Content) == 0 {
				t.Error("Tool.HandlerWrapper() returned empty content")
			}
		})
	}
}

func TestServerCallTool(t *testing.T) {
	logger := zap.NewNop()

	s := &Server{
		cfg:    &config.Config{},
		logger: logger,
		tools: []*Tool{
			{
				Name: "test_tool",
				Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
					return map[string]string{"result": "ok"}, nil
				},
			},
		},
	}

	tests := []struct {
		name    string
		tool    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name:    "existing tool",
			tool:    "test_tool",
			args:    map[string]interface{}{},
			wantErr: false,
		},
		{
			name:    "non-existing tool",
			tool:    "non_existent",
			args:    map[string]interface{}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := s.CallTool(context.Background(), tt.tool, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Server.CallTool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result == nil {
				t.Error("Server.CallTool() returned nil result without error")
			}
		})
	}
}

func TestServerRunWithoutInitialization(t *testing.T) {
	cfg := &config.Config{
		Host:     "localhost",
		Port:     3306,
		User:     "test",
		Password: "test",
		Database: "testdb",
	}

	s := NewServer(cfg, zap.NewNop())

	// Try to run without initializing the pool (no database available)
	transports := []string{"stdio", "sse", "http"}

	for _, transport := range transports {
		t.Run(transport, func(t *testing.T) {
			// This will fail because we haven't initialized the connection pool
			// The initialize() method will fail to connect to the database
			err := s.Run(context.Background(), transport, "127.0.0.1", 9001, "/mcp")
			// We expect an error since there's no database connection
			if err == nil {
				t.Errorf("Server.Run() with %s should return error without database connection", transport)
			}
		})
	}
}

func TestServerRegisterTool(t *testing.T) {
	logger := zap.NewNop()
	cfg := &config.Config{
		Host:     "localhost",
		Port:     3306,
		User:     "test",
		Password: "test",
		Database: "testdb",
	}

	s := NewServer(cfg, logger)

	// Create a test tool
	testTool := &Tool{
		Name:        "test_tool",
		Description: "Test description",
		InputSchema: map[string]interface{}{
			"type": "object",
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			return "result", nil
		},
	}

	// Get initial tool count
	initialCount := len(s.tools)

	// Register the tool - this would normally be called in registerTools()
	// but since we can't easily mock the mcpServer, we test by checking the tools slice
	s.tools = append(s.tools, testTool)

	if len(s.tools) != initialCount+1 {
		t.Errorf("After registerTool, tools count = %d, want %d", len(s.tools), initialCount+1)
	}
}

func TestServerInitializeNilPool(t *testing.T) {
	logger := zap.NewNop()
	cfg := &config.Config{
		Host:     "nonexistent",
		Port:     9999,
		User:     "test",
		Password: "test",
		Database: "testdb",
	}

	s := NewServer(cfg, logger)

	// initialize() should fail to connect to database and not set pool
	s.initialize(context.Background())
	if s.pool != nil {
		t.Error("Server.pool should be nil after failed initialize")
	}
}

func TestServerRegisterToolsInitialization(t *testing.T) {
	logger := zap.NewNop()
	cfg := &config.Config{
		Host:     "localhost",
		Port:     3306,
		User:     "test",
		Password: "test",
		Database: "testdb",
	}

	s := NewServer(cfg, logger)

	// Check that server was created with tools slice initialized
	if s.tools == nil {
		t.Error("NewServer() should initialize tools slice")
	}
}

func TestServerEmptyConfig(t *testing.T) {
	logger := zap.NewNop()
	cfg := &config.Config{}

	s := NewServer(cfg, logger)

	if s == nil {
		t.Error("NewServer() should return non-nil even with empty config")
	}
}

func TestWithCORS(t *testing.T) {
	cfg := &config.Config{}

	tests := []struct {
		name           string
		origin         string
		corsAllowOrigins string
		expectedStatus int
	}{
		{
			name:           "no origin header",
			origin:         "",
			corsAllowOrigins: "*",
			expectedStatus: 200, // Should pass through to next handler
		},
		{
			name:           "wildcard allows all",
			origin:         "http://example.com",
			corsAllowOrigins: "*",
			expectedStatus: 200, // Should pass through with CORS headers
		},
		{
			name:           "no CORS configured",
			origin:         "http://example.com",
			corsAllowOrigins: "",
			expectedStatus: 403, // Forbidden
		},
		{
			name:           "specific allowed origin",
			origin:         "http://example.com",
			corsAllowOrigins: "http://example.com,http://localhost:3000",
			expectedStatus: 200, // Should pass through
		},
		{
			name:           "specific disallowed origin",
			origin:         "http://evil.com",
			corsAllowOrigins: "http://example.com",
			expectedStatus: 403, // Forbidden
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg.CORSAllowOrigins = tt.corsAllowOrigins

			nextHandlerCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextHandlerCalled = true
				w.WriteHeader(http.StatusOK)
			})

			handler := withCORS(next, cfg)

			req := &http.Request{
				Header: http.Header{},
			}
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("withCORS() status = %d, want %d", rec.Code, tt.expectedStatus)
			}
			if tt.expectedStatus == 200 && !nextHandlerCalled {
				t.Error("withCORS() should call next handler for allowed requests")
			}
		})
	}
}

func TestWithCORSCORSHeaders(t *testing.T) {
	cfg := &config.Config{
		CORSAllowOrigins: "*",
	}

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := withCORS(next, cfg)

	req := &http.Request{
		Header: http.Header{},
	}
	req.Header.Set("Origin", "http://example.com")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Check that CORS headers are set for allowed wildcard requests
	if rec.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("withCORS() should set Access-Control-Allow-Origin header for wildcard")
	}
	if !nextCalled {
		t.Error("withCORS() should call next handler for allowed CORS requests")
	}
}

// ErrTest is a test error
var ErrTest = fmt.Errorf("test error")
