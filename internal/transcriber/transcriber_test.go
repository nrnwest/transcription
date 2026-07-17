package transcriber

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mockTranscriber is a test implementation of Transcriber.
// It returns a fixed text without actually calling whisper.
type mockTranscriber struct{}

func (m *mockTranscriber) Transcribe(wavPath string) (string, error) {
	return "Це тестовий транскрибований текст.", nil
}

func TestMockTranscriberImplementsInterface(t *testing.T) {
	var tr Transcriber = &mockTranscriber{}

	text, err := tr.Transcribe("/video/AI-2.wav")
	if err != nil {
		t.Fatalf("did not expect an error, got: %v", err)
	}
	if text != "Це тестовий транскрибований текст." {
		t.Errorf("expected the fixed text, got: %s", text)
	}
}

func TestWhisperTranscriberFormsCorrectCommand(t *testing.T) {
	var capturedName string
	var capturedArgs []string

	mockRunner := func(name string, args ...string) ([]byte, error) {
		capturedName = name
		capturedArgs = args
		return []byte(""), nil
	}

	// Create a temporary directory with the .txt result file
	// that whisper would have created
	dir := t.TempDir()
	wavPath := filepath.Join(dir, "lecture.wav")
	txtPath := filepath.Join(dir, "lecture.wav.txt")

	// Simulate the whisper result — a .txt file with the transcription
	err := os.WriteFile(txtPath, []byte("транскрибований текст"), 0644)
	if err != nil {
		t.Fatalf("failed to create test .txt file: %v", err)
	}

	tr := &WhisperTranscriber{RunCmd: mockRunner, OutputDir: dir}
	_, err = tr.Transcribe(wavPath)
	if err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}

	// Verify that whisper was invoked
	if capturedName != "whisper" {
		t.Errorf("expected command 'whisper', got '%s'", capturedName)
	}

	// Verify that the .wav path is among the arguments
	argsStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsStr, wavPath) {
		t.Errorf("arguments must contain the .wav path: %s", argsStr)
	}
}

func TestWhisperTranscriberReadsTxtResult(t *testing.T) {
	dir := t.TempDir()
	wavPath := filepath.Join(dir, "lecture.wav")
	txtPath := filepath.Join(dir, "lecture.wav.txt")

	expectedText := "Це транскрибований текст з лекції про Go."
	err := os.WriteFile(txtPath, []byte(expectedText), 0644)
	if err != nil {
		t.Fatalf("failed to create .txt file: %v", err)
	}

	// Mock runner — does nothing, the .txt is already created
	mockRunner := func(name string, args ...string) ([]byte, error) {
		return []byte(""), nil
	}

	tr := &WhisperTranscriber{RunCmd: mockRunner, OutputDir: dir}
	text, err := tr.Transcribe(wavPath)
	if err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}

	if text != expectedText {
		t.Errorf("expected text '%s', got '%s'", expectedText, text)
	}
}

func TestWhisperTranscriberHandlesError(t *testing.T) {
	mockRunner := func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("whisper: модель не знайдено")
	}

	tr := &WhisperTranscriber{RunCmd: mockRunner}
	_, err := tr.Transcribe("/tmp/lecture.wav")

	if err == nil {
		t.Fatal("expected an error from whisper, but got nil")
	}

	if !strings.Contains(err.Error(), "whisper") {
		t.Errorf("error must contain 'whisper', got: %v", err)
	}
}

func TestWhisperTranscriberParsesWhisperError(t *testing.T) {
	cases := []struct {
		name        string
		output      string
		wantContain []string
	}{
		{
			name:        "unknown language",
			output:      "whisper_lang_id: unknown language 'ua'\nerror: unknown language 'ua'\n\nusage: whisper [options] file0 file1 ...\n",
			wantContain: []string{`invalid language code "ua"`, "'uk'", "Ukrainian"},
		},
		{
			name:        "generic error line",
			output:      "some preamble\nerror: failed to load model\nmore noise\n",
			wantContain: []string{"failed to load model"},
		},
		{
			name:        "no error line — fallback",
			output:      "just some noise without an error marker\n",
			wantContain: []string{"whisper"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outputBytes := []byte(tc.output)
			mockRunner := func(name string, args ...string) ([]byte, error) {
				return outputBytes, fmt.Errorf("exit status 1")
			}

			tr := &WhisperTranscriber{RunCmd: mockRunner}
			_, err := tr.Transcribe("/tmp/lecture.wav")
			if err == nil {
				t.Fatalf("expected an error, got nil")
			}
			msg := err.Error()
			for _, want := range tc.wantContain {
				if !strings.Contains(msg, want) {
					t.Errorf("error must contain %q, got: %q", want, msg)
				}
			}
		})
	}
}

func TestWhisperTranscriberImplementsInterface(t *testing.T) {
	mockRunner := func(name string, args ...string) ([]byte, error) {
		return []byte(""), nil
	}

	// Compilation guarantees interface conformance
	var _ Transcriber = &WhisperTranscriber{RunCmd: mockRunner}
}

func TestWhisperTranscriberLangAuto(t *testing.T) {
	var capturedArgs []string

	mockRunner := func(name string, args ...string) ([]byte, error) {
		capturedArgs = args
		return []byte(""), nil
	}

	dir := t.TempDir()
	wavPath := filepath.Join(dir, "test.wav")
	txtPath := filepath.Join(dir, "test.wav.txt")
	os.WriteFile(txtPath, []byte("text"), 0644)

	tr := &WhisperTranscriber{RunCmd: mockRunner, OutputDir: dir}
	_, err := tr.Transcribe(wavPath)
	if err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}

	argsStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsStr, "-l auto") {
		t.Errorf("expected '-l auto' in the arguments, got: %s", argsStr)
	}
}

func TestWhisperTranscriberLangUk(t *testing.T) {
	var capturedArgs []string

	mockRunner := func(name string, args ...string) ([]byte, error) {
		capturedArgs = args
		return []byte(""), nil
	}

	dir := t.TempDir()
	wavPath := filepath.Join(dir, "test.wav")
	txtPath := filepath.Join(dir, "test.wav.txt")
	os.WriteFile(txtPath, []byte("текст"), 0644)

	tr := &WhisperTranscriber{RunCmd: mockRunner, Lang: "uk", OutputDir: dir}
	_, err := tr.Transcribe(wavPath)
	if err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}

	argsStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsStr, "-l uk") {
		t.Errorf("expected '-l uk' in the arguments, got: %s", argsStr)
	}
}

func TestWhisperTranscriberLangRu(t *testing.T) {
	var capturedArgs []string

	mockRunner := func(name string, args ...string) ([]byte, error) {
		capturedArgs = args
		return []byte(""), nil
	}

	dir := t.TempDir()
	wavPath := filepath.Join(dir, "test.wav")
	txtPath := filepath.Join(dir, "test.wav.txt")
	os.WriteFile(txtPath, []byte("текст"), 0644)

	tr := &WhisperTranscriber{RunCmd: mockRunner, Lang: "ru", OutputDir: dir}
	_, err := tr.Transcribe(wavPath)
	if err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}

	argsStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsStr, "-l ru") {
		t.Errorf("expected '-l ru' in the arguments, got: %s", argsStr)
	}
}

const whisperJSONFixture = `{
	"transcription": [
		{"timestamps": {"from": "00:00:00,000", "to": "00:00:05,000"}, "offsets": {"from": 0, "to": 5000}, "text": " Hello there."},
		{"timestamps": {"from": "00:00:05,000", "to": "00:00:09,500"}, "offsets": {"from": 5000, "to": 9500}, "text": " General Kenobi."}
	]
}`

func TestTranscribeSegmentsPassesJSONFlag(t *testing.T) {
	var capturedArgs []string

	dir := t.TempDir()
	wavPath := filepath.Join(dir, "talk.wav")
	os.WriteFile(filepath.Join(dir, "talk.wav.txt"), []byte("Hello there. General Kenobi."), 0644)
	os.WriteFile(filepath.Join(dir, "talk.wav.json"), []byte(whisperJSONFixture), 0644)

	mockRunner := func(name string, args ...string) ([]byte, error) {
		capturedArgs = args
		return []byte(""), nil
	}

	tr := &WhisperTranscriber{RunCmd: mockRunner, OutputDir: dir}
	_, _, err := tr.TranscribeSegments(wavPath)
	if err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}

	argsStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsStr, "-oj") {
		t.Errorf("expected '-oj' in the arguments, got: %s", argsStr)
	}
	if !strings.Contains(argsStr, "-otxt") {
		t.Errorf("expected '-otxt' in the arguments, got: %s", argsStr)
	}
}

func TestTranscribeSegmentsParsesJSON(t *testing.T) {
	dir := t.TempDir()
	wavPath := filepath.Join(dir, "talk.wav")
	os.WriteFile(filepath.Join(dir, "talk.wav.txt"), []byte("Hello there. General Kenobi.\n"), 0644)
	os.WriteFile(filepath.Join(dir, "talk.wav.json"), []byte(whisperJSONFixture), 0644)

	mockRunner := func(name string, args ...string) ([]byte, error) {
		return []byte(""), nil
	}

	tr := &WhisperTranscriber{RunCmd: mockRunner, OutputDir: dir}
	text, segs, err := tr.TranscribeSegments(wavPath)
	if err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}

	if text != "Hello there. General Kenobi." {
		t.Errorf("unexpected text: %q", text)
	}
	if len(segs) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segs))
	}
	if segs[0].FromMS != 0 || segs[0].ToMS != 5000 || strings.TrimSpace(segs[0].Text) != "Hello there." {
		t.Errorf("unexpected first segment: %+v", segs[0])
	}
	if segs[1].FromMS != 5000 || segs[1].ToMS != 9500 || strings.TrimSpace(segs[1].Text) != "General Kenobi." {
		t.Errorf("unexpected second segment: %+v", segs[1])
	}
}

func TestTranscribeSegmentsMissingJSONDegrades(t *testing.T) {
	dir := t.TempDir()
	wavPath := filepath.Join(dir, "talk.wav")
	os.WriteFile(filepath.Join(dir, "talk.wav.txt"), []byte("plain text"), 0644)

	mockRunner := func(name string, args ...string) ([]byte, error) {
		return []byte(""), nil
	}

	tr := &WhisperTranscriber{RunCmd: mockRunner, OutputDir: dir}
	text, segs, err := tr.TranscribeSegments(wavPath)
	if err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}
	if text != "plain text" {
		t.Errorf("unexpected text: %q", text)
	}
	if segs != nil {
		t.Errorf("expected nil segments for a missing JSON, got: %+v", segs)
	}
}

func TestTranscribeSegmentsCorruptJSONDegrades(t *testing.T) {
	dir := t.TempDir()
	wavPath := filepath.Join(dir, "talk.wav")
	os.WriteFile(filepath.Join(dir, "talk.wav.txt"), []byte("plain text"), 0644)
	os.WriteFile(filepath.Join(dir, "talk.wav.json"), []byte("{not json"), 0644)

	mockRunner := func(name string, args ...string) ([]byte, error) {
		return []byte(""), nil
	}

	tr := &WhisperTranscriber{RunCmd: mockRunner, OutputDir: dir}
	text, segs, err := tr.TranscribeSegments(wavPath)
	if err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}
	if text != "plain text" {
		t.Errorf("unexpected text: %q", text)
	}
	if segs != nil {
		t.Errorf("expected nil segments for a corrupt JSON, got: %+v", segs)
	}
}

func TestTranscribeSegmentsEmptyTranscription(t *testing.T) {
	dir := t.TempDir()
	wavPath := filepath.Join(dir, "talk.wav")
	os.WriteFile(filepath.Join(dir, "talk.wav.txt"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "talk.wav.json"), []byte(`{"transcription": []}`), 0644)

	mockRunner := func(name string, args ...string) ([]byte, error) {
		return []byte(""), nil
	}

	tr := &WhisperTranscriber{RunCmd: mockRunner, OutputDir: dir}
	_, segs, err := tr.TranscribeSegments(wavPath)
	if err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}
	if len(segs) != 0 {
		t.Errorf("expected no segments, got: %+v", segs)
	}
}

func TestTranscribeSegmentsCommandError(t *testing.T) {
	mockRunner := func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("boom")
	}

	tr := &WhisperTranscriber{RunCmd: mockRunner}
	_, _, err := tr.TranscribeSegments("/tmp/talk.wav")
	if err == nil {
		t.Fatal("expected an error from whisper, got nil")
	}
}

func TestWhisperTranscriberPlainTranscribeHasNoJSONFlag(t *testing.T) {
	var capturedArgs []string

	dir := t.TempDir()
	wavPath := filepath.Join(dir, "test.wav")
	os.WriteFile(filepath.Join(dir, "test.wav.txt"), []byte("text"), 0644)

	mockRunner := func(name string, args ...string) ([]byte, error) {
		capturedArgs = args
		return []byte(""), nil
	}

	tr := &WhisperTranscriber{RunCmd: mockRunner, OutputDir: dir}
	if _, err := tr.Transcribe(wavPath); err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}

	for _, a := range capturedArgs {
		if a == "-oj" {
			t.Error("plain Transcribe must not pass '-oj'")
		}
	}
}

func TestWhisperTranscriberNoGPU(t *testing.T) {
	var capturedArgs []string

	mockRunner := func(name string, args ...string) ([]byte, error) {
		capturedArgs = args
		return []byte(""), nil
	}

	dir := t.TempDir()
	wavPath := filepath.Join(dir, "test.wav")
	txtPath := filepath.Join(dir, "test.wav.txt")
	os.WriteFile(txtPath, []byte("text"), 0644)

	tr := &WhisperTranscriber{RunCmd: mockRunner, OutputDir: dir, NoGPU: true}
	_, err := tr.Transcribe(wavPath)
	if err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}

	argsStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsStr, "-ng") {
		t.Errorf("expected '-ng' in the arguments, got: %s", argsStr)
	}
}
