// Package config is responsible for loading configuration from the .env file.
// Supports the KEY=VALUE format, ignoring comments and empty lines.
// Used to determine the operating mode (dev/prod) and the log file path.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const defaultWhisperModel = "medium"

// defaultMaxWorkers is the default number of files processed in parallel.
// 4 balances throughput against ffmpeg/whisper being CPU-bound (see pipeline).
const defaultMaxWorkers = 4

// defaultDiarizeLabel is the speaker label prefix in the output markdown
// ("speaker-" → "**speaker-1:**"). Overridable via DIARIZE_LABEL (e.g. "актор").
const defaultDiarizeLabel = "speaker-"

// defaultDiarizeEngine is the diarization backend used unless overridden.
const defaultDiarizeEngine = "sherpa"

// defaultDiarizePython is the interpreter for the pyannote engine.
const defaultDiarizePython = "python3"

var validDiarizeEngines = map[string]bool{
	"sherpa":   true,
	"pyannote": true,
}

var whisperModelFiles = map[string]string{
	"tiny":     "ggml-tiny.bin",
	"small":    "ggml-small.bin",
	"medium":   "ggml-medium.bin",
	"large-v3": "ggml-large-v3.bin",
}

// AppConfig — the application configuration loaded from the .env file.
type AppConfig struct {
	Mode         string
	LogPath      string
	OutputDir    string
	WhisperModel string
	WhisperLang  string
	WhisperNoGPU bool
	MaxWorkers   int
	// Diarize enables speaker diarization via the sherpa-onnx CLI.
	Diarize bool
	// DiarizeSpeakers is a hint for the number of speakers (0 = auto-detect).
	DiarizeSpeakers int
	// DiarizeLabel is the speaker label prefix in the output ("актор" → "speaker-1:").
	DiarizeLabel string
	// DiarizeThreshold is the sherpa clustering threshold (0 = sherpa default).
	// Lower values split voices more aggressively; used only when speakers = 0.
	// Ignored by the pyannote engine.
	DiarizeThreshold float64
	// DiarizeEngine selects the diarization backend: "sherpa" or "pyannote".
	DiarizeEngine string
	// DiarizePython is the Python interpreter for the pyannote engine
	// (may point into a virtualenv).
	DiarizePython string
}

// WhisperModelFilename returns the filename of the active model in models.
func (c *AppConfig) WhisperModelFilename() string {
	return whisperModelFiles[c.WhisperModel]
}

var validModes = map[string]bool{
	"dev":  true,
	"prod": true,
}

// LoadConfig loads configuration from the .env file at the given path.
// If the file does not exist, it returns the default configuration without a warning.
// If MODE has an unknown value, it sets "prod" and returns a warning.
// The second return value is the warning string (empty if everything is fine).
func LoadConfig(path string) (*AppConfig, string) {
	cfg := &AppConfig{
		Mode:          "prod",
		LogPath:       "tmp/dev.log",
		OutputDir:     "",
		WhisperModel:  defaultWhisperModel,
		WhisperLang:   "auto",
		WhisperNoGPU:  false,
		MaxWorkers:    defaultMaxWorkers,
		Diarize:       false,
		DiarizeLabel:  defaultDiarizeLabel,
		DiarizeEngine: defaultDiarizeEngine,
		DiarizePython: defaultDiarizePython,
	}

	// Raw MAX_WORKERS / DIARIZE_SPEAKERS values, validated after parsing so an
	// invalid entry falls back to the default with a warning instead of a silent zero.
	maxWorkersRaw := ""
	diarizeSpeakersRaw := ""
	diarizeThresholdRaw := ""

	// .env may be absent in a prod environment — return defaults without an error.
	file, err := os.Open(path)
	if err != nil {
		return cfg, ""
	}
	defer file.Close()

	s := bufio.NewScanner(file)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// SplitN with n=2 splits only on the first '=',
		// so the value can contain the '=' character
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		switch key {
		case "MODE":
			cfg.Mode = value
		case "LOG_PATH":
			cfg.LogPath = value
		case "OUTPUT_DIR":
			cfg.OutputDir = value
		case "WHISPER_MODEL":
			cfg.WhisperModel = value
		case "WHISPER_LANG":
			cfg.WhisperLang = value
		case "WHISPER_NO_GPU":
			cfg.WhisperNoGPU = parseBool(value)
		case "MAX_WORKERS":
			maxWorkersRaw = value
		case "DIARIZE":
			cfg.Diarize = parseBool(value)
		case "DIARIZE_SPEAKERS":
			diarizeSpeakersRaw = value
		case "DIARIZE_THRESHOLD":
			diarizeThresholdRaw = value
		case "DIARIZE_LABEL":
			if v := strings.TrimSpace(value); v != "" {
				cfg.DiarizeLabel = v
			}
		case "DIARIZE_ENGINE":
			if v := strings.TrimSpace(value); v != "" {
				cfg.DiarizeEngine = v
			}
		case "DIARIZE_PYTHON":
			if v := strings.TrimSpace(value); v != "" {
				cfg.DiarizePython = v
			}
		}
	}

	var warnings []string

	if !validModes[cfg.Mode] {
		warnings = append(warnings, fmt.Sprintf("unknown MODE value '%s', using 'prod'", cfg.Mode))
		cfg.Mode = "prod"
	}

	if maxWorkersRaw != "" {
		n, err := strconv.Atoi(strings.TrimSpace(maxWorkersRaw))
		if err != nil || n < 1 {
			warnings = append(warnings, fmt.Sprintf(
				"invalid MAX_WORKERS value '%s', must be a positive integer; using %d",
				maxWorkersRaw, defaultMaxWorkers,
			))
			cfg.MaxWorkers = defaultMaxWorkers
		} else {
			cfg.MaxWorkers = n
		}
	}

	if diarizeSpeakersRaw != "" {
		n, err := strconv.Atoi(strings.TrimSpace(diarizeSpeakersRaw))
		if err != nil || n < 0 {
			warnings = append(warnings, fmt.Sprintf(
				"invalid DIARIZE_SPEAKERS value '%s', must be a non-negative integer; using auto-detect",
				diarizeSpeakersRaw,
			))
			cfg.DiarizeSpeakers = 0
		} else {
			cfg.DiarizeSpeakers = n
		}
	}

	if !validDiarizeEngines[cfg.DiarizeEngine] {
		warnings = append(warnings, fmt.Sprintf(
			"unknown DIARIZE_ENGINE value '%s', available: sherpa, pyannote; using '%s'",
			cfg.DiarizeEngine, defaultDiarizeEngine,
		))
		cfg.DiarizeEngine = defaultDiarizeEngine
	}

	if diarizeThresholdRaw != "" {
		f, err := strconv.ParseFloat(strings.TrimSpace(diarizeThresholdRaw), 64)
		if err != nil || f <= 0 {
			warnings = append(warnings, fmt.Sprintf(
				"invalid DIARIZE_THRESHOLD value '%s', must be a positive number; using the sherpa default",
				diarizeThresholdRaw,
			))
			cfg.DiarizeThreshold = 0
		} else {
			cfg.DiarizeThreshold = f
		}
	}

	if _, ok := whisperModelFiles[cfg.WhisperModel]; !ok {
		warnings = append(
			warnings,
			fmt.Sprintf(
				"unknown model '%s', available: tiny, small, medium, large-v3; using '%s'",
				cfg.WhisperModel,
				defaultWhisperModel,
			),
		)
		cfg.WhisperModel = defaultWhisperModel
	}

	return cfg, strings.Join(warnings, "; ")
}

// parseBool parses simple truthy values from .env.
func parseBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}
