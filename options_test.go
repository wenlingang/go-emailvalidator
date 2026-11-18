package emailvalidator

import (
	"errors"
	"testing"
)

type optionCase struct {
	opts []Option
	in   string
	want error
}

// optionCases is a package-level var so the fuzz task can reuse it as seed corpus.
var optionCases = []optionCase{
	{[]Option{AllowQuotedLocal()}, "\"zhang san\"@example.com", nil},
	{[]Option{AllowQuotedLocal()}, "\"zhang<san>\"@example.com", nil},
	{[]Option{AllowQuotedLocal()}, "\"zhang(san)\"@example.com", nil},
	{[]Option{AllowQuotedLocal()}, "\"zhang\\\"san\"@example.com", nil},
	{[]Option{AllowQuotedLocal()}, "\"zhang@san\"@example.com", nil},
	{[]Option{AllowQuotedLocal()}, "\"\"@example.com", nil},
	{[]Option{AllowQuotedLocal()}, "\"zhangsan@example.com", ErrLocalChar},
	{[]Option{AllowQuotedLocal()}, "\"zhang\"san@example.com", ErrLocalChar},
	{[]Option{AllowQuotedLocal()}, "\"zhang\\\"@example.com", ErrLocalChar},
	{[]Option{AllowQuotedLocal()}, "\"zhang\tsan\"@example.com", ErrLocalChar},
	{[]Option{AllowQuotedLocal()}, "\"张三\"@example.com", ErrNonASCIILocal},
	{[]Option{AllowQuotedLocal()}, "zhang<san>@example.com", ErrDisplayName},
	{nil, "\"zhang san\"@example.com", ErrQuotedLocal},

	{[]Option{AllowIPLiteral()}, "zhangsan@[192.0.2.1]", nil},
	{[]Option{AllowIPLiteral()}, "zhangsan@[IPv6:2001:db8::1]", nil},
	{[]Option{AllowIPLiteral()}, "zhangsan@[2001:db8::1]", ErrIPLiteral},
	{[]Option{AllowIPLiteral()}, "zhangsan@[192.0.2.256]", ErrIPLiteral},
	{[]Option{AllowIPLiteral()}, "zhangsan@[192.0.2.1", ErrIPLiteral},
	{[]Option{AllowIPLiteral()}, "zhangsan@[IPv6:192.0.2.1]", ErrIPLiteral},
	{[]Option{AllowIPLiteral()}, "zhangsan@[IPv6:2001:db8::1%eth0]", ErrIPLiteral},
	{[]Option{AllowIPLiteral()}, "zhangsan@[]", ErrIPLiteral},
	{nil, "zhangsan@[192.0.2.1]", ErrIPLiteral},

	{[]Option{AllowNoTLD()}, "zhangsan@localhost", nil},
	{[]Option{AllowNoTLD()}, "zhangsan@example.123", ErrNumericTLD},
	{[]Option{AllowNoTLD()}, "zhangsan@123", ErrNumericTLD},
	{nil, "zhangsan@localhost", ErrNoTLD},
}

func TestAllowOptions(t *testing.T) {
	for _, tc := range optionCases {
		err := New(tc.opts...).Validate(tc.in)
		if tc.want == nil {
			if err != nil {
				t.Errorf("Validate(%q) = %v, want nil", tc.in, err)
			}
			continue
		}
		if !errors.Is(err, tc.want) {
			t.Errorf("Validate(%q) = %v, want %v", tc.in, err, tc.want)
		}
	}
}

func TestProfileString(t *testing.T) {
	for p, want := range map[Profile]string{ASCII: "ASCII", IDN: "IDN", EAI: "EAI", 99: "Profile(99)", -1: "Profile(-1)"} {
		if got := p.String(); got != want {
			t.Errorf("Profile(%d).String() = %q, want %q", int(p), got, want)
		}
	}
}

func TestInvalidProfileClampsToASCII(t *testing.T) {
	for _, p := range []Profile{99, -1} {
		v := New(WithProfile(p))
		if err := v.Validate("zhangsan@中文.example"); !errors.Is(err, ErrNonASCIIDomain) {
			t.Errorf("profile %d: got %v, want ErrNonASCIIDomain", int(p), err)
		}
	}
}

func TestNilOptionAndZeroValue(t *testing.T) {
	v := New(nil, AllowNoTLD(), nil)
	if err := v.Validate("zhangsan@localhost"); err != nil {
		t.Errorf("New(nil, ...) lost options: %v", err)
	}
	var zero Validator
	def := New()
	for _, in := range seedCorpus() {
		a, b := zero.Validate(in), def.Validate(in)
		if (a == nil) != (b == nil) || (a != nil && !errors.Is(a, b.(*Error).Reason)) {
			t.Fatalf("zero Validator differs from New() on %q: %v vs %v", in, a, b)
		}
	}
}
