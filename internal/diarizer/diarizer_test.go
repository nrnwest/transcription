package diarizer

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestSherpaDiarizerFormsCorrectCommand(t *testing.T) {
	var capturedName string
	var capturedArgs []string

	mockRunner := func(name string, args ...string) ([]byte, error) {
		capturedName = name
		capturedArgs = args
		return []byte("0.0 -- 1.0 speaker_00\n"), nil
	}

	d := &SherpaDiarizer{
		RunCmd:            mockRunner,
		Binary:            "sherpa-onnx-offline-speaker-diarization",
		SegmentationModel: "/models/diarization/segmentation.onnx",
		EmbeddingModel:    "/models/diarization/embedding.onnx",
	}

	segs, err := d.Diarize("/tmp/talk.wav")
	if err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}
	if len(segs) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segs))
	}

	if capturedName != "sherpa-onnx-offline-speaker-diarization" {
		t.Errorf("unexpected binary: %s", capturedName)
	}
	argsStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsStr, "--segmentation.pyannote-model=/models/diarization/segmentation.onnx") {
		t.Errorf("missing segmentation model flag: %s", argsStr)
	}
	if !strings.Contains(argsStr, "--embedding.model=/models/diarization/embedding.onnx") {
		t.Errorf("missing embedding model flag: %s", argsStr)
	}
	if !strings.Contains(argsStr, "/tmp/talk.wav") {
		t.Errorf("missing wav path: %s", argsStr)
	}
	if strings.Contains(argsStr, "--clustering.num-clusters") {
		t.Errorf("num-clusters must be absent when NumSpeakers is 0: %s", argsStr)
	}
}

func TestSherpaDiarizerNumSpeakers(t *testing.T) {
	var capturedArgs []string

	mockRunner := func(name string, args ...string) ([]byte, error) {
		capturedArgs = args
		return []byte(""), nil
	}

	d := &SherpaDiarizer{RunCmd: mockRunner, NumSpeakers: 2}
	if _, err := d.Diarize("/tmp/talk.wav"); err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}

	argsStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsStr, "--clustering.num-clusters=2") {
		t.Errorf("expected '--clustering.num-clusters=2', got: %s", argsStr)
	}
}

func TestSherpaDiarizerThreshold(t *testing.T) {
	var capturedArgs []string

	mockRunner := func(name string, args ...string) ([]byte, error) {
		capturedArgs = args
		return []byte(""), nil
	}

	d := &SherpaDiarizer{RunCmd: mockRunner, Threshold: 0.3}
	if _, err := d.Diarize("/tmp/talk.wav"); err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}

	argsStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsStr, "--clustering.cluster-threshold=0.3") {
		t.Errorf("expected '--clustering.cluster-threshold=0.3', got: %s", argsStr)
	}
	if strings.Contains(argsStr, "--clustering.num-clusters") {
		t.Errorf("num-clusters must be absent when NumSpeakers is 0: %s", argsStr)
	}
}

func TestSherpaDiarizerNumSpeakersWinsOverThreshold(t *testing.T) {
	var capturedArgs []string

	mockRunner := func(name string, args ...string) ([]byte, error) {
		capturedArgs = args
		return []byte(""), nil
	}

	d := &SherpaDiarizer{RunCmd: mockRunner, NumSpeakers: 2, Threshold: 0.3}
	if _, err := d.Diarize("/tmp/talk.wav"); err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}

	argsStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsStr, "--clustering.num-clusters=2") {
		t.Errorf("expected '--clustering.num-clusters=2', got: %s", argsStr)
	}
	if strings.Contains(argsStr, "--clustering.cluster-threshold") {
		t.Errorf("threshold must be absent when NumSpeakers is set: %s", argsStr)
	}
}

func TestSherpaDiarizerCommandError(t *testing.T) {
	mockRunner := func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("exit status 1")
	}

	d := &SherpaDiarizer{RunCmd: mockRunner}
	_, err := d.Diarize("/tmp/talk.wav")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "diarization") {
		t.Errorf("error must mention diarization, got: %v", err)
	}
}

func TestSherpaDiarizerEchoesConsoleOutput(t *testing.T) {
	mockRunner := func(name string, args ...string) ([]byte, error) {
		return []byte("some sherpa output\n"), nil
	}

	var sink bytes.Buffer
	d := &SherpaDiarizer{RunCmd: mockRunner, ConsoleOutput: &sink}
	if _, err := d.Diarize("/tmp/talk.wav"); err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}

	if !strings.Contains(sink.String(), "some sherpa output") {
		t.Errorf("expected the sherpa output echoed to ConsoleOutput, got: %q", sink.String())
	}
}

func TestSherpaDiarizerImplementsInterface(t *testing.T) {
	mockRunner := func(name string, args ...string) ([]byte, error) {
		return []byte(""), nil
	}

	// Compilation guarantees interface conformance
	var _ Diarizer = &SherpaDiarizer{RunCmd: mockRunner}
}
