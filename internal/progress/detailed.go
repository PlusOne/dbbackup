package progress

import (
	"fmt"
	"sync"
	"time"
)

// DetailedReporter provides comprehensive progress reporting with timestamps and status
type DetailedReporter struct {
	mu         sync.RWMutex
	operations []OperationStatus
	startTime  time.Time
	indicator  Indicator
	logger     Logger
}

// OperationStatus represents the status of a backup/restore operation
type OperationStatus struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"` // "backup", "restore", "verify"
	Status      string            `json:"status"` // "running", "completed", "failed"
	StartTime   time.Time         `json:"start_time"`
	EndTime     *time.Time        `json:"end_time,omitempty"`
	Duration    time.Duration     `json:"duration"`
	Progress    int               `json:"progress"` // 0-100
	Message     string            `json:"message"`
	Details     map[string]string `json:"details"`
	Steps       []StepStatus      `json:"steps"`
	BytesTotal  int64             `json:"bytes_total"`
	BytesDone   int64             `json:"bytes_done"`
	FilesTotal  int               `json:"files_total"`
	FilesDone   int               `json:"files_done"`
	Errors      []string          `json:"errors,omitempty"`
}

// StepStatus represents individual steps within an operation
type StepStatus struct {
	Name      string     `json:"name"`
	Status    string     `json:"status"`
	StartTime time.Time  `json:"start_time"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	Duration  time.Duration `json:"duration"`
	Message   string     `json:"message"`
}

// Logger interface for detailed reporting
type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Debug(msg string, args ...any)
}

// NewDetailedReporter creates a new detailed progress reporter
func NewDetailedReporter(indicator Indicator, logger Logger) *DetailedReporter {
	return &DetailedReporter{
		operations: make([]OperationStatus, 0),
		indicator:  indicator,
		logger:     logger,
	}
}

// StartOperation begins tracking a new operation
func (dr *DetailedReporter) StartOperation(id, name, opType string) *OperationTracker {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	operation := OperationStatus{
		ID:        id,
		Name:      name,
		Type:      opType,
		Status:    "running",
		StartTime: time.Now(),
		Progress:  0,
		Details:   make(map[string]string),
		Steps:     make([]StepStatus, 0),
	}

	dr.operations = append(dr.operations, operation)
	
	if dr.startTime.IsZero() {
		dr.startTime = time.Now()
	}

	// Start visual indicator
	if dr.indicator != nil {
		dr.indicator.Start(fmt.Sprintf("Starting %s: %s", opType, name))
	}

	// Log operation start
	dr.logger.Info("Operation started", 
		"id", id, 
		"name", name, 
		"type", opType,
		"timestamp", operation.StartTime.Format(time.RFC3339))

	return &OperationTracker{
		reporter:    dr,
		operationID: id,
	}
}

// OperationTracker provides methods to update operation progress
type OperationTracker struct {
	reporter    *DetailedReporter
	operationID string
}

// UpdateProgress updates the progress of the operation
func (ot *OperationTracker) UpdateProgress(progress int, message string) {
	ot.reporter.mu.Lock()
	defer ot.reporter.mu.Unlock()

	for i := range ot.reporter.operations {
		if ot.reporter.operations[i].ID == ot.operationID {
			ot.reporter.operations[i].Progress = progress
			ot.reporter.operations[i].Message = message
			
			// Update visual indicator
			if ot.reporter.indicator != nil {
				progressMsg := fmt.Sprintf("[%d%%] %s", progress, message)
				ot.reporter.indicator.Update(progressMsg)
			}

			// Log progress update
			ot.reporter.logger.Debug("Progress update",
				"operation_id", ot.operationID,
				"progress", progress,
				"message", message,
				"timestamp", time.Now().Format(time.RFC3339))
			break
		}
	}
}

// AddStep adds a new step to the operation
func (ot *OperationTracker) AddStep(name, message string) *StepTracker {
	ot.reporter.mu.Lock()
	defer ot.reporter.mu.Unlock()

	step := StepStatus{
		Name:      name,
		Status:    "running",
		StartTime: time.Now(),
		Message:   message,
	}

	for i := range ot.reporter.operations {
		if ot.reporter.operations[i].ID == ot.operationID {
			ot.reporter.operations[i].Steps = append(ot.reporter.operations[i].Steps, step)
			
			// Log step start
			ot.reporter.logger.Info("Step started",
				"operation_id", ot.operationID,
				"step", name,
				"message", message,
				"timestamp", step.StartTime.Format(time.RFC3339))
			break
		}
	}

	return &StepTracker{
		reporter:    ot.reporter,
		operationID: ot.operationID,
		stepName:    name,
	}
}

// SetDetails adds metadata to the operation
func (ot *OperationTracker) SetDetails(key, value string) {
	ot.reporter.mu.Lock()
	defer ot.reporter.mu.Unlock()

	for i := range ot.reporter.operations {
		if ot.reporter.operations[i].ID == ot.operationID {
			ot.reporter.operations[i].Details[key] = value
			break
		}
	}
}

// SetFileProgress updates file-based progress
func (ot *OperationTracker) SetFileProgress(filesDone, filesTotal int) {
	ot.reporter.mu.Lock()
	defer ot.reporter.mu.Unlock()

	for i := range ot.reporter.operations {
		if ot.reporter.operations[i].ID == ot.operationID {
			ot.reporter.operations[i].FilesDone = filesDone
			ot.reporter.operations[i].FilesTotal = filesTotal
			
			if filesTotal > 0 {
				progress := (filesDone * 100) / filesTotal
				ot.reporter.operations[i].Progress = progress
			}
			break
		}
	}
}

// SetByteProgress updates byte-based progress
func (ot *OperationTracker) SetByteProgress(bytesDone, bytesTotal int64) {
	ot.reporter.mu.Lock()
	defer ot.reporter.mu.Unlock()

	for i := range ot.reporter.operations {
		if ot.reporter.operations[i].ID == ot.operationID {
			ot.reporter.operations[i].BytesDone = bytesDone
			ot.reporter.operations[i].BytesTotal = bytesTotal
			
			if bytesTotal > 0 {
				progress := int((bytesDone * 100) / bytesTotal)
				ot.reporter.operations[i].Progress = progress
			}
			break
		}
	}
}

// Complete marks the operation as completed
func (ot *OperationTracker) Complete(message string) {
	ot.reporter.mu.Lock()
	defer ot.reporter.mu.Unlock()

	now := time.Now()
	for i := range ot.reporter.operations {
		if ot.reporter.operations[i].ID == ot.operationID {
			ot.reporter.operations[i].Status = "completed"
			ot.reporter.operations[i].Progress = 100
			ot.reporter.operations[i].EndTime = &now
			ot.reporter.operations[i].Duration = now.Sub(ot.reporter.operations[i].StartTime)
			ot.reporter.operations[i].Message = message
			
			// Complete visual indicator
			if ot.reporter.indicator != nil {
				ot.reporter.indicator.Complete(fmt.Sprintf("✅ %s", message))
			}

			// Log completion with duration
			ot.reporter.logger.Info("Operation completed",
				"operation_id", ot.operationID,
				"message", message,
				"duration", ot.reporter.operations[i].Duration.String(),
				"timestamp", now.Format(time.RFC3339))
			break
		}
	}
}

// Fail marks the operation as failed
func (ot *OperationTracker) Fail(err error) {
	ot.reporter.mu.Lock()
	defer ot.reporter.mu.Unlock()

	now := time.Now()
	for i := range ot.reporter.operations {
		if ot.reporter.operations[i].ID == ot.operationID {
			ot.reporter.operations[i].Status = "failed"
			ot.reporter.operations[i].EndTime = &now
			ot.reporter.operations[i].Duration = now.Sub(ot.reporter.operations[i].StartTime)
			ot.reporter.operations[i].Message = err.Error()
			ot.reporter.operations[i].Errors = append(ot.reporter.operations[i].Errors, err.Error())
			
			// Fail visual indicator
			if ot.reporter.indicator != nil {
				ot.reporter.indicator.Fail(fmt.Sprintf("❌ %s", err.Error()))
			}

			// Log failure
			ot.reporter.logger.Error("Operation failed",
				"operation_id", ot.operationID,
				"error", err.Error(),
				"duration", ot.reporter.operations[i].Duration.String(),
				"timestamp", now.Format(time.RFC3339))
			break
		}
	}
}

// StepTracker manages individual step progress
type StepTracker struct {
	reporter    *DetailedReporter
	operationID string
	stepName    string
}

// Complete marks the step as completed
func (st *StepTracker) Complete(message string) {
	st.reporter.mu.Lock()
	defer st.reporter.mu.Unlock()

	now := time.Now()
	for i := range st.reporter.operations {
		if st.reporter.operations[i].ID == st.operationID {
			for j := range st.reporter.operations[i].Steps {
				if st.reporter.operations[i].Steps[j].Name == st.stepName {
					st.reporter.operations[i].Steps[j].Status = "completed"
					st.reporter.operations[i].Steps[j].EndTime = &now
					st.reporter.operations[i].Steps[j].Duration = now.Sub(st.reporter.operations[i].Steps[j].StartTime)
					st.reporter.operations[i].Steps[j].Message = message
					
					// Log step completion
					st.reporter.logger.Info("Step completed",
						"operation_id", st.operationID,
						"step", st.stepName,
						"message", message,
						"duration", st.reporter.operations[i].Steps[j].Duration.String(),
						"timestamp", now.Format(time.RFC3339))
					break
				}
			}
			break
		}
	}
}

// Fail marks the step as failed
func (st *StepTracker) Fail(err error) {
	st.reporter.mu.Lock()
	defer st.reporter.mu.Unlock()

	now := time.Now()
	for i := range st.reporter.operations {
		if st.reporter.operations[i].ID == st.operationID {
			for j := range st.reporter.operations[i].Steps {
				if st.reporter.operations[i].Steps[j].Name == st.stepName {
					st.reporter.operations[i].Steps[j].Status = "failed"
					st.reporter.operations[i].Steps[j].EndTime = &now
					st.reporter.operations[i].Steps[j].Duration = now.Sub(st.reporter.operations[i].Steps[j].StartTime)
					st.reporter.operations[i].Steps[j].Message = err.Error()
					
					// Log step failure
					st.reporter.logger.Error("Step failed",
						"operation_id", st.operationID,
						"step", st.stepName,
						"error", err.Error(),
						"duration", st.reporter.operations[i].Steps[j].Duration.String(),
						"timestamp", now.Format(time.RFC3339))
					break
				}
			}
			break
		}
	}
}

// GetOperationStatus returns the current status of an operation
func (dr *DetailedReporter) GetOperationStatus(id string) *OperationStatus {
	dr.mu.RLock()
	defer dr.mu.RUnlock()

	for _, op := range dr.operations {
		if op.ID == id {
			return &op
		}
	}
	return nil
}

// GetAllOperations returns all tracked operations
func (dr *DetailedReporter) GetAllOperations() []OperationStatus {
	dr.mu.RLock()
	defer dr.mu.RUnlock()

	return append([]OperationStatus(nil), dr.operations...)
}

// GetSummary returns a summary of all operations
func (dr *DetailedReporter) GetSummary() OperationSummary {
	dr.mu.RLock()
	defer dr.mu.RUnlock()

	summary := OperationSummary{
		TotalOperations:     len(dr.operations),
		CompletedOperations: 0,
		FailedOperations:    0,
		RunningOperations:   0,
		TotalDuration:       time.Since(dr.startTime),
	}

	for _, op := range dr.operations {
		switch op.Status {
		case "completed":
			summary.CompletedOperations++
		case "failed":
			summary.FailedOperations++
		case "running":
			summary.RunningOperations++
		}
	}

	return summary
}

// OperationSummary provides overall statistics
type OperationSummary struct {
	TotalOperations     int           `json:"total_operations"`
	CompletedOperations int           `json:"completed_operations"`
	FailedOperations    int           `json:"failed_operations"`
	RunningOperations   int           `json:"running_operations"`
	TotalDuration       time.Duration `json:"total_duration"`
}

// FormatSummary returns a formatted string representation of the summary
func (os *OperationSummary) FormatSummary() string {
	return fmt.Sprintf(
		"📊 Operations Summary:\n"+
		"  Total: %d | Completed: %d | Failed: %d | Running: %d\n"+
		"  Total Duration: %s",
		os.TotalOperations,
		os.CompletedOperations,
		os.FailedOperations,
		os.RunningOperations,
		formatDuration(os.TotalDuration))
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}