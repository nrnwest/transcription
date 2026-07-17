package diarizer

import (
	"fmt"
	"math"
	"strings"

	"transcription/internal/transcriber"
)

// Merge combines whisper's timestamped segments with speaker segments into
// markdown paragraphs prefixed with speaker labels ("**speaker-1:** …").
// Speakers are numbered by order of first appearance, not by diarizer ids.
// With no diarization segments it returns the plain joined text, so callers
// can use the result unconditionally.
func Merge(whisperSegs []transcriber.Segment, diarSegs []Segment, label string) string {
	var texts []string
	var speakers []int

	prevSpeaker := -1
	for _, ws := range whisperSegs {
		text := strings.TrimSpace(ws.Text)
		if text == "" {
			continue
		}
		speaker := prevSpeaker
		if len(diarSegs) > 0 {
			speaker = assignSpeaker(ws, diarSegs, prevSpeaker)
		}
		texts = append(texts, text)
		speakers = append(speakers, speaker)
		prevSpeaker = speaker
	}

	if len(texts) == 0 {
		return ""
	}

	if len(diarSegs) == 0 {
		return strings.Join(texts, " ")
	}

	// Renumber speakers by first appearance and fold consecutive
	// same-speaker segments into one paragraph.
	numbers := map[int]int{}
	var paragraphs []string
	current := ""
	currentSpeaker := -2 // sentinel distinct from any assigned value
	for i, text := range texts {
		if speakers[i] != currentSpeaker {
			if current != "" {
				paragraphs = append(paragraphs, current)
			}
			if _, ok := numbers[speakers[i]]; !ok {
				numbers[speakers[i]] = len(numbers) + 1
			}
			currentSpeaker = speakers[i]
			current = fmt.Sprintf("**%s%d:** %s", label, numbers[currentSpeaker], text)
		} else {
			current += " " + text
		}
	}
	paragraphs = append(paragraphs, current)

	return strings.Join(paragraphs, "\n\n")
}

// assignSpeaker picks the speaker whose diarization segments overlap the
// whisper segment the most. Ties go to the earlier-starting speaker (segments
// come time-ordered from sherpa). With zero overlap the previous speaker is
// kept; for the very first segment the nearest segment by midpoint wins.
func assignSpeaker(ws transcriber.Segment, diarSegs []Segment, prevSpeaker int) int {
	wStart := float64(ws.FromMS) / 1000
	wEnd := float64(ws.ToMS) / 1000

	overlaps := map[int]float64{}
	var order []int
	for _, ds := range diarSegs {
		o := math.Min(wEnd, ds.End) - math.Max(wStart, ds.Start)
		if o <= 0 {
			continue
		}
		if _, seen := overlaps[ds.Speaker]; !seen {
			order = append(order, ds.Speaker)
		}
		overlaps[ds.Speaker] += o
	}

	if len(overlaps) > 0 {
		best := order[0]
		for _, sp := range order[1:] {
			if overlaps[sp] > overlaps[best] {
				best = sp
			}
		}
		return best
	}

	if prevSpeaker >= 0 {
		return prevSpeaker
	}

	// First segment with no overlap: nearest diarization segment by midpoint.
	mid := (wStart + wEnd) / 2
	best := diarSegs[0].Speaker
	bestDist := math.Inf(1)
	for _, ds := range diarSegs {
		dist := math.Abs((ds.Start+ds.End)/2 - mid)
		if dist < bestDist {
			bestDist = dist
			best = ds.Speaker
		}
	}
	return best
}
