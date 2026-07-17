package scanner

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestVideoFileCreation(t *testing.T) {
	modTime := time.Date(2026, 3, 29, 10, 0, 0, 0, time.UTC)

	vf := VideoFile{
		Path:      "/video/AI-2.mkv",
		Name:      "AI-2",
		Extension: "mkv",
		ModTime:   modTime,
	}

	if vf.Path != "/video/AI-2.mkv" {
		t.Errorf("expected Path = /video/AI-2.mkv, got %s", vf.Path)
	}
	if vf.Name != "AI-2" {
		t.Errorf("expected Name = AI-2, got %s", vf.Name)
	}
	if vf.Extension != "mkv" {
		t.Errorf("expected Extension = mkv, got %s", vf.Extension)
	}
	if !vf.ModTime.Equal(modTime) {
		t.Errorf("expected ModTime = %v, got %v", modTime, vf.ModTime)
	}
}

func TestScanDirFindsVideoFiles(t *testing.T) {
	dir := t.TempDir()

	extensions := []string{
		"mkv", "mp4", "avi", "webm", "mov",
		"mp3", "m4a", "aac", "wav", "flac", "ogg", "opus", "wma", "amr", "aiff", "aif", "3gp", "caf",
	}
	for _, ext := range extensions {
		name := "video." + ext
		err := os.WriteFile(filepath.Join(dir, name), []byte("fake"), 0644)
		if err != nil {
			t.Fatalf("failed to create test file %s: %v", name, err)
		}
	}

	files, err := ScanDir(dir)
	if err != nil {
		t.Fatalf("ScanDir returned an error: %v", err)
	}

	if len(files) != len(extensions) {
		t.Errorf("expected %d files, got %d", len(extensions), len(files))
	}

	foundExts := make(map[string]bool)
	for _, f := range files {
		foundExts[f.Extension] = true
	}
	for _, ext := range extensions {
		if !foundExts[ext] {
			t.Errorf("extension %s not found in results", ext)
		}
	}
}

func TestScanDirFindsAudioFiles(t *testing.T) {
	dir := t.TempDir()

	// Typical phone / dictaphone recording formats.
	audio := []string{"mp3", "m4a", "amr", "ogg", "wav"}
	for _, ext := range audio {
		name := "recording." + ext
		if err := os.WriteFile(filepath.Join(dir, name), []byte("fake"), 0644); err != nil {
			t.Fatalf("failed to create test file %s: %v", name, err)
		}
	}

	files, err := ScanDir(dir)
	if err != nil {
		t.Fatalf("ScanDir returned an error: %v", err)
	}
	if len(files) != len(audio) {
		t.Errorf("expected %d audio files, got %d", len(audio), len(files))
	}
}

func TestScanDirSortsByModTime(t *testing.T) {
	dir := t.TempDir()

	// Creation order: third, first, second — to test the sorting
	files := []struct {
		name    string
		modTime time.Time
	}{
		{"third.mkv", time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)},
		{"first.mp4", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		{"second.avi", time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)},
	}

	for _, f := range files {
		path := filepath.Join(dir, f.name)
		err := os.WriteFile(path, []byte("fake"), 0644)
		if err != nil {
			t.Fatalf("failed to create file %s: %v", f.name, err)
		}
		err = os.Chtimes(path, f.modTime, f.modTime)
		if err != nil {
			t.Fatalf("failed to change file time %s: %v", f.name, err)
		}
	}

	result, err := ScanDir(dir)
	if err != nil {
		t.Fatalf("ScanDir returned an error: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("expected 3 files, got %d", len(result))
	}

	if result[0].Name != "first" {
		t.Errorf("first file must be 'first', got '%s'", result[0].Name)
	}
	if result[1].Name != "second" {
		t.Errorf("second file must be 'second', got '%s'", result[1].Name)
	}
	if result[2].Name != "third" {
		t.Errorf("third file must be 'third', got '%s'", result[2].Name)
	}
}

func TestScanDirIgnoresNonVideoFiles(t *testing.T) {
	dir := t.TempDir()

	nonVideo := []string{"notes.txt", "image.jpg", "doc.pdf", "script.go"}
	for _, name := range nonVideo {
		err := os.WriteFile(filepath.Join(dir, name), []byte("fake"), 0644)
		if err != nil {
			t.Fatalf("failed to create file %s: %v", name, err)
		}
	}
	err := os.WriteFile(filepath.Join(dir, "real.mp4"), []byte("fake"), 0644)
	if err != nil {
		t.Fatalf("failed to create video file: %v", err)
	}

	files, err := ScanDir(dir)
	if err != nil {
		t.Fatalf("ScanDir returned an error: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("expected 1 video file, got %d", len(files))
	}
	if len(files) > 0 && files[0].Extension != "mp4" {
		t.Errorf("expected extension mp4, got %s", files[0].Extension)
	}
}

func TestScanDirEmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	files, err := ScanDir(dir)
	if err != nil {
		t.Fatalf("ScanDir returned an error for an empty directory: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("expected an empty slice, got %d files", len(files))
	}
}

func TestScanDirVideoFileFields(t *testing.T) {
	dir := t.TempDir()

	err := os.WriteFile(filepath.Join(dir, "AI-2.mkv"), []byte("fake"), 0644)
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	files, err := ScanDir(dir)
	if err != nil {
		t.Fatalf("ScanDir returned an error: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	vf := files[0]
	if vf.Name != "AI-2" {
		t.Errorf("expected Name = 'AI-2', got '%s'", vf.Name)
	}
	if vf.Extension != "mkv" {
		t.Errorf("expected Extension = 'mkv', got '%s'", vf.Extension)
	}
	expectedPath := filepath.Join(dir, "AI-2.mkv")
	if vf.Path != expectedPath {
		t.Errorf("expected Path = '%s', got '%s'", expectedPath, vf.Path)
	}
}
