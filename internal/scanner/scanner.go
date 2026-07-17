// Package scanner is responsible for finding video files in a directory.
// It finds files with supported extensions and sorts them by modification date.
package scanner

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// VideoFile represents a video file in the input directory.
type VideoFile struct {
	Path      string
	Name      string
	Extension string
	ModTime   time.Time
}

// SupportedExtensions is the list of supported video and audio file extensions
// (case-insensitive). Audio covers common phone/dictaphone recordings
// (m4a, amr, 3gp, ogg, etc.); ffmpeg decodes all of them.
var SupportedExtensions = map[string]bool{
	// video
	"mkv":  true,
	"mp4":  true,
	"avi":  true,
	"webm": true,
	"mov":  true,
	// audio
	"mp3":  true,
	"m4a":  true,
	"aac":  true,
	"wav":  true,
	"flac": true,
	"ogg":  true,
	"opus": true,
	"wma":  true,
	"amr":  true,
	"aiff": true,
	"aif":  true,
	"3gp":  true,
	"caf":  true,
}

// ScanDir returns supported video files in dir, sorted by ModTime (oldest first).
func ScanDir(dir string) ([]VideoFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	// Empty slice instead of nil — important for correct behavior on an empty directory
	result := make([]VideoFile, 0)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		ext := filepath.Ext(name)
		if ext == "" {
			continue
		}
		extLower := strings.ToLower(ext[1:])

		if !SupportedExtensions[extLower] {
			continue
		}

		fullPath := filepath.Join(dir, name)
		info, err := os.Stat(fullPath)
		if err != nil {
			return nil, err
		}

		baseName := strings.TrimSuffix(name, ext)

		result = append(result, VideoFile{
			Path:      fullPath,
			Name:      baseName,
			Extension: extLower,
			ModTime:   info.ModTime(),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ModTime.Before(result[j].ModTime)
	})

	return result, nil
}
