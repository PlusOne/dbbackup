package progress

import (
	"fmt"
	"time"
)

// ETAEstimator calculates estimated time remaining for operations
type ETAEstimator struct {
	startTime     time.Time
	operation     string
	totalItems    int
	itemsComplete int
	lastUpdate    time.Time
}

// NewETAEstimator creates a new ETA estimator
func NewETAEstimator(operation string, totalItems int) *ETAEstimator {
	now := time.Now()
	return &ETAEstimator{
		startTime:     now,
		operation:     operation,
		totalItems:    totalItems,
		itemsComplete: 0,
		lastUpdate:    now,
	}
}

// UpdateProgress updates the completed item count
func (e *ETAEstimator) UpdateProgress(itemsComplete int) {
	e.itemsComplete = itemsComplete
	e.lastUpdate = time.Now()
}

// GetElapsed returns elapsed time since start
func (e *ETAEstimator) GetElapsed() time.Duration {
	return time.Since(e.startTime)
}

// GetETA calculates estimated time remaining
func (e *ETAEstimator) GetETA() time.Duration {
	if e.itemsComplete == 0 || e.totalItems == 0 {
		return 0
	}
	
	elapsed := e.GetElapsed()
	avgTimePerItem := elapsed / time.Duration(e.itemsComplete)
	remainingItems := e.totalItems - e.itemsComplete
	
	return avgTimePerItem * time.Duration(remainingItems)
}

// GetProgress returns current progress as percentage
func (e *ETAEstimator) GetProgress() float64 {
	if e.totalItems == 0 {
		return 0
	}
	return float64(e.itemsComplete) / float64(e.totalItems) * 100
}

// FormatElapsed returns formatted elapsed time (e.g., "25m 30s")
func (e *ETAEstimator) FormatElapsed() string {
	return FormatDuration(e.GetElapsed())
}

// FormatETA returns formatted ETA (e.g., "~40m remaining")
func (e *ETAEstimator) FormatETA() string {
	eta := e.GetETA()
	if eta == 0 {
		return "calculating..."
	}
	return "~" + FormatDuration(eta) + " remaining"
}

// FormatProgress returns formatted progress string (e.g., "5/13 (38%)")
func (e *ETAEstimator) FormatProgress() string {
	return fmt.Sprintf("%d/%d (%.0f%%)", e.itemsComplete, e.totalItems, e.GetProgress())
}

// GetFullStatus returns complete status line with all info
func (e *ETAEstimator) GetFullStatus(baseMessage string) string {
	if e.totalItems == 0 {
		// No items to track, just show elapsed
		return fmt.Sprintf("%s | Elapsed: %s", baseMessage, e.FormatElapsed())
	}
	
	if e.itemsComplete == 0 {
		// Just started
		return fmt.Sprintf("%s | 0/%d | Starting...", baseMessage, e.totalItems)
	}
	
	// Full status with progress and ETA
	return fmt.Sprintf("%s | %s | Elapsed: %s | ETA: %s",
		baseMessage,
		e.FormatProgress(),
		e.FormatElapsed(),
		e.FormatETA())
}

// FormatDuration formats a duration in human-readable format
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return "< 1s"
	}
	
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	
	if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("%dh %dm", hours, minutes)
		}
		return fmt.Sprintf("%dh", hours)
	}
	
	if minutes > 0 {
		if seconds > 5 { // Only show seconds if > 5
			return fmt.Sprintf("%dm %ds", minutes, seconds)
		}
		return fmt.Sprintf("%dm", minutes)
	}
	
	return fmt.Sprintf("%ds", seconds)
}

// EstimateSizeBasedDuration estimates duration based on size (fallback when no progress tracking)
func EstimateSizeBasedDuration(sizeBytes int64, cores int) time.Duration {
	sizeMB := float64(sizeBytes) / (1024 * 1024)
	
	// Base estimate: ~100MB per minute on average hardware
	baseMinutes := sizeMB / 100.0
	
	// Adjust for CPU cores (more cores = faster, but not linear)
	// Use square root to represent diminishing returns
	if cores > 1 {
		speedup := 1.0 + (0.3 * (float64(cores) - 1)) // 30% improvement per core
		baseMinutes = baseMinutes / speedup
	}
	
	// Add 20% buffer for safety
	baseMinutes = baseMinutes * 1.2
	
	return time.Duration(baseMinutes * float64(time.Minute))
}
