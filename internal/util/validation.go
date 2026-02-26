package util

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode"
)

// MaxIdentifierLength is the maximum allowed length for SQL identifiers
const MaxIdentifierLength = 64

// Reserved SQL keywords that cannot be used as identifiers without quoting
var reservedKeywords = map[string]bool{
	// MariaDB reserved keywords
	"ADD": true, "ALL": true, "ALTER": true, "ANALYZE": true, "AND": true,
	"AS": true, "ASC": true, "ASENSITIVE": true, "BEFORE": true, "BETWEEN": true,
	"BIGINT": true, "BINARY": true, "BLOB": true, "BOTH": true, "BY": true,
	"CALL": true, "CASCADE": true, "CASE": true, "CHANGE": true, "CHAR": true,
	"CHARACTER": true, "CHECK": true, "COLLATE": true, "COLUMN": true, "CONDITION": true,
	"CONNECTION": true, "CONSTRAINT": true, "CONTINUE": true, "CONVERT": true,
	"CREATE": true, "CROSS": true, "CUBE": true, "CUME_DIST": true, "CURRENT_DATE": true,
	"CURRENT_TIME": true, "CURRENT_TIMESTAMP": true, "CURRENT_USER": true, "CURSOR": true,
	"DATABASE": true, "DATABASES": true, "DAY_HOUR": true, "DAY_MICROSECOND": true,
	"DAY_MINUTE": true, "DAY_SECOND": true, "DECLARE": true, "DECIMAL": true,
	"DEFAULT": true, "DELAYED": true, "DELETE": true, "DENSE_RANK": true, "DESC": true,
	"DESCRIBE": true, "DETERMINISTIC": true, "DISTINCT": true, "DISTINCTROW": true,
	"DIV": true, "DOUBLE": true, "DROP": true, "DUAL": true, "EACH": true, "ELSE": true,
	"ENCLOSED": true, "ESCAPED": true, "EXCEPT": true, "EXISTS": true, "EXIT": true,
	"EXPLAIN": true, "FALSE": true, "FETCH": true, "FIRST_VALUE": true, "FLOAT": true,
	"FLOAT4": true, "FLOAT8": true, "FOR": true, "FORCE": true, "FOREIGN": true,
	"FROM": true, "FULLTEXT": true, "FUNCTION": true, "GENERAL": true, "GRANT": true,
	"GROUP": true, "GROUPING": true, "GROUPS": true, "HAVING": true, "HIGH_PRIORITY": true,
	"HOUR_MICROSECOND": true, "HOUR_MINUTE": true, "HOUR_SECOND": true, "IF": true,
	"IGNORE": true, "IN": true, "INDEX": true, "INFILE": true, "INNER": true,
	"INOUT": true, "INSENSITIVE": true, "INSERT": true, "INT": true, "INT1": true,
	"INT2": true, "INT3": true, "INT4": true, "INT8": true, "INTEGER": true,
	"INTERVAL": true, "INTO": true, "IO_AFTER_GTIDS": true, "IO_BEFORE_GTIDS": true,
	"IS": true, "ITERATE": true, "JOIN": true, "JSON_TABLE": true, "KEY": true,
	"KEYS": true, "KILL": true, "LAG": true, "LAST_VALUE": true, "LATERAL": true,
	"LEAD": true, "LEADING": true, "LEAVE": true, "LEFT": true, "LIKE": true,
	"LIMIT": true, "LINEAR": true, "LINES": true, "LOAD": true, "LOCALTIME": true,
	"LOCALTIMESTAMP": true, "LOCK": true, "LONG": true, "LONGBLOB": true,
	"LONGTEXT": true, "LOOP": true, "LOW_PRIORITY": true, "MASTER_BIND": true,
	"MASTER_SSL_VERIFY_SERVER_CERT": true, "MATCH": true, "MAXVALUE": true,
	"MEDIUMBLOB": true, "MEDIUMINT": true, "MEDIUMTEXT": true, "MIDDLEINT": true,
	"MINUTE_MICROSECOND": true, "MINUTE_SECOND": true, "MOD": true, "MODIFIES": true,
	"NATURAL": true, "NOT": true, "NO_WRITE_TO_BINLOG": true, "NTH_VALUE": true,
	"NTILE": true, "NULL": true, "NUMERIC": true, "OF": true, "ON": true,
	"OPTIMIZE": true, "OPTIMIZER_COSTS": true, "OPTION": true, "OPTIONALLY": true,
	"OR": true, "ORDER": true, "OUT": true, "OUTER": true, "OUTFILE": true,
	"OVER": true, "PARTITION": true, "PERCENT_RANK": true, "PRECISION": true,
	"PRIMARY": true, "PROCEDURE": true, "PURGE": true, "RANK": true, "READ": true,
	"READS": true, "READ_WRITE": true, "REAL": true, "RECURSIVE": true,
	"REFERENCES": true, "REGEXP": true, "RELEASE": true, "RENAME": true,
	"REPEAT": true, "REPLACE": true, "REQUIRE": true, "RESIGNAL": true,
	"RESTRICT": true, "RETURN": true, "REVOKE": true, "RIGHT": true,
	"RLIKE": true, "ROW": true, "ROWS": true, "ROW_NUMBER": true, "SCHEMA": true,
	"SCHEMAS": true, "SECOND_MICROSECOND": true, "SELECT": true, "SENSITIVE": true,
	"SEPARATOR": true, "SET": true, "SHOW": true, "SIGNAL": true, "SMALLINT": true,
	"SPATIAL": true, "SPECIFIC": true, "SQL": true, "SQLEXCEPTION": true,
	"SQLSTATE": true, "SQLWARNING": true, "SQL_BIG_RESULT": true, "SQL_CALC_FOUND_ROWS": true,
	"SQL_SMALL_RESULT": true, "SSL": true, "STARTING": true, "STORED": true,
	"STRAIGHT_JOIN": true, "SYSTEM": true, "TABLE": true, "TERMINATED": true,
	"THEN": true, "TINYBLOB": true, "TINYINT": true, "TINYTEXT": true, "TO": true,
	"TRAILING": true, "TRIGGER": true, "TRUE": true, "UNDO": true, "UNION": true,
	"UNIQUE": true, "UNLOCK": true, "UNSIGNED": true, "UPDATE": true, "USAGE": true,
	"USE": true, "USING": true, "UTC_DATE": true, "UTC_TIME": true,
	"UTC_TIMESTAMP": true, "VALUES": true, "VARBINARY": true, "VARCHAR": true,
	"VARCHARACTER": true, "VARYING": true, "VIRTUAL": true, "WHEN": true,
	"WHERE": true, "WHILE": true, "WINDOW": true, "WITH": true, "WRITE": true,
	"XOR": true, "YEAR_MONTH": true, "ZEROFILL": true,
}

// Dangerous SQL patterns that could indicate injection attempts
var dangerousPatterns = []struct {
	pattern *regexp.Regexp
	name    string
}{
	{regexp.MustCompile(`(?i)\bSELECT\b.*?\bINTO\s+(OUTFILE|DUMPFILE)\b`), "SELECT INTO OUTFILE/DUMPFILE"},
	{regexp.MustCompile(`(?i)\bLOAD\s+(DATA|XML)\s+INFILE\b`), "LOAD DATA/XML INFILE"},
	{regexp.MustCompile(`(?i)\bINTO\s+OUTFILE\b`), "INTO OUTFILE"},
	{regexp.MustCompile(`(?i)\bINTO\s+DUMPFILE\b`), "INTO DUMPFILE"},
	{regexp.MustCompile(`;.*\b(DROP|DELETE|INSERT|UPDATE|CREATE|ALTER|TRUNCATE|EXEC|EXECUTE)\b`), "multiple statements"},
	{regexp.MustCompile(`--.*;`), "comment with semicolon"},
	{regexp.MustCompile(`#.*;`), "hash comment with semicolon"},
	{regexp.MustCompile(`/\*.*;\*/`), "block comment with semicolon"},
}

// ValidateSQLIdentifier checks if a string is a valid SQL identifier
// It enforces:
// - ASCII-only characters (prevents Unicode homograph attacks)
// - Maximum length of 64 characters
// - Must start with letter or underscore
// - Can only contain letters, digits, and underscores
// - Cannot be a reserved SQL keyword
func ValidateSQLIdentifier(name string) error {
	if name == "" {
		return fmt.Errorf("identifier cannot be empty")
	}

	// Check length
	if len(name) > MaxIdentifierLength {
		return fmt.Errorf("identifier exceeds maximum length of %d characters", MaxIdentifierLength)
	}

	// Check for ASCII-only (prevents Unicode homograph attacks)
	for i, r := range name {
		if r > unicode.MaxASCII {
			return fmt.Errorf("identifier contains non-ASCII character at position %d: '%c' (U+%04X)", i, r, r)
		}
	}

	// Check if identifier is valid (alphanumeric, underscore, not starting with digit)
	for i, r := range name {
		if i == 0 && !unicode.IsLetter(r) && r != '_' {
			return fmt.Errorf("identifier '%s' must start with letter or underscore, got: '%c'", name, r)
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return fmt.Errorf("identifier '%s' contains invalid character at position %d: '%c'", name, i, r)
		}
	}

	// Check for reserved keywords (case-insensitive)
	upperName := strings.ToUpper(name)
	if reservedKeywords[upperName] {
		return fmt.Errorf("identifier '%s' is a reserved SQL keyword", name)
	}

	return nil
}

// ValidateQueryForInjection checks for dangerous SQL patterns that could
// indicate injection attempts or unauthorized file operations
func ValidateQueryForInjection(query string) error {
	// Check for dangerous patterns
	for _, dp := range dangerousPatterns {
		if dp.pattern.MatchString(query) {
			return fmt.Errorf("query contains dangerous pattern '%s', blocked for security reasons", dp.name)
		}
	}

	// Check for shell command patterns in string literals
	shellPatterns := []string{
		"`", // Backtick execution (shell)
		"$(", // Command substitution
		"\\x", // Hex escape sequences
		"\\0", // Null byte
	}

	for _, pattern := range shellPatterns {
		if strings.Contains(query, pattern) {
			// Allow backticks for SQL identifiers (they're in quoted strings)
			if pattern == "`" && strings.Contains(query, "\\`") {
				return fmt.Errorf("query contains potentially dangerous shell escape pattern")
			}
		}
	}

	// Check for very large nested expressions (potential DoS)
	depth := 0
	maxDepth := 50
	for _, r := range query {
		if r == '(' {
			depth++
			if depth > maxDepth {
				return fmt.Errorf("query exceeds maximum nesting depth of %d (potential DoS)", maxDepth)
			}
		} else if r == ')' {
			depth--
		}
	}

	return nil
}

// ValidateAPIKeyFormat checks if an API key has the expected format for its provider
func ValidateAPIKeyFormat(provider, key string) error {
	if key == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	// Check for whitespace
	if strings.ContainsAny(key, " \t\n\r") {
		return fmt.Errorf("API key contains whitespace")
	}

	switch strings.ToLower(provider) {
	case "openai":
		if !strings.HasPrefix(key, "sk-") {
			return fmt.Errorf("OpenAI API key must start with 'sk-'")
		}
		if len(key) < 20 {
			return fmt.Errorf("OpenAI API key is too short (minimum 20 characters)")
		}
	case "gemini":
		if len(key) < 20 {
			return fmt.Errorf("Gemini API key is too short (minimum 20 characters)")
		}
	default:
		// Unknown provider - do basic validation only
		if len(key) < 10 {
			return fmt.Errorf("API key is too short (minimum 10 characters)")
		}
	}

	return nil
}

// validatePort checks if a port number is valid
func validatePort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got: %d", port)
	}
	return nil
}

// validateSSLFile checks if an SSL certificate file exists and has appropriate permissions
func validateSSLFile(path string, isKeyFile bool) error {
	if path == "" {
		return nil // Empty path is allowed (SSL not configured)
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("SSL file does not exist: %s", path)
		}
		return fmt.Errorf("cannot access SSL file: %w", err)
	}

	if info.IsDir() {
		return fmt.Errorf("SSL path is a directory, not a file: %s", path)
	}

	// Check file permissions
	mode := info.Mode()

	if isKeyFile {
		// Key files should have stricter permissions
		if mode.Perm()&0077 != 0 {
			// Warn if key is readable by group or others
			return fmt.Errorf("SSL key file has insecure permissions: %s (should not be readable by group/other)", path)
		}
	} else {
		// Cert files should not be world-readable
		if mode.Perm()&0004 != 0 {
			// Warn if cert is world-readable
			return fmt.Errorf("SSL certificate file has insecure permissions: %s (should not be world-readable)", path)
		}
	}

	return nil
}

// ValidatePort is a public wrapper for validatePort
func ValidatePort(port int) error {
	return validatePort(port)
}

// ValidateSSLFile is a public wrapper for validateSSLFile
func ValidateSSLFile(path string, isKeyFile bool) error {
	return validateSSLFile(path, isKeyFile)
}
