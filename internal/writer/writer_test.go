package writer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"transcription/internal/scanner"
)

func TestTranscriptionResultSuccess(t *testing.T) {
	result := TranscriptionResult{
		Video: scanner.VideoFile{
			Path:      "/video/AI-2.mkv",
			Name:      "AI-2",
			Extension: "mkv",
			ModTime:   time.Now(),
		},
		Text:    "Транскрибований текст лекції.",
		Success: true,
		Error:   nil,
	}

	if !result.Success {
		t.Error("expected Success = true")
	}
	if result.Text == "" {
		t.Error("text must not be empty on success")
	}
	if result.Error != nil {
		t.Errorf("did not expect an error on success, got: %v", result.Error)
	}
}

func TestTranscriptionResultFailure(t *testing.T) {
	result := TranscriptionResult{
		Video: scanner.VideoFile{
			Path: "/video/broken.mkv",
			Name: "broken",
		},
		Text:    "",
		Success: false,
		Error:   fmt.Errorf("ffmpeg: файл пошкоджений"),
	}

	if result.Success {
		t.Error("expected Success = false")
	}
	if result.Text != "" {
		t.Error("text must be empty on error")
	}
	if result.Error == nil {
		t.Error("expected an error on failed processing")
	}
}

func TestOutputSectionTitle(t *testing.T) {
	section := OutputSection{
		Title: "AI-2",
		Text:  "Текст транскрибації.",
	}

	if section.Title != "AI-2" {
		t.Errorf("expected Title = AI-2, got %s", section.Title)
	}
}

func TestOutputDocument(t *testing.T) {
	doc := OutputDocument{
		Path:    "/output/video.md",
		DirName: "video",
		Sections: []OutputSection{
			{Title: "AI-1", Text: "Текст 1."},
			{Title: "AI-2", Text: "Текст 2."},
		},
	}

	if len(doc.Sections) != 2 {
		t.Errorf("expected 2 sections, got %d", len(doc.Sections))
	}
	if doc.DirName != "video" {
		t.Errorf("expected DirName = video, got %s", doc.DirName)
	}
}

func TestWriteMarkdownHeader(t *testing.T) {
	results := []TranscriptionResult{
		{
			Video: scanner.VideoFile{
				Name:      "lecture-1",
				Extension: "mkv",
				ModTime:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			Text:    "Текст лекції.",
			Success: true,
		},
	}

	md := WriteMarkdown("my-videos", results)

	if !strings.HasPrefix(md, "# Transcription: my-videos") {
		t.Errorf("document must start with '# Transcription: my-videos', got:\n%s", md)
	}
}

func TestWriteMarkdownSections(t *testing.T) {
	results := []TranscriptionResult{
		{
			Video: scanner.VideoFile{
				Name:      "video-1",
				Extension: "mp4",
				ModTime:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			Text:    "Перший текст.",
			Success: true,
		},
		{
			Video: scanner.VideoFile{
				Name:      "video-2",
				Extension: "mkv",
				ModTime:   time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
			},
			Text:    "Другий текст.",
			Success: true,
		},
	}

	md := WriteMarkdown("lectures", results)

	if !strings.Contains(md, "## video-1") {
		t.Error("document must contain section '## video-1'")
	}
	if !strings.Contains(md, "## video-2") {
		t.Error("document must contain section '## video-2'")
	}
	if !strings.Contains(md, "Перший текст.") {
		t.Error("document must contain the text of the first section")
	}
	if !strings.Contains(md, "Другий текст.") {
		t.Error("document must contain the text of the second section")
	}
}

func TestWriteMarkdownSortsByModTime(t *testing.T) {
	results := []TranscriptionResult{
		{
			Video: scanner.VideoFile{
				Name:      "newest",
				Extension: "mp4",
				ModTime:   time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			},
			Text:    "Третій.",
			Success: true,
		},
		{
			Video: scanner.VideoFile{
				Name:      "oldest",
				Extension: "mkv",
				ModTime:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			Text:    "Перший.",
			Success: true,
		},
		{
			Video: scanner.VideoFile{
				Name:      "middle",
				Extension: "avi",
				ModTime:   time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
			},
			Text:    "Другий.",
			Success: true,
		},
	}

	md := WriteMarkdown("mixed", results)

	posOldest := strings.Index(md, "## oldest")
	posMiddle := strings.Index(md, "## middle")
	posNewest := strings.Index(md, "## newest")

	if posOldest == -1 || posMiddle == -1 || posNewest == -1 {
		t.Fatalf("not all sections found in the document:\n%s", md)
	}

	if posOldest >= posMiddle {
		t.Errorf("'oldest' must come before 'middle' (positions: %d >= %d)", posOldest, posMiddle)
	}
	if posMiddle >= posNewest {
		t.Errorf("'middle' must come before 'newest' (positions: %d >= %d)", posMiddle, posNewest)
	}
}

func TestWriteMarkdownNameConflict(t *testing.T) {
	results := []TranscriptionResult{
		{
			Video: scanner.VideoFile{
				Name:      "video",
				Extension: "mkv",
				ModTime:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			Text:    "Текст з mkv.",
			Success: true,
		},
		{
			Video: scanner.VideoFile{
				Name:      "video",
				Extension: "mp4",
				ModTime:   time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
			},
			Text:    "Текст з mp4.",
			Success: true,
		},
	}

	md := WriteMarkdown("dups", results)

	if !strings.Contains(md, "## video (mkv)") {
		t.Errorf("expected '## video (mkv)' on a name conflict, got:\n%s", md)
	}
	if !strings.Contains(md, "## video (mp4)") {
		t.Errorf("expected '## video (mp4)' on a name conflict, got:\n%s", md)
	}
}

func TestWriteMarkdownEmptyResults(t *testing.T) {
	md := WriteMarkdown("empty-dir", nil)

	if !strings.Contains(md, "# Transcription: empty-dir") {
		t.Errorf("even an empty document must have a heading, got:\n%s", md)
	}

	if strings.Contains(md, "##") {
		t.Errorf("an empty document must not contain ## sections, got:\n%s", md)
	}
}

func TestWriteMarkdownSkipsFailedResults(t *testing.T) {
	results := []TranscriptionResult{
		{
			Video: scanner.VideoFile{
				Name:      "good",
				Extension: "mp4",
				ModTime:   time.Now(),
			},
			Text:    "Гарний текст.",
			Success: true,
		},
		{
			Video: scanner.VideoFile{
				Name:      "broken",
				Extension: "mkv",
				ModTime:   time.Now(),
			},
			Text:    "",
			Success: false,
			Error:   fmt.Errorf("помилка"),
		},
	}

	md := WriteMarkdown("mixed", results)

	if !strings.Contains(md, "## good") {
		t.Error("document must contain the successful section 'good'")
	}
	if strings.Contains(md, "## broken") {
		t.Error("document must not contain the failed section 'broken'")
	}
}

func TestRenderMarkdownWrapsTextAt120Characters(t *testing.T) {
	text := strings.Repeat("українське слово ", 20)
	document := OutputDocument{
		DirName:  "videos",
		Sections: []OutputSection{{Title: "lecture", Text: text}},
	}

	md := RenderMarkdown(document)

	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		if length := len([]rune(line)); length > MaxLineLength {
			t.Errorf("line has %d characters, maximum %d: %q", length, MaxLineLength, line)
		}
	}
}

func TestWrapTextPreservesParagraphs(t *testing.T) {
	text := "Перший абзац із кількома словами.\n\nДругий абзац."

	got := wrapText(text, MaxLineLength)

	if got != text {
		t.Errorf("expected preserved paragraphs:\n%q\ngot:\n%q", text, got)
	}
}

func TestWrapTextSplitsSingleLongWord(t *testing.T) {
	text := strings.Repeat("я", MaxLineLength+10)

	got := wrapText(text, MaxLineLength)
	lines := strings.Split(got, "\n")

	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if len([]rune(lines[0])) != MaxLineLength {
		t.Errorf("first line must contain %d characters", MaxLineLength)
	}
	if len([]rune(lines[1])) != 10 {
		t.Errorf("second line must contain 10 characters")
	}
}

func TestWrapTextCountsUnicodeCharacters(t *testing.T) {
	text := strings.Repeat("ї", 60) + " " + strings.Repeat("є", 59)

	got := wrapText(text, MaxLineLength)

	if strings.Contains(got, "\n") {
		t.Errorf("a line of exactly %d Unicode characters must not be wrapped", MaxLineLength)
	}
}

func TestChunkDocumentNoSplitNeeded(t *testing.T) {
	sections := []OutputSection{
		{Title: "video-1", Text: "Короткий текст."},
		{Title: "video-2", Text: "Ще один текст."},
	}

	paths := ChunkDocument("/output/video.md", sections)

	if len(paths) != 1 {
		t.Errorf("expected 1 file, got %d", len(paths))
	}
	if len(paths) > 0 && paths[0] != "/output/video.md" {
		t.Errorf("expected path '/output/video.md', got '%s'", paths[0])
	}
}

func TestChunkDocumentSplitsOnLimit(t *testing.T) {
	bigText := strings.Repeat("слово ", 250000)

	sections := []OutputSection{
		{Title: "part-1", Text: bigText},
		{Title: "part-2", Text: bigText},
		{Title: "part-3", Text: bigText},
	}

	paths := ChunkDocument("/output/video.md", sections)

	if len(paths) < 2 {
		t.Errorf("expected at least 2 files for 750K words, got %d", len(paths))
	}
}

func TestChunkDocumentFileNames(t *testing.T) {
	bigText := strings.Repeat("word ", 250000)

	sections := []OutputSection{
		{Title: "s1", Text: bigText},
		{Title: "s2", Text: bigText},
		{Title: "s3", Text: bigText},
	}

	paths := ChunkDocument("/output/video.md", sections)

	if len(paths) < 2 {
		t.Fatalf("expected at least 2 files, got %d", len(paths))
	}

	if paths[0] != "/output/video.md" {
		t.Errorf("first file must be '/output/video.md', got '%s'", paths[0])
	}

	if len(paths) > 1 && paths[1] != "/output/video-chunk2.md" {
		t.Errorf("second file must be '/output/video-chunk2.md', got '%s'", paths[1])
	}

	if len(paths) > 2 && paths[2] != "/output/video-chunk3.md" {
		t.Errorf("third file must be '/output/video-chunk3.md', got '%s'", paths[2])
	}
}

func TestChunkDocumentAtomicSection(t *testing.T) {
	hugeText := strings.Repeat("word ", 500000)

	sections := []OutputSection{
		{Title: "huge", Text: hugeText},
	}

	paths := ChunkDocument("/output/video.md", sections)

	if len(paths) != 1 {
		t.Errorf("an atomic section >490K words must stay in 1 file, got %d", len(paths))
	}
}

func TestChunkDocumentSplitsAtSectionBoundary(t *testing.T) {
	medText := strings.Repeat("word ", 200000)

	sections := []OutputSection{
		{Title: "s1", Text: medText},
		{Title: "s2", Text: medText},
		{Title: "s3", Text: medText},
	}

	paths := ChunkDocument("/output/video.md", sections)

	if len(paths) != 2 {
		t.Errorf("expected 2 files for 600K words, got %d", len(paths))
	}
}

func TestBuildDocumentsDistributesSections(t *testing.T) {
	results := []TranscriptionResult{
		{
			Video:   scanner.VideoFile{Name: "s1", ModTime: time.Unix(1, 0)},
			Text:    "один два три",
			Success: true,
		},
		{
			Video:   scanner.VideoFile{Name: "s2", ModTime: time.Unix(2, 0)},
			Text:    "чотири п'ять шість",
			Success: true,
		},
		{
			Video:   scanner.VideoFile{Name: "s3", ModTime: time.Unix(3, 0)},
			Text:    "сім вісім",
			Success: true,
		},
	}

	documents := buildDocuments("videos", "/output/videos.md", results, 5)

	if len(documents) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(documents))
	}
	if documents[0].Path != "/output/videos.md" {
		t.Errorf("incorrect path of the first document: %s", documents[0].Path)
	}
	if documents[1].Path != "/output/videos-chunk2.md" {
		t.Errorf("incorrect path of the second document: %s", documents[1].Path)
	}
	if len(documents[0].Sections) != 1 || documents[0].Sections[0].Title != "s1" {
		t.Errorf("first document must contain only s1: %#v", documents[0].Sections)
	}
	if len(documents[1].Sections) != 2 ||
		documents[1].Sections[0].Title != "s2" ||
		documents[1].Sections[1].Title != "s3" {
		t.Errorf("second document must contain s2 and s3: %#v", documents[1].Sections)
	}
}

func TestWriteDocumentsWritesEveryChunk(t *testing.T) {
	dir := t.TempDir()
	documents := []OutputDocument{
		{
			Path:     filepath.Join(dir, "videos.md"),
			DirName:  "videos",
			Sections: []OutputSection{{Title: "s1", Text: "перший текст"}},
		},
		{
			Path:     filepath.Join(dir, "videos-chunk2.md"),
			DirName:  "videos",
			Sections: []OutputSection{{Title: "s2", Text: "другий текст"}},
		},
	}

	if err := WriteDocuments(documents); err != nil {
		t.Fatalf("failed to write documents: %v", err)
	}

	for _, document := range documents {
		data, err := os.ReadFile(document.Path)
		if err != nil {
			t.Fatalf("failed to read %s: %v", document.Path, err)
		}
		content := string(data)
		if !strings.Contains(content, "# Transcription: videos") {
			t.Errorf("%s does not contain the document heading", document.Path)
		}
		if !strings.Contains(content, "## "+document.Sections[0].Title) {
			t.Errorf("%s does not contain section %s", document.Path, document.Sections[0].Title)
		}
	}
}

func TestSanitizeFilenameSpacesToDashes(t *testing.T) {
	result := SanitizeFilename("my video file")
	if result != "my-video-file" {
		t.Errorf("expected 'my-video-file', got '%s'", result)
	}
}

func TestSanitizeFilenameRemovesSpecialChars(t *testing.T) {
	result := SanitizeFilename("video!@#$%file")
	if result != "videofile" {
		t.Errorf("expected 'videofile', got '%s'", result)
	}
}

func TestSanitizeFilenameKeepsCyrillic(t *testing.T) {
	result := SanitizeFilename("лекція")
	if result != "лекція" {
		t.Errorf("expected 'лекція', got '%s'", result)
	}
}

func TestSanitizeFilenameLowercase(t *testing.T) {
	result := SanitizeFilename("MyVideo")
	if result != "myvideo" {
		t.Errorf("expected 'myvideo', got '%s'", result)
	}
}

func TestSanitizeFilenameComplexExample(t *testing.T) {
	result := SanitizeFilename("Мої відео 2026!")
	expected := "мої-відео-2026"
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

func TestSanitizeFilenameKeepsDigitsAndDashes(t *testing.T) {
	result := SanitizeFilename("video-123")
	if result != "video-123" {
		t.Errorf("expected 'video-123', got '%s'", result)
	}
}
