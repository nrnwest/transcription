package transcriber

import "testing"

// TestIsValidLang verifies the language validator for whisper:
// — "auto" and whisper.cpp codes are valid
// — invalid codes (countries, full names, empty, wrong case) are not
func TestIsValidLang(t *testing.T) {
	cases := []struct {
		lang string
		want bool
	}{
		// valid
		{"auto", true},
		{"uk", true},
		{"en", true},
		{"ru", true},
		{"de", true},
		{"yue", true}, // Cantonese — the 99th, last added code

		// invalid
		{"ua", false},      // ISO country code, not a language — the most common mistake
		{"xx", false},      // not a code at all
		{"russian", false}, // full name instead of a code
		{"", false},        // empty
		{"UK", false},      // wrong case — whisper is lowercase only
		{"En", false},      // same thing
	}

	for _, tc := range cases {
		t.Run(tc.lang, func(t *testing.T) {
			got := IsValidLang(tc.lang)
			if got != tc.want {
				t.Errorf("IsValidLang(%q) = %v, expected %v", tc.lang, got, tc.want)
			}
		})
	}
}

// TestSupportedLangsCount pins the exact number of languages (100 + auto = 101).
// If whisper.cpp adds a new language, this test will remind us to update the constants.
func TestSupportedLangsCount(t *testing.T) {
	const want = 101 // 100 whisper ISO codes + "auto"
	if got := len(supportedLangs); got != want {
		t.Errorf("len(supportedLangs) = %d, expected %d", got, want)
	}
}
