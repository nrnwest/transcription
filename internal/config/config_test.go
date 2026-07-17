// Package config_test — tests for loading configuration from the .env file.
// We check key-value parsing, default values,
// handling of comments and unknown MODE values.
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigModeDev(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("MODE=dev\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, warning := LoadConfig(envPath)

	if cfg.Mode != "dev" {
		t.Errorf("expected Mode='dev', got '%s'", cfg.Mode)
	}
	if warning != "" {
		t.Errorf("expected no warning, got: %s", warning)
	}
}

func TestLoadConfigModeProd(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("MODE=prod\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, warning := LoadConfig(envPath)

	if cfg.Mode != "prod" {
		t.Errorf("expected Mode='prod', got '%s'", cfg.Mode)
	}
	if warning != "" {
		t.Errorf("expected no warning, got: %s", warning)
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	cfg, warning := LoadConfig("/nonexistent/path/.env")

	if cfg.Mode != "prod" {
		t.Errorf("expected Mode='prod' by default, got '%s'", cfg.Mode)
	}
	if cfg.LogPath != "tmp/dev.log" {
		t.Errorf("expected LogPath='tmp/dev.log', got '%s'", cfg.LogPath)
	}
	if warning != "" {
		t.Errorf("expected no warning for a missing file, got: %s", warning)
	}
}

func TestLoadConfigUnknownMode(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("MODE=staging\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, warning := LoadConfig(envPath)

	if cfg.Mode != "prod" {
		t.Errorf("expected Mode='prod' for an unknown value, got '%s'", cfg.Mode)
	}
	if warning == "" {
		t.Error("expected a warning for an unknown MODE value")
	}
}

func TestLoadConfigCommentsAndEmptyLines(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := "# This is a comment\n\nMODE=dev\n\n# Another comment\nLOG_PATH=logs/app.log\n"
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, warning := LoadConfig(envPath)

	if cfg.Mode != "dev" {
		t.Errorf("expected Mode='dev', got '%s'", cfg.Mode)
	}
	if cfg.LogPath != "logs/app.log" {
		t.Errorf("expected LogPath='logs/app.log', got '%s'", cfg.LogPath)
	}
	if warning != "" {
		t.Errorf("expected no warning, got: %s", warning)
	}
}

func TestLoadConfigKeyValueFormat(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := "MODE=dev\nLOG_PATH=/var/log/app.log\n"
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, warning := LoadConfig(envPath)

	if cfg.Mode != "dev" {
		t.Errorf("expected Mode='dev', got '%s'", cfg.Mode)
	}
	if cfg.LogPath != "/var/log/app.log" {
		t.Errorf("expected LogPath='/var/log/app.log', got '%s'", cfg.LogPath)
	}
	if warning != "" {
		t.Errorf("expected no warning, got: %s", warning)
	}
}

func TestLoadConfigOutputDir(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := "MODE=dev\nOUTPUT_DIR=/tmp/transcription-out\n"
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, warning := LoadConfig(envPath)

	if cfg.OutputDir != "/tmp/transcription-out" {
		t.Errorf("expected OutputDir='/tmp/transcription-out', got '%s'", cfg.OutputDir)
	}
	if warning != "" {
		t.Errorf("expected no warning, got: %s", warning)
	}
}

func TestLoadConfigOutputDirDefault(t *testing.T) {
	cfg, _ := LoadConfig("/nonexistent/.env")

	if cfg.OutputDir != "" {
		t.Errorf("expected an empty OutputDir by default, got '%s'", cfg.OutputDir)
	}
}

func TestLoadConfigWhisperModelDefault(t *testing.T) {
	cfg, _ := LoadConfig("/nonexistent/.env")

	if cfg.WhisperModel != "medium" {
		t.Errorf("expected WhisperModel='medium', got '%s'", cfg.WhisperModel)
	}
}

func TestLoadConfigWhisperModelUnknown(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := "MODE=dev\nWHISPER_MODEL=nonexistent\n"
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, warning := LoadConfig(envPath)

	if cfg.WhisperModel != "medium" {
		t.Errorf("expected fallback WhisperModel='medium', got '%s'", cfg.WhisperModel)
	}
	if warning == "" {
		t.Error("expected a warning for an unknown model")
	}
}

func TestLoadConfigWhisperModelChoices(t *testing.T) {
	for _, model := range []string{"tiny", "small", "medium", "large-v3"} {
		t.Run(model, func(t *testing.T) {
			dir := t.TempDir()
			envPath := filepath.Join(dir, ".env")
			if err := os.WriteFile(envPath, []byte("WHISPER_MODEL="+model+"\n"), 0644); err != nil {
				t.Fatal(err)
			}

			cfg, warning := LoadConfig(envPath)

			if cfg.WhisperModel != model {
				t.Errorf("expected WhisperModel=%q, got %q", model, cfg.WhisperModel)
			}
			if warning != "" {
				t.Errorf("expected no warning, got: %s", warning)
			}
		})
	}
}

func TestWhisperModelFilename(t *testing.T) {
	cfg := &AppConfig{WhisperModel: "large-v3"}

	if got := cfg.WhisperModelFilename(); got != "ggml-large-v3.bin" {
		t.Errorf("expected ggml-large-v3.bin, got %q", got)
	}
}

func TestLoadConfigWhisperLang(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := "MODE=dev\nWHISPER_LANG=uk\n"
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, _ := LoadConfig(envPath)

	if cfg.WhisperLang != "uk" {
		t.Errorf("expected WhisperLang='uk', got '%s'", cfg.WhisperLang)
	}
}

func TestLoadConfigWhisperLangDefault(t *testing.T) {
	cfg, _ := LoadConfig("/nonexistent/.env")

	if cfg.WhisperLang != "auto" {
		t.Errorf("expected WhisperLang='auto', got '%s'", cfg.WhisperLang)
	}
}

func TestLoadConfigWhisperNoGPU(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := "MODE=dev\nWHISPER_NO_GPU=true\n"
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, _ := LoadConfig(envPath)

	if !cfg.WhisperNoGPU {
		t.Error("expected WhisperNoGPU=true")
	}
}

func TestLoadConfigWhisperNoGPUDefault(t *testing.T) {
	cfg, _ := LoadConfig("/nonexistent/.env")

	if cfg.WhisperNoGPU {
		t.Error("expected WhisperNoGPU=false by default")
	}
}

func TestLoadConfigMaxWorkers(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("MAX_WORKERS=6\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, warning := LoadConfig(envPath)

	if cfg.MaxWorkers != 6 {
		t.Errorf("expected MaxWorkers=6, got %d", cfg.MaxWorkers)
	}
	if warning != "" {
		t.Errorf("expected no warning, got: %s", warning)
	}
}

func TestLoadConfigMaxWorkersDefault(t *testing.T) {
	cfg, _ := LoadConfig("/nonexistent/.env")

	if cfg.MaxWorkers != 4 {
		t.Errorf("expected MaxWorkers=4 by default, got %d", cfg.MaxWorkers)
	}
}

func TestLoadConfigMaxWorkersInvalid(t *testing.T) {
	for _, value := range []string{"abc", "0", "-2"} {
		t.Run(value, func(t *testing.T) {
			dir := t.TempDir()
			envPath := filepath.Join(dir, ".env")
			if err := os.WriteFile(envPath, []byte("MAX_WORKERS="+value+"\n"), 0644); err != nil {
				t.Fatal(err)
			}

			cfg, warning := LoadConfig(envPath)

			if cfg.MaxWorkers != 4 {
				t.Errorf("expected fallback MaxWorkers=4, got %d", cfg.MaxWorkers)
			}
			if warning == "" {
				t.Errorf("expected a warning for invalid MAX_WORKERS=%q", value)
			}
		})
	}
}

func TestLoadConfigDiarizeDefault(t *testing.T) {
	cfg, _ := LoadConfig("/nonexistent/.env")

	if cfg.Diarize {
		t.Error("expected Diarize=false by default")
	}
	if cfg.DiarizeSpeakers != 0 {
		t.Errorf("expected DiarizeSpeakers=0 by default, got %d", cfg.DiarizeSpeakers)
	}
	if cfg.DiarizeLabel != "speaker-" {
		t.Errorf("expected DiarizeLabel='speaker-' by default, got '%s'", cfg.DiarizeLabel)
	}
}

func TestLoadConfigDiarizeEnabled(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("DIARIZE=true\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, warning := LoadConfig(envPath)

	if !cfg.Diarize {
		t.Error("expected Diarize=true")
	}
	if warning != "" {
		t.Errorf("expected no warning, got: %s", warning)
	}
}

func TestLoadConfigDiarizeGarbage(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("DIARIZE=maybe\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, _ := LoadConfig(envPath)

	if cfg.Diarize {
		t.Error("expected Diarize=false for a non-truthy value")
	}
}

func TestLoadConfigDiarizeSpeakers(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("DIARIZE_SPEAKERS=3\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, warning := LoadConfig(envPath)

	if cfg.DiarizeSpeakers != 3 {
		t.Errorf("expected DiarizeSpeakers=3, got %d", cfg.DiarizeSpeakers)
	}
	if warning != "" {
		t.Errorf("expected no warning, got: %s", warning)
	}
}

func TestLoadConfigDiarizeSpeakersInvalid(t *testing.T) {
	for _, value := range []string{"abc", "-1"} {
		t.Run(value, func(t *testing.T) {
			dir := t.TempDir()
			envPath := filepath.Join(dir, ".env")
			if err := os.WriteFile(envPath, []byte("DIARIZE_SPEAKERS="+value+"\n"), 0644); err != nil {
				t.Fatal(err)
			}

			cfg, warning := LoadConfig(envPath)

			if cfg.DiarizeSpeakers != 0 {
				t.Errorf("expected fallback DiarizeSpeakers=0, got %d", cfg.DiarizeSpeakers)
			}
			if warning == "" {
				t.Errorf("expected a warning for invalid DIARIZE_SPEAKERS=%q", value)
			}
		})
	}
}

func TestLoadConfigDiarizeEngineDefault(t *testing.T) {
	cfg, _ := LoadConfig("/nonexistent/.env")

	if cfg.DiarizeEngine != "sherpa" {
		t.Errorf("expected DiarizeEngine='sherpa' by default, got '%s'", cfg.DiarizeEngine)
	}
	if cfg.DiarizePython != "python3" {
		t.Errorf("expected DiarizePython='python3' by default, got '%s'", cfg.DiarizePython)
	}
}

func TestLoadConfigDiarizeEnginePyannote(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("DIARIZE_ENGINE=pyannote\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, warning := LoadConfig(envPath)

	if cfg.DiarizeEngine != "pyannote" {
		t.Errorf("expected DiarizeEngine='pyannote', got '%s'", cfg.DiarizeEngine)
	}
	if warning != "" {
		t.Errorf("expected no warning, got: %s", warning)
	}
}

func TestLoadConfigDiarizeEngineInvalid(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("DIARIZE_ENGINE=whisperx\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, warning := LoadConfig(envPath)

	if cfg.DiarizeEngine != "sherpa" {
		t.Errorf("expected fallback DiarizeEngine='sherpa', got '%s'", cfg.DiarizeEngine)
	}
	if warning == "" {
		t.Error("expected a warning for an unknown DIARIZE_ENGINE value")
	}
}

func TestLoadConfigDiarizePython(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("DIARIZE_PYTHON=/opt/venv/bin/python3\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, warning := LoadConfig(envPath)

	if cfg.DiarizePython != "/opt/venv/bin/python3" {
		t.Errorf("expected DiarizePython='/opt/venv/bin/python3', got '%s'", cfg.DiarizePython)
	}
	if warning != "" {
		t.Errorf("expected no warning, got: %s", warning)
	}
}

func TestLoadConfigDiarizePythonBlankKeepsDefault(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("DIARIZE_PYTHON=  \n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, _ := LoadConfig(envPath)

	if cfg.DiarizePython != "python3" {
		t.Errorf("expected DiarizePython='python3' for a blank value, got '%s'", cfg.DiarizePython)
	}
}

func TestLoadConfigDiarizeThreshold(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("DIARIZE_THRESHOLD=0.3\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, warning := LoadConfig(envPath)

	if cfg.DiarizeThreshold != 0.3 {
		t.Errorf("expected DiarizeThreshold=0.3, got %v", cfg.DiarizeThreshold)
	}
	if warning != "" {
		t.Errorf("expected no warning, got: %s", warning)
	}
}

func TestLoadConfigDiarizeThresholdDefault(t *testing.T) {
	cfg, _ := LoadConfig("/nonexistent/.env")

	if cfg.DiarizeThreshold != 0 {
		t.Errorf("expected DiarizeThreshold=0 by default, got %v", cfg.DiarizeThreshold)
	}
}

func TestLoadConfigDiarizeThresholdInvalid(t *testing.T) {
	for _, value := range []string{"abc", "-0.5", "0"} {
		t.Run(value, func(t *testing.T) {
			dir := t.TempDir()
			envPath := filepath.Join(dir, ".env")
			if err := os.WriteFile(envPath, []byte("DIARIZE_THRESHOLD="+value+"\n"), 0644); err != nil {
				t.Fatal(err)
			}

			cfg, warning := LoadConfig(envPath)

			if cfg.DiarizeThreshold != 0 {
				t.Errorf("expected fallback DiarizeThreshold=0, got %v", cfg.DiarizeThreshold)
			}
			if warning == "" {
				t.Errorf("expected a warning for invalid DIARIZE_THRESHOLD=%q", value)
			}
		})
	}
}

func TestLoadConfigDiarizeLabel(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("DIARIZE_LABEL=Speaker\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, warning := LoadConfig(envPath)

	if cfg.DiarizeLabel != "Speaker" {
		t.Errorf("expected DiarizeLabel='Speaker', got '%s'", cfg.DiarizeLabel)
	}
	if warning != "" {
		t.Errorf("expected no warning, got: %s", warning)
	}
}

func TestLoadConfigLogPathDefault(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("MODE=dev\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, _ := LoadConfig(envPath)

	if cfg.LogPath != "tmp/dev.log" {
		t.Errorf("expected LogPath='tmp/dev.log' by default, got '%s'", cfg.LogPath)
	}
}
