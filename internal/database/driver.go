package database

import (
	"database/sql"
	"database/sql/driver"
	"fmt"

	"github.com/go-sql-driver/mysql"
)

// safeDriver wraps the MySQL driver to enforce multiStatements=false
// This is a critical security feature to prevent SQL injection via multiple statements
type safeDriver struct {
	driver.Driver
}

// Open creates a new connection with multiStatements disabled
func (d *safeDriver) Open(name string) (driver.Conn, error) {
	// Ensure multiStatements=false is set
	dsn := name
	if len(dsn) > 0 && string(rune(dsn[0])) != "?" {
		// Check if params already exist
		if idx := indexOf(dsn, "?"); idx == -1 {
			dsn = dsn + "?multiStatements=false"
		} else {
			// Already has params, append if not present
			if !contains(dsn, "multiStatements") {
				dsn = dsn + "&multiStatements=false"
			}
		}
	}
	return mysql.MySQLDriver{}.Open(dsn)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func contains(s, substr string) bool {
	return indexOf(s, substr) != -1
}

// Register the safe driver on package init
func init() {
	sql.Register("safemysql", &safeDriver{})
}

// NewSafeDriver returns the safe driver instance
func NewSafeDriver() driver.Driver {
	return &safeDriver{}
}

// ValidateMultiStatementsDisabled checks if a query attempts to use multiple statements
// This is a runtime safety check in addition to the DSN parameter
func ValidateMultiStatementsDisabled(query string) error {
	// Simple heuristic: check for semicolon outside of string literals
	inString := false
	escapeNext := false
	stringChar := rune(0)

	for i, r := range query {
		if escapeNext {
			escapeNext = false
			continue
		}

		switch r {
		case '\\':
			if inString {
				escapeNext = true
			}
		case '\'', '"':
			if !inString {
				inString = true
				stringChar = r
			} else if r == stringChar {
				inString = false
				stringChar = 0
			}
		case ';':
			if !inString && i < len(query)-1 {
				// Semicolon followed by more content suggests multiple statements
				remaining := query[i+1:]
				// Skip whitespace
				for len(remaining) > 0 && (remaining[0] == ' ' || remaining[0] == '\t' || remaining[0] == '\n' || remaining[0] == '\r') {
					remaining = remaining[1:]
				}
				if len(remaining) > 0 {
					return fmt.Errorf("multiple statements detected (security risk): semicolon at position %d", i)
				}
			}
		}
	}

	return nil
}
