package diarizer

import (
	"testing"

	"transcription/internal/transcriber"
)

func TestMergeTwoSpeakers(t *testing.T) {
	whisperSegs := []transcriber.Segment{
		{FromMS: 0, ToMS: 5000, Text: " Привіт, як справи?"},
		{FromMS: 5000, ToMS: 9000, Text: " Все добре, дякую."},
	}
	diarSegs := []Segment{
		{Start: 0, End: 5, Speaker: 0},
		{Start: 5, End: 9, Speaker: 1},
	}

	got := Merge(whisperSegs, diarSegs, "speaker-")
	want := "**speaker-1:** Привіт, як справи?\n\n**speaker-2:** Все добре, дякую."

	if got != want {
		t.Errorf("expected:\n%s\ngot:\n%s", want, got)
	}
}

func TestMergeConsecutiveSameSpeaker(t *testing.T) {
	whisperSegs := []transcriber.Segment{
		{FromMS: 0, ToMS: 3000, Text: " Перше речення."},
		{FromMS: 3000, ToMS: 6000, Text: " Друге речення."},
		{FromMS: 6000, ToMS: 9000, Text: " Відповідь."},
	}
	diarSegs := []Segment{
		{Start: 0, End: 6, Speaker: 0},
		{Start: 6, End: 9, Speaker: 1},
	}

	got := Merge(whisperSegs, diarSegs, "speaker-")
	want := "**speaker-1:** Перше речення. Друге речення.\n\n**speaker-2:** Відповідь."

	if got != want {
		t.Errorf("expected:\n%s\ngot:\n%s", want, got)
	}
}

func TestMergeStraddlingSegmentMaxOverlapWins(t *testing.T) {
	// Whisper segment 2..8 overlaps speaker 0 (0..4 → 2s) and speaker 1 (4..10 → 4s).
	whisperSegs := []transcriber.Segment{
		{FromMS: 2000, ToMS: 8000, Text: " Спірний сегмент."},
	}
	diarSegs := []Segment{
		{Start: 0, End: 4, Speaker: 0},
		{Start: 4, End: 10, Speaker: 1},
	}

	got := Merge(whisperSegs, diarSegs, "speaker-")
	want := "**speaker-1:** Спірний сегмент."

	// Speaker 1 wins by overlap but is renumbered by first appearance → speaker-1.
	if got != want {
		t.Errorf("expected:\n%s\ngot:\n%s", want, got)
	}
}

func TestMergeTieGoesToEarlierSpeaker(t *testing.T) {
	// Equal 3s overlap with both speakers → the one appearing earlier wins.
	whisperSegs := []transcriber.Segment{
		{FromMS: 0, ToMS: 6000, Text: " Перший."},
		{FromMS: 6000, ToMS: 12000, Text: " Нічия."},
	}
	diarSegs := []Segment{
		{Start: 0, End: 6, Speaker: 0},
		{Start: 6, End: 9, Speaker: 0},
		{Start: 9, End: 12, Speaker: 1},
	}

	got := Merge(whisperSegs, diarSegs, "speaker-")
	want := "**speaker-1:** Перший. Нічия."

	if got != want {
		t.Errorf("expected:\n%s\ngot:\n%s", want, got)
	}
}

func TestMergeNoOverlapInheritsPreviousSpeaker(t *testing.T) {
	whisperSegs := []transcriber.Segment{
		{FromMS: 0, ToMS: 4000, Text: " В зоні."},
		{FromMS: 20000, ToMS: 24000, Text: " Поза зоною."},
	}
	diarSegs := []Segment{
		{Start: 0, End: 4, Speaker: 0},
	}

	got := Merge(whisperSegs, diarSegs, "speaker-")
	want := "**speaker-1:** В зоні. Поза зоною."

	if got != want {
		t.Errorf("expected:\n%s\ngot:\n%s", want, got)
	}
}

func TestMergeFirstSegmentNoOverlapUsesNearest(t *testing.T) {
	// The first whisper segment (0..2 s) touches nothing; nearest by midpoint
	// is speaker 1 (3..5) rather than speaker 0 (30..40).
	whisperSegs := []transcriber.Segment{
		{FromMS: 0, ToMS: 2000, Text: " Початок."},
		{FromMS: 3000, ToMS: 5000, Text: " Далі."},
	}
	diarSegs := []Segment{
		{Start: 3, End: 5, Speaker: 1},
		{Start: 30, End: 40, Speaker: 0},
	}

	got := Merge(whisperSegs, diarSegs, "speaker-")
	want := "**speaker-1:** Початок. Далі."

	if got != want {
		t.Errorf("expected:\n%s\ngot:\n%s", want, got)
	}
}

func TestMergeSingleSpeaker(t *testing.T) {
	whisperSegs := []transcriber.Segment{
		{FromMS: 0, ToMS: 5000, Text: " Монолог."},
	}
	diarSegs := []Segment{
		{Start: 0, End: 5, Speaker: 0},
	}

	got := Merge(whisperSegs, diarSegs, "speaker-")
	want := "**speaker-1:** Монолог."

	if got != want {
		t.Errorf("expected:\n%s\ngot:\n%s", want, got)
	}
}

func TestMergeEmptyDiarizationReturnsPlainText(t *testing.T) {
	whisperSegs := []transcriber.Segment{
		{FromMS: 0, ToMS: 3000, Text: " Одне."},
		{FromMS: 3000, ToMS: 6000, Text: " Друге."},
	}

	got := Merge(whisperSegs, nil, "speaker-")
	want := "Одне. Друге."

	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestMergeSkipsEmptyWhisperSegments(t *testing.T) {
	whisperSegs := []transcriber.Segment{
		{FromMS: 0, ToMS: 1000, Text: "   "},
		{FromMS: 1000, ToMS: 5000, Text: " Текст."},
		{FromMS: 5000, ToMS: 5100, Text: ""},
	}
	diarSegs := []Segment{
		{Start: 0, End: 6, Speaker: 0},
	}

	got := Merge(whisperSegs, diarSegs, "speaker-")
	want := "**speaker-1:** Текст."

	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestMergeRenumbersByFirstAppearance(t *testing.T) {
	// sherpa's speaker_01 speaks first → they become speaker-1.
	whisperSegs := []transcriber.Segment{
		{FromMS: 0, ToMS: 4000, Text: " Хто перший."},
		{FromMS: 4000, ToMS: 8000, Text: " Хто другий."},
	}
	diarSegs := []Segment{
		{Start: 0, End: 4, Speaker: 1},
		{Start: 4, End: 8, Speaker: 0},
	}

	got := Merge(whisperSegs, diarSegs, "speaker-")
	want := "**speaker-1:** Хто перший.\n\n**speaker-2:** Хто другий."

	if got != want {
		t.Errorf("expected:\n%s\ngot:\n%s", want, got)
	}
}

func TestMergeCustomLabel(t *testing.T) {
	whisperSegs := []transcriber.Segment{
		{FromMS: 0, ToMS: 5000, Text: " Hello."},
	}
	diarSegs := []Segment{
		{Start: 0, End: 5, Speaker: 0},
	}

	got := Merge(whisperSegs, diarSegs, "Speaker")
	want := "**Speaker1:** Hello."

	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestMergeNoWhisperSegments(t *testing.T) {
	got := Merge(nil, []Segment{{Start: 0, End: 5, Speaker: 0}}, "speaker-")
	if got != "" {
		t.Errorf("expected empty output, got %q", got)
	}
}
