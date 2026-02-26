package util

import (
	"strings"
	"testing"
)

func TestValidateSQLIdentifier(t *testing.T) {
	tests := []struct {
		name      string
		identifier string
		wantErr   bool
		errMsg    string
	}{
		// Valid identifiers
		{"valid simple", "valid_name", false, ""},
		{"valid with underscore", "_valid", false, ""},
		{"valid with numbers", "table123", false, ""},
		{"valid mixed", "my_table_2024", false, ""},
		{"max length", "a" + strings.Repeat("b", 62) + "c", false, ""}, // 64 chars

		// Empty identifier
		{"empty", "", true, "identifier cannot be empty"},

		// Too long
		{"too long", string(make([]byte, 65)), true, "exceeds maximum length"},

		// Non-ASCII characters (homograph attacks)
		{"unicode latin", "tablе", true, "non-ASCII"}, // Cyrillic 'е' looks like Latin 'e'
		{"unicode greek", "αtable", true, "non-ASCII"},
		{"emoji", "table😀", true, "non-ASCII"},

		// Starts with digit
		{"starts with digit", "1table", true, "must start with letter or underscore"},

		// Invalid characters
		{"hyphen", "my-table", true, "invalid character"},
		{"space", "my table", true, "invalid character"},
		{"dot", "my.table", true, "invalid character"},
		{"special", "table@test", true, "invalid character"},

		// SQL injection patterns
		{"with semicolon", "table; DROP", true, "invalid character"},
		{"with quote", "table' OR '1'='1", true, "invalid character"},
		{"with union", "table UNION SELECT", true, "invalid character"},

		// Reserved keywords
		{"select", "SELECT", true, "reserved SQL keyword"},
		{"select lower", "select", true, "reserved SQL keyword"},
		{"from", "FROM", true, "reserved SQL keyword"},
		{"where", "WHERE", true, "reserved SQL keyword"},
		{"insert", "INSERT", true, "reserved SQL keyword"},
		{"table", "TABLE", true, "reserved SQL keyword"},
		{"drop", "DROP", true, "reserved SQL keyword"},
		{"create", "CREATE", true, "reserved SQL keyword"},
		{"database", "DATABASE", true, "reserved SQL keyword"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSQLIdentifier(tt.identifier)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSQLIdentifier() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				// Check if error message contains expected substring
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateSQLIdentifier() error = %v, expected to contain %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func TestValidateQueryForInjection(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantErr bool
		errMsg  string
	}{
		// Valid queries
		{"simple select", "SELECT * FROM users", false, ""},
		{"select with where", "SELECT id FROM users WHERE name = ?", false, ""},
		{"explain", "EXPLAIN SELECT * FROM users", false, ""},
		{"with", "WITH cte AS (SELECT 1) SELECT * FROM cte", false, ""},
		{"show", "SHOW TABLES", false, ""},
		{"describe", "DESCRIBE users", false, ""},

		// INTO OUTFILE/DUMPFILE
		{"into outfile", "SELECT * INTO OUTFILE '/tmp/file'", true, "INTO OUTFILE/DUMPFILE"},
		{"into dumpfile", "SELECT * INTO DUMPFILE '/tmp/file'", true, "INTO OUTFILE/DUMPFILE"},
		{"outfile pattern", "SELECT * FROM users INTO OUTFILE '/etc/passwd'", true, "dangerous"},

		// LOAD DATA INFILE
		{"load data infile", "LOAD DATA INFILE '/tmp/file'", true, "LOAD DATA/XML INFILE"},
		{"load xml infile", "LOAD XML INFILE '/tmp/file'", true, "LOAD DATA/XML INFILE"},

		// Multiple statements
		{"semicolon with drop", "SELECT *; DROP TABLE users", true, "multiple statements"},
		{"semicolon with insert", "SELECT *; INSERT INTO", true, "multiple statements"},

		// Comment with semicolon
		{"dash comment semicolon", "-- comment; DROP TABLE", true, "multiple statements"},
		{"hash comment semicolon", "# comment; DROP TABLE", true, "multiple statements"},
		{"block comment semicolon", "/* comment; */ DROP TABLE", true, "multiple statements"},

		// Deep nesting (DoS)
		{"deep nesting", "SELECT * FROM (" + strings.Repeat("(", 51) + "SELECT 1" + strings.Repeat(")", 51) + ") AS t", true, "nesting depth"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateQueryForInjection(tt.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateQueryForInjection() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateQueryForInjection() error = %v, expected to contain %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestValidateAPIKeyFormat(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		key      string
		wantErr  bool
		errMsg   string
	}{
		// Valid OpenAI keys
		{"valid openai key", "openai", "sk-1234567890abcdefg", false, ""},
		{"valid openai longer", "openai", "sk-proj-abc123def456", false, ""},

		// Invalid OpenAI keys
		{"openai missing prefix", "openai", "1234567890abcdef", true, "must start with 'sk-"},
		{"openai too short", "openai", "sk-123", true, "too short"},
		{"openai with whitespace", "openai", "sk-1234 5678", true, "whitespace"},

		// Valid Gemini keys
		{"valid gemini key", "gemini", "AIzaSyD1234567890abcdefg", false, ""},
		{"valid gemini longer", "gemini", "AIzaSyD" + string(make([]byte, 30)), false, ""},

		// Invalid Gemini keys
		{"gemini too short", "gemini", "abc123", true, "too short"},
		{"gemini with whitespace", "gemini", "AIza 123", true, "whitespace"},

		// Empty key
		{"empty key", "openai", "", true, "cannot be empty"},

		// Unknown provider (basic validation)
		{"unknown provider valid", "unknown", "my-api-key-12345", false, ""},
		{"unknown provider short", "unknown", "short", true, "too short"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAPIKeyFormat(tt.provider, tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAPIKeyFormat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateAPIKeyFormat() error = %v, expected to contain %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestValidatePort(t *testing.T) {
	tests := []struct {
		name    string
		port    int
		wantErr bool
	}{
		{"valid low", 1, false},
		{"valid common", 3306, false},
		{"valid high", 65535, false},
		{"zero", 0, true},
		{"negative", -1, true},
		{"too high", 65536, true},
		{"way too high", 99999, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePort(tt.port)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePort() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSSLFile(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		isKey    bool
		wantErr  bool
		errMsg   string
	}{
		// Empty path (SSL not configured)
		{"empty path", "", false, false, ""},
		{"empty key path", "", true, false, ""},

		// Non-existent file
		{"non-existent cert", "/nonexistent/cert.pem", false, true, "does not exist"},
		{"non-existent key", "/nonexistent/key.pem", true, true, "does not exist"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSSLFile(tt.path, tt.isKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSSLFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateSSLFile() error = %v, expected to contain %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}
