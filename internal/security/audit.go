package security

import (
	"os"
	"time"

	"dbbackup/internal/logger"
)

// AuditEvent represents an auditable event
type AuditEvent struct {
	Timestamp  time.Time
	User       string
	Action     string
	Resource   string
	Result     string
	Details    map[string]interface{}
}

// AuditLogger provides audit logging functionality
type AuditLogger struct {
	log      logger.Logger
	enabled  bool
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(log logger.Logger, enabled bool) *AuditLogger {
	return &AuditLogger{
		log:     log,
		enabled: enabled,
	}
}

// LogBackupStart logs backup operation start
func (a *AuditLogger) LogBackupStart(user, database, backupType string) {
	if !a.enabled {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		User:      user,
		Action:    "BACKUP_START",
		Resource:  database,
		Result:    "INITIATED",
		Details: map[string]interface{}{
			"backup_type": backupType,
		},
	}

	a.logEvent(event)
}

// LogBackupComplete logs successful backup completion
func (a *AuditLogger) LogBackupComplete(user, database, archivePath string, sizeBytes int64) {
	if !a.enabled {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		User:      user,
		Action:    "BACKUP_COMPLETE",
		Resource:  database,
		Result:    "SUCCESS",
		Details: map[string]interface{}{
			"archive_path": archivePath,
			"size_bytes":   sizeBytes,
		},
	}

	a.logEvent(event)
}

// LogBackupFailed logs backup failure
func (a *AuditLogger) LogBackupFailed(user, database string, err error) {
	if !a.enabled {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		User:      user,
		Action:    "BACKUP_FAILED",
		Resource:  database,
		Result:    "FAILURE",
		Details: map[string]interface{}{
			"error": err.Error(),
		},
	}

	a.logEvent(event)
}

// LogRestoreStart logs restore operation start
func (a *AuditLogger) LogRestoreStart(user, database, archivePath string) {
	if !a.enabled {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		User:      user,
		Action:    "RESTORE_START",
		Resource:  database,
		Result:    "INITIATED",
		Details: map[string]interface{}{
			"archive_path": archivePath,
		},
	}

	a.logEvent(event)
}

// LogRestoreComplete logs successful restore completion
func (a *AuditLogger) LogRestoreComplete(user, database string, duration time.Duration) {
	if !a.enabled {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		User:      user,
		Action:    "RESTORE_COMPLETE",
		Resource:  database,
		Result:    "SUCCESS",
		Details: map[string]interface{}{
			"duration_seconds": duration.Seconds(),
		},
	}

	a.logEvent(event)
}

// LogRestoreFailed logs restore failure
func (a *AuditLogger) LogRestoreFailed(user, database string, err error) {
	if !a.enabled {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		User:      user,
		Action:    "RESTORE_FAILED",
		Resource:  database,
		Result:    "FAILURE",
		Details: map[string]interface{}{
			"error": err.Error(),
		},
	}

	a.logEvent(event)
}

// LogConfigChange logs configuration changes
func (a *AuditLogger) LogConfigChange(user, setting, oldValue, newValue string) {
	if !a.enabled {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		User:      user,
		Action:    "CONFIG_CHANGE",
		Resource:  setting,
		Result:    "SUCCESS",
		Details: map[string]interface{}{
			"old_value": oldValue,
			"new_value": newValue,
		},
	}

	a.logEvent(event)
}

// LogConnectionAttempt logs database connection attempts
func (a *AuditLogger) LogConnectionAttempt(user, host string, success bool, err error) {
	if !a.enabled {
		return
	}

	result := "SUCCESS"
	details := map[string]interface{}{
		"host": host,
	}

	if !success {
		result = "FAILURE"
		if err != nil {
			details["error"] = err.Error()
		}
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		User:      user,
		Action:    "DB_CONNECTION",
		Resource:  host,
		Result:    result,
		Details:   details,
	}

	a.logEvent(event)
}

// logEvent writes the audit event to log
func (a *AuditLogger) logEvent(event AuditEvent) {
	fields := map[string]interface{}{
		"audit":     true,
		"timestamp": event.Timestamp.Format(time.RFC3339),
		"user":      event.User,
		"action":    event.Action,
		"resource":  event.Resource,
		"result":    event.Result,
	}

	// Merge event details
	for k, v := range event.Details {
		fields[k] = v
	}

	a.log.WithFields(fields).Info("AUDIT")
}

// GetCurrentUser returns the current system user
func GetCurrentUser() string {
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	if user := os.Getenv("USERNAME"); user != "" {
		return user
	}
	return "unknown"
}
