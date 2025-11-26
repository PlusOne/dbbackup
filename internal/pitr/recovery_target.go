package pitr

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// RecoveryTarget represents a PostgreSQL recovery target
type RecoveryTarget struct {
	Type     string // "time", "xid", "lsn", "name", "immediate"
	Value    string // The target value (timestamp, XID, LSN, or restore point name)
	Action   string // "promote", "pause", "shutdown"
	Timeline string // Timeline to follow ("latest" or timeline ID)
	Inclusive bool   // Whether target is inclusive (default: true)
}

// RecoveryTargetType constants
const (
	TargetTypeTime      = "time"
	TargetTypeXID       = "xid"
	TargetTypeLSN       = "lsn"
	TargetTypeName      = "name"
	TargetTypeImmediate = "immediate"
)

// RecoveryAction constants
const (
	ActionPromote  = "promote"
	ActionPause    = "pause"
	ActionShutdown = "shutdown"
)

// ParseRecoveryTarget creates a RecoveryTarget from CLI flags
func ParseRecoveryTarget(
	targetTime, targetXID, targetLSN, targetName string,
	targetImmediate bool,
	targetAction, timeline string,
	inclusive bool,
) (*RecoveryTarget, error) {
	rt := &RecoveryTarget{
		Action:    targetAction,
		Timeline:  timeline,
		Inclusive: inclusive,
	}

	// Validate action
	if rt.Action == "" {
		rt.Action = ActionPromote // Default
	}
	if !isValidAction(rt.Action) {
		return nil, fmt.Errorf("invalid recovery action: %s (must be promote, pause, or shutdown)", rt.Action)
	}

	// Determine target type (only one can be specified)
	targetsSpecified := 0
	if targetTime != "" {
		rt.Type = TargetTypeTime
		rt.Value = targetTime
		targetsSpecified++
	}
	if targetXID != "" {
		rt.Type = TargetTypeXID
		rt.Value = targetXID
		targetsSpecified++
	}
	if targetLSN != "" {
		rt.Type = TargetTypeLSN
		rt.Value = targetLSN
		targetsSpecified++
	}
	if targetName != "" {
		rt.Type = TargetTypeName
		rt.Value = targetName
		targetsSpecified++
	}
	if targetImmediate {
		rt.Type = TargetTypeImmediate
		rt.Value = "immediate"
		targetsSpecified++
	}

	if targetsSpecified == 0 {
		return nil, fmt.Errorf("no recovery target specified (use --target-time, --target-xid, --target-lsn, --target-name, or --target-immediate)")
	}
	if targetsSpecified > 1 {
		return nil, fmt.Errorf("multiple recovery targets specified, only one allowed")
	}

	// Validate the target
	if err := rt.Validate(); err != nil {
		return nil, err
	}

	return rt, nil
}

// Validate validates the recovery target configuration
func (rt *RecoveryTarget) Validate() error {
	if rt.Type == "" {
		return fmt.Errorf("recovery target type not specified")
	}

	switch rt.Type {
	case TargetTypeTime:
		return rt.validateTime()
	case TargetTypeXID:
		return rt.validateXID()
	case TargetTypeLSN:
		return rt.validateLSN()
	case TargetTypeName:
		return rt.validateName()
	case TargetTypeImmediate:
		// Immediate has no value to validate
		return nil
	default:
		return fmt.Errorf("unknown recovery target type: %s", rt.Type)
	}
}

// validateTime validates a timestamp target
func (rt *RecoveryTarget) validateTime() error {
	if rt.Value == "" {
		return fmt.Errorf("recovery target time is empty")
	}

	// Try parsing various timestamp formats
	formats := []string{
		"2006-01-02 15:04:05",           // Standard format
		"2006-01-02 15:04:05.999999",    // With microseconds
		"2006-01-02T15:04:05",           // ISO 8601
		"2006-01-02T15:04:05Z",          // ISO 8601 with UTC
		"2006-01-02T15:04:05-07:00",     // ISO 8601 with timezone
		time.RFC3339,                     // RFC3339
		time.RFC3339Nano,                 // RFC3339 with nanoseconds
	}

	var parseErr error
	for _, format := range formats {
		_, err := time.Parse(format, rt.Value)
		if err == nil {
			return nil // Successfully parsed
		}
		parseErr = err
	}

	return fmt.Errorf("invalid timestamp format '%s': %w (expected format: YYYY-MM-DD HH:MM:SS)", rt.Value, parseErr)
}

// validateXID validates a transaction ID target
func (rt *RecoveryTarget) validateXID() error {
	if rt.Value == "" {
		return fmt.Errorf("recovery target XID is empty")
	}

	// XID must be a positive integer
	xid, err := strconv.ParseUint(rt.Value, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid transaction ID '%s': must be a positive integer", rt.Value)
	}

	if xid == 0 {
		return fmt.Errorf("invalid transaction ID 0: XID must be greater than 0")
	}

	return nil
}

// validateLSN validates a Log Sequence Number target
func (rt *RecoveryTarget) validateLSN() error {
	if rt.Value == "" {
		return fmt.Errorf("recovery target LSN is empty")
	}

	// LSN format: XXX/XXXXXXXX (hex/hex)
	// Example: 0/3000000, 1/A2000000
	lsnPattern := regexp.MustCompile(`^[0-9A-Fa-f]+/[0-9A-Fa-f]+$`)
	if !lsnPattern.MatchString(rt.Value) {
		return fmt.Errorf("invalid LSN format '%s': expected format XXX/XXXXXXXX (e.g., 0/3000000)", rt.Value)
	}

	// Validate both parts are valid hex
	parts := strings.Split(rt.Value, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid LSN format '%s': must contain exactly one '/'", rt.Value)
	}

	for i, part := range parts {
		if _, err := strconv.ParseUint(part, 16, 64); err != nil {
			return fmt.Errorf("invalid LSN component %d '%s': must be hexadecimal", i+1, part)
		}
	}

	return nil
}

// validateName validates a restore point name target
func (rt *RecoveryTarget) validateName() error {
	if rt.Value == "" {
		return fmt.Errorf("recovery target name is empty")
	}

	// PostgreSQL restore point names have some restrictions
	// They should be valid identifiers
	if len(rt.Value) > 63 {
		return fmt.Errorf("restore point name too long: %d characters (max 63)", len(rt.Value))
	}

	// Check for invalid characters (only alphanumeric, underscore, hyphen)
	validName := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !validName.MatchString(rt.Value) {
		return fmt.Errorf("invalid restore point name '%s': only alphanumeric, underscore, and hyphen allowed", rt.Value)
	}

	return nil
}

// isValidAction checks if the recovery action is valid
func isValidAction(action string) bool {
	switch strings.ToLower(action) {
	case ActionPromote, ActionPause, ActionShutdown:
		return true
	default:
		return false
	}
}

// ToPostgreSQLConfig converts the recovery target to PostgreSQL configuration parameters
// Returns a map of config keys to values suitable for postgresql.auto.conf or recovery.conf
func (rt *RecoveryTarget) ToPostgreSQLConfig() map[string]string {
	config := make(map[string]string)

	// Set recovery target based on type
	switch rt.Type {
	case TargetTypeTime:
		config["recovery_target_time"] = rt.Value
	case TargetTypeXID:
		config["recovery_target_xid"] = rt.Value
	case TargetTypeLSN:
		config["recovery_target_lsn"] = rt.Value
	case TargetTypeName:
		config["recovery_target_name"] = rt.Value
	case TargetTypeImmediate:
		config["recovery_target"] = "immediate"
	}

	// Set recovery target action
	config["recovery_target_action"] = rt.Action

	// Set timeline
	if rt.Timeline != "" {
		config["recovery_target_timeline"] = rt.Timeline
	} else {
		config["recovery_target_timeline"] = "latest"
	}

	// Set inclusive flag (only for time, xid, lsn targets)
	if rt.Type != TargetTypeImmediate && rt.Type != TargetTypeName {
		if rt.Inclusive {
			config["recovery_target_inclusive"] = "true"
		} else {
			config["recovery_target_inclusive"] = "false"
		}
	}

	return config
}

// FormatConfigLine formats a config key-value pair for PostgreSQL config files
func FormatConfigLine(key, value string) string {
	// Quote values that contain spaces or special characters
	needsQuoting := strings.ContainsAny(value, " \t#'\"\\")
	if needsQuoting {
		// Escape single quotes
		value = strings.ReplaceAll(value, "'", "''")
		return fmt.Sprintf("%s = '%s'", key, value)
	}
	return fmt.Sprintf("%s = %s", key, value)
}

// String returns a human-readable representation of the recovery target
func (rt *RecoveryTarget) String() string {
	var sb strings.Builder
	
	sb.WriteString("Recovery Target:\n")
	sb.WriteString(fmt.Sprintf("  Type:      %s\n", rt.Type))
	
	if rt.Type != TargetTypeImmediate {
		sb.WriteString(fmt.Sprintf("  Value:     %s\n", rt.Value))
	}
	
	sb.WriteString(fmt.Sprintf("  Action:    %s\n", rt.Action))
	
	if rt.Timeline != "" {
		sb.WriteString(fmt.Sprintf("  Timeline:  %s\n", rt.Timeline))
	}
	
	if rt.Type != TargetTypeImmediate && rt.Type != TargetTypeName {
		sb.WriteString(fmt.Sprintf("  Inclusive: %v\n", rt.Inclusive))
	}
	
	return sb.String()
}

// Summary returns a one-line summary of the recovery target
func (rt *RecoveryTarget) Summary() string {
	switch rt.Type {
	case TargetTypeTime:
		return fmt.Sprintf("Restore to time: %s", rt.Value)
	case TargetTypeXID:
		return fmt.Sprintf("Restore to transaction ID: %s", rt.Value)
	case TargetTypeLSN:
		return fmt.Sprintf("Restore to LSN: %s", rt.Value)
	case TargetTypeName:
		return fmt.Sprintf("Restore to named point: %s", rt.Value)
	case TargetTypeImmediate:
		return "Restore to earliest consistent point"
	default:
		return "Unknown recovery target"
	}
}
