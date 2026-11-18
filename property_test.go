package emailvalidator

import (
	"errors"
	"net/mail"
	"testing"
)

// seedCorpus gathers every input from the table tests in this package.
func seedCorpus() []string {
	var s []string
	for _, c := range asciiCases {
		s = append(s, c.in)
	}
	for _, c := range commonMistakeCases {
		s = append(s, c.in)
	}
	for _, c := range idnCases {
		s = append(s, c.in)
	}
	for _, c := range optionCases {
		s = append(s, c.in)
	}
	for _, c := range eaiCases {
		s = append(s, c.in)
	}
	for _, c := range eaiQuotedCases {
		s = append(s, c.in)
	}
	for _, c := range mixedScriptCases {
		s = append(s, c.in)
	}
	return s
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

var sentinels = []error{
	ErrEmpty, ErrWhitespace, ErrDisplayName, ErrMissingAt, ErrTooLong,
	ErrLocalEmpty, ErrLocalTooLong, ErrLocalChar, ErrNonASCIILocal, ErrDot,
	ErrQuotedLocal, ErrComment, ErrDomainEmpty, ErrDomainTooLong, ErrLabel,
	ErrLabelTooLong, ErrNonASCIIDomain, ErrIDN, ErrNoTLD, ErrNumericTLD,
	ErrIPLiteral, ErrMixedScript, ErrNotNFC,
}

// reasonOf checks the structural invariants of a non-nil error produced for
// input in and returns its sentinel Reason.
func reasonOf(t testing.TB, err error, in string) error {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("%q: error %T is not *Error", in, err)
	}
	if e.Input != in {
		t.Fatalf("%q: Error.Input = %q", in, e.Input)
	}
	if e.Pos < -1 || e.Pos > len(in) {
		t.Fatalf("%q: Pos %d out of range [-1, %d]", in, e.Pos, len(in))
	}
	for _, s := range sentinels {
		if errors.Is(err, s) {
			return s
		}
	}
	t.Fatalf("%q: Reason %v is not an exported sentinel", in, e.Reason)
	return nil
}

// sameResult requires two results for the same input to agree exactly:
// both nil, or the same Reason and Pos.
func sameResult(t testing.TB, in string, a, b error) {
	t.Helper()
	if (a == nil) != (b == nil) {
		t.Fatalf("%q: results disagree: %v vs %v", in, a, b)
	}
	if a == nil {
		return
	}
	ra, rb := reasonOf(t, a, in), reasonOf(t, b, in)
	if ra != rb {
		t.Fatalf("%q: reasons disagree: %v vs %v", in, ra, rb)
	}
	if a.(*Error).Pos != b.(*Error).Pos {
		t.Fatalf("%q: Pos disagree: %d vs %d", in, a.(*Error).Pos, b.(*Error).Pos)
	}
}

func checkProperties(t testing.TB, in string) {
	// Default config: strict agreement with net/mail.
	err := Validate(in)
	sameResult(t, in, err, Validate(in))
	if err != nil {
		_ = reasonOf(t, err, in)
	} else {
		a, perr := mail.ParseAddress(in)
		if perr != nil || a.Address != in {
			t.Fatalf("default accepts %q but net/mail gives %v, %v", in, a, perr)
		}
	}

	// IDN / EAI: assert on Normalize output.
	for _, p := range []Profile{IDN, EAI} {
		v := New(WithProfile(p))
		out, nerr := v.Normalize(in)
		out2, nerr2 := v.Normalize(in)
		if out != out2 {
			t.Fatalf("unstable Normalize(%q) profile %d", in, p)
		}
		sameResult(t, in, nerr, nerr2)
		if nerr != nil {
			_ = reasonOf(t, nerr, in) // Pos is checked against the untrimmed input
			continue
		}
		if verr := v.Validate(out); verr != nil {
			t.Fatalf("profile %d: Normalize(%q)=%q not idempotent: %v", p, in, out, verr)
		}
		a, perr := mail.ParseAddress(out)
		if perr != nil {
			t.Fatalf("profile %d: net/mail rejects Normalize(%q)=%q: %v", p, in, out, perr)
		}
		if (p == IDN || isASCII(out)) && a.Address != out {
			t.Fatalf("profile %d: net/mail address %q != Normalize output %q", p, a.Address, out)
		}
	}

	// Every profile/options combination: stable, well-formed errors, and
	// Validate and Parse agree.
	for _, v := range []*Validator{
		New(),
		New(WithProfile(IDN)),
		New(WithProfile(EAI)),
		New(AllowQuotedLocal()),
		New(AllowIPLiteral()),
		New(AllowNoTLD()),
		New(WithProfile(EAI), AllowQuotedLocal(), AllowIPLiteral(), AllowNoTLD(), AllowMixedScript()),
	} {
		e1, e2 := v.Validate(in), v.Validate(in)
		sameResult(t, in, e1, e2)
		_, pe := v.Parse(in)
		if (e1 == nil) != (pe == nil) {
			t.Fatalf("%q: Validate=%v but Parse=%v", in, e1, pe)
		}
		if e1 != nil {
			if reasonOf(t, e1, in) != reasonOf(t, pe, in) {
				t.Fatalf("%q: Validate reason %v != Parse reason %v", in, e1, pe)
			}
		}
	}
}

func TestSeedCorpusProperties(t *testing.T) {
	for _, in := range seedCorpus() {
		checkProperties(t, in)
	}
}

func FuzzValidate(f *testing.F) {
	for _, s := range seedCorpus() {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) { checkProperties(t, in) })
}
