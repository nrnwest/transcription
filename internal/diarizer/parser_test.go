package diarizer

import "testing"

func TestParseSegments(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []Segment
	}{
		{
			name:  "two speakers",
			input: "0.318 -- 5.121 speaker_00\n5.400 -- 9.000 speaker_01\n",
			want: []Segment{
				{Start: 0.318, End: 5.121, Speaker: 0},
				{Start: 5.4, End: 9, Speaker: 1},
			},
		},
		{
			name: "sherpa log noise around segments",
			input: "Started\nOfflineSpeakerDiarizationConfig(...)\n" +
				"0.5 -- 2.0 speaker_00\n" +
				"Duration : 60.000 s\nElapsed seconds: 2.5 s\nReal time factor (RTF): 0.04\n",
			want: []Segment{{Start: 0.5, End: 2, Speaker: 0}},
		},
		{
			name:  "varying whitespace and integer seconds",
			input: "  10 --  15.5   speaker_02  \n",
			want:  []Segment{{Start: 10, End: 15.5, Speaker: 2}},
		},
		{
			name:  "malformed lines skipped",
			input: "abc -- def speaker_00\n1.0 - 2.0 speaker_00\n3.0 -- 4.0 speaker_zz\n5.0 -- 6.0 speaker_01\n",
			want:  []Segment{{Start: 5, End: 6, Speaker: 1}},
		},
		{
			name:  "empty output",
			input: "",
			want:  nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseSegments([]byte(tc.input))
			if len(got) != len(tc.want) {
				t.Fatalf("expected %d segments, got %d: %+v", len(tc.want), len(got), got)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("segment %d: expected %+v, got %+v", i, tc.want[i], got[i])
				}
			}
		})
	}
}
