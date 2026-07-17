package transcriber

// supportedLangs — the full set of language codes supported by whisper.cpp,
// plus "auto" for auto-detection. 100 languages in total (confirmed against two
// official sources):
//   - whisper.cpp/src/whisper.cpp — the g_lang constant
//     https://github.com/ggml-org/whisper.cpp/blob/master/src/whisper.cpp
//   - openai/whisper/whisper/tokenizer.py — the LANGUAGES dictionary
//     https://github.com/openai/whisper/blob/main/whisper/tokenizer.py
//
// Update when new languages are added to whisper.cpp.
var supportedLangs = map[string]struct{}{
	"auto": {}, "en": {}, "zh": {}, "de": {}, "es": {}, "ru": {}, "ko": {},
	"fr": {}, "ja": {}, "pt": {}, "tr": {}, "pl": {}, "ca": {}, "nl": {},
	"ar": {}, "sv": {}, "it": {}, "id": {}, "hi": {}, "fi": {}, "vi": {},
	"he": {}, "uk": {}, "el": {}, "ms": {}, "cs": {}, "ro": {}, "da": {},
	"hu": {}, "ta": {}, "no": {}, "th": {}, "ur": {}, "hr": {}, "bg": {},
	"lt": {}, "la": {}, "mi": {}, "ml": {}, "cy": {}, "sk": {}, "te": {},
	"fa": {}, "lv": {}, "bn": {}, "sr": {}, "az": {}, "sl": {}, "kn": {},
	"et": {}, "mk": {}, "br": {}, "eu": {}, "is": {}, "hy": {}, "ne": {},
	"mn": {}, "bs": {}, "kk": {}, "sq": {}, "sw": {}, "gl": {}, "mr": {},
	"pa": {}, "si": {}, "km": {}, "sn": {}, "yo": {}, "so": {}, "af": {},
	"oc": {}, "ka": {}, "be": {}, "tg": {}, "sd": {}, "gu": {}, "am": {},
	"yi": {}, "lo": {}, "uz": {}, "fo": {}, "ht": {}, "ps": {}, "tk": {},
	"nn": {}, "mt": {}, "sa": {}, "lb": {}, "my": {}, "bo": {}, "tl": {},
	"mg": {}, "as": {}, "tt": {}, "haw": {}, "ln": {}, "ha": {}, "ba": {},
	"jw": {}, "su": {}, "yue": {},
}

// IsValidLang returns true if the language code is supported by whisper.cpp
// or is the special value "auto" for auto-detection.
// Case-sensitive: whisper accepts only lowercase.
func IsValidLang(lang string) bool {
	_, ok := supportedLangs[lang]
	return ok
}
