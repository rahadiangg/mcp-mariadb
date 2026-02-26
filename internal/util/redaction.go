package util

import (
	"regexp"
	"strings"
)

// Secret patterns that should be redacted from logs
var secretPatterns = []struct {
	pattern    *regexp.Regexp
	replacement string
}{
	// API keys
	{regexp.MustCompile(`(?i)(api[_-]?key|apikey|secret[_-]?key|access[_-]?key|auth[_-]?token|bearer)[\s:=]+['"]?([a-zA-Z0-9_\-\.]{10,})['"]?`), "${1}=***REDACTED***"},
	// Passwords in connection strings
	{regexp.MustCompile(`://([^:@]+):([^@]+)@`), "://$1:***REDACTED***@"},
	// JWT tokens
	{regexp.MustCompile(`(?i)eyJ[a-zA-Z0-9_-]+\.(?:eyJ[a-zA-Z0-9_-]+\.)?[a-zA-Z0-9_-]+`), "***REDACTED_JWT***"},
	// Generic sensitive keys
	{regexp.MustCompile(`(?i)(sk-|pk-|ghp_|gho_|ghu_|ghs_|ghr_|glpat-)`), "***REDACTED_KEY***"},
	// Credit card numbers (basic pattern)
	{regexp.MustCompile(`\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`), "***REDACTED_CARD***"},
	// AWS Access Keys
	{regexp.MustCompile(`(?i)AKIA[0-9A-Z]{16}`), "***REDACTED_AWS_KEY***"},
	// OAuth tokens
	{regexp.MustCompile(`(?i)(oauth[_-]?token|access[_-]?token)[\s:=]+['"]?([a-zA-Z0-9_\-\.]{20,})['"]?`), "${1}=***REDACTED***"},
}

// RedactAPIKey redacts an API key for logging purposes
// It shows only the first few characters (type) and last 4 characters
func RedactAPIKey(key string) string {
	if key == "" {
		return ""
	}

	// Show prefix and suffix only
	if len(key) <= 8 {
		return "***"
	}

	prefixLen := 4
	suffixLen := 4

	if len(key) < prefixLen+suffixLen+1 {
		return key[:3] + "***"
	}

	return key[:prefixLen] + "***" + key[len(key)-suffixLen:]
}

// RedactConnectionString redacts sensitive information from a connection string
func RedactConnectionString(connStr string) string {
	if connStr == "" {
		return ""
	}

	redacted := connStr

	// Redact password in DSN format: user:password@host
	re := regexp.MustCompile(`://([^:@]+):([^@]+)@`)
	redacted = re.ReplaceAllString(redacted, "://$1:***REDACTED***@")

	// Redact password in URL parameters
	re = regexp.MustCompile(`([?&]password=)[^&]+`)
	redacted = re.ReplaceAllString(redacted, "${1}***REDACTED***")

	return redacted
}

// ContainsSecretPatterns checks if a string contains patterns that look like secrets
// This is useful for validating that secrets aren't being passed where they shouldn't
func ContainsSecretPatterns(s string) bool {
	// Check for API key patterns
	if strings.Contains(s, "sk-") || strings.Contains(s, "pk-") {
		return true
	}

	// Check for AWS keys
	if regexp.MustCompile(`AKIA[0-9A-Z]{16}`).MatchString(s) {
		return true
	}

	// Check for JWT tokens
	if regexp.MustCompile(`eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+`).MatchString(s) {
		return true
	}

	// Check for potential passwords in connection strings
	if strings.Contains(s, ":") && strings.Contains(s, "@") {
		// Could be user:pass@host format
		return true
	}

	return false
}

// RedactSecrets redacts all detected secret patterns from a string
func RedactSecrets(s string) string {
	if s == "" {
		return ""
	}

	redacted := s

	for _, sp := range secretPatterns {
		redacted = sp.pattern.ReplaceAllString(redacted, sp.replacement)
	}

	return redacted
}

// SanitizeForLogging removes or redacts sensitive information from a string
// that is about to be logged
func SanitizeForLogging(s string) string {
	if s == "" {
		return ""
	}

	// First, apply general secret redaction
	result := RedactSecrets(s)

	// Truncate if too long (to avoid logging massive payloads)
	maxLogLength := 1000
	if len(result) > maxLogLength {
		result = result[:maxLogLength] + "... [truncated]"
	}

	return result
}

// IsSQLKeyword checks if a string is a SQL keyword
func IsSQLKeyword(s string) bool {
	upper := strings.ToUpper(s)
	return reservedKeywords[upper]
}
