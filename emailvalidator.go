// Package emailvalidator validates email addresses strictly for delivery
// through ESPs and SMTP relays, rejecting input that is merely RFC-tolerated.
package emailvalidator

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const maxAddrLen = 254

// Validator is stateless and safe for concurrent use. The zero value is
// ready to use and is equivalent to New(): the default ASCII profile.
type Validator struct{ cfg config }

// New returns a Validator with the given options. nil options are ignored.
// An out-of-range profile passed to WithProfile is clamped to ASCII.
func New(opts ...Option) *Validator {
	v := &Validator{}
	for _, o := range opts {
		if o != nil {
			o(&v.cfg)
		}
	}
	if v.cfg.profile < ASCII || v.cfg.profile > EAI {
		v.cfg.profile = ASCII
	}
	return v
}

var defaultValidator = New()

// Validate checks addr with the default (ASCII) configuration.
func Validate(addr string) error { return defaultValidator.Validate(addr) }

// IsValid reports whether addr passes Validate.
func IsValid(addr string) bool { return Validate(addr) == nil }

// Validate returns nil if addr is acceptable, otherwise an *Error.
func (v *Validator) Validate(addr string) error {
	if err := v.validate(addr); err != nil {
		err.Input = addr
		return err
	}
	return nil
}

func (v *Validator) validate(addr string) *Error {
	at, err := v.precheck(addr)
	if err != nil {
		return err
	}
	if _, err := v.parseLocal(addr[:at], 0); err != nil {
		return asError(err)
	}
	return v.checkDomain(addr[at+1:], at+1)
}

// precheck runs the whole-address checks and returns the index of the last @.
func (v *Validator) precheck(addr string) (int, *Error) {
	if addr == "" {
		return 0, fail(ErrEmpty, -1)
	}
	if pos := edgeSpace(addr); pos >= 0 {
		return 0, fail(ErrWhitespace, pos)
	}
	at := lastAt(addr)
	if at < 0 {
		return 0, fail(ErrMissingAt, -1)
	}
	if err := scanBrackets(addr); err != nil {
		return 0, err
	}
	if len(addr) > maxAddrLen {
		return 0, fail(ErrTooLong, -1)
	}
	return at, nil
}

// Address is a parsed email address.
type Address struct {
	Local string // local part as given
	// Domain is the domain as given and may be Unicode. Do not send it to an
	// SMTP relay or ESP under the IDN or EAI profile; use ASCIIDomain.
	Domain      string
	ASCIIDomain string // lowercase ASCII domain; punycode for IDN input
}

// String returns the address as given: Local@Domain.
//
// WARNING: under the IDN or EAI profile the domain may be Unicode, so the
// result is NOT safe to send. Send the output of Normalize, or
// Local + "@" + ASCIIDomain, instead.
func (a Address) String() string { return a.Local + "@" + a.Domain }

// Parse validates addr and splits it. Unlike Validate it allocates.
func (v *Validator) Parse(addr string) (Address, error) {
	a, err := v.parse(addr)
	if err != nil {
		err.Input = addr
		return Address{}, err
	}
	return a, nil
}

func (v *Validator) parse(addr string) (Address, *Error) {
	at, err := v.precheck(addr)
	if err != nil {
		return Address{}, err
	}
	local, lerr := v.parseLocal(addr[:at], 0)
	if lerr != nil {
		return Address{}, asError(lerr)
	}
	domain, ascii, derr := v.parseDomain(addr[at+1:], at+1)
	if derr != nil {
		return Address{}, asError(derr)
	}
	return Address{Local: local, Domain: domain, ASCIIDomain: ascii}, nil
}

// Normalize trims surrounding whitespace, validates, and returns
// Local@ASCIIDomain.
//
// Senders MUST use this result rather than the original input: for IDN/EAI
// profiles the domain is converted to punycode, which is the only form SMTP
// relays and ESPs reliably accept.
func (v *Validator) Normalize(addr string) (string, error) {
	trimmed := strings.TrimSpace(addr)
	// NFC may change the local part's byte length; remember both lengths so
	// error positions can be mapped back onto the original input.
	oldLocal, newLocal := -1, -1
	if v.cfg.profile == EAI {
		trimmed, oldLocal, newLocal = nfcLocal(trimmed)
	}
	a, err := v.Parse(trimmed)
	if err != nil {
		if e, ok := err.(*Error); ok {
			e.Input = addr
			if e.Pos >= 0 && oldLocal >= 0 {
				if e.Pos >= newLocal {
					e.Pos += oldLocal - newLocal // after the local part: shift back
				} else if e.Pos >= oldLocal {
					e.Pos = oldLocal - 1 // inside the recomposed local: clamp
				}
			}
			if e.Pos >= 0 { // make Pos index into the untrimmed Input
				e.Pos += len(addr) - len(strings.TrimLeftFunc(addr, unicode.IsSpace))
			}
		}
		return "", err
	}
	return a.Local + "@" + a.ASCIIDomain, nil
}

func asError(err error) *Error {
	if e, ok := err.(*Error); ok {
		return e
	}
	return fail(err, -1)
}

// edgeSpace returns the byte offset of leading whitespace (0) or of the
// trailing whitespace rune, or -1 if there is none.
func edgeSpace(s string) int {
	if r, _ := utf8.DecodeRuneInString(s); unicode.IsSpace(r) {
		return 0
	}
	last := len(s) - 1
	for last > 0 && s[last]&0xC0 == 0x80 { // back up to rune start
		last--
	}
	if r, _ := utf8.DecodeRuneInString(s[last:]); unicode.IsSpace(r) {
		return last
	}
	return -1
}

func lastAt(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '@' {
			return i
		}
	}
	return -1
}

// scanBrackets finds <, > and ( outside double-quoted segments.
func scanBrackets(s string) *Error {
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inQuote:
			switch c {
			case '\\':
				i++
			case '"':
				inQuote = false
			}
		case c == '"':
			inQuote = true
		case c == '<' || c == '>':
			return fail(ErrDisplayName, i)
		case c == '(':
			return fail(ErrComment, i)
		}
	}
	return nil
}

// nfcLocal returns addr with its local part NFC-normalized, plus the local
// part's byte length before and after. If nothing changed (or there is no @)
// addr is returned as is, with both lengths -1.
func nfcLocal(addr string) (string, int, int) {
	at := lastAt(addr)
	if at < 0 || norm.NFC.IsNormalString(addr[:at]) {
		return addr, -1, -1
	}
	n := norm.NFC.String(addr[:at])
	return n + addr[at:], at, len(n)
}
