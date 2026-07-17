// Package extractor is responsible for extracting audio from video files.
// It defines the AudioExtractor interface to abstract away the concrete implementation,
// which allows using a mock in tests without depending on ffmpeg.
package extractor

import (
	"fmt"
	"path/filepath"
	"strings"
)

// CommandRunner is a function for running external commands.
type CommandRunner func(name string, args ...string) ([]byte, error)

// AudioExtractor is the interface for extracting audio from a video file.
type AudioExtractor interface {
	Extract(videoPath string) (wavPath string, err error)
}

// FfmpegExtractor is an AudioExtractor implementation backed by ffmpeg.
type FfmpegExtractor struct {
	RunCmd CommandRunner
	// Binary is the path to ffmpeg. If empty, "ffmpeg" from PATH is used.
	Binary string
}

// Extract extracts audio from a video file using ffmpeg into a .wav file.
func (f *FfmpegExtractor) Extract(videoPath string) (string, error) {
	ext := filepath.Ext(videoPath)
	wavPath := strings.TrimSuffix(videoPath, ext) + ".wav"

	// -y overwrites an existing .wav without prompting — otherwise ffmpeg waits
	// for confirmation and exits with an error in non-interactive mode.
	args := []string{
		"-y",
		"-i", videoPath,
		"-ar", "16000",
		"-ac", "1",
		"-c:a", "pcm_s16le",
		wavPath,
	}

	binary := f.Binary
	if binary == "" {
		binary = "ffmpeg"
	}

	_, err := f.RunCmd(binary, args...)
	if err != nil {
		return "", fmt.Errorf("ffmpeg: %w", err)
	}

	return wavPath, nil
}
