// Package logger_test — tests for the structured logger.
// We check the message format, concurrent access,
// StepTimer, the file logger and rotation.
package logger

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLoggerStartFormat(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf)

	l.Start("ffmpeg", "video.mkv")

	line := buf.String()
	if !strings.Contains(line, "START") {
		t.Errorf("expected type START, got: %s", line)
	}
	if !strings.Contains(line, "ffmpeg") {
		t.Errorf("expected step 'ffmpeg', got: %s", line)
	}
	if !strings.Contains(line, "video.mkv") {
		t.Errorf("expected filename 'video.mkv', got: %s", line)
	}
	if !strings.HasPrefix(line, "[") {
		t.Errorf("line should start with '[', got: %s", line)
	}
	if !strings.Contains(line, "]") {
		t.Errorf("line should contain ']' (end of timestamp), got: %s", line)
	}
}

func TestLoggerEndFormat(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf)

	l.End("ffmpeg", "video.mkv", 2*time.Second+500*time.Millisecond)

	line := buf.String()
	if !strings.Contains(line, "DONE") {
		t.Errorf("expected type DONE, got: %s", line)
	}
	if !strings.Contains(line, "ffmpeg") {
		t.Errorf("expected step 'ffmpeg', got: %s", line)
	}
	if !strings.Contains(line, "video.mkv") {
		t.Errorf("expected filename 'video.mkv', got: %s", line)
	}
	if !strings.Contains(line, "2.50s") {
		t.Errorf("expected duration '2.50s', got: %s", line)
	}
}

func TestLoggerErrorFormat(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf)

	l.Error("whisper", "video.mkv", errors.New("модель не знайдено"))

	line := buf.String()
	if !strings.Contains(line, "ERROR") {
		t.Errorf("expected type ERROR, got: %s", line)
	}
	if !strings.Contains(line, "whisper") {
		t.Errorf("expected step 'whisper', got: %s", line)
	}
	if !strings.Contains(line, "video.mkv") {
		t.Errorf("expected filename 'video.mkv', got: %s", line)
	}
	if !strings.Contains(line, "модель не знайдено") {
		t.Errorf("expected error text, got: %s", line)
	}
}

func TestLoggerInfoFormat(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf)

	l.Info("програма запущена")

	line := buf.String()
	if !strings.Contains(line, "INFO") {
		t.Errorf("expected type INFO, got: %s", line)
	}
	if !strings.Contains(line, "програма запущена") {
		t.Errorf("expected a message, got: %s", line)
	}
}

func TestLoggerConcurrentWrites(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf)

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			l.Info(fmt.Sprintf("горутина %d", idx))
		}(i)
	}

	wg.Wait()

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != goroutines {
		t.Errorf("expected %d lines, got %d", goroutines, len(lines))
	}
	for i, line := range lines {
		if !strings.Contains(line, "INFO") {
			t.Errorf("line %d does not contain INFO: %s", i, line)
		}
		if !strings.HasPrefix(line, "[") {
			t.Errorf("line %d should start with '[': %s", i, line)
		}
	}
}

func TestStepTimerDone(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf)

	timer := NewStep(l, "ffmpeg", "video.mkv")
	time.Sleep(10 * time.Millisecond)
	timer.Done()

	line := buf.String()
	if !strings.Contains(line, "DONE") {
		t.Errorf("Done() should call End with type DONE, got: %s", line)
	}
	if !strings.Contains(line, "ffmpeg") {
		t.Errorf("expected step 'ffmpeg', got: %s", line)
	}
	if !strings.Contains(line, "video.mkv") {
		t.Errorf("expected filename 'video.mkv', got: %s", line)
	}
}

func TestStepTimerFail(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf)

	timer := NewStep(l, "whisper", "video.mkv")
	timer.Fail(errors.New("збій транскрибації"))

	line := buf.String()
	if !strings.Contains(line, "ERROR") {
		t.Errorf("Fail() should call Error with type ERROR, got: %s", line)
	}
	if !strings.Contains(line, "збій транскрибації") {
		t.Errorf("expected error text, got: %s", line)
	}
}

func TestNewFileLoggerCreatesFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	l, err := NewFileLogger(logPath)
	if err != nil {
		t.Fatalf("failed to create the file logger: %v", err)
	}

	l.Info("тестове повідомлення")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read the log file: %v", err)
	}
	if !strings.Contains(string(data), "тестове повідомлення") {
		t.Errorf("file should contain the message, got: %s", string(data))
	}
}

func TestNewFileLoggerCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "subdir", "nested", "test.log")

	l, err := NewFileLogger(logPath)
	if err != nil {
		t.Fatalf("failed to create the file logger with a nested directory: %v", err)
	}

	l.Info("test")

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("the log file should exist after creation")
	}
}

func TestNewFileLoggerRotation(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	bigData := make([]byte, 11*1024*1024) // 11MB
	if err := os.WriteFile(logPath, bigData, 0644); err != nil {
		t.Fatalf("failed to create the large file: %v", err)
	}

	l, err := NewFileLogger(logPath)
	if err != nil {
		t.Fatalf("failed to create the logger with rotation: %v", err)
	}

	l.Info("нове повідомлення")

	oldPath := logPath + ".old"
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		t.Error("the .old file should exist after rotation")
	}

	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("failed to get info about the new file: %v", err)
	}
	if info.Size() > 1024*1024 { // less than 1MB
		t.Errorf("the new file should be small after rotation, size: %d", info.Size())
	}
}

func TestLoggerMultiWriter(t *testing.T) {
	var fileBuf, stdoutBuf bytes.Buffer

	l := New(multiWriter(&fileBuf, &stdoutBuf))

	l.Info("dual output test")

	fileContent := fileBuf.String()
	stdoutContent := stdoutBuf.String()

	if !strings.Contains(fileContent, "dual output test") {
		t.Errorf("file should contain the message, got: %s", fileContent)
	}
	if !strings.Contains(stdoutContent, "dual output test") {
		t.Errorf("stdout should contain the message, got: %s", stdoutContent)
	}
}

func TestLoggerFileOnly(t *testing.T) {
	var fileBuf bytes.Buffer

	l := New(&fileBuf)
	l.Info("file only test")

	if !strings.Contains(fileBuf.String(), "file only test") {
		t.Errorf("file should contain the message, got: %s", fileBuf.String())
	}
}
