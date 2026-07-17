// Package writer is responsible for building and writing the output .md file.
// It collects transcription results into a structured Markdown document
// with headings for each video file.
package writer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"transcription/internal/scanner"
)

// TranscriptionResult is the result of processing a single video file.
type TranscriptionResult struct {
	Video   scanner.VideoFile
	Text    string
	Success bool
	Error   error
}

// OutputSection is a single section in the output .md document.
type OutputSection struct {
	Title string
	Text  string
}

// OutputDocument is the final .md document with all transcriptions.
type OutputDocument struct {
	Path     string
	DirName  string
	Sections []OutputSection
}

// MaxWordsPerFile is the maximum number of words in a single .md file; when exceeded the document is split into chunks.
const MaxWordsPerFile = 490000

// MaxLineLength is the maximum line length of the transcribed text.
const MaxLineLength = 120

// detectNameConflicts returns the set of names that occur more than once among successful results.
func detectNameConflicts(results []TranscriptionResult) map[string]bool {
	counts := make(map[string]int)
	for _, r := range results {
		if r.Success {
			counts[r.Video.Name]++
		}
	}
	conflicts := make(map[string]bool)
	for name, count := range counts {
		if count > 1 {
			conflicts[name] = true
		}
	}
	return conflicts
}

// sectionTitle builds the section heading, appending the extension in parentheses on a name conflict.
func sectionTitle(video scanner.VideoFile, conflicts map[string]bool) string {
	if conflicts[video.Name] {
		return fmt.Sprintf("%s (%s)", video.Name, video.Extension)
	}
	return video.Name
}

// buildSections converts successful results into sorted sections.
func buildSections(results []TranscriptionResult) []OutputSection {
	var successful []TranscriptionResult
	for _, r := range results {
		if r.Success {
			successful = append(successful, r)
		}
	}

	sort.Slice(successful, func(i, j int) bool {
		return successful[i].Video.ModTime.Before(successful[j].Video.ModTime)
	})

	conflicts := detectNameConflicts(results)

	sections := make([]OutputSection, 0, len(successful))
	for _, r := range successful {
		sections = append(sections, OutputSection{
			Title: sectionTitle(r.Video, conflicts),
			Text:  r.Text,
		})
	}

	return sections
}

// RenderMarkdown builds the content of a single Markdown document.
func RenderMarkdown(document OutputDocument) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Transcription: %s\n\n", document.DirName))

	for _, section := range document.Sections {
		sb.WriteString(fmt.Sprintf(
			"## %s\n\n%s\n\n",
			section.Title,
			wrapText(section.Text, MaxLineLength),
		))
	}

	return sb.String()
}

// wrapText wraps text at word boundaries without exceeding maxLength characters.
// Existing line breaks are preserved. Words longer than the limit are split,
// since otherwise the maximum line length cannot be guaranteed.
func wrapText(text string, maxLength int) string {
	if maxLength <= 0 || text == "" {
		return text
	}

	inputLines := strings.Split(text, "\n")
	outputLines := make([]string, 0, len(inputLines))

	for _, inputLine := range inputLines {
		if strings.TrimSpace(inputLine) == "" {
			outputLines = append(outputLines, "")
			continue
		}
		outputLines = append(outputLines, wrapLine(inputLine, maxLength)...)
	}

	return strings.Join(outputLines, "\n")
}

func wrapLine(line string, maxLength int) []string {
	words := strings.Fields(line)
	lines := make([]string, 0, 1)
	current := make([]rune, 0, maxLength)

	flush := func() {
		if len(current) > 0 {
			lines = append(lines, string(current))
			current = current[:0]
		}
	}

	for _, word := range words {
		wordRunes := []rune(word)

		if len(wordRunes) > maxLength {
			flush()
			for len(wordRunes) > maxLength {
				lines = append(lines, string(wordRunes[:maxLength]))
				wordRunes = wordRunes[maxLength:]
			}
			current = append(current, wordRunes...)
			continue
		}

		required := len(wordRunes)
		if len(current) > 0 {
			required++
		}
		if len(current)+required > maxLength {
			flush()
		}
		if len(current) > 0 {
			current = append(current, ' ')
		}
		current = append(current, wordRunes...)
	}

	flush()
	return lines
}

// WriteMarkdown builds a single Markdown document from the transcription results.
// For writing with automatic splitting by limit, use BuildDocuments.
func WriteMarkdown(dirName string, results []TranscriptionResult) string {
	return RenderMarkdown(OutputDocument{
		DirName:  dirName,
		Sections: buildSections(results),
	})
}

// splitSections groups sections so that each group does not exceed the limit.
// A section stays atomic even if it alone exceeds maxWords.
func splitSections(sections []OutputSection, maxWords int) [][]OutputSection {
	if len(sections) == 0 {
		return [][]OutputSection{{}}
	}
	if maxWords <= 0 {
		maxWords = MaxWordsPerFile
	}

	chunks := make([][]OutputSection, 0, 1)
	current := make([]OutputSection, 0)
	currentWords := 0

	for _, section := range sections {
		sectionWords := len(strings.Fields(section.Text))
		if len(current) > 0 && currentWords+sectionWords > maxWords {
			chunks = append(chunks, current)
			current = make([]OutputSection, 0)
			currentWords = 0
		}

		current = append(current, section)
		currentWords += sectionWords
	}

	return append(chunks, current)
}

// chunkPaths builds the paths base.md, base-chunk2.md, base-chunk3.md, etc.
func chunkPaths(basePath string, count int) []string {
	ext := filepath.Ext(basePath)
	base := strings.TrimSuffix(basePath, ext)
	paths := make([]string, count)

	for i := range count {
		if i == 0 {
			paths[i] = basePath
			continue
		}
		paths[i] = fmt.Sprintf("%s-chunk%d%s", base, i+1, ext)
	}

	return paths
}

// ChunkDocument splits the document into parts if the word count exceeds MaxWordsPerFile.
// It returns a slice of paths to the created files.
// Name format: base.md, base-chunk2.md, base-chunk3.md
// Splitting happens at section boundaries (##); a section is an atomic unit.
func ChunkDocument(basePath string, sections []OutputSection) []string {
	return chunkPaths(basePath, len(splitSections(sections, MaxWordsPerFile)))
}

// BuildDocuments builds documents while respecting the word limit.
func BuildDocuments(dirName, basePath string, results []TranscriptionResult) []OutputDocument {
	return buildDocuments(dirName, basePath, results, MaxWordsPerFile)
}

func buildDocuments(dirName, basePath string, results []TranscriptionResult, maxWords int) []OutputDocument {
	chunks := splitSections(buildSections(results), maxWords)
	paths := chunkPaths(basePath, len(chunks))
	documents := make([]OutputDocument, len(chunks))

	for i, sections := range chunks {
		documents[i] = OutputDocument{
			Path:     paths[i],
			DirName:  dirName,
			Sections: sections,
		}
	}

	return documents
}

// WriteDocuments writes all the built Markdown documents.
func WriteDocuments(documents []OutputDocument) error {
	for _, document := range documents {
		if err := os.WriteFile(document.Path, []byte(RenderMarkdown(document)), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", document.Path, err)
		}
	}

	return nil
}

// SanitizeFilename converts a string into a safe file name.
// Spaces are replaced with dashes, special characters are removed,
// Cyrillic is preserved, and everything is lowercased.
func SanitizeFilename(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")

	// Keep only letters (including Cyrillic), digits, and dashes
	var sb strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			sb.WriteRune(r)
		}
	}
	name = sb.String()

	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}

	name = strings.Trim(name, "-")

	return name
}
