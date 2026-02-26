package test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/rahadiangg/mcp-mariadb/internal/config"
	"github.com/rahadiangg/mcp-mariadb/internal/mcp"
)

func setupTestServer(t *testing.T) (*mcp.Server, func()) {
	// Set environment variables for testing
	os.Setenv("MCP_MARIADB_HOST", "localhost")
	os.Setenv("MCP_MARIADB_PORT", "3306")
	os.Setenv("MCP_MARIADB_USER", "mcpuser")
	os.Setenv("MCP_MARIADB_PASSWORD", "mcppass")
	os.Setenv("MCP_MARIADB_DATABASE", "testdb")
	os.Setenv("MCP_MARIADB_READ_ONLY", "true")
	os.Setenv("MCP_MARIADB_MAX_POOL_SIZE", "10")

	cfg, logger, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	server := mcp.NewServer(cfg, logger)

	// Initialize the server
	ctx := context.Background()
	if err := server.Run(ctx, "stdio", "", 0, ""); err != nil {
		t.Fatalf("Failed to initialize server: %v", err)
	}

	cleanup := func() {
		server.Close()
	}

	return server, cleanup
}

func TestListDatabases(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Get the tools handler directly through the server
	ctx := context.Background()

	// We'll test by making a direct call to the listDatabases method
	// This is a simple integration test to verify database connectivity
	result, err := server.CallTool(ctx, "list_databases", map[string]interface{}{})
	if err != nil {
		t.Fatalf("list_databases failed: %v", err)
	}

	// Parse the result
	var dbList struct {
		Databases []string `json:"databases"`
	}
	resultBytes, _ := json.Marshal(result)
	if err := json.Unmarshal(resultBytes, &dbList); err == nil {
		t.Logf("Found databases: %v", dbList.Databases)
	} else {
		t.Logf("Result: %s", string(resultBytes))
	}
}

func TestListTables(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	result, err := server.CallTool(ctx, "list_tables", map[string]interface{}{
		"database_name": "testdb",
	})
	if err != nil {
		t.Fatalf("list_tables failed: %v", err)
	}

	resultBytes, _ := json.Marshal(result)
	t.Logf("Tables in testdb: %s", string(resultBytes))
}

func TestGetTableSchema(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	result, err := server.CallTool(ctx, "get_table_schema", map[string]interface{}{
		"database_name": "testdb",
		"table_name":    "users",
	})
	if err != nil {
		t.Fatalf("get_table_schema failed: %v", err)
	}

	resultBytes, _ := json.Marshal(result)
	t.Logf("Users table schema: %s", string(resultBytes))
}

func TestExecuteSQL(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	result, err := server.CallTool(ctx, "execute_sql", map[string]interface{}{
		"database_name": "testdb",
		"sql_query":     "SELECT * FROM users LIMIT 3",
	})
	if err != nil {
		t.Fatalf("execute_sql failed: %v", err)
	}

	resultBytes, _ := json.Marshal(result)
	t.Logf("Query result: %s", string(resultBytes))
}

func TestGetTableSchemaWithRelations(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	result, err := server.CallTool(ctx, "get_table_schema_with_relations", map[string]interface{}{
		"database_name": "testdb",
		"table_name":    "posts",
	})
	if err != nil {
		t.Fatalf("get_table_schema_with_relations failed: %v", err)
	}

	resultBytes, _ := json.Marshal(result)
	t.Logf("Posts table with relations: %s", string(resultBytes))
}

func TestCreateDatabase(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	// First, verify that create_database is blocked in READ_ONLY mode
	t.Run("blocked in read-only mode", func(t *testing.T) {
		_, err := server.CallTool(ctx, "create_database", map[string]interface{}{
			"database_name": "should_be_blocked",
		})
		if err == nil {
			t.Error("Expected create_database to be blocked in READ_ONLY mode")
		} else {
			t.Logf("Correctly blocked: %v", err)
		}
	})

	// Now test with READ_ONLY=false by creating a new server instance
	t.Run("succeeds when read-only disabled", func(t *testing.T) {
		os.Setenv("MCP_MARIADB_READ_ONLY", "false")
		defer os.Setenv("MCP_MARIADB_READ_ONLY", "true")

		cfg, logger, err := config.Load()
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		server2 := mcp.NewServer(cfg, logger)
		defer server2.Close()

		ctx2 := context.Background()
		if err := server2.Run(ctx2, "stdio", "", 0, ""); err != nil {
			t.Fatalf("Failed to initialize server: %v", err)
		}

		testDBName := "test_create_db"
		result, err := server2.CallTool(ctx, "create_database", map[string]interface{}{
			"database_name": testDBName,
		})
		if err != nil {
			t.Fatalf("create_database failed: %v", err)
		}

		resultBytes, _ := json.Marshal(result)
		t.Logf("Create database result: %s", string(resultBytes))

		// Verify the database was created
		listResult, err := server2.CallTool(ctx, "list_databases", map[string]interface{}{})
		if err != nil {
			t.Fatalf("list_databases failed: %v", err)
		}

		var dbList struct {
			Databases []string `json:"databases"`
		}
		listBytes, _ := json.Marshal(listResult)
		if err := json.Unmarshal(listBytes, &dbList); err == nil {
			found := false
			for _, db := range dbList.Databases {
				if db == testDBName {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Database %s was not created", testDBName)
			}
		}
	})
}

// Security tests for the hardening features
func TestSecurityHardening(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("read-only blocks write operations", func(t *testing.T) {
		// Test that INSERT is blocked
		_, err := server.CallTool(ctx, "execute_sql", map[string]interface{}{
			"database_name": "testdb",
			"sql_query":     "INSERT INTO users (username, email) VALUES ('test', 'test@example.com')",
		})
		if err == nil {
			t.Error("Expected INSERT to be blocked in READ_ONLY mode")
		}
		t.Logf("INSERT correctly blocked: %v", err)

		// Test that UPDATE is blocked
		_, err = server.CallTool(ctx, "execute_sql", map[string]interface{}{
			"database_name": "testdb",
			"sql_query":     "UPDATE users SET username='modified' WHERE id=1",
		})
		if err == nil {
			t.Error("Expected UPDATE to be blocked in READ_ONLY mode")
		}
	})

	t.Run("read-only allows select operations", func(t *testing.T) {
		// Test that SELECT works
		result, err := server.CallTool(ctx, "execute_sql", map[string]interface{}{
			"database_name": "testdb",
			"sql_query":     "SELECT COUNT(*) as count FROM users",
		})
		if err != nil {
			t.Errorf("SELECT should work in READ_ONLY mode: %v", err)
		}
		t.Logf("SELECT result: %v", result)
	})

	t.Run("comment bypass prevention", func(t *testing.T) {
		// Test comment bypass attempts - these should be detected as write operations
		bypassQueries := []string{
			"-- comment\nINSERT INTO users (username, email) VALUES ('hacker', 'hacker@example.com')",
			"# comment\nDROP TABLE users",
			"/* comment */ DELETE FROM users",
		}

		for _, query := range bypassQueries {
			_, err := server.CallTool(ctx, "execute_sql", map[string]interface{}{
				"database_name": "testdb",
				"sql_query":     query,
			})
			if err == nil {
				t.Errorf("Query bypass should be blocked: %s", query)
			} else {
				t.Logf("Bypass correctly blocked: %v", err)
			}
		}
	})

	t.Run("query size limit", func(t *testing.T) {
		// Create a very large query (over 1MB - using 2 million characters)
		// Each "1," is 2 chars, need ~1M iterations to exceed 1MB
		largeQuery := "SELECT * FROM users WHERE id IN (" + strings.Repeat("1,", 1000000) + "1)"

		// Log the actual query size
		t.Logf("Query size: %d bytes (limit: 1048576)", len(largeQuery))

		_, err := server.CallTool(ctx, "execute_sql", map[string]interface{}{
			"database_name": "testdb",
			"sql_query":     largeQuery,
		})
		if err == nil {
			t.Error("Oversized query should be blocked")
		} else {
			t.Logf("Oversized query correctly blocked: %v", err)
		}
	})
}

// Main test runner
func TestMain(m *testing.M) {
	fmt.Println("====================================")
	fmt.Println("MariaDB MCP Server Integration Tests")
	fmt.Println("====================================")
	fmt.Println()
	fmt.Println("Make sure MariaDB is running on localhost:3306")
	fmt.Println("with user 'mcpuser' and password 'mcppass'")
	fmt.Println()

	// Run tests
	code := m.Run()
	os.Exit(code)
}
