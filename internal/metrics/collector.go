package metrics

import (
	"sync"
	"time"

	"dbbackup/internal/logger"
)

// OperationMetrics holds performance metrics for database operations
type OperationMetrics struct {
	Operation        string        `json:"operation"`
	Database         string        `json:"database"`
	StartTime        time.Time     `json:"start_time"`
	Duration         time.Duration `json:"duration"`
	SizeBytes        int64         `json:"size_bytes"`
	CompressionRatio float64       `json:"compression_ratio,omitempty"`
	ThroughputMBps   float64       `json:"throughput_mbps"`
	ErrorCount       int           `json:"error_count"`
	Success          bool          `json:"success"`
}

// MetricsCollector collects and reports operation metrics
type MetricsCollector struct {
	metrics []OperationMetrics
	mu      sync.RWMutex
	logger  logger.Logger
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(log logger.Logger) *MetricsCollector {
	return &MetricsCollector{
		metrics: make([]OperationMetrics, 0),
		logger:  log,
	}
}

// RecordOperation records metrics for a completed operation
func (mc *MetricsCollector) RecordOperation(operation, database string, start time.Time, sizeBytes int64, success bool, errorCount int) {
	duration := time.Since(start)
	throughput := calculateThroughput(sizeBytes, duration)
	
	metric := OperationMetrics{
		Operation:      operation,
		Database:       database,
		StartTime:      start,
		Duration:       duration,
		SizeBytes:      sizeBytes,
		ThroughputMBps: throughput,
		ErrorCount:     errorCount,
		Success:        success,
	}
	
	mc.mu.Lock()
	mc.metrics = append(mc.metrics, metric)
	mc.mu.Unlock()
	
	// Log structured metrics
	if mc.logger != nil {
		fields := map[string]interface{}{
			"metric_type":     "operation_complete",
			"operation":       operation,
			"database":        database,
			"duration_ms":     duration.Milliseconds(),
			"size_bytes":      sizeBytes,
			"throughput_mbps": throughput,
			"error_count":     errorCount,
			"success":         success,
		}
		
		if success {
			mc.logger.WithFields(fields).Info("Operation completed successfully")
		} else {
			mc.logger.WithFields(fields).Error("Operation failed")
		}
	}
}

// RecordCompressionRatio updates compression ratio for a recorded operation
func (mc *MetricsCollector) RecordCompressionRatio(operation, database string, ratio float64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	// Find and update the most recent matching operation
	for i := len(mc.metrics) - 1; i >= 0; i-- {
		if mc.metrics[i].Operation == operation && mc.metrics[i].Database == database {
			mc.metrics[i].CompressionRatio = ratio
			break
		}
	}
}

// GetMetrics returns a copy of all collected metrics
func (mc *MetricsCollector) GetMetrics() []OperationMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	result := make([]OperationMetrics, len(mc.metrics))
	copy(result, mc.metrics)
	return result
}

// GetAverages calculates average performance metrics
func (mc *MetricsCollector) GetAverages() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	if len(mc.metrics) == 0 {
		return map[string]interface{}{}
	}
	
	var totalDuration time.Duration
	var totalSize, totalThroughput float64
	var successCount, errorCount int
	
	for _, m := range mc.metrics {
		totalDuration += m.Duration
		totalSize += float64(m.SizeBytes)
		totalThroughput += m.ThroughputMBps
		if m.Success {
			successCount++
		}
		errorCount += m.ErrorCount
	}
	
	count := len(mc.metrics)
	return map[string]interface{}{
		"total_operations":      count,
		"success_rate":          float64(successCount) / float64(count) * 100,
		"avg_duration_ms":       totalDuration.Milliseconds() / int64(count),
		"avg_size_mb":           totalSize / float64(count) / 1024 / 1024,
		"avg_throughput_mbps":   totalThroughput / float64(count),
		"total_errors":          errorCount,
	}
}

// Clear removes all collected metrics
func (mc *MetricsCollector) Clear() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.metrics = make([]OperationMetrics, 0)
}

// calculateThroughput calculates MB/s throughput
func calculateThroughput(bytes int64, duration time.Duration) float64 {
	if duration == 0 {
		return 0
	}
	seconds := duration.Seconds()
	if seconds == 0 {
		return 0
	}
	return float64(bytes) / seconds / 1024 / 1024
}

// Global metrics collector instance
var GlobalMetrics *MetricsCollector

// InitGlobalMetrics initializes the global metrics collector
func InitGlobalMetrics(log logger.Logger) {
	GlobalMetrics = NewMetricsCollector(log)
}