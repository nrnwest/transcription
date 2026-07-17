package diarizer

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"strconv"
)

// pyannoteScript is the embedded Python helper that runs the pyannote 3.1
// pipeline and prints segments in the same line format as sherpa-onnx.
//
//go:embed pyannote_diarize.py
var pyannoteScript []byte

// DefaultPython is the Python interpreter used when Python is not set.
const DefaultPython = "python3"

// WriteScript materializes the embedded pyannote helper script into dir
// ("" = the system temp directory) and returns its path. The caller is
// responsible for removing the file.
func WriteScript(dir string) (string, error) {
	if dir == "" {
		dir = os.TempDir()
	}
	f, err := os.CreateTemp(dir, "pyannote_diarize_*.py")
	if err != nil {
		return "", fmt.Errorf("diarization: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(pyannoteScript); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("diarization: %w", err)
	}
	return f.Name(), nil
}

// PyannoteDiarizer is a Diarizer implementation using the pyannote 3.1
// pipeline via the embedded Python helper script. Unlike SherpaDiarizer it
// has no Threshold: the pyannote pipeline clusters speakers on its own.
type PyannoteDiarizer struct {
	RunCmd CommandRunner
	// Python is the interpreter (if empty — DefaultPython from PATH).
	// May be a path into a virtualenv.
	Python string
	// ScriptPath is the materialized helper script (see WriteScript).
	ScriptPath string
	// NumSpeakers is the expected number of speakers (0 = auto-detect).
	NumSpeakers int
	// ModelDir, if set, is a local pipeline directory (config.yaml + weights)
	// used instead of the HuggingFace hub — no account or token needed.
	ModelDir string
	// ConsoleOutput, if not nil, receives a copy of the script output.
	ConsoleOutput io.Writer
}

// Diarize runs the pyannote helper on a .wav file and returns speaker
// segments parsed from its stdout.
func (d *PyannoteDiarizer) Diarize(wavPath string) ([]Segment, error) {
	python := d.Python
	if python == "" {
		python = DefaultPython
	}

	args := []string{d.ScriptPath}
	if d.NumSpeakers > 0 {
		args = append(args, "--num-speakers", strconv.Itoa(d.NumSpeakers))
	}
	if d.ModelDir != "" {
		args = append(args, "--model-dir", d.ModelDir)
	}
	args = append(args, wavPath)

	output, err := d.RunCmd(python, args...)

	// Echo the raw output only when a sink is provided (dev mode).
	if d.ConsoleOutput != nil && len(output) > 0 {
		fmt.Fprintf(d.ConsoleOutput, "%s\n", output)
	}

	if err != nil {
		return nil, fmt.Errorf("diarization: %w", err)
	}

	return ParseSegments(output), nil
}
