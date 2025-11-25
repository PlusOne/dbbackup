package progress

import (
	"testing"
	"time"
)

func TestNewETAEstimator(t *testing.T) {
	estimator := NewETAEstimator("Test Operation", 10)
	
	if estimator.operation != "Test Operation" {
		t.Errorf("Expected operation 'Test Operation', got '%s'", estimator.operation)
	}
	
	if estimator.totalItems != 10 {
		t.Errorf("Expected totalItems 10, got %d", estimator.totalItems)
	}
	
	if estimator.itemsComplete != 0 {
		t.Errorf("Expected itemsComplete 0, got %d", estimator.itemsComplete)
	}
	
	if estimator.startTime.IsZero() {
		t.Error("Expected startTime to be set")
	}
}

func TestUpdateProgress(t *testing.T) {
	estimator := NewETAEstimator("Test", 10)
	
	estimator.UpdateProgress(5)
	if estimator.itemsComplete != 5 {
		t.Errorf("Expected itemsComplete 5, got %d", estimator.itemsComplete)
	}
	
	estimator.UpdateProgress(8)
	if estimator.itemsComplete != 8 {
		t.Errorf("Expected itemsComplete 8, got %d", estimator.itemsComplete)
	}
}

func TestGetProgress(t *testing.T) {
	estimator := NewETAEstimator("Test", 10)
	
	// Test 0% progress
	if progress := estimator.GetProgress(); progress != 0 {
		t.Errorf("Expected 0%%, got %.2f%%", progress)
	}
	
	// Test 50% progress
	estimator.UpdateProgress(5)
	if progress := estimator.GetProgress(); progress != 50.0 {
		t.Errorf("Expected 50%%, got %.2f%%", progress)
	}
	
	// Test 100% progress
	estimator.UpdateProgress(10)
	if progress := estimator.GetProgress(); progress != 100.0 {
		t.Errorf("Expected 100%%, got %.2f%%", progress)
	}
	
	// Test zero division
	zeroEstimator := NewETAEstimator("Test", 0)
	if progress := zeroEstimator.GetProgress(); progress != 0 {
		t.Errorf("Expected 0%% for zero totalItems, got %.2f%%", progress)
	}
}

func TestGetElapsed(t *testing.T) {
	estimator := NewETAEstimator("Test", 10)
	
	// Wait a bit
	time.Sleep(100 * time.Millisecond)
	
	elapsed := estimator.GetElapsed()
	if elapsed < 100*time.Millisecond {
		t.Errorf("Expected elapsed time >= 100ms, got %v", elapsed)
	}
}

func TestGetETA(t *testing.T) {
	estimator := NewETAEstimator("Test", 10)
	
	// No progress yet, ETA should be 0
	if eta := estimator.GetETA(); eta != 0 {
		t.Errorf("Expected ETA 0 for no progress, got %v", eta)
	}
	
	// Simulate 5 items completed in 5 seconds
	estimator.startTime = time.Now().Add(-5 * time.Second)
	estimator.UpdateProgress(5)
	
	eta := estimator.GetETA()
	// Should be approximately 5 seconds (5 items remaining at 1 sec/item)
	if eta < 4*time.Second || eta > 6*time.Second {
		t.Errorf("Expected ETA around 5s, got %v", eta)
	}
}

func TestFormatProgress(t *testing.T) {
	estimator := NewETAEstimator("Test", 13)
	
	// Test at 0%
	if result := estimator.FormatProgress(); result != "0/13 (0%)" {
		t.Errorf("Expected '0/13 (0%%)', got '%s'", result)
	}
	
	// Test at 38%
	estimator.UpdateProgress(5)
	if result := estimator.FormatProgress(); result != "5/13 (38%)" {
		t.Errorf("Expected '5/13 (38%%)', got '%s'", result)
	}
	
	// Test at 100%
	estimator.UpdateProgress(13)
	if result := estimator.FormatProgress(); result != "13/13 (100%)" {
		t.Errorf("Expected '13/13 (100%%)', got '%s'", result)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{500 * time.Millisecond, "< 1s"},
		{5 * time.Second, "5s"},
		{65 * time.Second, "1m"},                        // 5 seconds not shown (<=5)
		{125 * time.Second, "2m"},                       // 5 seconds not shown (<=5)
		{3 * time.Minute, "3m"},
		{3*time.Minute + 3*time.Second, "3m"},           // < 5 seconds not shown
		{3*time.Minute + 10*time.Second, "3m 10s"},      // > 5 seconds shown
		{90 * time.Minute, "1h 30m"},
		{120 * time.Minute, "2h"},
		{150 * time.Minute, "2h 30m"},
	}
	
	for _, tt := range tests {
		result := FormatDuration(tt.duration)
		if result != tt.expected {
			t.Errorf("FormatDuration(%v) = '%s', expected '%s'", tt.duration, result, tt.expected)
		}
	}
}

func TestFormatETA(t *testing.T) {
	estimator := NewETAEstimator("Test", 10)
	
	// No progress - should show "calculating..."
	if result := estimator.FormatETA(); result != "calculating..." {
		t.Errorf("Expected 'calculating...', got '%s'", result)
	}
	
	// With progress
	estimator.startTime = time.Now().Add(-10 * time.Second)
	estimator.UpdateProgress(5)
	
	result := estimator.FormatETA()
	if result != "~10s remaining" {
		t.Errorf("Expected '~10s remaining', got '%s'", result)
	}
}

func TestFormatElapsed(t *testing.T) {
	estimator := NewETAEstimator("Test", 10)
	estimator.startTime = time.Now().Add(-45 * time.Second)
	
	result := estimator.FormatElapsed()
	if result != "45s" {
		t.Errorf("Expected '45s', got '%s'", result)
	}
}

func TestGetFullStatus(t *testing.T) {
	estimator := NewETAEstimator("Backing up cluster", 13)
	
	// Just started (0 items)
	result := estimator.GetFullStatus("Backing up cluster")
	if result != "Backing up cluster | 0/13 | Starting..." {
		t.Errorf("Unexpected result for 0 items: '%s'", result)
	}
	
	// With progress
	estimator.startTime = time.Now().Add(-30 * time.Second)
	estimator.UpdateProgress(5)
	
	result = estimator.GetFullStatus("Backing up cluster")
	// Should contain all components
	if len(result) < 50 { // Reasonable minimum length
		t.Errorf("Result too short: '%s'", result)
	}
	
	// Check it contains key elements (format may vary slightly)
	if !contains(result, "5/13") {
		t.Errorf("Result missing progress '5/13': '%s'", result)
	}
	if !contains(result, "38%") {
		t.Errorf("Result missing percentage '38%%': '%s'", result)
	}
	if !contains(result, "Elapsed:") {
		t.Errorf("Result missing 'Elapsed:': '%s'", result)
	}
	if !contains(result, "ETA:") {
		t.Errorf("Result missing 'ETA:': '%s'", result)
	}
}

func TestGetFullStatusWithZeroItems(t *testing.T) {
	estimator := NewETAEstimator("Test Operation", 0)
	estimator.startTime = time.Now().Add(-5 * time.Second)
	
	result := estimator.GetFullStatus("Test Operation")
	// Should only show elapsed time when no items to track
	if !contains(result, "Test Operation") || !contains(result, "Elapsed:") {
		t.Errorf("Unexpected result for 0 total items: '%s'", result)
	}
	if contains(result, "0/0") {
		t.Errorf("Should not show 0/0 progress: '%s'", result)
	}
}

func TestEstimateSizeBasedDuration(t *testing.T) {
	// Test 100MB with 1 core
	duration := EstimateSizeBasedDuration(100*1024*1024, 1)
	// Should be around 1.2 minutes (1 minute base + 20% buffer)
	if duration < 60*time.Second || duration > 90*time.Second {
		t.Errorf("Expected ~1.2 minutes for 100MB/1core, got %v", duration)
	}
	
	// Test 100MB with 8 cores (should be faster)
	duration8cores := EstimateSizeBasedDuration(100*1024*1024, 8)
	if duration8cores >= duration {
		t.Errorf("Expected faster with more cores: %v vs %v", duration8cores, duration)
	}
	
	// Test larger file
	duration1GB := EstimateSizeBasedDuration(1024*1024*1024, 1)
	if duration1GB <= duration {
		t.Errorf("Expected longer duration for larger file: %v vs %v", duration1GB, duration)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		len(s) > len(substr) && (
			s[:len(substr)] == substr || 
			s[len(s)-len(substr):] == substr ||
			indexHelper(s, substr) >= 0))
}

func indexHelper(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
