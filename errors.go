package emailvalidator

import (
	"errors"
	"strconv"
)

// Error describes why an address was rejected.
type Error struct {
	Reason error  // one of the Err* sentinels
	Input  string // original input
	// Pos is a byte offset into Input: -1 <= Pos <= len(Input). len(Input)
	// means "at end of input" (for example the empty domain in "a@"), and -1
	// means the position is unknown.
	Pos int
}

// Error implements the error interface. It is safe to call on an Error whose
// Reason is nil.
func (e *Error) Error() string {
	reason := "invalid address"
	if e.Reason != nil {
		reason = e.Reason.Error()
	}
	s := "emailvalidator: " + reason
	if e.Pos >= 0 {
		s += " at byte " + strconv.Itoa(e.Pos)
	}
	return s
}

// Unwrap returns the Err* sentinel, so errors.Is(err, ErrLabel) works.
func (e *Error) Unwrap() error { return e.Reason }

var (
	ErrEmpty          = errors.New("empty address")
	ErrWhitespace     = errors.New("leading or trailing whitespace")
	ErrDisplayName    = errors.New("display name or angle brackets not allowed")
	ErrMissingAt      = errors.New("missing @")
	ErrTooLong        = errors.New("address exceeds 254 octets")
	ErrLocalEmpty     = errors.New("empty local part")
	ErrLocalTooLong   = errors.New("local part exceeds 64 octets")
	ErrLocalChar      = errors.New("invalid character in local part")
	ErrNonASCIILocal  = errors.New("non-ASCII local part requires EAI profile")
	ErrDot            = errors.New("leading, trailing or consecutive dot")
	ErrQuotedLocal    = errors.New("quoted local part not allowed")
	ErrComment        = errors.New("comments not allowed")
	ErrDomainEmpty    = errors.New("empty domain")
	ErrDomainTooLong  = errors.New("domain exceeds 253 octets")
	ErrLabel          = errors.New("invalid domain label")
	ErrLabelTooLong   = errors.New("domain label exceeds 63 octets")
	ErrNonASCIIDomain = errors.New("non-ASCII domain requires IDN or EAI profile")
	ErrIDN            = errors.New("invalid internationalized domain")
	ErrNoTLD          = errors.New("domain has no TLD")
	ErrNumericTLD     = errors.New("TLD is all numeric")
	ErrIPLiteral      = errors.New("IP literal not allowed")
	ErrMixedScript    = errors.New("local part mixes scripts")
	ErrNotNFC         = errors.New("local part is not NFC normalized")
)

// fail builds an *Error without Input; Validate fills Input in.
func fail(reason error, pos int) *Error { return &Error{Reason: reason, Pos: pos} }
