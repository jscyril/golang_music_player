package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// Level represents log severity
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
	FATAL
)

var levelNames = [...]string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"}

func (l Level) String() string {
	if int(l) < len(levelNames) {
		return levelNames[l]
	}
	return "UNKNOWN"
}

const (
	maxLogSize    = 5 * 1024 * 1024 // 5 MB
	maxLogBackups = 1
)

// Logger is a file-based logger with rotation support.
type Logger struct {
	mu       sync.Mutex
	file     *os.File
	writer   io.Writer
	path     string
	level    Level
	isClosed bool
}

var (
	globalLogger *Logger
	globalMu     sync.RWMutex
)

// Init initializes the global logger. Must be called before any Log calls.
// logDir is the directory where log files will be written (e.g. ~/.config/gtmpc/logs/).
// level sets the minimum log level to write.
func Init(logDir string, level Level) error {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	logPath := filepath.Join(logDir, "gtmpc.log")

	l, err := newLogger(logPath, level)
	if err != nil {
		return err
	}

	globalMu.Lock()
	globalLogger = l
	globalMu.Unlock()

	l.Info("Logger initialized (log_dir=%s, level=%s)", logDir, level)
	return nil
}

// Close closes the global logger.
func Close() {
	globalMu.Lock()
	defer globalMu.Unlock()
	if globalLogger != nil {
		globalLogger.close()
		globalLogger = nil
	}
}

// GetLogPath returns the path of the current log file, or empty string if not initialized.
func GetLogPath() string {
	globalMu.RLock()
	defer globalMu.RUnlock()
	if globalLogger != nil {
		return globalLogger.path
	}
	return ""
}

func newLogger(path string, level Level) (*Logger, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}

	return &Logger{
		file:   file,
		writer: file,
		path:   path,
		level:  level,
	}, nil
}

func (l *Logger) close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil && !l.isClosed {
		l.file.Close()
		l.isClosed = true
	}
}

func (l *Logger) log(level Level, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.isClosed {
		return
	}

	// Rotate if needed
	if info, err := l.file.Stat(); err == nil && info.Size() > maxLogSize {
		l.rotate()
	}

	ts := time.Now().Format("2006-01-02 15:04:05.000")
	msg := fmt.Sprintf(format, args...)

	// Get caller info (skip 3 frames: log -> Debug/Info/etc -> caller)
	_, file, line, ok := runtime.Caller(2)
	caller := "???"
	if ok {
		caller = fmt.Sprintf("%s:%d", filepath.Base(file), line)
	}

	entry := fmt.Sprintf("[%s] %-5s %s | %s\n", ts, level, caller, msg)
	l.writer.Write([]byte(entry))
}

func (l *Logger) rotate() {
	l.file.Close()

	// Remove oldest backup
	for i := maxLogBackups; i > 0; i-- {
		old := fmt.Sprintf("%s.%d", l.path, i)
		if i == maxLogBackups {
			os.Remove(old)
		}
		if i > 1 {
			prev := fmt.Sprintf("%s.%d", l.path, i-1)
			os.Rename(prev, old)
		}
	}

	// Move current to .1
	os.Rename(l.path, l.path+".1")

	// Open new file
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// Can't do much here — write to stderr as a last resort
		fmt.Fprintf(os.Stderr, "logger: failed to rotate: %v\n", err)
		return
	}
	l.file = file
	l.writer = file
}

// --- Global convenience functions ---

// Debug logs at DEBUG level.
func Debug(format string, args ...interface{}) {
	globalMu.RLock()
	l := globalLogger
	globalMu.RUnlock()
	if l != nil {
		l.log(DEBUG, format, args...)
	}
}

// Info logs at INFO level.
func Info(format string, args ...interface{}) {
	globalMu.RLock()
	l := globalLogger
	globalMu.RUnlock()
	if l != nil {
		l.log(INFO, format, args...)
	}
}

// Warn logs at WARN level.
func Warn(format string, args ...interface{}) {
	globalMu.RLock()
	l := globalLogger
	globalMu.RUnlock()
	if l != nil {
		l.log(WARN, format, args...)
	}
}

// Error logs at ERROR level.
func Error(format string, args ...interface{}) {
	globalMu.RLock()
	l := globalLogger
	globalMu.RUnlock()
	if l != nil {
		l.log(ERROR, format, args...)
	}
}

// Fatal logs at FATAL level. Does NOT call os.Exit — the caller decides what to do.
func Fatal(format string, args ...interface{}) {
	globalMu.RLock()
	l := globalLogger
	globalMu.RUnlock()
	if l != nil {
		l.log(FATAL, format, args...)
	}
}

// Info logs at INFO level (method on Logger).
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

// WritePanic writes a recovered panic value and stack trace to the log file.
// This is intended to be called from a deferred recovery handler.
func WritePanic(r interface{}) {
	globalMu.RLock()
	l := globalLogger
	globalMu.RUnlock()

	if l == nil {
		// Logger not initialized — dump to stderr
		fmt.Fprintf(os.Stderr, "PANIC: %v\n", r)
		return
	}

	// Capture stack trace
	buf := make([]byte, 8192)
	n := runtime.Stack(buf, false)
	stack := string(buf[:n])

	l.log(FATAL, "PANIC: %v\n%s", r, stack)

	// Also try to write to stderr in case terminal is still readable
	fmt.Fprintf(os.Stderr, "PANIC (see log at %s): %v\n", l.path, r)
}
