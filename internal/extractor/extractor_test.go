package extractor

import (
	"fmt"
	"strings"
	"testing"
)

// mockExtractor is a test implementation of AudioExtractor.
type mockExtractor struct{}

func (m *mockExtractor) Extract(videoPath string) (string, error) {
	return videoPath + ".wav", nil
}

func TestMockExtractorImplementsInterface(t *testing.T) {
	var e AudioExtractor = &mockExtractor{}

	wavPath, err := e.Extract("/video/AI-2.mkv")
	if err != nil {
		t.Fatalf("did not expect an error, got: %v", err)
	}
	if wavPath != "/video/AI-2.mkv.wav" {
		t.Errorf("expected /video/AI-2.mkv.wav, got %s", wavPath)
	}
}

func TestFfmpegExtractorFormsCorrectCommand(t *testing.T) {
	var capturedName string
	var capturedArgs []string

	mockRunner := func(name string, args ...string) ([]byte, error) {
		capturedName = name
		capturedArgs = args
		return []byte(""), nil
	}

	ext := &FfmpegExtractor{RunCmd: mockRunner}
	_, err := ext.Extract("/video/lecture.mkv")
	if err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}

	if capturedName != "ffmpeg" {
		t.Errorf("expected command 'ffmpeg', got '%s'", capturedName)
	}

	argsStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsStr, "/video/lecture.mkv") {
		t.Errorf("arguments must contain the input path, got: %s", argsStr)
	}

	if !strings.Contains(argsStr, ".wav") {
		t.Errorf("arguments must contain the output .wav path, got: %s", argsStr)
	}
}

func TestFfmpegExtractorReturnsWavPath(t *testing.T) {
	mockRunner := func(name string, args ...string) ([]byte, error) {
		return []byte(""), nil
	}

	ext := &FfmpegExtractor{RunCmd: mockRunner}
	wavPath, err := ext.Extract("/video/lecture.mkv")
	if err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}

	expected := "/video/lecture.wav"
	if wavPath != expected {
		t.Errorf("expected wavPath = '%s', got '%s'", expected, wavPath)
	}
}

func TestFfmpegExtractorHandlesError(t *testing.T) {
	mockRunner := func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("ffmpeg: exit status 1")
	}

	ext := &FfmpegExtractor{RunCmd: mockRunner}
	_, err := ext.Extract("/video/broken.mkv")

	if err == nil {
		t.Fatal("expected an error from ffmpeg, but got nil")
	}

	if !strings.Contains(err.Error(), "ffmpeg") {
		t.Errorf("error must contain 'ffmpeg', got: %v", err)
	}
}

func TestFfmpegExtractorImplementsInterface(t *testing.T) {
	mockRunner := func(name string, args ...string) ([]byte, error) {
		return []byte(""), nil
	}

	var _ AudioExtractor = &FfmpegExtractor{RunCmd: mockRunner}
}

func TestFfmpegExtractorUsesConfiguredBinary(t *testing.T) {
	var capturedName string
	mockRunner := func(name string, args ...string) ([]byte, error) {
		capturedName = name
		return nil, nil
	}

	ext := &FfmpegExtractor{
		RunCmd: mockRunner,
		Binary: "/custom/bin/ffmpeg",
	}
	if _, err := ext.Extract("/video/lecture.mkv"); err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}

	if capturedName != "/custom/bin/ffmpeg" {
		t.Errorf("expected the configured binary, got %q", capturedName)
	}
}
