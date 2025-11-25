package tui

import (
	"context"
	"fmt"
	"sync"
	"time"

	"dbbackup/internal/backup"
	"dbbackup/internal/config"
	"dbbackup/internal/database"
	"dbbackup/internal/logger"
	"dbbackup/internal/progress"
)

// TUIProgressReporter is a progress reporter that integrates with the TUI
type TUIProgressReporter struct {
	mu                 sync.RWMutex
	operations         map[string]*progress.OperationStatus
	callbacks          []func([]progress.OperationStatus)
	defaultOperationID string
}

// NewTUIProgressReporter creates a new TUI-compatible progress reporter
func NewTUIProgressReporter() *TUIProgressReporter {
	return &TUIProgressReporter{
		operations: make(map[string]*progress.OperationStatus),
		callbacks:  make([]func([]progress.OperationStatus), 0),
	}
}

// AddCallback adds a callback function to be called when operations update
func (t *TUIProgressReporter) AddCallback(callback func([]progress.OperationStatus)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.callbacks = append(t.callbacks, callback)
}

// notifyCallbacks calls all registered callbacks with current operations
func (t *TUIProgressReporter) notifyCallbacks() {
	operations := make([]progress.OperationStatus, 0, len(t.operations))
	for _, op := range t.operations {
		operations = append(operations, *op)
	}

	for _, callback := range t.callbacks {
		go callback(operations)
	}
}

func (t *TUIProgressReporter) ensureDefaultOperationLocked(message string) *progress.OperationStatus {
	if t.defaultOperationID == "" {
		t.defaultOperationID = fmt.Sprintf("tui-progress-%d", time.Now().UnixNano())
	}

	op, exists := t.operations[t.defaultOperationID]
	if !exists {
		op = &progress.OperationStatus{
			ID:        t.defaultOperationID,
			Name:      "Backup Progress",
			Type:      "indicator",
			Status:    "running",
			StartTime: time.Now(),
			Message:   message,
			Progress:  0,
			Details:   make(map[string]string),
			Steps:     make([]progress.StepStatus, 0),
		}
		t.operations[t.defaultOperationID] = op
	}

	if message != "" {
		op.Message = message
	}
	return op
}

func (t *TUIProgressReporter) Start(message string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	op := t.ensureDefaultOperationLocked(message)
	now := time.Now()
	op.Status = "running"
	op.StartTime = now
	op.EndTime = nil
	op.Progress = 0
	op.Message = message
	t.notifyCallbacks()
}

func (t *TUIProgressReporter) Update(message string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	op := t.ensureDefaultOperationLocked(message)
	if op.Progress < 95 {
		op.Progress += 5
	}
	op.Message = message
	t.notifyCallbacks()
}

func (t *TUIProgressReporter) Complete(message string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.defaultOperationID == "" {
		return
	}
	if op, exists := t.operations[t.defaultOperationID]; exists {
		now := time.Now()
		op.Status = "completed"
		op.Message = message
		op.Progress = 100
		op.EndTime = &now
		op.Duration = now.Sub(op.StartTime)
		t.notifyCallbacks()
	}
}

func (t *TUIProgressReporter) Fail(message string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.defaultOperationID == "" {
		return
	}
	if op, exists := t.operations[t.defaultOperationID]; exists {
		now := time.Now()
		op.Status = "failed"
		op.Message = message
		op.EndTime = &now
		op.Duration = now.Sub(op.StartTime)
		t.notifyCallbacks()
	}
}

func (t *TUIProgressReporter) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.defaultOperationID == "" {
		return
	}
	if op, exists := t.operations[t.defaultOperationID]; exists {
		if op.Status == "running" {
			now := time.Now()
			op.Status = "stopped"
			op.EndTime = &now
			op.Duration = now.Sub(op.StartTime)
		}
		t.notifyCallbacks()
	}
}

// StartOperation starts tracking a new operation
func (t *TUIProgressReporter) StartOperation(id, name, opType string) *TUIOperationTracker {
	t.mu.Lock()
	defer t.mu.Unlock()

	operation := &progress.OperationStatus{
		ID:        id,
		Name:      name,
		Type:      opType,
		Status:    "running",
		StartTime: time.Now(),
		Progress:  0,
		Message:   fmt.Sprintf("Starting %s: %s", opType, name),
		Details:   make(map[string]string),
		Steps:     make([]progress.StepStatus, 0),
	}

	t.operations[id] = operation
	t.notifyCallbacks()

	return &TUIOperationTracker{
		reporter:    t,
		operationID: id,
	}
}

// TUIOperationTracker tracks progress for TUI display
type TUIOperationTracker struct {
	reporter    *TUIProgressReporter
	operationID string
}

// UpdateProgress updates the operation progress
func (t *TUIOperationTracker) UpdateProgress(progress int, message string) {
	t.reporter.mu.Lock()
	defer t.reporter.mu.Unlock()

	if op, exists := t.reporter.operations[t.operationID]; exists {
		op.Progress = progress
		op.Message = message
		t.reporter.notifyCallbacks()
	}
}

// Complete marks the operation as completed
func (t *TUIOperationTracker) Complete(message string) {
	t.reporter.mu.Lock()
	defer t.reporter.mu.Unlock()

	if op, exists := t.reporter.operations[t.operationID]; exists {
		now := time.Now()
		op.Status = "completed"
		op.Progress = 100
		op.Message = message
		op.EndTime = &now
		op.Duration = now.Sub(op.StartTime)
		t.reporter.notifyCallbacks()
	}
}

// Fail marks the operation as failed
func (t *TUIOperationTracker) Fail(message string) {
	t.reporter.mu.Lock()
	defer t.reporter.mu.Unlock()

	if op, exists := t.reporter.operations[t.operationID]; exists {
		now := time.Now()
		op.Status = "failed"
		op.Message = message
		op.EndTime = &now
		op.Duration = now.Sub(op.StartTime)
		t.reporter.notifyCallbacks()
	}
}

// GetOperations returns all current operations
func (t *TUIProgressReporter) GetOperations() []progress.OperationStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()

	operations := make([]progress.OperationStatus, 0, len(t.operations))
	for _, op := range t.operations {
		operations = append(operations, *op)
	}
	return operations
}

// SilentLogger implements logger.Logger but doesn't output anything
type SilentLogger struct{}

func (s *SilentLogger) Info(msg string, args ...any)  {}
func (s *SilentLogger) Warn(msg string, args ...any)  {}
func (s *SilentLogger) Error(msg string, args ...any) {}
func (s *SilentLogger) Debug(msg string, args ...any) {}
func (s *SilentLogger) Time(msg string, args ...any)  {}
func (s *SilentLogger) StartOperation(name string) logger.OperationLogger {
	return &SilentOperation{}
}
func (s *SilentLogger) WithFields(fields map[string]interface{}) logger.Logger {
	return s
}
func (s *SilentLogger) WithField(key string, value interface{}) logger.Logger {
	return s
}

// SilentOperation implements logger.OperationLogger but doesn't output anything
type SilentOperation struct{}

func (s *SilentOperation) Update(message string, args ...any)   {}
func (s *SilentOperation) Complete(message string, args ...any) {}
func (s *SilentOperation) Fail(message string, args ...any)     {}

// SilentProgressIndicator implements progress.Indicator but doesn't output anything
type SilentProgressIndicator struct{}

func (s *SilentProgressIndicator) Start(message string)                       {}
func (s *SilentProgressIndicator) Update(message string)                      {}
func (s *SilentProgressIndicator) Complete(message string)                    {}
func (s *SilentProgressIndicator) Fail(message string)                        {}
func (s *SilentProgressIndicator) Stop()                                      {}
func (s *SilentProgressIndicator) SetEstimator(estimator *progress.ETAEstimator) {}

// RunBackupInTUI runs a backup operation with TUI-compatible progress reporting
func RunBackupInTUI(ctx context.Context, cfg *config.Config, log logger.Logger,
	backupType string, databaseName string, reporter *TUIProgressReporter) error {

	// Create database connection
	db, err := database.New(cfg, &SilentLogger{}) // Use silent logger
	if err != nil {
		return fmt.Errorf("failed to create database connection: %w", err)
	}
	defer db.Close()

	err = db.Connect(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Create backup engine with silent progress indicator and logger
	silentProgress := &SilentProgressIndicator{}
	engine := backup.NewSilent(cfg, &SilentLogger{}, db, silentProgress)

	// Start operation tracking
	operationID := fmt.Sprintf("%s_%d", backupType, time.Now().Unix())
	tracker := reporter.StartOperation(operationID, databaseName, backupType)

	// Run the appropriate backup type
	switch backupType {
	case "single":
		tracker.UpdateProgress(10, "Preparing single database backup...")
		err = engine.BackupSingle(ctx, databaseName)
	case "cluster":
		tracker.UpdateProgress(10, "Preparing cluster backup...")
		err = engine.BackupCluster(ctx)
	case "sample":
		tracker.UpdateProgress(10, "Preparing sample backup...")
		err = engine.BackupSample(ctx, databaseName)
	default:
		err = fmt.Errorf("unknown backup type: %s", backupType)
	}

	// Update final status
	if err != nil {
		tracker.Fail(fmt.Sprintf("Backup failed: %v", err))
		return err
	} else {
		tracker.Complete(fmt.Sprintf("%s backup completed successfully", backupType))
		return nil
	}
}
