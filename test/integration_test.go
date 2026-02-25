package test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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

	// Create a test database
	testDBName := "test_create_db"
	result, err := server.CallTool(ctx, "create_database", map[string]interface{}{
		"database_name": testDBName,
	})
	if err != nil {
		t.Fatalf("create_database failed: %v", err)
	}

	resultBytes, _ := json.Marshal(result)
	t.Logf("Create database result: %s", string(resultBytes))

	// Verify the database was created
	listResult, err := server.CallTool(ctx, "list_databases", map[string]interface{}{})
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
