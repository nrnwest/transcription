// Package pipeline implements parallel processing of video files.
// It uses goroutines with concurrency limited via a buffered channel (semaphore).
// Each file is processed independently: audio extraction → transcription → .wav cleanup.
package pipeline

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"transcription/internal/diarizer"
	"transcription/internal/extractor"
	"transcription/internal/logger"
	"transcription/internal/scanner"
	"transcription/internal/transcriber"
	"transcription/internal/writer"
)

// defaultMaxWorkers is the default number of goroutines.
// 4 was chosen because ffmpeg and whisper are CPU-intensive,
// and greater parallelism may slow down the overall processing.
const defaultMaxWorkers = 4

// Pipeline coordinates parallel processing of video files.
type Pipeline struct {
	Extractor      extractor.AudioExtractor
	Transcriber    transcriber.Transcriber
	MaxWorkers     int
	ProgressWriter io.Writer
	ErrorWriter    io.Writer
	Logger         *logger.Logger
	// StopOnError, if true, stops processing after the first error.
	StopOnError bool
	// Diarizer, if set, enables speaker diarization: the transcript is split
	// into speaker-labeled paragraphs. Diarization failures degrade to the
	// plain transcript with a warning instead of failing the file.
	Diarizer diarizer.Diarizer
	// SpeakerLabel is the label prefix for speakers ("актор" → "**speaker-1:**").
	SpeakerLabel string
}

// NewPipeline creates a Pipeline with default settings.
// MaxWorkers is set to defaultMaxWorkers (4).
func NewPipeline(ext extractor.AudioExtractor, tr transcriber.Transcriber) *Pipeline {
	return &Pipeline{
		Extractor:   ext,
		Transcriber: tr,
		MaxWorkers:  defaultMaxWorkers,
	}
}

// indexedResult is an internal structure for storing a result together with its index.
// Needed to restore ordering after parallel processing.
type indexedResult struct {
	index  int
	result writer.TranscriptionResult
}

// Run starts parallel processing of all video files.
// It returns results in the same order as the input files slice.
//
// Algorithm:
//  1. The buffered channel sem limits the number of concurrent goroutines (semaphore)
//  2. WaitGroup waits for all goroutines to finish
//  3. The resultsCh channel collects results from the goroutines
//  4. A separate collector goroutine reads from the channel and writes into the slice by index
func (p *Pipeline) Run(files []scanner.VideoFile) []writer.TranscriptionResult {
	if len(files) == 0 {
		return []writer.TranscriptionResult{}
	}

	// Determine the number of workers — if not set, use the default value
	maxWorkers := p.MaxWorkers
	if maxWorkers <= 0 {
		maxWorkers = defaultMaxWorkers
	}

	total := len(files)

	// Semaphore — a buffered channel limits the number of concurrent goroutines.
	// A goroutine "acquires" a slot by writing to the channel and "releases" it by reading.
	sem := make(chan struct{}, maxWorkers)

	// Channel for results — buffered to the number of files,
	// so goroutines don't block when sending
	resultsCh := make(chan indexedResult, total)

	// WaitGroup waits for all processing goroutines to finish
	var wg sync.WaitGroup

	// stopped is an atomic flag for StopOnError.
	// If set, new goroutines don't start processing.
	var stopped atomic.Bool

	// Start a goroutine for each file — the launch order matches the file order.
	// With StopOnError we acquire the semaphore ON THE MAIN goroutine before launching the goroutine —
	// this guarantees sequencing: the next file won't start processing until the previous one
	// releases its slot and we check the stop flag.
	for i, vf := range files {
		// Check the StopOnError flag before launching a new goroutine.
		if p.StopOnError && stopped.Load() {
			break
		}

		// With StopOnError we acquire the semaphore synchronously — this blocks the loop
		// until the previous goroutine finishes processing and releases its slot.
		// Without this, all goroutines launch at once.
		if p.StopOnError {
			sem <- struct{}{}
			// Re-check after waiting — while blocked,
			// another goroutine could have failed with an error
			if stopped.Load() {
				<-sem
				break
			}
		}

		wg.Add(1)
		go func(idx int, video scanner.VideoFile) {
			defer wg.Done()

			// In normal mode (without StopOnError) the semaphore is acquired here
			if !p.StopOnError {
				sem <- struct{}{}
			}
			// Release the slot after processing finishes
			defer func() { <-sem }()

			p.logProgress("[%d/%d] Processing: %s\n", idx+1, total, filepath.Base(video.Path))

			result := p.processFile(video)

			if result.Success {
				p.logProgress("[%d/%d] Done: %s ✓\n", idx+1, total, filepath.Base(video.Path))
			} else {
				p.logError("[%d/%d] Error: %s — %v\n", idx+1, total, filepath.Base(video.Path), result.Error)
				// Set the stop flag — subsequent files won't start processing
				if p.StopOnError {
					stopped.Store(true)
				}
			}

			resultsCh <- indexedResult{index: idx, result: result}
		}(i, vf)
	}

	// Close the results channel after all goroutines finish.
	// Do this in a separate goroutine so as not to block the main goroutine.
	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	// Collect results from the channel, preserving order by index
	results := make([]writer.TranscriptionResult, total)
	for ir := range resultsCh {
		results[ir.index] = ir.result
	}

	return results
}

// processFile processes a single video file: extract → transcribe → cleanup.
// It returns a TranscriptionResult regardless of success or failure.
// The .wav file is removed after transcription (or on a transcription error).
// If Logger is set, it logs START/DONE/ERROR for each step.
func (p *Pipeline) processFile(video scanner.VideoFile) writer.TranscriptionResult {
	filename := filepath.Base(video.Path)

	var ffmpegTimer *logger.StepTimer
	if p.Logger != nil {
		ffmpegTimer = logger.NewStep(p.Logger, "ffmpeg", filename)
	}

	wavPath, err := p.Extractor.Extract(video.Path)

	if err != nil {
		if ffmpegTimer != nil {
			ffmpegTimer.Fail(err)
		}
		return writer.TranscriptionResult{
			Video:   video,
			Success: false,
			Error:   err,
		}
	}

	if ffmpegTimer != nil {
		ffmpegTimer.Done()
	}

	var whisperTimer *logger.StepTimer
	if p.Logger != nil {
		whisperTimer = logger.NewStep(p.Logger, "whisper", filename)
	}

	text, segments, err := p.transcribe(wavPath, filename)

	// Remove temporary files regardless of the transcription result.
	// .wav is the extracted audio, .wav.txt/.wav.json are whisper results.
	// Deferred because the diarizer below still reads the .wav.
	// We ignore the os.Remove error — the file may no longer exist.
	defer func() {
		os.Remove(wavPath)
		os.Remove(wavPath + ".txt")
		os.Remove(wavPath + ".json")
	}()

	if err != nil {
		if whisperTimer != nil {
			whisperTimer.Fail(err)
		}
		return writer.TranscriptionResult{
			Video:   video,
			Success: false,
			Error:   err,
		}
	}

	if whisperTimer != nil {
		whisperTimer.Done()
	}

	if p.Diarizer != nil && len(segments) > 0 {
		if merged := p.diarize(wavPath, segments, filename); merged != "" {
			text = merged
		}
	}

	return writer.TranscriptionResult{
		Video:   video,
		Text:    text,
		Success: true,
	}
}

// transcribe runs the transcriber, requesting timestamped segments when
// diarization is enabled and the transcriber supports them.
func (p *Pipeline) transcribe(wavPath, filename string) (string, []transcriber.Segment, error) {
	if p.Diarizer != nil {
		if st, ok := p.Transcriber.(transcriber.SegmentTranscriber); ok {
			return st.TranscribeSegments(wavPath)
		}
		p.logError("Warning: diarization skipped for %s — transcriber does not provide segments\n", filename)
	}
	text, err := p.Transcriber.Transcribe(wavPath)
	return text, nil, err
}

// diarize runs the diarizer and merges its speaker segments with the whisper
// segments. Any failure returns "" so the caller keeps the plain transcript:
// a lost speaker markup must not fail an otherwise good transcription.
func (p *Pipeline) diarize(wavPath string, segments []transcriber.Segment, filename string) string {
	var diarizeTimer *logger.StepTimer
	if p.Logger != nil {
		diarizeTimer = logger.NewStep(p.Logger, "diarize", filename)
	}

	diarSegs, err := p.Diarizer.Diarize(wavPath)
	if err != nil {
		if diarizeTimer != nil {
			diarizeTimer.Fail(err)
		}
		p.logError("Warning: diarization failed for %s — using plain transcript (%v)\n", filename, err)
		return ""
	}

	if diarizeTimer != nil {
		diarizeTimer.Done()
	}

	if len(diarSegs) == 0 {
		p.logError("Warning: diarization found no speakers for %s — using plain transcript\n", filename)
		return ""
	}

	return diarizer.Merge(segments, diarSegs, p.SpeakerLabel)
}

// logProgress writes a progress message to ProgressWriter.
// If ProgressWriter is not set, the message is ignored.
// Used for informational messages (start/finish of processing).
func (p *Pipeline) logProgress(format string, args ...any) {
	if p.ProgressWriter != nil {
		fmt.Fprintf(p.ProgressWriter, format, args...)
	}
}

// logError writes an error message to ErrorWriter.
// If ErrorWriter is not set, it writes to os.Stderr (backward compatibility).
// Used for messages about processing errors of individual files.
func (p *Pipeline) logError(format string, args ...any) {
	w := p.ErrorWriter
	if w == nil {
		w = os.Stderr
	}
	fmt.Fprintf(w, format, args...)
}
