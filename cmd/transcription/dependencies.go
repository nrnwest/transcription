package main

import (
	"fmt"
	"os/exec"
)

type dependencyManager struct {
	lookPath func(string) (string, error)
}

func newDependencyManager() dependencyManager {
	return dependencyManager{
		lookPath: exec.LookPath,
	}
}

// ensureFFmpeg returns the ffmpeg path from PATH, or an error pointing to the README.
func (m dependencyManager) ensureFFmpeg() (string, error) {
	path, err := m.lookPath("ffmpeg")
	if err != nil {
		return "", fmt.Errorf(`ffmpeg not found in PATH — see the "Installation" section in the README`)
	}
	return path, nil
}

// resolveSherpa resolves the sherpa-onnx diarization binary in PATH.
// Called only when diarization is enabled — it is an optional dependency.
func resolveSherpa() (string, error) {
	return resolveSherpaWith(exec.LookPath)
}

func resolveSherpaWith(lookPath func(string) (string, error)) (string, error) {
	path, err := lookPath("sherpa-onnx-offline-speaker-diarization")
	if err != nil {
		return "", fmt.Errorf(
			`sherpa-onnx-offline-speaker-diarization not found in PATH — see the "Speaker diarization" section in the README`,
		)
	}
	return path, nil
}

// resolvePython resolves the Python interpreter for the pyannote engine.
// bin may be a bare name ("python3") or a path into a virtualenv.
func resolvePython(bin string) (string, error) {
	return resolvePythonWith(exec.LookPath, bin)
}

func resolvePythonWith(lookPath func(string) (string, error), bin string) (string, error) {
	path, err := lookPath(bin)
	if err != nil {
		return "", fmt.Errorf(
			`python interpreter %q not found — install Python 3 or set DIARIZE_PYTHON (see the "Speaker diarization" section in the README)`,
			bin,
		)
	}
	return path, nil
}

// pyannotePreflight locates pyannote.audio without importing it (a real
// import pulls in torch and takes seconds; find_spec is near-instant).
const pyannotePreflight = "import importlib.util, sys; sys.exit(0 if importlib.util.find_spec('pyannote.audio') else 1)"

// checkPyannote verifies that pyannote.audio is installed for the interpreter.
func checkPyannote(runCmd func(string, ...string) ([]byte, error), pythonPath string) error {
	if _, err := runCmd(pythonPath, "-c", pyannotePreflight); err != nil {
		return fmt.Errorf(
			`pyannote.audio is not installed for %s — run "pip install pyannote.audio" (see the "Speaker diarization" section in the README)`,
			pythonPath,
		)
	}
	return nil
}
