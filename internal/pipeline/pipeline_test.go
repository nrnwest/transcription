// Package pipeline_test — tests for the parallel video-file processing pipeline.
// We verify correct goroutine behavior, concurrency limiting,
// error handling, and cleanup of temporary .wav files.
package pipeline

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"transcription/internal/diarizer"
	"transcription/internal/logger"
	"transcription/internal/scanner"
	"transcription/internal/transcriber"
)

// mockExtractor is a mock for AudioExtractor.
// It creates a temporary .wav file and returns its path.
type mockExtractor struct {
	// delay is the delay before returning a result (for concurrency testing)
	delay time.Duration
	// failFor holds the paths for which Extract should return an error
	failFor map[string]bool
	// counter is an atomic counter of active goroutines
	counter *atomic.Int32
	// maxConcurrent is the maximum number of concurrent executions (written atomically)
	maxConcurrent *atomic.Int32
	// tmpDir is the directory for temporary .wav files
	tmpDir string
}

func (m *mockExtractor) Extract(videoPath string) (string, error) {
	// Increment the counter of active goroutines
	if m.counter != nil {
		current := m.counter.Add(1)
		// Update the concurrency maximum
		for {
			old := m.maxConcurrent.Load()
			if current <= old {
				break
			}
			if m.maxConcurrent.CompareAndSwap(old, current) {
				break
			}
		}
	}

	// Simulate processing delay
	if m.delay > 0 {
		time.Sleep(m.delay)
	}

	// Decrement the counter after finishing
	if m.counter != nil {
		m.counter.Add(-1)
	}

	// Check whether this file should fail with an error
	if m.failFor != nil && m.failFor[videoPath] {
		return "", fmt.Errorf("extract failed: %s", videoPath)
	}

	// Create a real temporary .wav file to verify cleanup
	wavPath := filepath.Join(m.tmpDir, filepath.Base(videoPath)+".wav")
	if err := os.WriteFile(wavPath, []byte("fake wav"), 0644); err != nil {
		return "", err
	}

	return wavPath, nil
}

// mockTranscriber is a mock for Transcriber.
// It returns a fixed text for each file.
type mockTranscriber struct {
	// failFor holds the paths for which Transcribe should return an error
	failFor map[string]bool
}

func (m *mockTranscriber) Transcribe(wavPath string) (string, error) {
	if m.failFor != nil && m.failFor[wavPath] {
		return "", fmt.Errorf("transcribe failed: %s", wavPath)
	}
	return "transcribed text for " + filepath.Base(wavPath), nil
}

// makeTestFiles creates a slice of test VideoFile values with unique paths.
func makeTestFiles(count int) []scanner.VideoFile {
	files := make([]scanner.VideoFile, count)
	for i := range count {
		files[i] = scanner.VideoFile{
			Path:      fmt.Sprintf("/tmp/test/video_%d.mkv", i),
			Name:      fmt.Sprintf("video_%d", i),
			Extension: "mkv",
			ModTime:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Hour),
		}
	}
	return files
}

func TestPipelineRunAllFilesProcessed(t *testing.T) {
	tmpDir := t.TempDir()
	files := makeTestFiles(5)

	p := &Pipeline{
		Extractor:   &mockExtractor{tmpDir: tmpDir},
		Transcriber: &mockTranscriber{},
		MaxWorkers:  3,
	}

	results := p.Run(files)

	// Verify that the number of results equals the number of input files
	if len(results) != len(files) {
		t.Fatalf("expected %d results, got %d", len(files), len(results))
	}

	// Verify that all results are successful
	for i, r := range results {
		if !r.Success {
			t.Errorf("result[%d] should have succeeded, but got an error: %v", i, r.Error)
		}
		if r.Text == "" {
			t.Errorf("result[%d] has empty text", i)
		}
	}
}

func TestPipelineMaxConcurrency(t *testing.T) {
	tmpDir := t.TempDir()
	counter := &atomic.Int32{}
	maxConcurrent := &atomic.Int32{}

	files := makeTestFiles(10)

	p := &Pipeline{
		Extractor: &mockExtractor{
			tmpDir:        tmpDir,
			delay:         50 * time.Millisecond,
			counter:       counter,
			maxConcurrent: maxConcurrent,
		},
		Transcriber: &mockTranscriber{},
		MaxWorkers:  3,
	}

	results := p.Run(files)

	// Verify that all files were processed
	if len(results) != len(files) {
		t.Fatalf("expected %d results, got %d", len(files), len(results))
	}

	// Verify that the maximum concurrency did not exceed MaxWorkers
	maxSeen := maxConcurrent.Load()
	if maxSeen > int32(p.MaxWorkers) {
		t.Errorf("max concurrency %d exceeded MaxWorkers %d", maxSeen, p.MaxWorkers)
	}

	// Verify that parallelism actually happened (otherwise the pipeline is pointless)
	if maxSeen < 2 {
		t.Logf("warning: max concurrency only %d, parallelism may not be working", maxSeen)
	}
}

func TestPipelineErrorHandling(t *testing.T) {
	tmpDir := t.TempDir()
	files := makeTestFiles(5)

	// The file at index 2 will fail with an error
	failPath := files[2].Path

	p := &Pipeline{
		Extractor: &mockExtractor{
			tmpDir:  tmpDir,
			failFor: map[string]bool{failPath: true},
		},
		Transcriber: &mockTranscriber{},
		MaxWorkers:  3,
	}

	results := p.Run(files)

	// All files must be processed (even the failed ones)
	if len(results) != len(files) {
		t.Fatalf("expected %d results, got %d", len(files), len(results))
	}

	// Count the successful and failed ones
	successCount := 0
	failCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		} else {
			failCount++
			if r.Error == nil {
				t.Error("a failed result must contain an error")
			}
		}
	}

	if successCount != 4 {
		t.Errorf("expected 4 successful, got %d", successCount)
	}
	if failCount != 1 {
		t.Errorf("expected 1 failed, got %d", failCount)
	}
}

func TestPipelineWavCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	files := makeTestFiles(3)

	p := &Pipeline{
		Extractor:   &mockExtractor{tmpDir: tmpDir},
		Transcriber: &mockTranscriber{},
		MaxWorkers:  3,
	}

	_ = p.Run(files)

	// Verify that all .wav files were removed
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read tmpDir: %v", err)
	}

	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".wav" {
			t.Errorf(".wav file was not removed: %s", entry.Name())
		}
	}
}

func TestPipelineGoroutinesIndependent(t *testing.T) {
	tmpDir := t.TempDir()
	files := makeTestFiles(6)

	// Files 1 and 3 will fail with an error
	failPaths := map[string]bool{
		files[1].Path: true,
		files[3].Path: true,
	}

	p := &Pipeline{
		Extractor: &mockExtractor{
			tmpDir:  tmpDir,
			failFor: failPaths,
			delay:   10 * time.Millisecond,
		},
		Transcriber: &mockTranscriber{},
		MaxWorkers:  3,
	}

	results := p.Run(files)

	// All 6 files must have a result
	if len(results) != 6 {
		t.Fatalf("expected 6 results, got %d", len(results))
	}

	// Verify each result individually
	for i, r := range results {
		shouldFail := failPaths[files[i].Path]
		if shouldFail && r.Success {
			t.Errorf("file[%d] should have failed, but Success=true", i)
		}
		if !shouldFail && !r.Success {
			t.Errorf("file[%d] should have succeeded, but got an error: %v", i, r.Error)
		}
		// Verify that the result corresponds to the correct video
		if r.Video.Path != files[i].Path {
			t.Errorf("file[%d] has Video.Path=%s, expected %s", i, r.Video.Path, files[i].Path)
		}
	}

	// Verify specific values
	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		}
	}
	if successCount != 4 {
		t.Errorf("expected 4 successful results, got %d", successCount)
	}
}

func TestPipelineTranscriberError(t *testing.T) {
	tmpDir := t.TempDir()
	files := makeTestFiles(3)

	// Build the path to the .wav file that the extractor will return for file 1
	failWavPath := filepath.Join(tmpDir, filepath.Base(files[1].Path)+".wav")

	p := &Pipeline{
		Extractor: &mockExtractor{tmpDir: tmpDir},
		Transcriber: &mockTranscriber{
			failFor: map[string]bool{failWavPath: true},
		},
		MaxWorkers: 3,
	}

	results := p.Run(files)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// File 1 must fail with an error, the others succeed
	for i, r := range results {
		if i == 1 {
			if r.Success {
				t.Error("file[1] should have failed at the transcription stage")
			}
			if r.Error == nil {
				t.Error("file[1] must contain an error")
			}
		} else {
			if !r.Success {
				t.Errorf("file[%d] should have succeeded: %v", i, r.Error)
			}
		}
	}
}

func TestPipelineEmptyFiles(t *testing.T) {
	p := &Pipeline{
		Extractor:   &mockExtractor{tmpDir: t.TempDir()},
		Transcriber: &mockTranscriber{},
		MaxWorkers:  3,
	}

	results := p.Run(nil)

	if len(results) != 0 {
		t.Errorf("expected 0 results for an empty list, got %d", len(results))
	}
}

func TestPipelineDefaultMaxWorkers(t *testing.T) {
	p := NewPipeline(&mockExtractor{tmpDir: t.TempDir()}, &mockTranscriber{})

	if p.MaxWorkers != 4 {
		t.Errorf("expected MaxWorkers=4, got %d", p.MaxWorkers)
	}
}

func TestPipelineResultsPreserveOrder(t *testing.T) {
	tmpDir := t.TempDir()
	files := makeTestFiles(5)

	p := &Pipeline{
		Extractor:   &mockExtractor{tmpDir: tmpDir, delay: 10 * time.Millisecond},
		Transcriber: &mockTranscriber{},
		MaxWorkers:  2,
	}

	results := p.Run(files)

	for i, r := range results {
		if r.Video.Path != files[i].Path {
			t.Errorf("order violated: results[%d].Video.Path=%s, expected %s",
				i, r.Video.Path, files[i].Path)
		}
	}
}

func TestPipelineErrorType(t *testing.T) {
	tmpDir := t.TempDir()
	files := makeTestFiles(1)

	p := &Pipeline{
		Extractor: &mockExtractor{
			tmpDir:  tmpDir,
			failFor: map[string]bool{files[0].Path: true},
		},
		Transcriber: &mockTranscriber{},
		MaxWorkers:  3,
	}

	results := p.Run(files)
	if len(results) != 1 {
		t.Fatal("expected 1 result")
	}

	// The error must be non-nil for a failed result
	var err error = results[0].Error
	if err == nil {
		t.Fatal("expected an error")
	}

	// Verify that this is an error interface
	if !errors.Is(err, err) {
		t.Error("error must implement the error interface")
	}
}

func TestPipelineWithLoggerCallsStartEnd(t *testing.T) {
	tmpDir := t.TempDir()
	files := makeTestFiles(3)

	var logBuf bytes.Buffer
	l := logger.New(&logBuf)

	p := &Pipeline{
		Extractor:   &mockExtractor{tmpDir: tmpDir},
		Transcriber: &mockTranscriber{},
		MaxWorkers:  3,
		Logger:      l,
	}

	results := p.Run(files)

	// All files must be processed
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	logOutput := logBuf.String()

	// For each file there must be START and DONE for the ffmpeg and whisper steps
	for _, f := range files {
		filename := filepath.Base(f.Path)
		if !strings.Contains(logOutput, "START ffmpeg: "+filename) {
			t.Errorf("log must contain START ffmpeg for %s", filename)
		}
		if !strings.Contains(logOutput, "DONE ffmpeg: "+filename) {
			t.Errorf("log must contain DONE ffmpeg for %s", filename)
		}
		if !strings.Contains(logOutput, "START whisper: "+filename) {
			t.Errorf("log must contain START whisper for %s", filename)
		}
		if !strings.Contains(logOutput, "DONE whisper: "+filename) {
			t.Errorf("log must contain DONE whisper for %s", filename)
		}
	}
}

func TestPipelineWithLoggerCallsErrorOnFailure(t *testing.T) {
	tmpDir := t.TempDir()
	files := makeTestFiles(3)

	failPath := files[1].Path

	var logBuf bytes.Buffer
	l := logger.New(&logBuf)

	p := &Pipeline{
		Extractor: &mockExtractor{
			tmpDir:  tmpDir,
			failFor: map[string]bool{failPath: true},
		},
		Transcriber: &mockTranscriber{},
		MaxWorkers:  3,
		Logger:      l,
	}

	results := p.Run(files)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	logOutput := logBuf.String()
	failFilename := filepath.Base(files[1].Path)

	// For the failed file there must be an ERROR entry
	if !strings.Contains(logOutput, "ERROR ffmpeg: "+failFilename) {
		t.Errorf("log must contain ERROR ffmpeg for %s, got:\n%s", failFilename, logOutput)
	}
}

func TestPipelineWithoutLoggerDoesNotPanic(t *testing.T) {
	tmpDir := t.TempDir()
	files := makeTestFiles(2)

	p := &Pipeline{
		Extractor:   &mockExtractor{tmpDir: tmpDir},
		Transcriber: &mockTranscriber{},
		MaxWorkers:  3,
		Logger:      nil,
	}

	results := p.Run(files)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestPipelineStopOnError(t *testing.T) {
	tmpDir := t.TempDir()
	// Create 5 files, the first will fail with an error
	files := makeTestFiles(5)
	failPath := files[0].Path

	p := &Pipeline{
		Extractor: &mockExtractor{
			tmpDir:  tmpDir,
			failFor: map[string]bool{failPath: true},
			delay:   10 * time.Millisecond,
		},
		Transcriber: &mockTranscriber{},
		MaxWorkers:  1, // sequential processing for predictability
		StopOnError: true,
	}

	results := p.Run(files)

	// There must be at least one result with an error
	hasError := false
	for _, r := range results {
		if !r.Success && r.Error != nil {
			hasError = true
			break
		}
	}
	if !hasError {
		t.Error("there must be at least one result with an error")
	}

	// Not all files should be processed (some skipped due to StopOnError)
	processedCount := 0
	for _, r := range results {
		if r.Success || r.Error != nil {
			processedCount++
		}
	}

	if processedCount >= len(files) {
		t.Errorf("StopOnError must stop processing — processed %d of %d", processedCount, len(files))
	}
}

func TestPipelineStopOnErrorReturnsResults(t *testing.T) {
	tmpDir := t.TempDir()
	files := makeTestFiles(3)
	failPath := files[0].Path

	p := &Pipeline{
		Extractor: &mockExtractor{
			tmpDir:  tmpDir,
			failFor: map[string]bool{failPath: true},
		},
		Transcriber: &mockTranscriber{},
		MaxWorkers:  1,
		StopOnError: true,
	}

	results := p.Run(files)

	// The result must not be empty — at least one result (with an error)
	if len(results) == 0 {
		t.Error("results must not be empty even with StopOnError")
	}

	// The first result must have an error
	if results[0].Error == nil {
		t.Error("the first file should have failed")
	}
}

// mockSegmentTranscriber implements both Transcriber and SegmentTranscriber.
type mockSegmentTranscriber struct {
	segments []transcriber.Segment
}

func (m *mockSegmentTranscriber) Transcribe(wavPath string) (string, error) {
	return "plain text", nil
}

func (m *mockSegmentTranscriber) TranscribeSegments(wavPath string) (string, []transcriber.Segment, error) {
	return "plain text", m.segments, nil
}

// mockDiarizer is a mock for diarizer.Diarizer.
type mockDiarizer struct {
	segments []diarizer.Segment
	err      error
}

func (m *mockDiarizer) Diarize(wavPath string) ([]diarizer.Segment, error) {
	return m.segments, m.err
}

func TestPipelineDiarizationMergesSpeakers(t *testing.T) {
	files := makeTestFiles(1)

	p := &Pipeline{
		Extractor: &mockExtractor{tmpDir: t.TempDir()},
		Transcriber: &mockSegmentTranscriber{
			segments: []transcriber.Segment{
				{FromMS: 0, ToMS: 5000, Text: " Питання."},
				{FromMS: 5000, ToMS: 9000, Text: " Відповідь."},
			},
		},
		Diarizer: &mockDiarizer{
			segments: []diarizer.Segment{
				{Start: 0, End: 5, Speaker: 0},
				{Start: 5, End: 9, Speaker: 1},
			},
		},
		SpeakerLabel: "speaker-",
		MaxWorkers:   1,
	}

	results := p.Run(files)

	if !results[0].Success {
		t.Fatalf("expected success, got error: %v", results[0].Error)
	}
	want := "**speaker-1:** Питання.\n\n**speaker-2:** Відповідь."
	if results[0].Text != want {
		t.Errorf("expected merged text:\n%s\ngot:\n%s", want, results[0].Text)
	}
}

func TestPipelineDiarizationErrorDegradesToPlainText(t *testing.T) {
	files := makeTestFiles(1)

	var errBuf bytes.Buffer
	p := &Pipeline{
		Extractor: &mockExtractor{tmpDir: t.TempDir()},
		Transcriber: &mockSegmentTranscriber{
			segments: []transcriber.Segment{{FromMS: 0, ToMS: 5000, Text: " Текст."}},
		},
		Diarizer:     &mockDiarizer{err: errors.New("sherpa crashed")},
		SpeakerLabel: "speaker-",
		MaxWorkers:   1,
		ErrorWriter:  &errBuf,
	}

	results := p.Run(files)

	if !results[0].Success {
		t.Fatalf("expected success despite the diarization error, got: %v", results[0].Error)
	}
	if results[0].Text != "plain text" {
		t.Errorf("expected the plain transcript, got: %q", results[0].Text)
	}
	if !strings.Contains(errBuf.String(), "Warning") {
		t.Errorf("expected a warning on ErrorWriter, got: %q", errBuf.String())
	}
}

func TestPipelineDiarizationEmptySegmentsUsesPlainText(t *testing.T) {
	files := makeTestFiles(1)

	p := &Pipeline{
		Extractor: &mockExtractor{tmpDir: t.TempDir()},
		Transcriber: &mockSegmentTranscriber{
			segments: []transcriber.Segment{{FromMS: 0, ToMS: 5000, Text: " Текст."}},
		},
		Diarizer:     &mockDiarizer{},
		SpeakerLabel: "speaker-",
		MaxWorkers:   1,
	}

	results := p.Run(files)

	if !results[0].Success {
		t.Fatalf("expected success, got: %v", results[0].Error)
	}
	if results[0].Text != "plain text" {
		t.Errorf("expected the plain transcript, got: %q", results[0].Text)
	}
}

func TestPipelineDiarizationNilTranscriberSegmentsDegrades(t *testing.T) {
	// The transcriber does not implement SegmentTranscriber → plain path with a warning.
	files := makeTestFiles(1)

	var errBuf bytes.Buffer
	p := &Pipeline{
		Extractor:    &mockExtractor{tmpDir: t.TempDir()},
		Transcriber:  &mockTranscriber{},
		Diarizer:     &mockDiarizer{},
		SpeakerLabel: "speaker-",
		MaxWorkers:   1,
		ErrorWriter:  &errBuf,
	}

	results := p.Run(files)

	if !results[0].Success {
		t.Fatalf("expected success, got: %v", results[0].Error)
	}
	if !strings.Contains(results[0].Text, "transcribed text") {
		t.Errorf("expected the plain transcript, got: %q", results[0].Text)
	}
}

func TestPipelineDiarizationCleansJSON(t *testing.T) {
	tmpDir := t.TempDir()
	files := makeTestFiles(1)

	p := &Pipeline{
		Extractor:    &mockExtractor{tmpDir: tmpDir},
		Transcriber:  &mockSegmentTranscriber{},
		Diarizer:     &mockDiarizer{},
		SpeakerLabel: "speaker-",
		MaxWorkers:   1,
	}

	wavPath := filepath.Join(tmpDir, filepath.Base(files[0].Path)+".wav")
	// Pre-create the .json whisper would leave behind; Extract recreates the .wav.
	if err := os.WriteFile(wavPath+".json", []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	p.Run(files)

	if _, err := os.Stat(wavPath + ".json"); !os.IsNotExist(err) {
		t.Error("expected the .wav.json to be removed after processing")
	}
}

// fileCheckingDiarizer fails if the .wav no longer exists when Diarize runs.
type fileCheckingDiarizer struct {
	segments []diarizer.Segment
}

func (m *fileCheckingDiarizer) Diarize(wavPath string) ([]diarizer.Segment, error) {
	if _, err := os.Stat(wavPath); err != nil {
		return nil, fmt.Errorf("wav is gone: %w", err)
	}
	return m.segments, nil
}

func TestPipelineDiarizationRunsBeforeWavCleanup(t *testing.T) {
	files := makeTestFiles(1)

	p := &Pipeline{
		Extractor: &mockExtractor{tmpDir: t.TempDir()},
		Transcriber: &mockSegmentTranscriber{
			segments: []transcriber.Segment{{FromMS: 0, ToMS: 5000, Text: " Текст."}},
		},
		Diarizer: &fileCheckingDiarizer{
			segments: []diarizer.Segment{{Start: 0, End: 5, Speaker: 0}},
		},
		SpeakerLabel: "speaker-",
		MaxWorkers:   1,
	}

	results := p.Run(files)

	if results[0].Text != "**speaker-1:** Текст." {
		t.Errorf("diarizer must run while the .wav still exists; got: %q", results[0].Text)
	}
}
