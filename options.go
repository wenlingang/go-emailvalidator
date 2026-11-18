package emailvalidator

import "strconv"

// Profile selects which character sets are accepted.
type Profile int

const (
	// ASCII accepts ASCII atext local parts and ASCII LDH domains (default).
	ASCII Profile = iota
	// IDN additionally accepts Unicode domains. A Unicode domain is valid if
	// it converts to a well-formed punycode (A-label) domain; Validate
	// reports only validity. Callers MUST send the result of Normalize (or
	// Parse's ASCIIDomain), never the original input, because SMTP relays
	// and ESPs reliably accept only the ASCII form.
	IDN
	// EAI additionally accepts Unicode local parts (RFC 6532).
	EAI
)

// String returns "ASCII", "IDN" or "EAI", or "Profile(N)" for other values.
func (p Profile) String() string {
	switch p {
	case ASCII:
		return "ASCII"
	case IDN:
		return "IDN"
	case EAI:
		return "EAI"
	}
	return "Profile(" + strconv.Itoa(int(p)) + ")"
}

type config struct {
	profile     Profile
	quotedLocal bool
	ipLiteral   bool
	noTLD       bool
	mixedScript bool
}

// Option configures a Validator.
type Option func(*config)

// WithProfile sets the profile. Values outside ASCII, IDN and EAI are
// clamped to ASCII by New.
func WithProfile(p Profile) Option { return func(c *config) { c.profile = p } }

// AllowQuotedLocal allows quoted-string local parts such as "john doe"@example.com.
func AllowQuotedLocal() Option { return func(c *config) { c.quotedLocal = true } }

// AllowIPLiteral allows domain literals such as a@[192.0.2.1].
func AllowIPLiteral() Option { return func(c *config) { c.ipLiteral = true } }

// AllowNoTLD allows single-label domains such as a@localhost.
func AllowNoTLD() Option { return func(c *config) { c.noTLD = true } }

// AllowMixedScript disables mixed-script detection (EAI profile).
func AllowMixedScript() Option { return func(c *config) { c.mixedScript = true } }
