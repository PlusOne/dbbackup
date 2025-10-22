package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
)

// Logger defines the interface for logging
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Time(msg string, args ...any)
	
	// Progress logging for operations
	StartOperation(name string) OperationLogger
}

// OperationLogger tracks timing for operations
type OperationLogger interface {
	Update(msg string, args ...any)
	Complete(msg string, args ...any)
	Fail(msg string, args ...any)
}

// logger implements Logger interface using slog
type logger struct {
	slog   *slog.Logger
	level  slog.Level
	format string
}

// operationLogger tracks a single operation
type operationLogger struct {
	name      string
	startTime time.Time
	parent    *logger
}

// New creates a new logger
func New(level, format string) Logger {
	var slogLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "info":
		slogLevel = slog.LevelInfo
	case "warn", "warning":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: slogLevel,
	}

	switch strings.ToLower(format) {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	default:
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return &logger{
		slog:   slog.New(handler),
		level:  slogLevel,
		format: format,
	}
}

func (l *logger) Debug(msg string, args ...any) {
	l.slog.Debug(msg, args...)
}

func (l *logger) Info(msg string, args ...any) {
	l.slog.Info(msg, args...)
}

func (l *logger) Warn(msg string, args ...any) {
	l.slog.Warn(msg, args...)
}

func (l *logger) Error(msg string, args ...any) {
	l.slog.Error(msg, args...)
}

func (l *logger) Time(msg string, args ...any) {
	// Time logs are always at info level with special formatting
	l.slog.Info("[TIME] "+msg, args...)
}

func (l *logger) StartOperation(name string) OperationLogger {
	return &operationLogger{
		name:      name,
		startTime: time.Now(),
		parent:    l,
	}
}

func (ol *operationLogger) Update(msg string, args ...any) {
	elapsed := time.Since(ol.startTime)
	ol.parent.Info(fmt.Sprintf("[%s] %s", ol.name, msg), 
		append(args, "elapsed", elapsed.String())...)
}

func (ol *operationLogger) Complete(msg string, args ...any) {
	elapsed := time.Since(ol.startTime)
	ol.parent.Info(fmt.Sprintf("[%s] COMPLETED: %s", ol.name, msg), 
		append(args, "duration", formatDuration(elapsed))...)
}

func (ol *operationLogger) Fail(msg string, args ...any) {
	elapsed := time.Since(ol.startTime)
	ol.parent.Error(fmt.Sprintf("[%s] FAILED: %s", ol.name, msg), 
		append(args, "duration", formatDuration(elapsed))...)
}

// formatDuration formats duration in human-readable format
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	} else if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	} else {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		seconds := int(d.Seconds()) % 60
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
}

// FileLogger creates a logger that writes to both stdout and a file
func FileLogger(level, format, filename string) (Logger, error) {
	var slogLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "info":
		slogLevel = slog.LevelInfo
	case "warn", "warning":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	// Open log file
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Create multi-writer (stdout + file)
	multiWriter := io.MultiWriter(os.Stdout, file)

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: slogLevel,
	}

	switch strings.ToLower(format) {
	case "json":
		handler = slog.NewJSONHandler(multiWriter, opts)
	default:
		handler = slog.NewTextHandler(multiWriter, opts)
	}

	return &logger{
		slog:   slog.New(handler),
		level:  slogLevel,
		format: format,
	}, nil
}