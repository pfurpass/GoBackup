package utils

import (
	"fmt"
	"log"
	"os"
	"time"
)

// LogLevel defines severity
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

// LogEntry is a single log line
type LogEntry struct {
	Time    time.Time
	Level   LogLevel
	Message string
}

// Logger writes to a file and optionally to a channel for UI display
type Logger struct {
	fileLogger *log.Logger
	ch         chan LogEntry
	level      LogLevel
}

// NewLogger creates a Logger. logPath is the file path; ch receives UI log entries (may be nil).
func NewLogger(logPath string, ch chan LogEntry) (*Logger, error) {
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("log-datei öffnen: %w", err)
	}
	return &Logger{
		fileLogger: log.New(f, "", log.LstdFlags),
		ch:         ch,
		level:      LevelInfo,
	}, nil
}

// NewConsoleLogger creates a Logger that only writes to stderr
func NewConsoleLogger() *Logger {
	return &Logger{
		fileLogger: log.New(os.Stderr, "", log.LstdFlags),
		level:      LevelDebug,
	}
}

func (l *Logger) log(lvl LogLevel, msg string) {
	if lvl < l.level {
		return
	}
	prefix := [...]string{"DEBUG", "INFO ", "WARN ", "ERROR"}[lvl]
	line := fmt.Sprintf("[%s] %s", prefix, msg)
	l.fileLogger.Println(line)

	if l.ch != nil {
		entry := LogEntry{Time: time.Now(), Level: lvl, Message: line}
		select {
		case l.ch <- entry:
		default: // drop if buffer full
		}
	}
}

func (l *Logger) Debugf(format string, args ...any) { l.log(LevelDebug, fmt.Sprintf(format, args...)) }
func (l *Logger) Infof(format string, args ...any)  { l.log(LevelInfo, fmt.Sprintf(format, args...)) }
func (l *Logger) Warnf(format string, args ...any)  { l.log(LevelWarn, fmt.Sprintf(format, args...)) }
func (l *Logger) Errorf(format string, args ...any) { l.log(LevelError, fmt.Sprintf(format, args...)) }
