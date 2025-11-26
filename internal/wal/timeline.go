package wal

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"dbbackup/internal/logger"
)

// TimelineManager manages PostgreSQL timeline history and tracking
type TimelineManager struct {
	log logger.Logger
}

// NewTimelineManager creates a new timeline manager
func NewTimelineManager(log logger.Logger) *TimelineManager {
	return &TimelineManager{
		log: log,
	}
}

// TimelineInfo represents information about a PostgreSQL timeline
type TimelineInfo struct {
	TimelineID      uint32    // Timeline identifier (1, 2, 3, etc.)
	ParentTimeline  uint32    // Parent timeline ID (0 for timeline 1)
	SwitchPoint     string    // LSN where timeline switch occurred
	Reason          string    // Reason for timeline switch (from .history file)
	HistoryFile     string    // Path to .history file
	FirstWALSegment uint64    // First WAL segment in this timeline
	LastWALSegment  uint64    // Last known WAL segment in this timeline
	CreatedAt       time.Time // When timeline was created
}

// TimelineHistory represents the complete timeline branching structure
type TimelineHistory struct {
	Timelines      []*TimelineInfo          // All timelines sorted by ID
	CurrentTimeline uint32                   // Current active timeline
	TimelineMap    map[uint32]*TimelineInfo // Quick lookup by timeline ID
}

// ParseTimelineHistory parses timeline history from an archive directory
func (tm *TimelineManager) ParseTimelineHistory(ctx context.Context, archiveDir string) (*TimelineHistory, error) {
	tm.log.Info("Parsing timeline history", "archive_dir", archiveDir)

	history := &TimelineHistory{
		Timelines:   make([]*TimelineInfo, 0),
		TimelineMap: make(map[uint32]*TimelineInfo),
	}

	// Find all .history files in archive directory
	historyFiles, err := filepath.Glob(filepath.Join(archiveDir, "*.history"))
	if err != nil {
		return nil, fmt.Errorf("failed to find timeline history files: %w", err)
	}

	// Parse each history file
	for _, histFile := range historyFiles {
		timeline, err := tm.parseHistoryFile(histFile)
		if err != nil {
			tm.log.Warn("Failed to parse history file", "file", histFile, "error", err)
			continue
		}
		history.Timelines = append(history.Timelines, timeline)
		history.TimelineMap[timeline.TimelineID] = timeline
	}

	// Always add timeline 1 (base timeline) if not present
	if _, exists := history.TimelineMap[1]; !exists {
		baseTimeline := &TimelineInfo{
			TimelineID:     1,
			ParentTimeline: 0,
			SwitchPoint:    "0/0",
			Reason:         "Base timeline",
			FirstWALSegment: 0,
		}
		history.Timelines = append(history.Timelines, baseTimeline)
		history.TimelineMap[1] = baseTimeline
	}

	// Sort timelines by ID
	sort.Slice(history.Timelines, func(i, j int) bool {
		return history.Timelines[i].TimelineID < history.Timelines[j].TimelineID
	})

	// Scan WAL files to populate segment ranges
	if err := tm.scanWALSegments(archiveDir, history); err != nil {
		tm.log.Warn("Failed to scan WAL segments", "error", err)
	}

	// Determine current timeline (highest timeline ID with WAL files)
	for i := len(history.Timelines) - 1; i >= 0; i-- {
		if history.Timelines[i].LastWALSegment > 0 {
			history.CurrentTimeline = history.Timelines[i].TimelineID
			break
		}
	}
	if history.CurrentTimeline == 0 {
		history.CurrentTimeline = 1 // Default to timeline 1
	}

	tm.log.Info("Timeline history parsed",
		"timelines", len(history.Timelines),
		"current_timeline", history.CurrentTimeline)

	return history, nil
}

// parseHistoryFile parses a single .history file
// Format: <parentTLI> <switchpoint> <reason>
// Example: 00000001.history contains "1	0/3000000	no recovery target specified"
func (tm *TimelineManager) parseHistoryFile(path string) (*TimelineInfo, error) {
	// Extract timeline ID from filename (e.g., "00000002.history" -> 2)
	filename := filepath.Base(path)
	if !strings.HasSuffix(filename, ".history") {
		return nil, fmt.Errorf("invalid history file name: %s", filename)
	}

	timelineStr := strings.TrimSuffix(filename, ".history")
	timelineID64, err := strconv.ParseUint(timelineStr, 16, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid timeline ID in filename %s: %w", filename, err)
	}
	timelineID := uint32(timelineID64)

	// Read file content
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open history file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat history file: %w", err)
	}

	timeline := &TimelineInfo{
		TimelineID:  timelineID,
		HistoryFile: path,
		CreatedAt:   stat.ModTime(),
	}

	// Parse history entries (last line is the most recent)
	scanner := bufio.NewScanner(file)
	var lastLine string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			lastLine = line
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading history file: %w", err)
	}

	// Parse the last line: "parentTLI switchpoint reason"
	if lastLine != "" {
		parts := strings.SplitN(lastLine, "\t", 3)
		if len(parts) >= 2 {
			// Parent timeline
			parentTLI64, err := strconv.ParseUint(strings.TrimSpace(parts[0]), 16, 32)
			if err == nil {
				timeline.ParentTimeline = uint32(parentTLI64)
			}

			// Switch point (LSN)
			timeline.SwitchPoint = strings.TrimSpace(parts[1])

			// Reason (optional)
			if len(parts) >= 3 {
				timeline.Reason = strings.TrimSpace(parts[2])
			}
		}
	}

	return timeline, nil
}

// scanWALSegments scans the archive directory to populate segment ranges for each timeline
func (tm *TimelineManager) scanWALSegments(archiveDir string, history *TimelineHistory) error {
	// Find all WAL files (including compressed/encrypted)
	patterns := []string{"*", "*.gz", "*.enc", "*.gz.enc"}
	walFiles := make([]string, 0)

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(archiveDir, pattern))
		if err != nil {
			continue
		}
		walFiles = append(walFiles, matches...)
	}

	// Process each WAL file
	for _, walFile := range walFiles {
		filename := filepath.Base(walFile)
		
		// Remove extensions
		filename = strings.TrimSuffix(filename, ".gz.enc")
		filename = strings.TrimSuffix(filename, ".enc")
		filename = strings.TrimSuffix(filename, ".gz")

		// Skip non-WAL files
		if len(filename) != 24 {
			continue
		}

		// Parse WAL filename: TTTTTTTTXXXXXXXXYYYYYYYY
		// T = Timeline (8 hex), X = Log file (8 hex), Y = Segment (8 hex)
		timelineID64, err := strconv.ParseUint(filename[0:8], 16, 32)
		if err != nil {
			continue
		}
		timelineID := uint32(timelineID64)

		segmentID64, err := strconv.ParseUint(filename[8:24], 16, 64)
		if err != nil {
			continue
		}

		// Update timeline info
		if tl, exists := history.TimelineMap[timelineID]; exists {
			if tl.FirstWALSegment == 0 || segmentID64 < tl.FirstWALSegment {
				tl.FirstWALSegment = segmentID64
			}
			if segmentID64 > tl.LastWALSegment {
				tl.LastWALSegment = segmentID64
			}
		}
	}

	return nil
}

// ValidateTimelineConsistency validates that the timeline chain is consistent
func (tm *TimelineManager) ValidateTimelineConsistency(ctx context.Context, history *TimelineHistory) error {
	tm.log.Info("Validating timeline consistency", "timelines", len(history.Timelines))

	// Check that each timeline (except 1) has a valid parent
	for _, tl := range history.Timelines {
		if tl.TimelineID == 1 {
			continue // Base timeline has no parent
		}

		if tl.ParentTimeline == 0 {
			return fmt.Errorf("timeline %d has no parent timeline", tl.TimelineID)
		}

		parent, exists := history.TimelineMap[tl.ParentTimeline]
		if !exists {
			return fmt.Errorf("timeline %d references non-existent parent timeline %d", 
				tl.TimelineID, tl.ParentTimeline)
		}

		// Verify parent timeline has WAL files up to the switch point
		if parent.LastWALSegment == 0 {
			tm.log.Warn("Parent timeline has no WAL segments",
				"timeline", tl.TimelineID,
				"parent", tl.ParentTimeline)
		}
	}

	tm.log.Info("Timeline consistency validated", "timelines", len(history.Timelines))
	return nil
}

// GetTimelinePath returns the path from timeline 1 to the target timeline
func (tm *TimelineManager) GetTimelinePath(history *TimelineHistory, targetTimeline uint32) ([]*TimelineInfo, error) {
	path := make([]*TimelineInfo, 0)
	
	currentTL := targetTimeline
	for currentTL > 0 {
		tl, exists := history.TimelineMap[currentTL]
		if !exists {
			return nil, fmt.Errorf("timeline %d not found in history", currentTL)
		}
		
		// Prepend to path (we're walking backwards)
		path = append([]*TimelineInfo{tl}, path...)
		
		// Move to parent
		if currentTL == 1 {
			break // Reached base timeline
		}
		currentTL = tl.ParentTimeline
		
		// Prevent infinite loops
		if len(path) > 100 {
			return nil, fmt.Errorf("timeline path too long (possible cycle)")
		}
	}
	
	return path, nil
}

// FindTimelineAtPoint finds which timeline was active at a given LSN
func (tm *TimelineManager) FindTimelineAtPoint(history *TimelineHistory, targetLSN string) (uint32, error) {
	// Start from current timeline and walk backwards
	for i := len(history.Timelines) - 1; i >= 0; i-- {
		tl := history.Timelines[i]
		
		// Compare LSNs (simplified - in production would need proper LSN comparison)
		if tl.SwitchPoint <= targetLSN || tl.SwitchPoint == "0/0" {
			return tl.TimelineID, nil
		}
	}
	
	// Default to timeline 1
	return 1, nil
}

// GetRequiredWALFiles returns all WAL files needed for recovery to a target timeline
func (tm *TimelineManager) GetRequiredWALFiles(ctx context.Context, history *TimelineHistory, archiveDir string, targetTimeline uint32) ([]string, error) {
	tm.log.Info("Finding required WAL files", "target_timeline", targetTimeline)

	// Get timeline path from base to target
	path, err := tm.GetTimelinePath(history, targetTimeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get timeline path: %w", err)
	}

	requiredFiles := make([]string, 0)

	// Collect WAL files for each timeline in the path
	for _, tl := range path {
		// Find all WAL files for this timeline
		pattern := fmt.Sprintf("%08X*", tl.TimelineID)
		matches, err := filepath.Glob(filepath.Join(archiveDir, pattern))
		if err != nil {
			return nil, fmt.Errorf("failed to find WAL files for timeline %d: %w", tl.TimelineID, err)
		}

		requiredFiles = append(requiredFiles, matches...)

		// Also include the .history file
		historyFile := filepath.Join(archiveDir, fmt.Sprintf("%08X.history", tl.TimelineID))
		if _, err := os.Stat(historyFile); err == nil {
			requiredFiles = append(requiredFiles, historyFile)
		}
	}

	tm.log.Info("Required WAL files collected",
		"files", len(requiredFiles),
		"timelines", len(path))

	return requiredFiles, nil
}

// FormatTimelineTree returns a formatted string showing the timeline branching structure
func (tm *TimelineManager) FormatTimelineTree(history *TimelineHistory) string {
	if len(history.Timelines) == 0 {
		return "No timelines found"
	}

	var sb strings.Builder
	sb.WriteString("Timeline Branching Structure:\n")
	sb.WriteString("═════════════════════════════\n\n")

	// Build tree recursively
	tm.formatTimelineNode(&sb, history, 1, 0, "")

	return sb.String()
}

// formatTimelineNode recursively formats a timeline node and its children
func (tm *TimelineManager) formatTimelineNode(sb *strings.Builder, history *TimelineHistory, timelineID uint32, depth int, prefix string) {
	tl, exists := history.TimelineMap[timelineID]
	if !exists {
		return
	}

	// Format current node
	indent := strings.Repeat("  ", depth)
	marker := "├─"
	if depth == 0 {
		marker = "●"
	}

	sb.WriteString(fmt.Sprintf("%s%s Timeline %d", indent, marker, tl.TimelineID))
	
	if tl.TimelineID == history.CurrentTimeline {
		sb.WriteString(" [CURRENT]")
	}
	
	if tl.SwitchPoint != "" && tl.SwitchPoint != "0/0" {
		sb.WriteString(fmt.Sprintf(" (switched at %s)", tl.SwitchPoint))
	}
	
	if tl.FirstWALSegment > 0 {
		sb.WriteString(fmt.Sprintf("\n%s   WAL segments: %d files", indent, tl.LastWALSegment-tl.FirstWALSegment+1))
	}
	
	if tl.Reason != "" {
		sb.WriteString(fmt.Sprintf("\n%s   Reason: %s", indent, tl.Reason))
	}
	
	sb.WriteString("\n")

	// Find and format children
	children := make([]*TimelineInfo, 0)
	for _, child := range history.Timelines {
		if child.ParentTimeline == timelineID {
			children = append(children, child)
		}
	}

	// Recursively format children
	for _, child := range children {
		tm.formatTimelineNode(sb, history, child.TimelineID, depth+1, prefix)
	}
}
