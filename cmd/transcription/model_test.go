package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindWhisperModelNextToBinary(t *testing.T) {
	root := t.TempDir()
	binaryPath := filepath.Join(root, "transcription")
	modelPath := createTestModel(t, root, "ggml-small.bin")

	got, err := findWhisperModelAt("ggml-small.bin", binaryPath, t.TempDir())

	if err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}
	if got != modelPath {
		t.Errorf("expected %q, got %q", modelPath, got)
	}
}

func TestFindWhisperModelInWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	modelPath := createTestModel(t, root, "ggml-tiny.bin")

	got, err := findWhisperModelAt(
		"ggml-tiny.bin",
		filepath.Join(t.TempDir(), "transcription"),
		root,
	)

	if err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}
	if got != modelPath {
		t.Errorf("expected %q, got %q", modelPath, got)
	}
}

func TestFindWhisperModelMissing(t *testing.T) {
	_, err := findWhisperModelAt(
		"ggml-large-v3.bin",
		filepath.Join(t.TempDir(), "transcription"),
		t.TempDir(),
	)

	if err == nil {
		t.Fatal("expected an error for a missing model")
	}
	if !strings.Contains(err.Error(), "ggml-large-v3.bin") {
		t.Errorf("error should contain the file name: %v", err)
	}
}

func createTestModel(t *testing.T, root, name string) string {
	t.Helper()
	modelDir := filepath.Join(root, "models")
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(modelDir, name)
	if err := os.WriteFile(path, []byte("model"), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFindDiarizationModelsNextToBinary(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "models", "diarization")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"segmentation.onnx", "embedding.onnx"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("model"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	seg, emb, err := findDiarizationModelsAt(filepath.Join(root, "transcription"), t.TempDir())
	if err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}
	if seg != filepath.Join(dir, "segmentation.onnx") {
		t.Errorf("unexpected segmentation path: %s", seg)
	}
	if emb != filepath.Join(dir, "embedding.onnx") {
		t.Errorf("unexpected embedding path: %s", emb)
	}
}

func TestFindDiarizationModelsInWorkingDirectory(t *testing.T) {
	cwd := t.TempDir()
	dir := filepath.Join(cwd, "models", "diarization")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"segmentation.onnx", "embedding.onnx"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("model"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	seg, emb, err := findDiarizationModelsAt(filepath.Join(t.TempDir(), "transcription"), cwd)
	if err != nil {
		t.Fatalf("did not expect an error: %v", err)
	}
	if seg == "" || emb == "" {
		t.Error("expected both model paths to be found in cwd")
	}
}

func TestFindDiarizationModelsMissingEmbedding(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "models", "diarization")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	// Only the segmentation model exists.
	if err := os.WriteFile(filepath.Join(dir, "segmentation.onnx"), []byte("model"), 0644); err != nil {
		t.Fatal(err)
	}

	_, _, err := findDiarizationModelsAt(filepath.Join(root, "transcription"), root)
	if err == nil {
		t.Fatal("expected an error for a missing embedding model")
	}
	if !strings.Contains(err.Error(), "embedding.onnx") {
		t.Errorf("error must name the missing file, got: %v", err)
	}
}

func TestFindPyannoteModelDirNextToBinary(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "models", "diarization-pyannote")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("pipeline:"), 0644); err != nil {
		t.Fatal(err)
	}

	got := findPyannoteModelDirAt(filepath.Join(root, "transcription"), t.TempDir())
	if got != dir {
		t.Errorf("expected %s, got %q", dir, got)
	}
}

func TestFindPyannoteModelDirInWorkingDirectory(t *testing.T) {
	cwd := t.TempDir()
	dir := filepath.Join(cwd, "models", "diarization-pyannote")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("pipeline:"), 0644); err != nil {
		t.Fatal(err)
	}

	got := findPyannoteModelDirAt(filepath.Join(t.TempDir(), "transcription"), cwd)
	if got != dir {
		t.Errorf("expected %s, got %q", dir, got)
	}
}

func TestFindPyannoteModelDirAbsent(t *testing.T) {
	// A missing local dir is NOT an error — the script falls back to the
	// HuggingFace hub. An empty string signals "not found".
	got := findPyannoteModelDirAt(filepath.Join(t.TempDir(), "transcription"), t.TempDir())
	if got != "" {
		t.Errorf("expected empty string for an absent dir, got %q", got)
	}
}

func TestFindPyannoteModelDirIgnoresDirWithoutConfig(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "models", "diarization-pyannote")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	// No config.yaml inside — the dir is unusable and must be skipped.

	got := findPyannoteModelDirAt(filepath.Join(root, "transcription"), root)
	if got != "" {
		t.Errorf("expected empty string for a dir without config.yaml, got %q", got)
	}
}
