package diarizer

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestPyannoteDiarizerFormsCorrectCommand(t *testing.T) {
	var capturedName string
	var capturedArgs []string

	mockRunner := func(name string, args ...string) ([]byte, error) {
		capturedName = name
		capturedArgs = args
		return []byte("0.318 -- 5.121 speaker_00\n"), nil
	}

	d := &PyannoteDiarizer{
		RunCmd:     mockRunner,
		Python:     "/opt/venv/bin/python3",
		ScriptPath: "/tmp/pyannote_diarize.py",
	}

	segs, err := d.Diarize("/tmp/talk.wav")
	if err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}
	if len(segs) != 1 || segs[0].Speaker != 0 {
		t.Fatalf("unexpected segments: %+v", segs)
	}

	if capturedName != "/opt/venv/bin/python3" {
		t.Errorf("unexpected python binary: %s", capturedName)
	}
	if len(capturedArgs) != 2 || capturedArgs[0] != "/tmp/pyannote_diarize.py" || capturedArgs[1] != "/tmp/talk.wav" {
		t.Errorf("expected [script wav] args, got: %v", capturedArgs)
	}
}

func TestPyannoteDiarizerDefaultPython(t *testing.T) {
	var capturedName string

	mockRunner := func(name string, args ...string) ([]byte, error) {
		capturedName = name
		return []byte(""), nil
	}

	d := &PyannoteDiarizer{RunCmd: mockRunner, ScriptPath: "/tmp/s.py"}
	if _, err := d.Diarize("/tmp/talk.wav"); err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}

	if capturedName != "python3" {
		t.Errorf("expected default 'python3', got %s", capturedName)
	}
}

func TestPyannoteDiarizerNumSpeakers(t *testing.T) {
	var capturedArgs []string

	mockRunner := func(name string, args ...string) ([]byte, error) {
		capturedArgs = args
		return []byte(""), nil
	}

	d := &PyannoteDiarizer{RunCmd: mockRunner, ScriptPath: "/tmp/s.py", NumSpeakers: 2}
	if _, err := d.Diarize("/tmp/talk.wav"); err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}

	argsStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsStr, "--num-speakers 2") {
		t.Errorf("expected '--num-speakers 2', got: %s", argsStr)
	}
	if capturedArgs[len(capturedArgs)-1] != "/tmp/talk.wav" {
		t.Errorf("wav path must be the last argument, got: %v", capturedArgs)
	}
}

func TestPyannoteDiarizerModelDir(t *testing.T) {
	var capturedArgs []string

	mockRunner := func(name string, args ...string) ([]byte, error) {
		capturedArgs = args
		return []byte(""), nil
	}

	d := &PyannoteDiarizer{RunCmd: mockRunner, ScriptPath: "/tmp/s.py", ModelDir: "/models/diarization-pyannote"}
	if _, err := d.Diarize("/tmp/talk.wav"); err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}

	argsStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsStr, "--model-dir /models/diarization-pyannote") {
		t.Errorf("expected '--model-dir /models/diarization-pyannote', got: %s", argsStr)
	}
}

func TestPyannoteDiarizerNoModelDirFlagWhenUnset(t *testing.T) {
	var capturedArgs []string

	mockRunner := func(name string, args ...string) ([]byte, error) {
		capturedArgs = args
		return []byte(""), nil
	}

	d := &PyannoteDiarizer{RunCmd: mockRunner, ScriptPath: "/tmp/s.py"}
	if _, err := d.Diarize("/tmp/talk.wav"); err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}

	if strings.Contains(strings.Join(capturedArgs, " "), "--model-dir") {
		t.Errorf("'--model-dir' must be absent when ModelDir is empty, got: %v", capturedArgs)
	}
}

func TestPyannoteDiarizerCommandError(t *testing.T) {
	mockRunner := func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("exit status 4")
	}

	d := &PyannoteDiarizer{RunCmd: mockRunner, ScriptPath: "/tmp/s.py"}
	_, err := d.Diarize("/tmp/talk.wav")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "diarization") {
		t.Errorf("error must mention diarization, got: %v", err)
	}
}

func TestPyannoteDiarizerEchoesConsoleOutput(t *testing.T) {
	mockRunner := func(name string, args ...string) ([]byte, error) {
		return []byte("pyannote progress noise\n"), nil
	}

	var sink bytes.Buffer
	d := &PyannoteDiarizer{RunCmd: mockRunner, ScriptPath: "/tmp/s.py", ConsoleOutput: &sink}
	if _, err := d.Diarize("/tmp/talk.wav"); err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}

	if !strings.Contains(sink.String(), "pyannote progress noise") {
		t.Errorf("expected the output echoed to ConsoleOutput, got: %q", sink.String())
	}
}

func TestPyannoteDiarizerImplementsInterface(t *testing.T) {
	mockRunner := func(name string, args ...string) ([]byte, error) {
		return []byte(""), nil
	}

	var d Diarizer = &PyannoteDiarizer{RunCmd: mockRunner}
	_ = d
}

func TestWriteScript(t *testing.T) {
	dir := t.TempDir()

	path, err := WriteScript(dir)
	if err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read the materialized script: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("materialized script is empty")
	}
	if !strings.Contains(string(data), "speaker-diarization-community-1") {
		t.Error("script must reference the pyannote/speaker-diarization-community-1 pipeline")
	}
	if !strings.Contains(string(data), "speaker-diarization-3.1") {
		t.Error("script must keep the speaker-diarization-3.1 fallback")
	}
	if !strings.HasSuffix(path, ".py") {
		t.Errorf("script path must end with .py, got %s", path)
	}
}
