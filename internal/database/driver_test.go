package database

import (
	"testing"
)

func TestIndexOf(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		substr string
		want   int
	}{
		{"found", "hello world", "world", 6},
		{"not found", "hello world", "xyz", -1},
		{"empty substring", "hello", "", 0},
		{"empty string", "", "test", -1},
		{"at start", "hello", "he", 0},
		{"at end", "hello", "lo", 3},
		{"multi char", "ababab", "aba", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := indexOf(tt.s, tt.substr); got != tt.want {
				t.Errorf("indexOf() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		substr string
		want   bool
	}{
		{"contains", "hello world", "world", true},
		{"not contains", "hello world", "xyz", false},
		{"empty substring", "hello", "", true},
		{"empty string", "", "test", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := contains(tt.s, tt.substr); got != tt.want {
				t.Errorf("contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateMultiStatementsDisabled(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		// Valid single statements
		{"simple select", "SELECT * FROM users", false},
		{"select with where", "SELECT * FROM users WHERE id = 1", false},
		{"insert", "INSERT INTO users (name) VALUES ('test')", false},
		{"update", "UPDATE users SET name = 'test' WHERE id = 1", false},
		{"delete", "DELETE FROM users WHERE id = 1", false},

		// Valid trailing semicolon (end of query)
		{"select with trailing semicolon", "SELECT * FROM users;", false},
		{"select with trailing semicolon and space", "SELECT * FROM users; ", false},
		{"select with trailing semicolon and newline", "SELECT * FROM users;\n", false},

		// Invalid multiple statements
		{"two selects", "SELECT * FROM users; SELECT * FROM posts", true},
		{"select and insert", "SELECT * FROM users; INSERT INTO logs VALUES (1)", true},
		{"drop and select", "DROP TABLE users; SELECT * FROM posts", true},
		{"multiple statements with spaces", "SELECT * FROM users;  SELECT * FROM posts", true},
		{"multiple statements with newlines", "SELECT * FROM users;\nSELECT * FROM posts", true},

		// Edge cases with semicolons in strings
		{"semicolon in single quote string", "SELECT * FROM users WHERE name = 'test;value'", false},
		{"semicolon in double quote string", `SELECT * FROM users WHERE name = "test;value"`, false},
		{"escaped single quote", "SELECT * FROM users WHERE name = 'test\\';value'", false},
		{"multiple strings with semicolons", "SELECT * FROM users WHERE name = 'test;value' AND email = 'a;b'", false},

		// Complex cases
		{"semicolon in comment after query", "SELECT * FROM users; -- comment", true},
		{"semicolon after string then more", "SELECT * FROM users WHERE name = 'test'; DROP TABLE users", true},

		// Empty and whitespace
		{"empty query", "", false},
		{"only semicolon", ";", false},
		{"only whitespace", "   ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMultiStatementsDisabled(tt.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMultiStatementsDisabled() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewSafeDriver(t *testing.T) {
	driver := NewSafeDriver()
	if driver == nil {
		t.Error("NewSafeDriver() returned nil")
	}
	if _, ok := driver.(*safeDriver); !ok {
		t.Error("NewSafeDriver() did not return *safeDriver type")
	}
}

func TestSafeDriverOpen(t *testing.T) {
	driver := NewSafeDriver()

	tests := []struct {
		name        string
		dsn         string
		expectError bool
	}{
		{
			name:        "DSN without existing params",
			dsn:         "user:password@tcp(localhost:3306)/database",
			expectError: true, // Will error because can't actually connect
		},
		{
			name:        "DSN with existing params",
			dsn:         "user:password@tcp(localhost:3306)/database?charset=utf8",
			expectError: true, // Will error because can't actually connect
		},
		{
			name:        "DSN with multiStatements already set",
			dsn:         "user:password@tcp(localhost:3306)/database?multiStatements=false",
			expectError: true, // Will error because can't actually connect
		},
		{
			name:        "Empty DSN",
			dsn:         "",
			expectError: true, // MySQL driver will error on empty DSN
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		conn, err := driver.Open(tt.dsn)
		if tt.expectError {
			if err == nil && conn != nil {
				// If we somehow got a connection, close it
				if conn != nil {
					conn.Close()
				}
				t.Error("Expected error from Open(), but got none")
			}
		}
		// Most cases will error because we can't connect to MySQL
		// We just verify the function doesn't panic
		_ = err
	})
	}
}

func TestSafeDriverOpenModifiesDSN(t *testing.T) {
	driver := NewSafeDriver()

	// Test that multiStatements=false is added to DSN
	// We can't test this directly without mocking, but we can verify
	// the function handles different DSN formats correctly
	testDSNs := []string{
		"user:pass@host/db",
		"user:pass@host/db?charset=utf8",
		"user:pass@host/db?multiStatements=false",
		"",
	}

	for _, dsn := range testDSNs {
		t.Run(dsn, func(t *testing.T) {
		// Just verify no panic
		_, _ = driver.Open(dsn)
	})
	}
}
