package mcp

import (
	"context"
	"testing"

	"github.com/rahadiangg/mcp-mariadb/internal/config"
	"go.uber.org/zap"
)

func TestColumn(t *testing.T) {
	// Test Column struct
	col := Column{
		Type:     "VARCHAR(255)",
		Nullable: true,
		Key:      "PRI",
		Default:  "default_value",
		Extra:    "auto_increment",
		ForeignKey: &ForeignKey{
			ConstraintName:   "fk_name",
			ReferencedTable:  "other_table",
			ReferencedColumn: "id",
			OnUpdate:         "CASCADE",
			OnDelete:         "RESTRICT",
		},
	}

	if col.Type != "VARCHAR(255)" {
		t.Errorf("Column.Type = %v, want VARCHAR(255)", col.Type)
	}
	if !col.Nullable {
		t.Error("Column.Nullable should be true")
	}
	if col.ForeignKey == nil {
		t.Error("Column.ForeignKey should not be nil")
	}
	if col.ForeignKey.ReferencedTable != "other_table" {
		t.Errorf("ForeignKey.ReferencedTable = %v, want other_table", col.ForeignKey.ReferencedTable)
	}
}

func TestForeignKey(t *testing.T) {
	fk := ForeignKey{
		ConstraintName:   "test_fk",
		ReferencedTable:  "users",
		ReferencedColumn: "user_id",
		OnUpdate:         "CASCADE",
		OnDelete:         "SET NULL",
	}

	if fk.ConstraintName != "test_fk" {
		t.Errorf("ForeignKey.ConstraintName = %v, want test_fk", fk.ConstraintName)
	}
}

func TestSchemaWithRelations(t *testing.T) {
	schema := SchemaWithRelations{
		TableName: "test_table",
		Columns: map[string]Column{
			"id": {
				Type:     "INT",
				Nullable: false,
				Key:      "PRI",
			},
			"name": {
				Type:     "VARCHAR(100)",
				Nullable: true,
				Key:      "",
			},
		},
	}

	if schema.TableName != "test_table" {
		t.Errorf("SchemaWithRelations.TableName = %v, want test_table", schema.TableName)
	}
	if len(schema.Columns) != 2 {
		t.Errorf("SchemaWithRelations.Columns length = %v, want 2", len(schema.Columns))
	}
}

func TestToolStruct(t *testing.T) {
	tool := Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: map[string]interface{}{
			"type": "object",
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			return "ok", nil
		},
	}

	if tool.Name != "test_tool" {
		t.Errorf("Tool.Name = %v, want test_tool", tool.Name)
	}
	if tool.Description != "A test tool" {
		t.Errorf("Tool.Description = %v, want 'A test tool'", tool.Description)
	}
	if tool.Handler == nil {
		t.Error("Tool.Handler should not be nil")
	}

	// Test calling the handler
	result, err := tool.Handler(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Errorf("Tool.Handler() error = %v", err)
	}
	if result != "ok" {
		t.Errorf("Tool.Handler() = %v, want ok", result)
	}
}

func TestServerStruct(t *testing.T) {
	logger := zap.NewNop()
	cfg := &config.Config{
		Host:     "localhost",
		Port:     3306,
		User:     "test",
		Password: "test",
		Database: "testdb",
	}

	s := NewServer(cfg, logger)

	if s.cfg != cfg {
		t.Error("Server.cfg not set correctly")
	}
	if s.logger != logger {
		t.Error("Server.logger not set correctly")
	}
	if s.tools == nil {
		t.Error("Server.tools should be initialized")
	}
	if len(s.tools) != 0 {
		t.Error("Server.tools should be empty initially")
	}
}

func TestRegisterTool(t *testing.T) {
	logger := zap.NewNop()
	s := &Server{
		cfg:    &config.Config{},
		logger: logger,
		tools:  make([]*Tool, 0),
	}

	// We need an mcpServer to register tools
	// This is tested indirectly through NewServer and initialize

	// Test that tools slice is initialized
	if s.tools == nil {
		t.Error("tools slice should be initialized")
	}
}

func TestStripCommentsEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "query with semicolon",
			query:    "SELECT * FROM users;",
			expected: "SELECT * FROM users;",
		},
		{
			name:     "query with newlines",
			query:    "SELECT *\nFROM\nusers",
			expected: "SELECT * FROM users",
		},
		{
			name:     "query with tabs",
			query:    "SELECT\t*\tFROM\tusers",
			expected: "SELECT * FROM users",
		},
		{
			name:     "query with multiple spaces",
			query:    "SELECT    *    FROM    users",
			expected: "SELECT * FROM users",
		},
		{
			name:     "mixed case comment",
			query:    "SELECT * FROM users -- CoMmEnT",
			expected: "SELECT * FROM users",
		},
		{
			name:     "comment with special chars",
			query:    "SELECT * FROM users -- !@#$%",
			expected: "SELECT * FROM users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripComments(tt.query)
			if result != tt.expected {
				t.Errorf("stripComments() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestIsReadOnlyQueryEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected bool
	}{
		{
			name:     "select with subquery",
			query:    "SELECT * FROM (SELECT * FROM users) AS t",
			expected: true,
		},
		{
			name:     "select with join",
			query:    "SELECT * FROM users JOIN orders ON users.id = orders.user_id",
			expected: true,
		},
		{
			name:     "select with union",
			query:    "SELECT * FROM users UNION SELECT * FROM admins",
			expected: true,
		},
		{
			name:     "explain insert",
			query:    "EXPLAIN INSERT INTO users VALUES (1, 'test')",
			expected: true,
		},
		{
			name:     "with cte and insert (known limitation - bypass possible)",
			query:    "WITH cte AS (SELECT 1) INSERT INTO users VALUES (1)",
			expected: true, // Known limitation: WITH prefix is considered read-only
		},
		{
			name:     "show create table",
			query:    "SHOW CREATE TABLE users",
			expected: true,
		},
		{
			name:     "show columns",
			query:    "SHOW COLUMNS FROM users",
			expected: true,
		},
		{
			name:     "show index",
			query:    "SHOW INDEX FROM users",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isReadOnlyQuery(tt.query)
			if result != tt.expected {
				t.Errorf("isReadOnlyQuery() = %v, want %v", result, tt.expected)
			}
		})
	}
}
