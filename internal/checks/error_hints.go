package checks

import (
	"fmt"
	"regexp"
	"strings"
)

// Compiled regex patterns for robust error matching
var errorPatterns = map[string]*regexp.Regexp{
	"already_exists":   regexp.MustCompile(`(?i)(already exists|duplicate key|unique constraint|relation.*exists)`),
	"disk_full":        regexp.MustCompile(`(?i)(no space left|disk.*full|write.*failed.*space|insufficient.*space)`),
	"lock_exhaustion":  regexp.MustCompile(`(?i)(max_locks_per_transaction|out of shared memory|lock.*exhausted|could not open large object)`),
	"syntax_error":     regexp.MustCompile(`(?i)syntax error at.*line \d+`),
	"permission_denied": regexp.MustCompile(`(?i)(permission denied|must be owner|access denied)`),
	"connection_failed": regexp.MustCompile(`(?i)(connection refused|could not connect|no pg_hba\.conf entry)`),
	"version_mismatch":  regexp.MustCompile(`(?i)(version mismatch|incompatible|unsupported version)`),
}

// ErrorClassification represents the severity and type of error
type ErrorClassification struct {
	Type     string // "ignorable", "warning", "critical", "fatal"
	Category string // "disk_space", "locks", "corruption", "permissions", "network", "syntax"
	Message  string
	Hint     string
	Action   string // Suggested command or action
	Severity int    // 0=info, 1=warning, 2=error, 3=fatal
}

// classifyErrorByPattern uses compiled regex patterns for robust error classification
func classifyErrorByPattern(msg string) string {
	for category, pattern := range errorPatterns {
		if pattern.MatchString(msg) {
			return category
		}
	}
	return "unknown"
}

// ClassifyError analyzes an error message and provides actionable hints
func ClassifyError(errorMsg string) *ErrorClassification {
	// Use regex pattern matching for robustness
	patternMatch := classifyErrorByPattern(errorMsg)
	lowerMsg := strings.ToLower(errorMsg)

	// Use pattern matching first, fall back to string matching
	switch patternMatch {
	case "already_exists":
		return &ErrorClassification{
			Type:     "ignorable",
			Category: "duplicate",
			Message:  errorMsg,
			Hint:     "Object already exists in target database - this is normal during restore",
			Action:   "No action needed - restore will continue",
			Severity: 0,
		}
	case "disk_full":
		return &ErrorClassification{
			Type:     "critical",
			Category: "disk_space",
			Message:  errorMsg,
			Hint:     "Insufficient disk space to complete operation",
			Action:   "Free up disk space: rm old_backups/* or increase storage",
			Severity: 3,
		}
	case "lock_exhaustion":
		return &ErrorClassification{
			Type:     "critical",
			Category: "locks",
			Message:  errorMsg,
			Hint:     "Lock table exhausted - typically caused by large objects in parallel restore",
			Action:   "Increase max_locks_per_transaction in postgresql.conf to 512 or higher",
			Severity: 2,
		}
	case "permission_denied":
		return &ErrorClassification{
			Type:     "critical",
			Category: "permissions",
			Message:  errorMsg,
			Hint:     "Insufficient permissions to perform operation",
			Action:   "Run as superuser or use --no-owner flag for restore",
			Severity: 2,
		}
	case "connection_failed":
		return &ErrorClassification{
			Type:     "critical",
			Category: "network",
			Message:  errorMsg,
			Hint:     "Cannot connect to database server",
			Action:   "Check database is running and pg_hba.conf allows connection",
			Severity: 2,
		}
	case "version_mismatch":
		return &ErrorClassification{
			Type:     "warning",
			Category: "version",
			Message:  errorMsg,
			Hint:     "PostgreSQL version mismatch between backup and restore target",
			Action:   "Review release notes for compatibility: https://www.postgresql.org/docs/",
			Severity: 1,
		}
	case "syntax_error":
		return &ErrorClassification{
			Type:     "critical",
			Category: "corruption",
			Message:  errorMsg,
			Hint:     "Syntax error in dump file - backup may be corrupted or incomplete",
			Action:   "Re-create backup with: dbbackup backup single <database>",
			Severity: 3,
		}
	}

	// Fallback to original string matching for backward compatibility
	if strings.Contains(lowerMsg, "already exists") {
		return &ErrorClassification{
			Type:     "ignorable",
			Category: "duplicate",
			Message:  errorMsg,
			Hint:     "Object already exists in target database - this is normal during restore",
			Action:   "No action needed - restore will continue",
			Severity: 0,
		}
	}

	// Disk space errors
	if strings.Contains(lowerMsg, "no space left") || strings.Contains(lowerMsg, "disk full") {
		return &ErrorClassification{
			Type:     "critical",
			Category: "disk_space",
			Message:  errorMsg,
			Hint:     "Insufficient disk space to complete operation",
			Action:   "Free up disk space: rm old_backups/* or increase storage",
			Severity: 3,
		}
	}

	// Lock exhaustion errors
	if strings.Contains(lowerMsg, "max_locks_per_transaction") || 
	   strings.Contains(lowerMsg, "out of shared memory") ||
	   strings.Contains(lowerMsg, "could not open large object") {
		return &ErrorClassification{
			Type:     "critical",
			Category: "locks",
			Message:  errorMsg,
			Hint:     "Lock table exhausted - typically caused by large objects in parallel restore",
			Action:   "Increase max_locks_per_transaction in postgresql.conf to 512 or higher",
			Severity: 2,
		}
	}

	// Syntax errors (corrupted dump)
	if strings.Contains(lowerMsg, "syntax error") {
		return &ErrorClassification{
			Type:     "critical",
			Category: "corruption",
			Message:  errorMsg,
			Hint:     "Syntax error in dump file - backup may be corrupted or incomplete",
			Action:   "Re-create backup with: dbbackup backup single <database>",
			Severity: 3,
		}
	}

	// Permission errors
	if strings.Contains(lowerMsg, "permission denied") || strings.Contains(lowerMsg, "must be owner") {
		return &ErrorClassification{
			Type:     "critical",
			Category: "permissions",
			Message:  errorMsg,
			Hint:     "Insufficient permissions to perform operation",
			Action:   "Run as superuser or use --no-owner flag for restore",
			Severity: 2,
		}
	}

	// Connection errors
	if strings.Contains(lowerMsg, "connection refused") || 
	   strings.Contains(lowerMsg, "could not connect") ||
	   strings.Contains(lowerMsg, "no pg_hba.conf entry") {
		return &ErrorClassification{
			Type:     "critical",
			Category: "network",
			Message:  errorMsg,
			Hint:     "Cannot connect to database server",
			Action:   "Check database is running and pg_hba.conf allows connection",
			Severity: 2,
		}
	}

	// Version compatibility warnings
	if strings.Contains(lowerMsg, "version mismatch") || strings.Contains(lowerMsg, "incompatible") {
		return &ErrorClassification{
			Type:     "warning",
			Category: "version",
			Message:  errorMsg,
			Hint:     "PostgreSQL version mismatch between backup and restore target",
			Action:   "Review release notes for compatibility: https://www.postgresql.org/docs/",
			Severity: 1,
		}
	}

	// Excessive errors (corrupted dump)
	if strings.Contains(errorMsg, "total errors:") {
		parts := strings.Split(errorMsg, "total errors:")
		if len(parts) > 1 {
			var count int
			if _, err := fmt.Sscanf(parts[1], "%d", &count); err == nil && count > 100000 {
				return &ErrorClassification{
					Type:     "fatal",
					Category: "corruption",
					Message:  errorMsg,
					Hint:     fmt.Sprintf("Excessive errors (%d) indicate severely corrupted dump file", count),
					Action:   "Re-create backup from source database",
					Severity: 3,
				}
			}
		}
	}

	// Default: unclassified error
	return &ErrorClassification{
		Type:     "error",
		Category: "unknown",
		Message:  errorMsg,
		Hint:     "An error occurred during operation",
		Action:   "Check logs for details or contact support",
		Severity: 2,
	}
}

// FormatErrorWithHint creates a user-friendly error message with hints
func FormatErrorWithHint(errorMsg string) string {
	classification := ClassifyError(errorMsg)

	var icon string
	switch classification.Type {
	case "ignorable":
		icon = "ℹ️ "
	case "warning":
		icon = "⚠️ "
	case "critical":
		icon = "❌"
	case "fatal":
		icon = "🛑"
	default:
		icon = "⚠️ "
	}

	output := fmt.Sprintf("%s %s Error\n\n", icon, strings.ToUpper(classification.Type))
	output += fmt.Sprintf("Category: %s\n", classification.Category)
	output += fmt.Sprintf("Message: %s\n\n", classification.Message)
	output += fmt.Sprintf("💡 Hint: %s\n\n", classification.Hint)
	output += fmt.Sprintf("🔧 Action: %s\n", classification.Action)

	return output
}

// FormatMultipleErrors formats multiple errors with classification
func FormatMultipleErrors(errors []string) string {
	if len(errors) == 0 {
		return "✓ No errors"
	}

	ignorable := 0
	warnings := 0
	critical := 0
	fatal := 0

	var criticalErrors []string

	for _, err := range errors {
		class := ClassifyError(err)
		switch class.Type {
		case "ignorable":
			ignorable++
		case "warning":
			warnings++
		case "critical":
			critical++
			if len(criticalErrors) < 3 { // Keep first 3 critical errors
				criticalErrors = append(criticalErrors, err)
			}
		case "fatal":
			fatal++
			criticalErrors = append(criticalErrors, err)
		}
	}

	output := "📊 Error Summary:\n\n"
	if ignorable > 0 {
		output += fmt.Sprintf("   ℹ️  %d ignorable (objects already exist)\n", ignorable)
	}
	if warnings > 0 {
		output += fmt.Sprintf("   ⚠️  %d warnings\n", warnings)
	}
	if critical > 0 {
		output += fmt.Sprintf("   ❌ %d critical errors\n", critical)
	}
	if fatal > 0 {
		output += fmt.Sprintf("   🛑 %d fatal errors\n", fatal)
	}

	if len(criticalErrors) > 0 {
		output += "\n📝 Critical Issues:\n\n"
		for i, err := range criticalErrors {
			class := ClassifyError(err)
			output += fmt.Sprintf("%d. %s\n", i+1, class.Hint)
			output += fmt.Sprintf("   Action: %s\n\n", class.Action)
		}
	}

	return output
}
