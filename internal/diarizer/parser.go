package diarizer

import (
	"regexp"
	"strconv"
	"strings"
)

// segmentLineRe matches sherpa-onnx result lines like "0.318 -- 5.121 speaker_00".
// Anything else in the output (config dump, timing stats) is ignored.
var segmentLineRe = regexp.MustCompile(`^\s*(\d+(?:\.\d+)?)\s*--\s*(\d+(?:\.\d+)?)\s+speaker_(\d+)\s*$`)

// ParseSegments extracts speaker segments from sherpa-onnx stdout.
// Malformed lines are skipped: the CLI mixes segments with log noise,
// so defensive parsing beats failing the whole file.
func ParseSegments(output []byte) []Segment {
	var segments []Segment
	for _, line := range strings.Split(string(output), "\n") {
		m := segmentLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		start, err1 := strconv.ParseFloat(m[1], 64)
		end, err2 := strconv.ParseFloat(m[2], 64)
		speaker, err3 := strconv.Atoi(m[3])
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		segments = append(segments, Segment{Start: start, End: end, Speaker: speaker})
	}
	return segments
}
