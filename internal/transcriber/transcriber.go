// Package transcriber is responsible for transcribing audio files into text.
// It defines the Transcriber interface to abstract away the whisper binary,
// which allows testing the pipeline without real transcription.
package transcriber

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// CommandRunner is a function for running external commands (whisper).
// It allows substituting the real exec with a mock in tests.
type CommandRunner func(name string, args ...string) ([]byte, error)

// Transcriber is the interface for transcribing an audio file into text.
// It takes the path to a .wav file and returns the transcribed text or an error.
type Transcriber interface {
	Transcribe(wavPath string) (text string, err error)
}

// Segment is one timestamped piece of the transcription (from whisper's JSON output).
type Segment struct {
	FromMS int64
	ToMS   int64
	Text   string
}

// SegmentTranscriber is an optional capability: transcribing with per-segment
// timestamps, needed for speaker diarization. Missing/corrupt segment data is
// not an error — callers fall back to the plain text.
type SegmentTranscriber interface {
	TranscribeSegments(wavPath string) (text string, segments []Segment, err error)
}

// DefaultModelPath is the default path to the whisper model.
const DefaultModelPath = "models/ggml-medium.bin"

// WhisperTranscriber is a Transcriber implementation using the whisper CLI.
// It uses a CommandRunner to run whisper, which enables testing without the binary.
type WhisperTranscriber struct {
	RunCmd CommandRunner
	// Binary is the whisper executable (if empty — "whisper").
	// Allows using the bundled whisper-cli from the installation directory.
	Binary string
	// ModelPath is the path to the whisper model (if empty — DefaultModelPath).
	ModelPath string
	// Lang is the transcription language ("auto", "uk", "ru", etc.). Defaults to "auto".
	Lang string
	// NoGPU adds -ng to whisper and disables GPU/Metal.
	NoGPU bool
	// OutputDir is the directory for whisper results (if empty — next to the .wav).
	OutputDir string
	// ConsoleOutput, if not nil, receives a copy of the whisper output.
	ConsoleOutput io.Writer
}

// Transcribe runs whisper to transcribe a .wav file.
// It returns the text from the resulting .txt file.
// Whisper creates the .txt file next to the .wav (or in OutputDir, if set).
func (w *WhisperTranscriber) Transcribe(wavPath string) (string, error) {
	return w.runWhisper(wavPath, nil)
}

// TranscribeSegments runs whisper with JSON output on top of the plain text.
// A missing or corrupt JSON is not an error: the plain text is still returned
// and segments are nil, so the caller can degrade to an unlabeled transcript.
func (w *WhisperTranscriber) TranscribeSegments(wavPath string) (string, []Segment, error) {
	text, err := w.runWhisper(wavPath, []string{"-oj"})
	if err != nil {
		return "", nil, err
	}

	data, err := os.ReadFile(w.outputPath(wavPath, ".json"))
	if err != nil {
		return text, nil, nil
	}

	var parsed whisperJSON
	if err := json.Unmarshal(data, &parsed); err != nil {
		return text, nil, nil
	}

	segments := make([]Segment, 0, len(parsed.Transcription))
	for _, s := range parsed.Transcription {
		segments = append(segments, Segment{FromMS: s.Offsets.From, ToMS: s.Offsets.To, Text: s.Text})
	}
	return text, segments, nil
}

// whisperJSON mirrors the parts of whisper.cpp's -oj output that we consume.
type whisperJSON struct {
	Transcription []struct {
		Offsets struct {
			From int64 `json:"from"`
			To   int64 `json:"to"`
		} `json:"offsets"`
		Text string `json:"text"`
	} `json:"transcription"`
}

// runWhisper invokes whisper with -otxt plus extraArgs and returns the .txt content.
func (w *WhisperTranscriber) runWhisper(wavPath string, extraArgs []string) (string, error) {
	modelPath := w.ModelPath
	if modelPath == "" {
		modelPath = DefaultModelPath
	}

	// Without -l, whisper defaults to English, so "auto" must be passed explicitly.
	lang := w.Lang
	if lang == "" {
		lang = "auto"
	}

	args := []string{
		"-m", modelPath,
		"-f", wavPath,
		"-l", lang,
		"-otxt",
	}
	args = append(args, extraArgs...)
	if w.NoGPU {
		args = append(args, "-ng")
	}

	// Binary name/path: the provided Binary or the legacy default "whisper".
	binary := w.Binary
	if binary == "" {
		binary = "whisper"
	}

	output, err := w.RunCmd(binary, args...)

	// Echo whisper's raw output only when a sink is provided (the caller
	// enables this in dev mode; prod leaves it nil to keep the console clean).
	if w.ConsoleOutput != nil && len(output) > 0 {
		fmt.Fprintf(w.ConsoleOutput, "%s\n", output)
	}

	if err != nil {
		if msg := parseWhisperError(output); msg != "" {
			return "", fmt.Errorf("whisper: %s", msg)
		}
		return "", fmt.Errorf("whisper: %w", err)
	}

	data, err := os.ReadFile(w.outputPath(wavPath, ".txt"))
	if err != nil {
		return "", fmt.Errorf("whisper: %w", err)
	}

	return strings.TrimSpace(string(data)), nil
}

// outputPath returns the path of a whisper output file for the given suffix.
// whisper appends the suffix to the full input file name:
// input.wav → input.wav.txt / input.wav.json (NOT input.txt)
func (w *WhisperTranscriber) outputPath(wavPath, suffix string) string {
	if w.OutputDir != "" {
		return filepath.Join(w.OutputDir, filepath.Base(wavPath)+suffix)
	}
	return wavPath + suffix
}

// unknownLangRe matches whisper's message about an unknown language.
// Whisper prints: `error: unknown language 'ua'`.
var unknownLangRe = regexp.MustCompile(`unknown language '([^']+)'`)

// parseWhisperError looks for a clear error cause in whisper's output and
// returns a short message in English. If there is nothing useful,
// it returns an empty string, in which case the caller uses the generic wrapper.
func parseWhisperError(output []byte) string {
	text := string(output)

	if m := unknownLangRe.FindStringSubmatch(text); m != nil {
		return fmt.Sprintf("invalid language code %q — use ISO 639-1 codes like 'uk' (Ukrainian), 'en', 'ru', 'de', or 'auto'", m[1])
	}

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "error:") {
			return strings.TrimPrefix(line, "error: ")
		}
	}
	return ""
}
