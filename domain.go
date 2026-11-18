package emailvalidator

import (
	"net/netip"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/idna"
)

const (
	maxDomainLen = 253
	maxLabelLen  = 63
)

// parseDomain validates the domain and returns it with its ASCII form: the
// lowercased domain, or the punycode (A-label) form for IDN/EAI input.
// offset is the domain's byte offset in the input.
func (v *Validator) parseDomain(s string, offset int) (domain, asciiDomain string, err error) {
	if s != "" && s[0] != '[' && s[len(s)-1] != '.' && v.cfg.profile != ASCII {
		if i := firstNonASCII(s); i >= 0 {
			a, e := v.toASCIIDomain(s, offset)
			if e != nil {
				return "", "", e
			}
			return s, a, nil
		}
	}
	if err := v.checkDomain(s, offset); err != nil {
		return "", "", err
	}
	if s[0] == '[' {
		return s, s, nil // IP literals are kept verbatim
	}
	return s, strings.ToLower(s), nil
}

// toASCIIDomain converts a Unicode domain to punycode and validates the
// result with the ASCII rules. Positions inside the converted form are
// meaningless to the caller, so errors point at the domain start.
func (v *Validator) toASCIIDomain(s string, offset int) (string, *Error) {
	if !utf8.ValidString(s) { // idna would silently map bad bytes to U+FFFD
		return "", fail(ErrIDN, offset)
	}
	a, err := idna.Lookup.ToASCII(s)
	if err != nil {
		return "", fail(ErrIDN, offset)
	}
	if e := v.checkASCIIDomain(a, offset, true); e != nil {
		return "", e
	}
	// Punycode can be longer than the input, so re-check the whole address
	// (offset is the length of "local@").
	if offset+len(a) > maxAddrLen {
		return "", fail(ErrTooLong, -1)
	}
	return a, nil
}

func firstNonASCII(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return i
		}
	}
	return -1
}

// checkDomain validates the domain (Validate hot path). The ASCII path does
// not allocate; idna is only invoked for non-ASCII domains in IDN/EAI profiles.
func (v *Validator) checkDomain(s string, offset int) *Error {
	if s == "" {
		return fail(ErrDomainEmpty, offset)
	}
	if s[0] == '[' {
		if !v.cfg.ipLiteral {
			return fail(ErrIPLiteral, offset)
		}
		return checkIPLiteral(s, offset)
	}
	if s[len(s)-1] == '.' {
		return fail(ErrLabel, offset+len(s)-1)
	}
	if i := firstNonASCII(s); i >= 0 {
		if v.cfg.profile == ASCII {
			return fail(ErrNonASCIIDomain, offset+i)
		}
		_, e := v.toASCIIDomain(s, offset)
		return e
	}
	return v.checkASCIIDomain(s, offset, false)
}

// checkASCIIDomain applies the length, label and TLD rules to an ASCII domain.
// converted marks a punycode result, whose positions do not map to the input.
func (v *Validator) checkASCIIDomain(s string, offset int, converted bool) *Error {
	pos := func(i int) int {
		if converted {
			return offset
		}
		return offset + i
	}
	if len(s) > maxDomainLen {
		return fail(ErrDomainTooLong, pos(maxDomainLen))
	}

	labels, start := 1, 0
	for i := 0; i <= len(s); i++ {
		if i < len(s) && s[i] != '.' {
			if !isLDH(s[i]) {
				return fail(ErrLabel, pos(i))
			}
			continue
		}
		n := i - start
		switch {
		case n == 0:
			return fail(ErrLabel, pos(i))
		case n > maxLabelLen:
			return fail(ErrLabelTooLong, pos(start+maxLabelLen))
		case s[start] == '-' || s[i-1] == '-':
			return fail(ErrLabel, pos(start))
		}
		if i < len(s) {
			labels++
		}
		start = i + 1
	}

	if labels < 2 && !v.cfg.noTLD {
		return fail(ErrNoTLD, offset)
	}
	tld := s[strings.LastIndexByte(s, '.')+1:]
	if allDigits(tld) {
		return fail(ErrNumericTLD, pos(len(s)-len(tld)))
	}
	return nil
}

func isLDH(c byte) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || c == '-'
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}

// checkIPLiteral validates "[IPv4]" or "[IPv6:IPv6-address]" (RFC 5321 4.1.3).
func checkIPLiteral(s string, offset int) *Error {
	if len(s) < 3 || s[len(s)-1] != ']' {
		return fail(ErrIPLiteral, offset)
	}
	body := s[1 : len(s)-1]
	if rest, ok := strings.CutPrefix(body, "IPv6:"); ok {
		a, err := netip.ParseAddr(rest)
		if err != nil || !a.Is6() || a.Zone() != "" {
			return fail(ErrIPLiteral, offset+1)
		}
		return nil
	}
	a, err := netip.ParseAddr(body)
	if err != nil || !a.Is4() {
		return fail(ErrIPLiteral, offset+1)
	}
	return nil
}
