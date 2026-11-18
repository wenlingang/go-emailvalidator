package emailvalidator

import (
	"errors"
	"testing"
)

func TestSentinelMessages(t *testing.T) {
	cases := map[error]string{
		ErrEmpty:          "empty address",
		ErrWhitespace:     "leading or trailing whitespace",
		ErrDisplayName:    "display name or angle brackets not allowed",
		ErrMissingAt:      "missing @",
		ErrTooLong:        "address exceeds 254 octets",
		ErrLocalEmpty:     "empty local part",
		ErrLocalTooLong:   "local part exceeds 64 octets",
		ErrLocalChar:      "invalid character in local part",
		ErrNonASCIILocal:  "non-ASCII local part requires EAI profile",
		ErrDot:            "leading, trailing or consecutive dot",
		ErrQuotedLocal:    "quoted local part not allowed",
		ErrComment:        "comments not allowed",
		ErrDomainEmpty:    "empty domain",
		ErrDomainTooLong:  "domain exceeds 253 octets",
		ErrLabel:          "invalid domain label",
		ErrLabelTooLong:   "domain label exceeds 63 octets",
		ErrNonASCIIDomain: "non-ASCII domain requires IDN or EAI profile",
		ErrIDN:            "invalid internationalized domain",
		ErrNoTLD:          "domain has no TLD",
		ErrNumericTLD:     "TLD is all numeric",
		ErrIPLiteral:      "IP literal not allowed",
		ErrMixedScript:    "local part mixes scripts",
		ErrNotNFC:         "local part is not NFC normalized",
	}
	for err, want := range cases {
		if err.Error() != want {
			t.Errorf("got %q, want %q", err.Error(), want)
		}
	}
}

func TestErrorUnwrapAndMessage(t *testing.T) {
	e := &Error{Reason: ErrDot, Input: "a..b@example.com", Pos: 2}
	if !errors.Is(e, ErrDot) || errors.Unwrap(e) != ErrDot {
		t.Error("Unwrap broken")
	}
	if got := e.Error(); got != "emailvalidator: leading, trailing or consecutive dot at byte 2" {
		t.Errorf("got %q", got)
	}
	e.Pos = -1
	if got := e.Error(); got != "emailvalidator: leading, trailing or consecutive dot" {
		t.Errorf("got %q", got)
	}
}

func TestErrorNilReason(t *testing.T) {
	e := &Error{Pos: -1}
	if got := e.Error(); got != "emailvalidator: invalid address" {
		t.Errorf("got %q", got)
	}
}

func TestWhitespacePos(t *testing.T) {
	cases := []struct {
		in  string
		pos int
	}{
		{" zhangsan@example.com", 0},
		{"zhangsan@example.com ", 20},
		{"zhangsan@example.com　", 20},
		{"zhangsan@example.com\n", 20},
	}
	for _, c := range cases {
		var e *Error
		if !errors.As(Validate(c.in), &e) || !errors.Is(e, ErrWhitespace) || e.Pos != c.pos {
			t.Errorf("Validate(%q) = %v, want ErrWhitespace at %d", c.in, e, c.pos)
		}
	}
}
