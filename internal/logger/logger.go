// Package logger implements a structured logger for transcription.
// All methods are protected by a mutex for safe concurrent access.
package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// maxLogFileSize — maximum log file size before rotation (10 MB).
// When exceeded, the file is renamed to .old and a new one is created.
const maxLogFileSize = 10 * 1024 * 1024

// Logger — a structured logger with a mutex for concurrent access.
type Logger struct {
	writer io.Writer
	// mu protects writer from concurrent writes.
	// Without it, goroutines could interleave bytes of different messages.
	mu sync.Mutex
}

// New creates a Logger with the given io.Writer.
// The writer can be a file, a buffer, a MultiWriter — anything.
func New(w io.Writer) *Logger {
	return &Logger{writer: w}
}

// timestamp returns the current time in the format [2006-01-02 15:04:05].
// Go uses a "reference time" — the specific date Mon Jan 2 15:04:05 MST 2006
// as the formatting template (instead of strftime-like specifiers).
func timestamp() string {
	return time.Now().Format("[2006-01-02 15:04:05]")
}

// Start logs the beginning of a processing step.
func (l *Logger) Start(step, filename string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.writer, "%s START %s: %s\n", timestamp(), step, filename)
}

// End logs the completion of a step with its duration.
func (l *Logger) End(step, filename string, d time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.writer, "%s DONE %s: %s (%.2fs)\n", timestamp(), step, filename, d.Seconds())
}

// Error logs a step error.
func (l *Logger) Error(step, filename string, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.writer, "%s ERROR %s: %s — %v\n", timestamp(), step, filename, err)
}

// Info logs an informational message.
func (l *Logger) Info(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.writer, "%s INFO %s\n", timestamp(), msg)
}

// StepTimer tracks the execution time of an individual step.
// Created via NewStep, finished via Done() or Fail().
// This avoids manually computing time.Since in every place.
type StepTimer struct {
	logger *Logger
	step   string
	file   string
	start  time.Time
}

// NewStep creates a StepTimer and records the start time.
// Automatically logs START — no need to call logger.Start() separately.
func NewStep(l *Logger, step, file string) *StepTimer {
	l.Start(step, file)
	return &StepTimer{
		logger: l,
		step:   step,
		file:   file,
		start:  time.Now(),
	}
}

// Done finishes the step successfully — logs DONE with the computed duration.
func (st *StepTimer) Done() {
	st.logger.End(st.step, st.file, time.Since(st.start))
}

// Fail finishes the step with an error — logs ERROR with the error text.
func (st *StepTimer) Fail(err error) {
	st.logger.Error(st.step, st.file, err)
}

// NewFileLogger creates a Logger that writes to a file at the given path.
// Creates the directory if it does not exist (os.MkdirAll).
// If the file exceeds maxLogFileSize (10MB), rotation happens: rename to .old.
func NewFileLogger(logPath string) (*Logger, error) {
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Check the size of the existing file for rotation.
	// If the file does not exist, os.Stat returns an error and we simply skip rotation.
	if info, err := os.Stat(logPath); err == nil {
		if info.Size() > maxLogFileSize {
			// Rotation: rename the current file to .old.
			// os.Rename atomically replaces .old if it already exists.
			oldPath := logPath + ".old"
			if err := os.Rename(logPath, oldPath); err != nil {
				return nil, fmt.Errorf("failed to rotate log file: %w", err)
			}
		}
	}

	// Open the file in append mode — new records are added at the end.
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return New(file), nil
}

// multiWriter — a wrapper over io.MultiWriter for easier testing.
// Writes simultaneously to several Writers (for example, file + stdout in dev mode).
func multiWriter(writers ...io.Writer) io.Writer {
	return io.MultiWriter(writers...)
}

// NewLoggerFromConfig creates a Logger based on the configuration.
// dev mode: writes both to the file and to stdout (MultiWriter).
// prod mode: writes only to the file.
// A file-open error is returned to the caller to handle according to the mode.
func NewLoggerFromConfig(mode, logPath string, stdout io.Writer) (*Logger, error) {
	// Create the file logger — it will create the directory and handle rotation
	fileLogger, err := NewFileLogger(logPath)
	if err != nil {
		return nil, err
	}

	if mode == "dev" {
		// dev mode: write both to the file and to stdout for development convenience.
		// io.MultiWriter duplicates each write to all provided Writers.
		mw := io.MultiWriter(fileLogger.writer, stdout)
		return New(mw), nil
	}

	// prod mode: file only — no clutter in stdout
	return fileLogger, nil
}
