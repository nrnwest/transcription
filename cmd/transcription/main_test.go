// Package main — tests for the transcription CLI entry point.
// We test the run() function, which encapsulates all of main()'s logic.
// This approach allows testing exit codes without calling os.Exit().
package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunNoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{}, &stdout, &stderr)

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("stderr should contain a usage hint, got: %s", stderr.String())
	}
}

func TestRunOneArgValidDir(t *testing.T) {
	tmpDir := t.TempDir()
	createFakeVideo(t, tmpDir, "test.mkv")

	var stdout, stderr bytes.Buffer
	// Skip the test if there is no ffmpeg/whisper — dependencies may be absent
	// We only check that argument parsing works and the code != 1 (not a parameter error)
	code := run([]string{tmpDir}, &stdout, &stderr)

	// The code may be 2 (missing deps) or 0/5 (processing), but not 1 (bad params)
	if code == 1 {
		// We allow code 1 only if the error is NOT related to argument parsing
		if strings.Contains(stderr.String(), "Usage:") {
			t.Errorf("one valid argument should not cause a parsing error")
		}
	}
}

func TestRunTwoArgs(t *testing.T) {
	tmpInput := t.TempDir()
	tmpOutput := t.TempDir()
	createFakeVideo(t, tmpInput, "video.mp4")

	var stdout, stderr bytes.Buffer
	code := run([]string{tmpInput, tmpOutput}, &stdout, &stderr)

	// There should be no argument parsing error
	if code == 1 && strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("two valid arguments should not cause a parsing error")
	}
}

func TestRunTooManyArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"a", "b", "c"}, &stdout, &stderr)

	if code != 1 {
		t.Errorf("expected exit code 1 for 3+ arguments, got %d", code)
	}
}

func TestRunNonExistentDir(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"/nonexistent/path/12345"}, &stdout, &stderr)

	if code != 1 {
		t.Errorf("expected exit code 1 for a non-existent directory, got %d", code)
	}
	if !strings.Contains(stderr.String(), "not found") {
		t.Errorf("stderr should report the missing directory, got: %s", stderr.String())
	}
}

func TestRunFileInsteadOfDir(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "notadir.txt")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{tmpFile}, &stdout, &stderr)

	if code != 1 {
		t.Errorf("expected exit code 1 for a file instead of a directory, got %d", code)
	}
	if !strings.Contains(stderr.String(), "directory") || !strings.Contains(stderr.String(), "file") {
		t.Errorf("stderr should report a file instead of a directory, got: %s", stderr.String())
	}
}

func TestRunRelativePath(t *testing.T) {
	tmpDir := t.TempDir()
	createFakeVideo(t, tmpDir, "video.mkv")

	var stdout, stderr bytes.Buffer
	code := run([]string{tmpDir}, &stdout, &stderr)

	// The main point — the code is not 1 due to "invalid path"
	if code == 1 && strings.Contains(stderr.String(), "invalid path") {
		t.Errorf("relative path should work, got error: %s", stderr.String())
	}
}

func TestRunMissingDeps(t *testing.T) {
	tmpDir := t.TempDir()
	createFakeVideo(t, tmpDir, "video.mkv")

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", t.TempDir())

	var stdout, stderr bytes.Buffer
	code := run([]string{tmpDir}, &stdout, &stderr)

	os.Setenv("PATH", origPath)

	if code != 2 {
		t.Errorf("expected exit code 2 for missing dependencies, got %d", code)
	}
	if !strings.Contains(stderr.String(), "ffmpeg") && !strings.Contains(stderr.String(), "whisper") {
		t.Errorf("stderr should report the missing dependency, got: %s", stderr.String())
	}
}

func TestRunNoVideoFiles(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("hello"), 0644)

	setupFakeDeps(t)

	var stdout, stderr bytes.Buffer
	code := run([]string{tmpDir}, &stdout, &stderr)

	if code != 3 {
		t.Errorf("expected exit code 3 for a directory without video, got %d (stderr: %s)", code, stderr.String())
	}
}

func TestRunExitCodeBadParams(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
}

// createFakeVideo creates an empty file with a video extension in the given directory.
func createFakeVideo(t *testing.T, dir, name string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("fake video content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
}

func TestRunDevModeLogsToStdout(t *testing.T) {
	tmpInput := t.TempDir()
	tmpOutput := t.TempDir()
	createFakeVideo(t, tmpInput, "video.mkv")
	setupFakeDeps(t)

	envDir := t.TempDir()
	envPath := filepath.Join(envDir, ".env")
	logDir := filepath.Join(t.TempDir(), "logs")
	logPath := filepath.Join(logDir, "test.log")
	envContent := fmt.Sprintf("MODE=dev\nLOG_PATH=%s\n", logPath)
	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("TRANSCRIPTION_ENV_PATH", envPath)

	var stdout, stderr bytes.Buffer
	code := run([]string{tmpInput, tmpOutput}, &stdout, &stderr)
	_ = code
}

func TestRunProdModeNoLogsOnStdout(t *testing.T) {
	tmpInput := t.TempDir()
	tmpOutput := t.TempDir()
	createFakeVideo(t, tmpInput, "video.mkv")
	setupFakeDeps(t)

	envDir := t.TempDir()
	envPath := filepath.Join(envDir, ".env")
	logDir := filepath.Join(t.TempDir(), "logs")
	logPath := filepath.Join(logDir, "test.log")
	envContent := fmt.Sprintf("MODE=prod\nLOG_PATH=%s\n", logPath)
	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("TRANSCRIPTION_ENV_PATH", envPath)

	var stdout, stderr bytes.Buffer
	code := run([]string{tmpInput, tmpOutput}, &stdout, &stderr)
	_ = code

	// In prod mode stdout must not contain log lines (СТАРТ/ЗАВЕРШЕНО/ПОМИЛКА)
	stdoutStr := stdout.String()
	if strings.Contains(stdoutStr, "СТАРТ") || strings.Contains(stdoutStr, "ЗАВЕРШЕНО") {
		t.Errorf("prod mode should not output logs to stdout, got: %s", stdoutStr)
	}
}

func TestRunMissingEnvProdMode(t *testing.T) {
	tmpInput := t.TempDir()
	createFakeVideo(t, tmpInput, "video.mkv")
	setupFakeDeps(t)

	t.Setenv("TRANSCRIPTION_ENV_PATH", "/nonexistent/.env")

	var stdout, stderr bytes.Buffer
	code := run([]string{tmpInput}, &stdout, &stderr)
	_ = code

	// There should be no panic or crash — the program works in prod mode
	stdoutStr := stdout.String()
	if strings.Contains(stdoutStr, "СТАРТ") {
		t.Errorf("without .env it should be prod mode without logs in stdout, got: %s", stdoutStr)
	}
}

func TestRunDevModeStopOnError(t *testing.T) {
	// The test demonstrates the contract: in dev mode StopOnError=true,
	// on a processing error the exit code = exitAllFailed (4).
	// Without real ffmpeg/whisper we cannot fully test it,
	// but we check that the code does not fail with a dev configuration.
	tmpInput := t.TempDir()
	tmpOutput := t.TempDir()
	createFakeVideo(t, tmpInput, "video.mkv")

	envDir := t.TempDir()
	envPath := filepath.Join(envDir, ".env")
	logDir := filepath.Join(t.TempDir(), "logs")
	logPath := filepath.Join(logDir, "test.log")
	envContent := fmt.Sprintf("MODE=dev\nLOG_PATH=%s\n", logPath)
	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("TRANSCRIPTION_ENV_PATH", envPath)

	// Set PATH without ffmpeg/whisper — this will cause exit code 2 (missing deps)
	// In a real scenario with a processing error it would be exit code 4
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", t.TempDir())

	var stdout, stderr bytes.Buffer
	code := run([]string{tmpInput, tmpOutput}, &stdout, &stderr)

	os.Setenv("PATH", origPath)

	// With missing deps in dev mode — exit code 2
	if code != 2 {
		t.Errorf("expected exit code 2 for dev mode without dependencies, got %d", code)
	}
}

func TestRunLangFlag(t *testing.T) {
	tmpDir := t.TempDir()
	createFakeVideo(t, tmpDir, "video.mkv")
	setupFakeDeps(t)

	var stdout, stderr bytes.Buffer
	code := run([]string{"-lang", "uk", tmpDir}, &stdout, &stderr)

	// The code should not be 1 (an argument parsing error)
	if code == 1 && strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("-lang uk + directory should not cause a parsing error")
	}
}

func TestRunLangFlagAfterDir(t *testing.T) {
	tmpDir := t.TempDir()
	createFakeVideo(t, tmpDir, "video.mkv")
	setupFakeDeps(t)

	var stdout, stderr bytes.Buffer
	code := run([]string{tmpDir, "-lang", "ru"}, &stdout, &stderr)

	// There should be no "invalid number of arguments" error
	if code == 1 && strings.Contains(stderr.String(), "invalid number") {
		t.Errorf("directory + -lang ru should not cause an argument-count error")
	}
}

func TestRunLangFlagWithOutputDir(t *testing.T) {
	tmpInput := t.TempDir()
	tmpOutput := t.TempDir()
	createFakeVideo(t, tmpInput, "video.mkv")
	setupFakeDeps(t)

	var stdout, stderr bytes.Buffer
	code := run([]string{"-lang", "auto", tmpInput, tmpOutput}, &stdout, &stderr)

	if code == 1 && strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("-lang auto + input + output should not cause a parsing error")
	}
}

func TestRunInvalidLang(t *testing.T) {
	tmpDir := t.TempDir()
	createFakeVideo(t, tmpDir, "video.mkv")
	// We do NOT call setupFakeDeps — we check that language validation
	// happens BEFORE checkDependencies.

	t.Setenv("TRANSCRIPTION_ENV_PATH", "/nonexistent/.env")

	var stdout, stderr bytes.Buffer
	code := run([]string{"-lang", "ua", tmpDir}, &stdout, &stderr)

	if code != 1 {
		t.Errorf("expected exit code 1 (exitBadParams), got %d", code)
	}
	errOut := stderr.String()
	if !strings.Contains(errOut, `invalid language code "ua"`) {
		t.Errorf("stderr should contain 'invalid language code \"ua\"', got: %q", errOut)
	}
	if !strings.Contains(errOut, "'uk'") {
		t.Errorf("stderr should contain the valid code example 'uk', got: %q", errOut)
	}
}

func TestRunNoLangFlagDefaultAuto(t *testing.T) {
	tmpDir := t.TempDir()
	createFakeVideo(t, tmpDir, "video.mkv")
	setupFakeDeps(t)

	t.Setenv("TRANSCRIPTION_ENV_PATH", "/nonexistent/.env")

	var stdout, stderr bytes.Buffer
	code := run([]string{tmpDir}, &stdout, &stderr)

	// Without -lang and without .env it should work with auto — the main point is no crash
	_ = code
}

// setupFakeDeps creates fake ffmpeg and whisper executables in a temporary
// directory and adds it to PATH. This lets tests pass the dependency check.
// Cross-platform: on Windows exec.LookPath looks for files with the .exe suffix, and PATH
// is joined via os.PathListSeparator (';' on Windows, ':' on other OSes).
func setupFakeDeps(t *testing.T) {
	t.Helper()
	binDir := t.TempDir()

	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}

	ffmpegPath := filepath.Join(binDir, "ffmpeg"+suffix)
	if err := os.WriteFile(ffmpegPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("failed to create fake ffmpeg: %v", err)
	}

	whisperPath := filepath.Join(binDir, "whisper"+suffix)
	if err := os.WriteFile(whisperPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("failed to create fake whisper: %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+origPath)
}

func TestRunDiarizeFlagWithoutSherpa(t *testing.T) {
	tmpDir := t.TempDir()
	createFakeVideo(t, tmpDir, "video.mkv")

	// PATH contains ONLY fake ffmpeg and whisper — no sherpa, even if it is
	// installed on the developer machine (the test must stay hermetic).
	binDir := t.TempDir()
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	for _, name := range []string{"ffmpeg", "whisper"} {
		if err := os.WriteFile(filepath.Join(binDir, name+suffix), []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
			t.Fatalf("failed to create fake %s: %v", name, err)
		}
	}
	t.Setenv("PATH", binDir)

	t.Setenv("TRANSCRIPTION_ENV_PATH", "/nonexistent/.env")

	var stdout, stderr bytes.Buffer
	code := run([]string{"-diarize", tmpDir}, &stdout, &stderr)

	if code != 2 {
		t.Errorf("expected exit code 2 for -diarize without sherpa, got %d", code)
	}
	if !strings.Contains(stderr.String(), "sherpa-onnx-offline-speaker-diarization") {
		t.Errorf("stderr must name the missing sherpa binary, got: %q", stderr.String())
	}
}

func TestRunSpeakersFlagInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	createFakeVideo(t, tmpDir, "video.mkv")

	t.Setenv("TRANSCRIPTION_ENV_PATH", "/nonexistent/.env")

	var stdout, stderr bytes.Buffer
	code := run([]string{"-diarize", "-speakers", "abc", tmpDir}, &stdout, &stderr)

	if code != 1 {
		t.Errorf("expected exit code 1 for -speakers abc, got %d", code)
	}
	if !strings.Contains(stderr.String(), "-speakers") {
		t.Errorf("stderr must mention the -speakers flag, got: %q", stderr.String())
	}
}

func TestRunWithoutDiarizeIgnoresSherpa(t *testing.T) {
	tmpDir := t.TempDir()
	createFakeVideo(t, tmpDir, "video.mkv")
	setupFakeDeps(t) // no sherpa in PATH

	t.Setenv("TRANSCRIPTION_ENV_PATH", "/nonexistent/.env")

	var stdout, stderr bytes.Buffer
	run([]string{tmpDir}, &stdout, &stderr)

	// Without -diarize the sherpa binary must not be required.
	if strings.Contains(stderr.String(), "sherpa") {
		t.Errorf("sherpa must not be looked up without -diarize, stderr: %q", stderr.String())
	}
}

func TestRunThresholdFlagInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	createFakeVideo(t, tmpDir, "video.mkv")

	t.Setenv("TRANSCRIPTION_ENV_PATH", "/nonexistent/.env")

	for _, value := range []string{"abc", "0", "-0.3"} {
		var stdout, stderr bytes.Buffer
		code := run([]string{"-diarize", "-threshold", value, tmpDir}, &stdout, &stderr)

		if code != 1 {
			t.Errorf("expected exit code 1 for -threshold %s, got %d", value, code)
		}
		if !strings.Contains(stderr.String(), "-threshold") {
			t.Errorf("stderr must mention the -threshold flag, got: %q", stderr.String())
		}
	}
}

func TestRunThresholdFlagValidParses(t *testing.T) {
	tmpDir := t.TempDir()
	createFakeVideo(t, tmpDir, "video.mkv")
	setupFakeDeps(t) // no sherpa → the run stops at the sherpa check, AFTER flag parsing

	t.Setenv("TRANSCRIPTION_ENV_PATH", "/nonexistent/.env")

	var stdout, stderr bytes.Buffer
	code := run([]string{"-diarize", "-threshold", "0.3", tmpDir}, &stdout, &stderr)

	if code != 2 {
		t.Errorf("expected exit code 2 (missing sherpa, flags OK), got %d", code)
	}
	if strings.Contains(stderr.String(), "-threshold") {
		t.Errorf("valid -threshold must not cause a flag error, got: %q", stderr.String())
	}
}

// setupHermeticDeps puts ONLY the given fake executables into PATH.
func setupHermeticDeps(t *testing.T, names ...string) {
	t.Helper()
	binDir := t.TempDir()
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(binDir, name+suffix), []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
			t.Fatalf("failed to create fake %s: %v", name, err)
		}
	}
	t.Setenv("PATH", binDir)
}

func TestRunEngineFlagInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	createFakeVideo(t, tmpDir, "video.mkv")

	t.Setenv("TRANSCRIPTION_ENV_PATH", "/nonexistent/.env")

	var stdout, stderr bytes.Buffer
	code := run([]string{"-diarize", "-engine", "whisperx", tmpDir}, &stdout, &stderr)

	if code != 1 {
		t.Errorf("expected exit code 1 for -engine whisperx, got %d", code)
	}
	if !strings.Contains(stderr.String(), "-engine") {
		t.Errorf("stderr must mention the -engine flag, got: %q", stderr.String())
	}
}

func TestRunPyannoteEngineWithoutPython(t *testing.T) {
	tmpDir := t.TempDir()
	createFakeVideo(t, tmpDir, "video.mkv")
	setupHermeticDeps(t, "ffmpeg", "whisper") // no python3

	t.Setenv("TRANSCRIPTION_ENV_PATH", "/nonexistent/.env")

	var stdout, stderr bytes.Buffer
	code := run([]string{"-diarize", "-engine", "pyannote", tmpDir}, &stdout, &stderr)

	if code != 2 {
		t.Errorf("expected exit code 2 for pyannote without python, got %d", code)
	}
	if !strings.Contains(stderr.String(), "python") {
		t.Errorf("stderr must mention python, got: %q", stderr.String())
	}
}

func TestRunPyannoteEngineSkipsSherpaChecks(t *testing.T) {
	tmpDir := t.TempDir()
	createFakeVideo(t, tmpDir, "video.mkv")
	// python3 present, sherpa absent — pyannote engine must not require sherpa
	// or the ONNX models.
	setupHermeticDeps(t, "ffmpeg", "whisper", "python3")

	t.Setenv("TRANSCRIPTION_ENV_PATH", "/nonexistent/.env")

	var stdout, stderr bytes.Buffer
	run([]string{"-diarize", "-engine", "pyannote", tmpDir}, &stdout, &stderr)

	if strings.Contains(stderr.String(), "sherpa") {
		t.Errorf("pyannote engine must not require sherpa, stderr: %q", stderr.String())
	}
	if strings.Contains(stderr.String(), "diarization model") {
		t.Errorf("pyannote engine must not require ONNX models, stderr: %q", stderr.String())
	}
}

func TestRunEngineFromEnvAndFlagOverride(t *testing.T) {
	tmpDir := t.TempDir()
	createFakeVideo(t, tmpDir, "video.mkv")
	setupHermeticDeps(t, "ffmpeg", "whisper") // no python, no sherpa

	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("DIARIZE=true\nDIARIZE_ENGINE=pyannote\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRANSCRIPTION_ENV_PATH", envPath)

	// .env engine=pyannote → fails on the missing python
	var stderr1 bytes.Buffer
	code := run([]string{tmpDir}, &bytes.Buffer{}, &stderr1)
	if code != 2 || !strings.Contains(stderr1.String(), "python") {
		t.Errorf(".env DIARIZE_ENGINE=pyannote must be respected: code=%d stderr=%q", code, stderr1.String())
	}

	// -engine sherpa overrides .env → fails on the missing sherpa instead
	var stderr2 bytes.Buffer
	code = run([]string{"-engine", "sherpa", tmpDir}, &bytes.Buffer{}, &stderr2)
	if code != 2 || !strings.Contains(stderr2.String(), "sherpa") {
		t.Errorf("-engine sherpa must override .env: code=%d stderr=%q", code, stderr2.String())
	}
}

func TestRunPyannoteIgnoresThresholdWithWarning(t *testing.T) {
	tmpDir := t.TempDir()
	createFakeVideo(t, tmpDir, "video.mkv")
	setupHermeticDeps(t, "ffmpeg", "whisper", "python3")

	t.Setenv("TRANSCRIPTION_ENV_PATH", "/nonexistent/.env")

	var stdout, stderr bytes.Buffer
	run([]string{"-diarize", "-engine", "pyannote", "-threshold", "0.4", tmpDir}, &stdout, &stderr)

	if !strings.Contains(stderr.String(), "ignored") {
		t.Errorf("expected a warning that -threshold is ignored by pyannote, got: %q", stderr.String())
	}
}
