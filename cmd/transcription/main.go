// Package main — entry point of the CLI utility for transcribing video files.
// The program takes a directory of videos, extracts audio via ffmpeg,
// transcribes via whisper and creates an .md file with the results.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"transcription/internal/config"
	"transcription/internal/diarizer"
	"transcription/internal/extractor"
	"transcription/internal/logger"
	"transcription/internal/pipeline"
	"transcription/internal/scanner"
	"transcription/internal/transcriber"
	"transcription/internal/writer"
)

// Exit codes — program termination codes.
const (
	exitSuccess      = 0 // full success — all files processed
	exitBadParams    = 1 // invalid command-line parameters
	exitMissingDeps  = 2 // missing external dependencies (ffmpeg, whisper)
	exitNoVideoFiles = 3 // no video files in the input directory
	exitAllFailed    = 4 // all files failed
	exitPartialFail  = 5 // some files processed, some not
	exitWriteError   = 6 // error writing the output file
)

// realCommandRunner wraps exec.Command to invoke external commands (ffmpeg, whisper).
func realCommandRunner(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// main delegates to run(), which returns an exit code testable without os.Exit().
func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// envPathKey overrides the path to the .env file (used in tests).
const envPathKey = "TRANSCRIPTION_ENV_PATH"

// run encapsulates the CLI logic and returns an exit code.
func run(args []string, stdout, stderr io.Writer) int {
	programStart := time.Now()

	// .env lookup priority: TRANSCRIPTION_ENV_PATH (override for tests) → ./.env.
	envPath := ".env"
	if p := os.Getenv(envPathKey); p != "" {
		envPath = p
	}

	cfg, warning := config.LoadConfig(envPath)
	if warning != "" {
		fmt.Fprintf(stderr, "Warning: %s\n", warning)
	}

	// If LogPath is relative — bind it to the binary's directory, not to cwd.
	// Otherwise `go test` (cwd=package) and a real run (cwd=where launched from)
	// create the log in different places.
	cfg.LogPath = resolveLogPath(cfg.LogPath)

	// dev mode: writes both to file and to stdout
	// prod mode: only to file
	var log *logger.Logger
	log, logErr := logger.NewLoggerFromConfig(cfg.Mode, cfg.LogPath, stdout)
	if logErr != nil {
		if cfg.Mode == "dev" {
			// In dev mode a logger error is critical, since the developer expects logs
			fmt.Fprintf(stderr, "Error: failed to create logger: %v\n", logErr)
			return exitWriteError
		}
		// In prod mode we continue without the logger, only warning
		fmt.Fprintf(stderr, "Warning: logger unavailable: %v\n", logErr)
		log = nil
	}

	// We support flags and 1-2 positional arguments:
	//   transcription [-lang uk] [-diarize] [-speakers N] [-threshold X] <input-dir> [output-dir]
	lang := cfg.WhisperLang // default language from .env
	diarize := cfg.Diarize
	speakers := cfg.DiarizeSpeakers
	threshold := cfg.DiarizeThreshold
	engine := cfg.DiarizeEngine
	positional := []string{}

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "-lang" && i+1 < len(args):
			lang = args[i+1]
			i++
		case args[i] == "-diarize":
			diarize = true
		case args[i] == "-speakers" && i+1 < len(args):
			n, convErr := strconv.Atoi(args[i+1])
			if convErr != nil || n < 1 {
				fmt.Fprintf(stderr, "Error: invalid -speakers value %q — must be a positive integer\n", args[i+1])
				return exitBadParams
			}
			speakers = n
			i++
		case args[i] == "-threshold" && i+1 < len(args):
			f, convErr := strconv.ParseFloat(args[i+1], 64)
			if convErr != nil || f <= 0 {
				fmt.Fprintf(stderr, "Error: invalid -threshold value %q — must be a positive number (e.g. 0.3)\n", args[i+1])
				return exitBadParams
			}
			threshold = f
			i++
		case args[i] == "-engine" && i+1 < len(args):
			engine = args[i+1]
			i++
		default:
			positional = append(positional, args[i])
		}
	}

	if len(positional) < 1 || len(positional) > 2 {
		fmt.Fprintln(stderr, "Error: invalid number of arguments")
		fmt.Fprintln(stderr, "Usage: transcription [-lang uk|ru|auto] [-diarize] [-speakers N] [-threshold X] [-engine sherpa|pyannote] <input-directory> [output-directory]")
		return exitBadParams
	}

	if engine != "sherpa" && engine != "pyannote" {
		fmt.Fprintf(stderr, "Error: invalid -engine value %q — must be 'sherpa' or 'pyannote'\n", engine)
		return exitBadParams
	}

	// pyannote's pipeline clusters speakers itself — the sherpa threshold
	// would be silently meaningless, so say it out loud.
	if diarize && engine == "pyannote" && threshold > 0 {
		fmt.Fprintln(stderr, "Warning: -threshold/DIARIZE_THRESHOLD is ignored by the pyannote engine")
	}

	// Pre-flight: reject an invalid language code BEFORE scanning and the pipeline.
	// Otherwise the pipeline starts ffmpeg on the first 3 files (5-10s each) before
	// whisper gets a chance to fail with 'unknown language'. Here — an instant fail.
	if !transcriber.IsValidLang(lang) {
		fmt.Fprintf(stderr, "Error: invalid language code %q — use ISO 639-1 codes like 'uk' (Ukrainian), 'en', 'ru', 'de', or 'auto'\n", lang)
		return exitBadParams
	}

	inputDir := positional[0]
	// Default: write the .md into the same directory being scanned.
	// .env (OUTPUT_DIR) and the second CLI argument override this default.
	outputDir := inputDir
	if cfg.OutputDir != "" {
		outputDir = cfg.OutputDir
	}

	if len(positional) == 2 {
		outputDir = positional[1]
	}

	inputDir, err := filepath.Abs(inputDir)
	if err != nil {
		fmt.Fprintf(stderr, "Error: invalid path: %v\n", err)
		return exitBadParams
	}

	outputDir, err = filepath.Abs(outputDir)
	if err != nil {
		fmt.Fprintf(stderr, "Error: invalid output path: %v\n", err)
		return exitBadParams
	}

	info, err := os.Stat(inputDir)
	if os.IsNotExist(err) {
		fmt.Fprintf(stderr, "Error: directory not found: %s\n", inputDir)
		return exitBadParams
	}
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return exitBadParams
	}
	if !info.IsDir() {
		fmt.Fprintln(stderr, "Error: expected a directory, got a file")
		return exitBadParams
	}

	// ffmpeg is needed to extract audio, whisper — for transcription.
	// We check for their presence before processing starts, to avoid wasting time.
	ffmpegPath, err := newDependencyManager().ensureFFmpeg()
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return exitMissingDeps
	}
	whisperPath, err := resolveWhisper()
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return exitMissingDeps
	}

	// Diarization dependencies are checked only when the feature is on:
	// they are optional and must not affect plain runs. Each engine has its
	// own set — sherpa needs the binary + two ONNX models, pyannote needs a
	// Python interpreter with pyannote.audio installed.
	var sherpaPath, segModel, embModel, pythonPath string
	if diarize {
		switch engine {
		case "pyannote":
			pythonPath, err = resolvePython(cfg.DiarizePython)
			if err != nil {
				fmt.Fprintf(stderr, "Error: %v\n", err)
				return exitMissingDeps
			}
			if err := checkPyannote(realCommandRunner, pythonPath); err != nil {
				fmt.Fprintf(stderr, "Error: %v\n", err)
				return exitMissingDeps
			}
		default: // sherpa
			sherpaPath, err = resolveSherpa()
			if err != nil {
				fmt.Fprintf(stderr, "Error: %v\n", err)
				return exitMissingDeps
			}
			segModel, embModel, err = findDiarizationModels()
			if err != nil {
				fmt.Fprintf(stderr, "Error: %v\n", err)
				return exitMissingDeps
			}
		}
	}

	var scanTimer *logger.StepTimer
	if log != nil {
		scanTimer = logger.NewStep(log, "scanning", inputDir)
	}

	files, err := scanner.ScanDir(inputDir)
	if err != nil {
		if scanTimer != nil {
			scanTimer.Fail(err)
		}
		fmt.Fprintf(stderr, "Scan error: %v\n", err)
		return exitBadParams
	}

	if scanTimer != nil {
		scanTimer.Done()
	}

	if len(files) == 0 {
		fmt.Fprintln(stderr, "Error: no video files in the directory")
		return exitNoVideoFiles
	}

	// The model selected in .env must reside in the local models directory.
	modelPath, err := findWhisperModel(cfg.WhisperModelFilename())
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return exitMissingDeps
	}

	if log != nil {
		log.Info(fmt.Sprintf("model: %s (%s), language: %s, no_gpu: %t", cfg.WhisperModel, modelPath, lang, cfg.WhisperNoGPU))
	}

	ext := &extractor.FfmpegExtractor{
		RunCmd: realCommandRunner,
		Binary: ffmpegPath,
	}
	// Echo whisper's own (verbose) output only in dev mode; prod keeps the
	// console clean, showing just the pipeline progress.
	var whisperConsole io.Writer
	if cfg.Mode == "dev" {
		whisperConsole = stderr
	}
	tr := &transcriber.WhisperTranscriber{
		RunCmd:        realCommandRunner,
		Binary:        whisperPath,
		ModelPath:     modelPath,
		Lang:          lang,
		NoGPU:         cfg.WhisperNoGPU,
		ConsoleOutput: whisperConsole,
	}

	p := pipeline.NewPipeline(ext, tr)
	p.MaxWorkers = cfg.MaxWorkers
	p.ProgressWriter = stdout
	p.ErrorWriter = stderr
	p.Logger = log

	if diarize {
		switch engine {
		case "pyannote":
			scriptPath, err := diarizer.WriteScript("")
			if err != nil {
				fmt.Fprintf(stderr, "Error: %v\n", err)
				return exitWriteError
			}
			defer os.Remove(scriptPath)

			modelDir := findPyannoteModelDir()
			p.Diarizer = &diarizer.PyannoteDiarizer{
				RunCmd:        realCommandRunner,
				Python:        pythonPath,
				ScriptPath:    scriptPath,
				NumSpeakers:   speakers,
				ModelDir:      modelDir,
				ConsoleOutput: whisperConsole,
			}
			if log != nil {
				source := modelDir
				if source == "" {
					source = "huggingface hub"
				}
				log.Info(fmt.Sprintf("diarization: on (engine: pyannote, speakers hint: %d, python: %s, models: %s)", speakers, pythonPath, source))
			}
		default: // sherpa
			p.Diarizer = &diarizer.SherpaDiarizer{
				RunCmd:            realCommandRunner,
				Binary:            sherpaPath,
				SegmentationModel: segModel,
				EmbeddingModel:    embModel,
				NumSpeakers:       speakers,
				Threshold:         threshold,
				ConsoleOutput:     whisperConsole,
			}
			if log != nil {
				log.Info(fmt.Sprintf("diarization: on (engine: sherpa, speakers hint: %d, threshold: %g, models: %s)", speakers, threshold, filepath.Dir(segModel)))
			}
		}
		p.SpeakerLabel = cfg.DiarizeLabel
	}

	// dev mode: stop processing after the first error for fast diagnostics
	if cfg.Mode == "dev" {
		p.StopOnError = true
	}

	results := p.Run(files)

	successCount := 0
	var failedFiles []string
	for _, r := range results {
		if r.Success {
			successCount++
		} else if r.Error != nil {
			failedFiles = append(failedFiles, filepath.Base(r.Video.Path))
		}
	}

	// If all files failed — there is nothing to write.
	// In dev mode with StopOnError we also return exitAllFailed if there are errors.
	if successCount == 0 {
		fmt.Fprintln(stderr, "Error: all files failed processing")
		return exitAllFailed
	}

	// dev mode with StopOnError: if there are errors — don't write the .md file
	if cfg.Mode == "dev" && p.StopOnError && len(failedFiles) > 0 {
		fmt.Fprintln(stderr, "Error: processing stopped (dev mode)")
		for _, f := range failedFiles {
			fmt.Fprintf(stderr, "  - %s\n", f)
		}
		return exitAllFailed
	}

	dirName := filepath.Base(inputDir)
	sanitized := writer.SanitizeFilename(dirName)
	mdFilename := sanitized + ".md"

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Fprintf(stderr, "Error creating directory: %v\n", err)
		return exitWriteError
	}

	var writeTimer *logger.StepTimer
	if log != nil {
		writeTimer = logger.NewStep(log, "writing", mdFilename)
	}

	// Generate and write one or more Markdown documents.
	// The split happens at section boundaries when the word limit is exceeded.
	mdPath := filepath.Join(outputDir, mdFilename)
	documents := writer.BuildDocuments(dirName, mdPath, results)

	if err := writer.WriteDocuments(documents); err != nil {
		if writeTimer != nil {
			writeTimer.Fail(err)
		}
		fmt.Fprintf(stderr, "Error writing file: %v\n", err)
		return exitWriteError
	}

	if writeTimer != nil {
		writeTimer.Done()
	}

	if len(documents) == 1 {
		fmt.Fprintf(stdout, "Result: %s (%d of %d files)\n", mdPath, successCount, len(results))
	} else {
		fmt.Fprintf(stdout, "Result: %d Markdown files (%d of %d files):\n", len(documents), successCount, len(results))
		for _, document := range documents {
			fmt.Fprintf(stdout, "  - %s\n", document.Path)
		}
	}

	if log != nil {
		totalDuration := time.Since(programStart)
		log.Info(fmt.Sprintf("DONE program: %d files (%.2fs)", len(files), totalDuration.Seconds()))
	}

	if len(failedFiles) > 0 {
		fmt.Fprintln(stderr, "Files with errors:")
		for _, f := range failedFiles {
			fmt.Fprintf(stderr, "  - %s\n", f)
		}
		return exitPartialFail
	}

	return exitSuccess
}

// resolveLogPath makes a relative log path absolute against the binary's directory.
func resolveLogPath(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	exe, err := os.Executable()
	if err != nil {
		return p
	}
	return filepath.Join(filepath.Dir(exe), p)
}

// findWhisperModel looks for the model in ./models next to the binary or in the cwd.
func findWhisperModel(filename string) (string, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not determine the program path: %w", err)
	}

	workingDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("could not determine the working directory: %w", err)
	}

	return findWhisperModelAt(filename, executablePath, workingDir)
}

func findWhisperModelAt(filename, executablePath, workingDir string) (string, error) {
	candidates := []string{
		filepath.Join(filepath.Dir(executablePath), "models", filename),
		filepath.Join(workingDir, "models", filename),
	}

	for _, candidate := range candidates {
		abs, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		info, err := os.Stat(abs)
		if err == nil && !info.IsDir() {
			return abs, nil
		}
	}

	return "", fmt.Errorf(
		"model %s not found in the models directory next to the program or in the current directory",
		filename,
	)
}

// findDiarizationModels looks for the two sherpa-onnx models in
// ./models/diarization next to the binary or in the cwd.
func findDiarizationModels() (segmentation, embedding string, err error) {
	executablePath, err := os.Executable()
	if err != nil {
		return "", "", fmt.Errorf("could not determine the program path: %w", err)
	}

	workingDir, err := os.Getwd()
	if err != nil {
		return "", "", fmt.Errorf("could not determine the working directory: %w", err)
	}

	return findDiarizationModelsAt(executablePath, workingDir)
}

func findDiarizationModelsAt(executablePath, workingDir string) (string, string, error) {
	roots := []string{
		filepath.Join(filepath.Dir(executablePath), "models", "diarization"),
		filepath.Join(workingDir, "models", "diarization"),
	}

	// Both models must live in the SAME directory — mixing locations would
	// make troubleshooting confusing.
	var lastMissing string
	for _, root := range roots {
		segmentation := filepath.Join(root, "segmentation.onnx")
		embedding := filepath.Join(root, "embedding.onnx")

		if !fileExists(segmentation) {
			lastMissing = segmentation
			continue
		}
		if !fileExists(embedding) {
			lastMissing = embedding
			continue
		}
		return segmentation, embedding, nil
	}

	return "", "", fmt.Errorf(
		"diarization model %s not found — put segmentation.onnx and embedding.onnx into models/diarization",
		lastMissing,
	)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// findPyannoteModelDir looks for a local pyannote pipeline directory in
// ./models/diarization-pyannote next to the binary or in the cwd.
// An empty result is not an error: the pyannote script then falls back to
// the HuggingFace hub (cache/token).
func findPyannoteModelDir() string {
	executablePath, err := os.Executable()
	if err != nil {
		return ""
	}
	workingDir, err := os.Getwd()
	if err != nil {
		return ""
	}
	return findPyannoteModelDirAt(executablePath, workingDir)
}

func findPyannoteModelDirAt(executablePath, workingDir string) string {
	candidates := []string{
		filepath.Join(filepath.Dir(executablePath), "models", "diarization-pyannote"),
		filepath.Join(workingDir, "models", "diarization-pyannote"),
	}
	for _, dir := range candidates {
		// config.yaml is the pipeline entry point — without it the dir is unusable.
		if fileExists(filepath.Join(dir, "config.yaml")) {
			return dir
		}
	}
	return ""
}

// resolveWhisper resolves the whisper binary in PATH (whisper-cli, then legacy whisper).
func resolveWhisper() (string, error) {
	if p, err := exec.LookPath("whisper-cli"); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("whisper"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf(
		`whisper-cli not found in PATH — see the "Installation" section in the README`,
	)
}
