package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
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

// logger implements Logger interface using logrus
type logger struct {
	logrus *logrus.Logger
	level  logrus.Level
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
	var logLevel logrus.Level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = logrus.DebugLevel
	case "info":
		logLevel = logrus.InfoLevel
	case "warn", "warning":
		logLevel = logrus.WarnLevel
	case "error":
		logLevel = logrus.ErrorLevel
	default:
		logLevel = logrus.InfoLevel
	}

	l := logrus.New()
	l.SetLevel(logLevel)
	l.SetOutput(os.Stdout)

	switch strings.ToLower(format) {
	case "json":
		l.SetFormatter(&logrus.JSONFormatter{})
	default:
		// Use custom clean formatter for human-readable output
		l.SetFormatter(&CleanFormatter{})
	}

	return &logger{
		logrus: l,
		level:  logLevel,
		format: format,
	}
}

func (l *logger) Debug(msg string, args ...any) {
	l.logWithFields(logrus.DebugLevel, msg, args...)
}

func (l *logger) Info(msg string, args ...any) {
	l.logWithFields(logrus.InfoLevel, msg, args...)
}

func (l *logger) Warn(msg string, args ...any) {
	l.logWithFields(logrus.WarnLevel, msg, args...)
}

func (l *logger) Error(msg string, args ...any) {
	l.logWithFields(logrus.ErrorLevel, msg, args...)
}

func (l *logger) Time(msg string, args ...any) {
	// Time logs are always at info level with special formatting
	l.logWithFields(logrus.InfoLevel, "[TIME] "+msg, args...)
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

// logWithFields forwards log messages with structured fields to logrus
func (l *logger) logWithFields(level logrus.Level, msg string, args ...any) {
	if l == nil || l.logrus == nil {
		return
	}

	fields := fieldsFromArgs(args...)
	entry := l.logrus.WithFields(fields)

	switch level {
	case logrus.DebugLevel:
		entry.Debug(msg)
	case logrus.WarnLevel:
		entry.Warn(msg)
	case logrus.ErrorLevel:
		entry.Error(msg)
	default:
		entry.Info(msg)
	}
}

// fieldsFromArgs converts variadic key/value pairs into logrus fields
func fieldsFromArgs(args ...any) logrus.Fields {
	fields := logrus.Fields{}

	for i := 0; i < len(args); {
		if i+1 < len(args) {
			if key, ok := args[i].(string); ok {
				fields[key] = args[i+1]
				i += 2
				continue
			}
		}

		fields[fmt.Sprintf("arg%d", i)] = args[i]
		i++
	}

	return fields
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

// CleanFormatter formats log entries in a clean, human-readable format
type CleanFormatter struct{}

// Format implements logrus.Formatter interface
func (f *CleanFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	timestamp := entry.Time.Format("2006-01-02T15:04:05")
	
	// Color codes for different log levels
	var levelColor, levelText string
	switch entry.Level {
	case logrus.DebugLevel:
		levelColor = "\033[36m" // Cyan
		levelText = "DEBUG"
	case logrus.InfoLevel:
		levelColor = "\033[32m" // Green
		levelText = "INFO "
	case logrus.WarnLevel:
		levelColor = "\033[33m" // Yellow
		levelText = "WARN "
	case logrus.ErrorLevel:
		levelColor = "\033[31m" // Red
		levelText = "ERROR"
	default:
		levelColor = "\033[0m" // Reset
		levelText = "INFO "
	}
	resetColor := "\033[0m"
	
	// Build the message with perfectly aligned columns
	var output strings.Builder
	
	// Column 1: Level (with color, fixed width 5 chars)
	output.WriteString(levelColor)
	output.WriteString(levelText)
	output.WriteString(resetColor)
	output.WriteString(" ")
	
	// Column 2: Timestamp (fixed format)
	output.WriteString("[")
	output.WriteString(timestamp)
	output.WriteString("] ")
	
	// Column 3: Message
	output.WriteString(entry.Message)
	
	// Append important fields in a clean format (skip internal fields)
	if len(entry.Data) > 0 {
		for k, v := range entry.Data {
			// Skip noisy internal fields
			if k == "elapsed" || k == "operation_id" || k == "step" || k == "timestamp" {
				continue
			}
			
			// Format duration nicely
			if k == "duration" {
				if str, ok := v.(string); ok {
					output.WriteString(fmt.Sprintf(" [duration: %s]", str))
				}
				continue
			}
			
			// Add other fields
			output.WriteString(fmt.Sprintf(" %s=%v", k, v))
		}
	}
	
	output.WriteString("\n")
	return []byte(output.String()), nil
}

// FileLogger creates a logger that writes to both stdout and a file
func FileLogger(level, format, filename string) (Logger, error) {
	var logLevel logrus.Level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = logrus.DebugLevel
	case "info":
		logLevel = logrus.InfoLevel
	case "warn", "warning":
		logLevel = logrus.WarnLevel
	case "error":
		logLevel = logrus.ErrorLevel
	default:
		logLevel = logrus.InfoLevel
	}

	// Open log file
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Create multi-writer (stdout + file)
	multiWriter := io.MultiWriter(os.Stdout, file)

	l := logrus.New()
	l.SetLevel(logLevel)
	l.SetOutput(multiWriter)

	switch strings.ToLower(format) {
	case "json":
		l.SetFormatter(&logrus.JSONFormatter{})
	default:
		// Use custom clean formatter for human-readable output
		l.SetFormatter(&CleanFormatter{})
	}

	return &logger{
		logrus: l,
		level:  logLevel,
		format: format,
	}, nil
}
