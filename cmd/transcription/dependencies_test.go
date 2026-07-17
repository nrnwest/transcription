package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestEnsureFFmpegFoundInPath(t *testing.T) {
	manager := dependencyManager{
		lookPath: func(name string) (string, error) {
			if name == "ffmpeg" {
				return "/usr/local/bin/ffmpeg", nil
			}
			return "", errors.New("не знайдено")
		},
	}

	path, err := manager.ensureFFmpeg()

	if err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}
	if path != "/usr/local/bin/ffmpeg" {
		t.Errorf("expected ffmpeg path from PATH, got %q", path)
	}
}

func TestEnsureFFmpegMissingFromPath(t *testing.T) {
	manager := dependencyManager{
		lookPath: func(name string) (string, error) {
			return "", errors.New("не знайдено")
		},
	}

	_, err := manager.ensureFFmpeg()

	if err == nil {
		t.Fatal("expected an error when ffmpeg is missing from PATH")
	}
	if !strings.Contains(err.Error(), "ffmpeg not found in PATH") {
		t.Errorf("error should reference ffmpeg missing from PATH, got: %v", err)
	}
	if !strings.Contains(err.Error(), "README") {
		t.Errorf("error should reference README, got: %v", err)
	}
}

func TestResolveSherpaFound(t *testing.T) {
	lookPath := func(name string) (string, error) {
		if name == "sherpa-onnx-offline-speaker-diarization" {
			return "/usr/local/bin/sherpa-onnx-offline-speaker-diarization", nil
		}
		return "", fmt.Errorf("not found: %s", name)
	}

	path, err := resolveSherpaWith(lookPath)
	if err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}
	if path != "/usr/local/bin/sherpa-onnx-offline-speaker-diarization" {
		t.Errorf("unexpected path: %s", path)
	}
}

func TestResolveSherpaMissing(t *testing.T) {
	lookPath := func(name string) (string, error) {
		return "", fmt.Errorf("not found: %s", name)
	}

	_, err := resolveSherpaWith(lookPath)
	if err == nil {
		t.Fatal("expected an error for a missing sherpa binary")
	}
	if !strings.Contains(err.Error(), "sherpa-onnx-offline-speaker-diarization") {
		t.Errorf("error must name the missing binary, got: %v", err)
	}
}

func TestResolvePythonFound(t *testing.T) {
	lookPath := func(name string) (string, error) {
		if name == "python3" {
			return "/usr/bin/python3", nil
		}
		return "", fmt.Errorf("not found: %s", name)
	}

	path, err := resolvePythonWith(lookPath, "python3")
	if err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}
	if path != "/usr/bin/python3" {
		t.Errorf("unexpected path: %s", path)
	}
}

func TestResolvePythonMissing(t *testing.T) {
	lookPath := func(name string) (string, error) {
		return "", fmt.Errorf("not found: %s", name)
	}

	_, err := resolvePythonWith(lookPath, "/opt/venv/bin/python3")
	if err == nil {
		t.Fatal("expected an error for a missing python")
	}
	if !strings.Contains(err.Error(), "/opt/venv/bin/python3") {
		t.Errorf("error must name the missing interpreter, got: %v", err)
	}
}

func TestCheckPyannoteOK(t *testing.T) {
	var capturedName string
	var capturedArgs []string

	runner := func(name string, args ...string) ([]byte, error) {
		capturedName = name
		capturedArgs = args
		return []byte(""), nil
	}

	if err := checkPyannote(runner, "/usr/bin/python3"); err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}
	if capturedName != "/usr/bin/python3" {
		t.Errorf("unexpected interpreter: %s", capturedName)
	}
	argsStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsStr, "find_spec('pyannote.audio')") {
		t.Errorf("expected a find_spec preflight, got: %s", argsStr)
	}
}

func TestCheckPyannoteMissing(t *testing.T) {
	runner := func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("exit status 1")
	}

	err := checkPyannote(runner, "/usr/bin/python3")
	if err == nil {
		t.Fatal("expected an error when pyannote.audio is missing")
	}
	if !strings.Contains(err.Error(), "pip install pyannote.audio") {
		t.Errorf("error must suggest the pip install command, got: %v", err)
	}
}
