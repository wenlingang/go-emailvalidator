package emailvalidator

import (
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const maxLocalLen = 64

// parseLocal validates the local part. offset is its byte offset in the input.
func (v *Validator) parseLocal(s string, offset int) (string, error) {
	local, err := v.parseLocalChars(s, offset)
	if err != nil {
		return "", err
	}
	if v.cfg.profile == EAI && hasNonASCII(s) {
		// Only EAI local parts with non-ASCII text pay for NFC and script checks.
		if !norm.NFC.IsNormalString(s) {
			return "", fail(ErrNotNFC, offset)
		}
		if !v.cfg.mixedScript && mixedScript(s) {
			return "", fail(ErrMixedScript, offset)
		}
	}
	return local, nil
}

func (v *Validator) parseLocalChars(s string, offset int) (string, error) {
	if s == "" {
		return "", fail(ErrLocalEmpty, offset)
	}
	if len(s) > maxLocalLen {
		return "", fail(ErrLocalTooLong, offset+maxLocalLen)
	}
	if s[0] == '"' {
		if !v.cfg.quotedLocal {
			return "", fail(ErrQuotedLocal, offset)
		}
		if err := checkQuoted(s, offset, v.cfg.profile == EAI); err != nil {
			return "", err
		}
		return s, nil
	}
	prevDot := true // treats a leading dot as a dot after a dot
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '.':
			if prevDot {
				return "", fail(ErrDot, offset+i)
			}
			prevDot = true
			continue
		case c >= 0x80:
			if v.cfg.profile != EAI {
				return "", fail(ErrNonASCIILocal, offset+i)
			}
			n, err := checkUTF8(s, i, offset)
			if err != nil {
				return "", err
			}
			i += n - 1
		case !isAtext(c):
			return "", fail(ErrLocalChar, offset+i)
		}
		prevDot = false
	}
	if prevDot {
		return "", fail(ErrDot, offset+len(s)-1)
	}
	return s, nil
}

// isAtext reports whether c is an RFC 5322 atext character (ASCII only).
func isAtext(c byte) bool {
	switch {
	case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9':
		return true
	}
	switch c {
	case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '/', '=', '?', '^', '_', '`', '{', '|', '}', '~':
		return true
	}
	return false
}

// checkQuoted validates an RFC 5321 quoted-string: DQUOTE *QcontentSMTP DQUOTE,
// where QcontentSMTP is qtextSMTP (ASCII 32-126 except '"' and '\') or a
// quoted-pairSMTP ('\' followed by ASCII 32-126). s must start with '"'.
// With eai, UTF-8 text is allowed too (RFC 6532).
func checkQuoted(s string, offset int, eai bool) *Error {
	for i := 1; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			if i != len(s)-1 {
				return fail(ErrLocalChar, offset+i+1) // data after closing quote
			}
			return nil
		case '\\':
			i++
			if i >= len(s) {
				return fail(ErrLocalChar, offset+i-1)
			}
			c = s[i]
		}
		if c >= 0x80 {
			if !eai {
				return fail(ErrNonASCIILocal, offset+i)
			}
			n, err := checkUTF8(s, i, offset)
			if err != nil {
				return err
			}
			i += n - 1
			continue
		}
		if c < 32 || c > 126 {
			return fail(ErrLocalChar, offset+i)
		}
	}
	return fail(ErrLocalChar, offset+len(s)-1) // unterminated
}

// checkUTF8 validates the non-ASCII rune at s[i:] for an EAI local part and
// returns its size. Invalid UTF-8, control, space, format (Cf) and private-use
// characters are rejected as ErrLocalChar.
func checkUTF8(s string, i, offset int) (int, *Error) {
	r, n := utf8.DecodeRuneInString(s[i:])
	if r == utf8.RuneError || unicode.IsControl(r) || unicode.IsSpace(r) ||
		unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Co, r) {
		return 0, fail(ErrLocalChar, offset+i)
	}
	return n, nil
}

func hasNonASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return true
		}
	}
	return false
}
