package exercise

import (
	"bytes"
	"strconv"
	"strings"
	"unicode/utf8"
)

// number is an exact JSON number literal, kept as text so that no value is
// rounded through float64.
type number string

// maxNesting bounds parser recursion. Valid definitions nest far less deeply
// (about two levels per subset), so deeper input is rejected early.
const maxNesting = 128

// parseJSON parses strict RFC 8259 JSON, as JSON.parse does. Objects become
// map[string]any, keeping the last value of a duplicated key like JSON.parse
// and Dart's jsonDecode; arrays []any; numbers number; and null nil.
func parseJSON(src []byte) (any, error) {
	p := parser{src: src}
	p.skipSpace()
	value, err := p.value()
	if err != nil {
		return nil, err
	}
	p.skipSpace()
	if p.pos != len(p.src) {
		return nil, p.fail("unexpected data after JSON value")
	}
	return value, nil
}

type parser struct {
	src   []byte
	pos   int
	depth int
}

func (p *parser) fail(message string) error {
	return invalidf("malformed JSON at byte %d: %s", p.pos, message)
}

func (p *parser) skipSpace() {
	for p.pos < len(p.src) {
		switch p.src[p.pos] {
		case ' ', '\t', '\n', '\r':
			p.pos++
		default:
			return
		}
	}
}

func (p *parser) consume(c byte) bool {
	if p.pos < len(p.src) && p.src[p.pos] == c {
		p.pos++
		return true
	}
	return false
}

func (p *parser) literal(word string) bool {
	if bytes.HasPrefix(p.src[p.pos:], []byte(word)) {
		p.pos += len(word)
		return true
	}
	return false
}

func (p *parser) value() (any, error) {
	if p.pos >= len(p.src) {
		return nil, p.fail("unexpected end of input")
	}
	switch c := p.src[p.pos]; {
	case c == '{':
		return p.object()
	case c == '[':
		return p.array()
	case c == '"':
		return p.string()
	case c == '-' || isDigit(c):
		return p.number()
	case p.literal("true"):
		return true, nil
	case p.literal("false"):
		return false, nil
	case p.literal("null"):
		return nil, nil
	default:
		return nil, p.fail("unexpected character")
	}
}

func (p *parser) enter() error {
	p.depth++
	if p.depth > maxNesting {
		return p.fail("nesting is too deep")
	}
	p.pos++ // the opening bracket
	p.skipSpace()
	return nil
}

func (p *parser) object() (any, error) {
	if err := p.enter(); err != nil {
		return nil, err
	}
	defer func() { p.depth-- }()
	object := map[string]any{}
	if p.consume('}') {
		return object, nil
	}
	for {
		if p.pos >= len(p.src) || p.src[p.pos] != '"' {
			return nil, p.fail("expected an object key")
		}
		key, err := p.string()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if !p.consume(':') {
			return nil, p.fail("expected ':'")
		}
		p.skipSpace()
		value, err := p.value()
		if err != nil {
			return nil, err
		}
		object[key] = value
		p.skipSpace()
		if p.consume('}') {
			return object, nil
		}
		if !p.consume(',') {
			return nil, p.fail("expected ',' or '}'")
		}
		p.skipSpace()
	}
}

func (p *parser) array() (any, error) {
	if err := p.enter(); err != nil {
		return nil, err
	}
	defer func() { p.depth-- }()
	array := []any{}
	if p.consume(']') {
		return array, nil
	}
	for {
		value, err := p.value()
		if err != nil {
			return nil, err
		}
		array = append(array, value)
		p.skipSpace()
		if p.consume(']') {
			return array, nil
		}
		if !p.consume(',') {
			return nil, p.fail("expected ',' or ']'")
		}
		p.skipSpace()
	}
}

func (p *parser) string() (string, error) {
	p.pos++ // the opening quote
	var text []byte
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		switch {
		case c == '"':
			p.pos++
			return string(text), nil
		case c == '\\':
			var err error
			if text, err = p.escape(text); err != nil {
				return "", err
			}
		case c < 0x20:
			return "", p.fail("control character in string")
		case c < utf8.RuneSelf:
			text = append(text, c)
			p.pos++
		default:
			r, size := utf8.DecodeRune(p.src[p.pos:])
			if r == utf8.RuneError && size == 1 {
				return "", p.fail("invalid UTF-8")
			}
			text = append(text, p.src[p.pos:p.pos+size]...)
			p.pos += size
		}
	}
	return "", p.fail("unterminated string")
}

func (p *parser) escape(text []byte) ([]byte, error) {
	p.pos++ // the backslash
	if p.pos >= len(p.src) {
		return nil, p.fail("unterminated escape")
	}
	c := p.src[p.pos]
	p.pos++
	switch c {
	case '"', '\\', '/':
		return append(text, c), nil
	case 'b':
		return append(text, '\b'), nil
	case 'f':
		return append(text, '\f'), nil
	case 'n':
		return append(text, '\n'), nil
	case 'r':
		return append(text, '\r'), nil
	case 't':
		return append(text, '\t'), nil
	case 'u':
		r, err := p.hex4()
		if err != nil {
			return nil, err
		}
		if isHighSurrogate(r) && bytes.HasPrefix(p.src[p.pos:], []byte(`\u`)) {
			start := p.pos
			p.pos += 2
			low, err := p.hex4()
			if err != nil {
				return nil, err
			}
			if isLowSurrogate(low) {
				r = (r-0xD800)<<10 + (low - 0xDC00) + 0x10000
			} else {
				p.pos = start
			}
		}
		return appendWTF8(text, r), nil
	default:
		return nil, p.fail("invalid escape")
	}
}

func (p *parser) hex4() (rune, error) {
	if p.pos+4 > len(p.src) {
		return 0, p.fail("invalid \\u escape")
	}
	var r rune
	for _, c := range p.src[p.pos : p.pos+4] {
		var digit byte
		switch {
		case isDigit(c):
			digit = c - '0'
		case c >= 'a' && c <= 'f':
			digit = c - 'a' + 10
		case c >= 'A' && c <= 'F':
			digit = c - 'A' + 10
		default:
			return 0, p.fail("invalid \\u escape")
		}
		r = r<<4 | rune(digit)
	}
	p.pos += 4
	return r, nil
}

func (p *parser) number() (any, error) {
	start := p.pos
	p.consume('-')
	switch {
	case p.consume('0'):
	case p.pos < len(p.src) && p.src[p.pos] >= '1' && p.src[p.pos] <= '9':
		p.digits()
	default:
		return nil, p.fail("invalid number")
	}
	if p.consume('.') && !p.digits() {
		return nil, p.fail("invalid number")
	}
	if p.consume('e') || p.consume('E') {
		if !p.consume('+') {
			p.consume('-')
		}
		if !p.digits() {
			return nil, p.fail("invalid number")
		}
	}
	return number(p.src[start:p.pos]), nil
}

func (p *parser) digits() bool {
	start := p.pos
	for p.pos < len(p.src) && isDigit(p.src[p.pos]) {
		p.pos++
	}
	return p.pos > start
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// exactInteger returns the value of a JSON number literal when it is
// mathematically an integer from minimum to maximum. Notation such as 2.0 or
// 1e3 is accepted; nothing is rounded.
func exactInteger(literal number, minimum, maximum int64) (int64, bool) {
	text := string(literal)
	negative := strings.HasPrefix(text, "-")
	text = strings.TrimPrefix(text, "-")
	mantissa, exponentText := text, "0"
	if i := strings.IndexAny(text, "eE"); i >= 0 {
		mantissa, exponentText = text[:i], text[i+1:]
	}
	whole, fraction, _ := strings.Cut(mantissa, ".")
	digits := strings.TrimLeft(whole+fraction, "0")
	significant := strings.TrimRight(digits, "0")
	if significant == "" {
		return 0, minimum <= 0 && 0 <= maximum
	}
	// The value is significant × 10^scale.
	exponent, err := strconv.ParseInt(exponentText, 10, 32)
	if err != nil {
		return 0, false // |exponent| ≥ 2^31: far outside every contract range
	}
	scale := exponent - int64(len(fraction)) + int64(len(digits)-len(significant))
	if scale < 0 || int64(len(significant))+scale > 19 {
		return 0, false
	}
	value, err := strconv.ParseInt(significant+strings.Repeat("0", int(scale)), 10, 64)
	if err != nil {
		return 0, false
	}
	if negative {
		value = -value
	}
	return value, minimum <= value && value <= maximum
}

// appendString appends s as JSON.stringify would: escaping quotes,
// backslashes, control characters and lone surrogates, and nothing else.
func appendString(b []byte, s string) []byte {
	const hex = "0123456789abcdef"
	b = append(b, '"')
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			switch c {
			case '"', '\\':
				b = append(b, '\\', c)
			case '\b':
				b = append(b, `\b`...)
			case '\f':
				b = append(b, `\f`...)
			case '\n':
				b = append(b, `\n`...)
			case '\r':
				b = append(b, `\r`...)
			case '\t':
				b = append(b, `\t`...)
			default:
				if c < 0x20 {
					b = append(b, '\\', 'u', '0', '0', hex[c>>4], hex[c&0xF])
				} else {
					b = append(b, c)
				}
			}
			i++
			continue
		}
		if r, ok := surrogateAt(s, i); ok {
			b = append(b, '\\', 'u', hex[r>>12], hex[r>>8&0xF], hex[r>>4&0xF], hex[r&0xF])
			i += 3
			continue
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		b = append(b, s[i:i+size]...)
		i += size
	}
	return append(b, '"')
}
