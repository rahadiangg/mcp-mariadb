package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rahadiangg/mcp-mariadb/internal/config"
	"github.com/rahadiangg/mcp-mariadb/internal/database"
	"github.com/rahadiangg/mcp-mariadb/internal/embeddings"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"
)

// Server represents the MariaDB MCP server
type Server struct {
	cfg              *config.Config
	logger           *zap.Logger
	pool             *database.SafePool
	embeddingService *embeddings.Service
	mcpServer        *server.MCPServer
	tools            []*Tool
	once             sync.Once
}

// NewServer creates a new MariaDB MCP server instance
func NewServer(cfg *config.Config, logger *zap.Logger) *Server {
	s := &Server{
		cfg:    cfg,
		logger: logger,
		tools:  make([]*Tool, 0),
	}

	// Initialize embedding service if configured
	if cfg.EmbeddingProvider != "" {
		var err error
		s.embeddingService, err = embeddings.NewService(cfg.EmbeddingProvider, embeddings.Config{
			OpenAIKey: cfg.OpenAIKey,
			GeminiKey: cfg.GeminiKey,
		}, logger)
		if err != nil {
			logger.Warn("Failed to initialize embedding service, vector features disabled",
				zap.Error(err),
			)
			s.embeddingService = nil
		} else {
			logger.Info("Embedding service initialized",
				zap.String("provider", cfg.EmbeddingProvider),
			)
		}
	}

	return s
}

// Run starts the MCP server with the specified transport
func (s *Server) Run(ctx context.Context, transport, host string, port int, path string) error {
	s.once.Do(func() {
		s.initialize(ctx)
	})
	if s.pool == nil {
		return fmt.Errorf("failed to initialize connection pool")
	}

	switch transport {
	case "stdio":
		return s.runStdio(ctx)
	case "sse":
		return s.runSSE(ctx, host, port)
	case "http":
		return s.runHTTP(ctx, host, port, path)
	default:
		return fmt.Errorf("unsupported transport: %s", transport)
	}
}

// initialize sets up the connection pool and registers tools
func (s *Server) initialize(ctx context.Context) {
	// Create connection pool
	dbCfg := &database.Config{
		Host:              s.cfg.Host,
		Port:              s.cfg.Port,
		User:              s.cfg.User,
		Password:          s.cfg.Password,
		Database:          s.cfg.Database,
		Charset:           s.cfg.Charset,
		SSL:               s.cfg.SSL,
		SSLCA:             s.cfg.SSLCA,
		SSLCert:           s.cfg.SSLCert,
		SSLKey:            s.cfg.SSLKey,
		SSLVerifyCert:     s.cfg.SSLVerifyCert,
		SSLVerifyIdentity: s.cfg.SSLVerifyIdentity,
		MaxOpenConns:      s.cfg.MaxPoolSize,
		ConnMaxLifetime:   3600 * time.Second,
		ConnMaxIdleTime:   time.Duration(s.cfg.ConnMaxIdleTime) * time.Second,
		ConnTimeout:       time.Duration(s.cfg.ConnTimeout) * time.Second,
		ReadTimeout:       time.Duration(s.cfg.ReadTimeout) * time.Second,
		WriteTimeout:      time.Duration(s.cfg.WriteTimeout) * time.Second,
	}

	pool, err := database.NewPool(ctx, dbCfg, s.logger)
	if err != nil {
		s.logger.Error("Failed to create connection pool", zap.Error(err))
		return
	}
	s.pool = pool

	// Create MCP server
	s.mcpServer = server.NewMCPServer(
		"mariadb-mcp-server",
		"0.1.0",
		server.WithToolCapabilities(true),
	)

	// Register all tools
	s.registerTools()

	s.logger.Info("Server initialized successfully",
		zap.Int("tools_registered", len(s.tools)),
	)
}

// registerTools registers all MCP tools
func (s *Server) registerTools() {
	// Core database tools (6 tools)
	s.registerTool(&Tool{
		Name:        "list_databases",
		Description: "Lists all accessible databases on the connected MariaDB server",
		Handler:     s.listDatabases,
		InputSchema: map[string]interface{}{},
	})

	s.registerTool(&Tool{
		Name:        "list_tables",
		Description: "Lists all tables within the specified database",
		Handler:     s.listTables,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"database_name": map[string]interface{}{
					"type":        "string",
					"description": "The database name",
				},
			},
			"required": []string{"database_name"},
		},
	})

	s.registerTool(&Tool{
		Name:        "get_table_schema",
		Description: "Retrieves the schema (column names, types, nullability, keys) for a specific table",
		Handler:     s.getTableSchema,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"database_name": map[string]interface{}{
					"type":        "string",
					"description": "The database name",
				},
				"table_name": map[string]interface{}{
					"type":        "string",
					"description": "The table name",
				},
			},
			"required": []string{"database_name", "table_name"},
		},
	})

	s.registerTool(&Tool{
		Name:        "get_table_schema_with_relations",
		Description: "Retrieves table schema with foreign key relationship information",
		Handler:     s.getTableSchemaWithRelations,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"database_name": map[string]interface{}{
					"type":        "string",
					"description": "The database name",
				},
				"table_name": map[string]interface{}{
					"type":        "string",
					"description": "The table name",
				},
			},
			"required": []string{"database_name", "table_name"},
		},
	})

	s.registerTool(&Tool{
		Name:        "execute_sql",
		Description: "Executes a read-only SQL query against a specified database",
		Handler:     s.executeSQL,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"sql_query": map[string]interface{}{
					"type":        "string",
					"description": "The SQL query to execute",
				},
				"database_name": map[string]interface{}{
					"type":        "string",
					"description": "The database to query against",
				},
				"parameters": map[string]interface{}{
					"type":        "array",
					"description": "Optional parameters for parameterized query",
					"items": map[string]interface{}{
						"type": "any",
					},
				},
			},
			"required": []string{"sql_query", "database_name"},
		},
	})

	s.registerTool(&Tool{
		Name:        "create_database",
		Description: "Creates a new database if it doesn't exist",
		Handler:     s.createDatabase,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"database_name": map[string]interface{}{
					"type":        "string",
					"description": "The name of the database to create",
				},
			},
			"required": []string{"database_name"},
		},
	})

	// Vector store tools (5 tools) - only if embedding service is available
	if s.embeddingService != nil {
		s.registerTool(&Tool{
			Name:        "create_vector_store",
			Description: "Creates a table which stores embeddings with vector indexing",
			Handler:     s.createVectorStore,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"database_name": map[string]interface{}{
						"type":        "string",
						"description": "The target database",
					},
					"vector_store_name": map[string]interface{}{
						"type":        "string",
						"description": "The name of the vector store table",
					},
					"model_name": map[string]interface{}{
						"type":        "string",
						"description": "Embedding model name (optional, uses provider default)",
					},
					"distance_function": map[string]interface{}{
						"type":        "string",
						"description": "Distance function: 'cosine' or 'euclidean' (default: 'cosine')",
						"enum":        []string{"cosine", "euclidean"},
					},
				},
				"required": []string{"database_name", "vector_store_name"},
			},
		})

		s.registerTool(&Tool{
			Name:        "list_vector_stores",
			Description: "Lists all vector stores (tables with indexed VECTOR columns) in a database",
			Handler:     s.listVectorStores,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"database_name": map[string]interface{}{
						"type":        "string",
						"description": "The database name",
					},
				},
				"required": []string{"database_name"},
			},
		})

		s.registerTool(&Tool{
			Name:        "delete_vector_store",
			Description: "Deletes a vector store from the specified database",
			Handler:     s.deleteVectorStore,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"database_name": map[string]interface{}{
						"type":        "string",
						"description": "The database name",
					},
					"vector_store_name": map[string]interface{}{
						"type":        "string",
						"description": "The vector store table name",
					},
				},
				"required": []string{"database_name", "vector_store_name"},
			},
		})

		s.registerTool(&Tool{
			Name:        "insert_docs_vector_store",
			Description: "Inserts a batch of documents with optional metadata into a vector store",
			Handler:     s.insertDocsVectorStore,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"database_name": map[string]interface{}{
						"type":        "string",
						"description": "The database name",
					},
					"vector_store_name": map[string]interface{}{
						"type":        "string",
						"description": "The vector store table name",
					},
					"documents": map[string]interface{}{
						"type":        "array",
						"description": "Array of document strings to insert",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
					"metadata": map[string]interface{}{
						"type":        "array",
						"description": "Optional array of metadata objects (same length as documents)",
						"items": map[string]interface{}{
							"type": "object",
						},
					},
				},
				"required": []string{"database_name", "vector_store_name", "documents"},
			},
		})

		s.registerTool(&Tool{
			Name:        "search_vector_store",
			Description: "Searches a vector store for the most similar documents using semantic search",
			Handler:     s.searchVectorStore,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"user_query": map[string]interface{}{
						"type":        "string",
						"description": "The search query string",
					},
					"database_name": map[string]interface{}{
						"type":        "string",
						"description": "The database name",
					},
					"vector_store_name": map[string]interface{}{
						"type":        "string",
						"description": "The vector store table name",
					},
					"k": map[string]interface{}{
						"type":        "number",
						"description": "Number of results to return (default: 7)",
					},
				},
				"required": []string{"user_query", "database_name", "vector_store_name"},
			},
		})
	}
}

// registerTool adds a tool to the MCP server
func (s *Server) registerTool(tool *Tool) {
	s.tools = append(s.tools, tool)

	// Create MCP tool definition
	mcpTool := mcp.NewTool(tool.Name, tool.Description, tool.InputSchema)

	// Register tool handler
	s.mcpServer.AddTool(mcpTool, tool.HandlerWrapper())
}

// runStdio runs the server using stdio transport
func (s *Server) runStdio(ctx context.Context) error {
	s.logger.Info("Starting server with stdio transport")

	// Start stdio server
	if err := server.ServeStdio(s.mcpServer); err != nil {
		return fmt.Errorf("stdio server error: %w", err)
	}
	return nil
}

// runSSE runs the server using SSE transport
func (s *Server) runSSE(ctx context.Context, host string, port int) error {
	s.logger.Info("Starting server with SSE transport",
		zap.String("host", host),
		zap.Int("port", port),
	)

	baseURL := fmt.Sprintf("http://%s:%d", host, port)
	sseServer := server.NewSSEServer(s.mcpServer, baseURL)

	addr := fmt.Sprintf("%s:%d", host, port)
	s.logger.Info("SSE server listening", zap.String("addr", addr))

	// Start the SSE server (blocking)
	if err := sseServer.Start(addr); err != nil {
		return fmt.Errorf("SSE server error: %w", err)
	}
	return nil
}

// runHTTP runs the server using HTTP transport
func (s *Server) runHTTP(ctx context.Context, host string, port int, path string) error {
	s.logger.Info("Starting server with HTTP transport",
		zap.String("host", host),
		zap.Int("port", port),
		zap.String("path", path),
	)

	// Create HTTP handler from MCP server
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == path {
			// Read request
			var rawMsg json.RawMessage
			if err := json.NewDecoder(r.Body).Decode(&rawMsg); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			// Handle message
			response := s.mcpServer.HandleMessage(ctx, rawMsg)

			// Write response
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		} else {
			http.NotFound(w, r)
		}
	})

	// Add CORS middleware
	wrappedHandler := withCORS(handler, s.cfg)

	addr := fmt.Sprintf("%s:%d", host, port)
	s.logger.Info("HTTP server listening", zap.String("addr", addr))

	if err := http.ListenAndServe(addr, wrappedHandler); err != nil {
		return fmt.Errorf("HTTP server error: %w", err)
	}
	return nil
}

// withCORS adds CORS headers to the handler
// By default, no CORS headers are set unless explicitly configured
func withCORS(next http.Handler, cfg *config.Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// If no origin is provided, skip CORS (same-origin request)
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Check if CORS is configured
		if cfg.CORSAllowOrigins == "" {
			// No CORS configured - reject cross-origin requests
			http.Error(w, "CORS policy: cross-origin requests not allowed", http.StatusForbidden)
			return
		}

		// Check if origin is allowed
		allowed := false
		if cfg.CORSAllowOrigins == "*" {
			// Wildcard allows all origins
			allowed = true
		} else {
			// Check against allowed origins list
			allowedOrigins := strings.Split(cfg.CORSAllowOrigins, ",")
			for _, allowedOrigin := range allowedOrigins {
				allowedOrigin = strings.TrimSpace(allowedOrigin)
				if allowedOrigin == origin || allowedOrigin == "*" {
					allowed = true
					break
				}
			}
		}

		if !allowed {
			http.Error(w, "CORS policy: origin not allowed", http.StatusForbidden)
			return
		}

		// Set CORS headers
		if cfg.CORSAllowOrigins == "*" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}

		if cfg.CORSAllowMethods != "" {
			w.Header().Set("Access-Control-Allow-Methods", cfg.CORSAllowMethods)
		}
		if cfg.CORSAllowHeaders != "" {
			w.Header().Set("Access-Control-Allow-Headers", cfg.CORSAllowHeaders)
		}
		if cfg.CORSAllowCredentials {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Close closes the server and releases resources
func (s *Server) Close() error {
	if s.pool != nil {
		return s.pool.Close()
	}
	return nil
}

// CallTool calls a tool by name (for testing purposes)
func (s *Server) CallTool(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
	for _, tool := range s.tools {
		if tool.Name == name {
			return tool.Handler(ctx, args)
		}
	}
	return nil, fmt.Errorf("tool not found: %s", name)
}

// Tool represents an MCP tool
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]interface{}
	Handler     ToolHandler
}

// ToolHandler is the function signature for tool handlers
type ToolHandler func(ctx context.Context, args map[string]interface{}) (interface{}, error)

// HandlerWrapper wraps a tool handler to return the correct type for mcp-go
func (t *Tool) HandlerWrapper() server.ToolHandlerFunc {
	return func(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
		ctx := context.Background()
		result, err := t.Handler(ctx, arguments)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []interface{}{
					mcp.NewTextContent(err.Error()),
				},
				IsError: true,
			}, nil
		}

		// Convert result to text content
		resultJSON, _ := json.Marshal(result)
		return &mcp.CallToolResult{
			Content: []interface{}{
				mcp.NewTextContent(string(resultJSON)),
			},
		}, nil
	}
}
