package exercise

import "unicode/utf8"

func isHighSurrogate(r rune) bool { return r >= 0xD800 && r < 0xDC00 }
func isLowSurrogate(r rune) bool  { return r >= 0xDC00 && r < 0xE000 }

// appendWTF8 appends r as UTF-8, writing a lone surrogate in the three-byte
// form UTF-8 would use for it (WTF-8).
func appendWTF8(b []byte, r rune) []byte {
	if isHighSurrogate(r) || isLowSurrogate(r) {
		return append(b, 0xE0|byte(r>>12), 0x80|byte(r>>6)&0x3F, 0x80|byte(r)&0x3F)
	}
	return utf8.AppendRune(b, r)
}

// surrogateAt reports the lone surrogate encoded at s[i], if any.
func surrogateAt(s string, i int) (rune, bool) {
	if i+2 >= len(s) || s[i] != 0xED || s[i+1] < 0xA0 || s[i+1] > 0xBF || s[i+2] < 0x80 || s[i+2] > 0xBF {
		return 0, false
	}
	return 0xD000 | rune(s[i+1]&0x3F)<<6 | rune(s[i+2]&0x3F), true
}

// codePoints counts the Unicode code points of s, counting a lone surrogate as
// one, like the TypeScript and Dart SDKs. It reports false unless s is UTF-8
// with optional WTF-8 lone surrogates, as decoding produces.
func codePoints(s string) (int, bool) {
	count := 0
	for i := 0; i < len(s); count++ {
		if r, ok := surrogateAt(s, i); ok {
			// A surrogate pair must be one four-byte character, as it would
			// otherwise encode differently from what JSON decoding produces.
			if next, ok := surrogateAt(s, i+3); ok && isHighSurrogate(r) && isLowSurrogate(next) {
				return 0, false
			}
			i += 3
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			return 0, false
		}
		i += size
	}
	return count, true
}

// isBlank reports whether s has only characters matched by the schema's title
// pattern exclusions: ECMAScript \s plus NEL. A lone surrogate is not blank.
func isBlank(s string) bool {
	for i := 0; i < len(s); {
		if _, ok := surrogateAt(s, i); ok {
			return false
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if !isSpace(r) {
			return false
		}
		i += size
	}
	return true
}

func isSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x85, 0xA0, 0x1680, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000, 0xFEFF:
		return true
	}
	return r >= 0x2000 && r <= 0x200A
}
