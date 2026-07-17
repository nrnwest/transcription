// Package diarizer is responsible for speaker diarization of audio files.
// It defines the Diarizer interface to abstract away the sherpa-onnx CLI,
// which allows testing the pipeline without the binary and its models.
package diarizer

import (
	"fmt"
	"io"
)

// CommandRunner is a function for running external commands (sherpa-onnx).
// It allows substituting the real exec with a mock in tests.
type CommandRunner func(name string, args ...string) ([]byte, error)

// Segment is one speaker turn: who speaks between Start and End (seconds).
type Segment struct {
	Start   float64
	End     float64
	Speaker int
}

// Diarizer is the interface for detecting speaker turns in an audio file.
type Diarizer interface {
	Diarize(wavPath string) ([]Segment, error)
}

// DefaultBinary is the sherpa-onnx speaker diarization executable name.
const DefaultBinary = "sherpa-onnx-offline-speaker-diarization"

// SherpaDiarizer is a Diarizer implementation using the sherpa-onnx CLI.
// It uses a CommandRunner to run the binary, which enables testing without it.
type SherpaDiarizer struct {
	RunCmd CommandRunner
	// Binary is the sherpa executable (if empty — DefaultBinary from PATH).
	Binary string
	// SegmentationModel is the path to the pyannote segmentation ONNX model.
	SegmentationModel string
	// EmbeddingModel is the path to the speaker embedding ONNX model.
	EmbeddingModel string
	// NumSpeakers is the expected number of speakers (0 = auto-detect).
	NumSpeakers int
	// Threshold is the clustering threshold for auto-detection (0 = sherpa
	// default). Lower values split voices more aggressively. Ignored when
	// NumSpeakers is set — a fixed cluster count takes precedence.
	Threshold float64
	// ConsoleOutput, if not nil, receives a copy of the sherpa output.
	ConsoleOutput io.Writer
}

// Diarize runs sherpa-onnx on a .wav file and returns speaker segments
// parsed from its stdout.
func (d *SherpaDiarizer) Diarize(wavPath string) ([]Segment, error) {
	binary := d.Binary
	if binary == "" {
		binary = DefaultBinary
	}

	args := []string{
		fmt.Sprintf("--segmentation.pyannote-model=%s", d.SegmentationModel),
		fmt.Sprintf("--embedding.model=%s", d.EmbeddingModel),
	}
	if d.NumSpeakers > 0 {
		args = append(args, fmt.Sprintf("--clustering.num-clusters=%d", d.NumSpeakers))
	} else if d.Threshold > 0 {
		args = append(args, fmt.Sprintf("--clustering.cluster-threshold=%g", d.Threshold))
	}
	args = append(args, wavPath)

	output, err := d.RunCmd(binary, args...)

	// Echo sherpa's raw output only when a sink is provided (dev mode).
	if d.ConsoleOutput != nil && len(output) > 0 {
		fmt.Fprintf(d.ConsoleOutput, "%s\n", output)
	}

	if err != nil {
		return nil, fmt.Errorf("diarization: %w", err)
	}

	return ParseSegments(output), nil
}
