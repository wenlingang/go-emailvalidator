package emailvalidator

import (
	"errors"
	"strings"
	"testing"
)

type validateCase struct {
	in   string
	want error
}

// asciiCases is a package-level var so the fuzz task can reuse it as seed corpus.
var asciiCases = []validateCase{
	{"zhangsan@example.com", nil},
	{"zhang.san+tag@mail.example.org", nil},
	{"ZhangSan@Example.COM", nil},
	{"a!#$%&'*+-/=?^_`{|}~@example.com", nil},
	{"", ErrEmpty},
	{" zhangsan@example.com", ErrWhitespace},
	{"zhangsan@example.com\n", ErrWhitespace},
	{"zhangsan.example.com", ErrMissingAt},
	{"张三 <zhangsan@example.com>", ErrDisplayName},
	{"Zhang San <zhangsan@example.com>", ErrDisplayName},
	{"zhangsan@exa<mple.com", ErrDisplayName}, // pre-pass catches < before domain checks
	{"zhangsan(comment)@example.com", ErrComment},
	{"请改为此邮箱zhangsan@example.com", ErrNonASCIILocal},
	{"@example.com", ErrLocalEmpty},
	{strings.Repeat("a", 65) + "@example.com", ErrLocalTooLong},
	{strings.Repeat("a", 64) + "@" + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 63) + ".com", ErrTooLong},
	{".zhangsan@example.com", ErrDot},
	{"zhangsan.@example.com", ErrDot},
	{"zhang..san@example.com", ErrDot},
	{"\"zhang san\"@example.com", ErrQuotedLocal},
	{"a@b@example.com", ErrLocalChar},
	{"zhang san@example.com", ErrLocalChar},
	// Splitting on the last @ leaves an @ in the local part, so ErrLocalChar.
	{"zhangsan@example.com\r\nBcc: x@example.com", ErrLocalChar},
	{"zhangsan@", ErrDomainEmpty},
	{"zhangsan@例子.中国", ErrNonASCIIDomain},
	{"zhangsan@[192.0.2.1]", ErrIPLiteral},
	{"zhangsan@example.com.", ErrLabel},
	{"zhangsan@example..com", ErrLabel},
	{"zhangsan@.example.com", ErrLabel},
	{"zhangsan@-example.com", ErrLabel},
	{"zhangsan@example-.com", ErrLabel},
	{"zhangsan@exa_mple.com", ErrLabel},
	{"zhangsan@" + strings.Repeat("a", 64) + ".com", ErrLabelTooLong},
	{"zhangsan@localhost", ErrNoTLD},
	{"zhangsan@192.0.2.1", ErrNumericTLD},
}

// commonMistakeCases models typical data-entry mistakes (fictional data only).
var commonMistakeCases = []validateCase{
	{"zhangsan@example.com ", ErrWhitespace},
	{"zhangsan@example.com\u202C\u200F", ErrNonASCIIDomain}, // invisible direction marks from copy-paste
	{"\u200Ezhang.san@example.com", ErrNonASCIILocal},
	{"zhangsan@example.com; lisi@example.org", ErrLocalChar}, // several addresses in one field
	{"zhangsan@example.com;", ErrLabel},
	{"mailto:zhangsan@example.com", ErrLocalChar},
	{"\"zhangsan@example.com\"", ErrQuotedLocal},
	{"zhangsan@example.com abc", ErrLabel},
	{"zhangsan@example.com!", ErrLabel},
	{"zhangsan@example.com'x", ErrLabel},
	{"zhangsan@example.1", ErrNumericTLD},
	{"zhangsan@example.com1", nil}, // only all-numeric TLDs are rejected, by design
	{"zhangsan@example", ErrNoTLD},
	{"zhang.san@_example.com", ErrLabel},
	{"张三123@example.com", ErrNonASCIILocal},
	{"señor9@example.com", ErrNonASCIILocal},
	{"a1b2c3d4e5f6a7b8", ErrMissingAt},
}

func runCases(t *testing.T, cases []validateCase) {
	t.Helper()
	for _, tc := range cases {
		err := Validate(tc.in)
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

func TestValidateASCII(t *testing.T)          { runCases(t, asciiCases) }
func TestValidateCommonMistakes(t *testing.T) { runCases(t, commonMistakeCases) }

func TestErrorFields(t *testing.T) {
	in := "zhang..san@example.com"
	err := Validate(in)
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("want *Error, got %T", err)
	}
	if e.Input != in || e.Pos != 6 || !errors.Is(err, ErrDot) {
		t.Errorf("got Input=%q Pos=%d Reason=%v", e.Input, e.Pos, e.Reason)
	}
	if !strings.Contains(err.Error(), "consecutive dot") {
		t.Errorf("message %q lacks %q", err.Error(), "consecutive dot")
	}
}

func TestIsValid(t *testing.T) {
	if !IsValid("zhangsan@example.com") || IsValid("zhangsan") {
		t.Error("IsValid mismatch")
	}
}

func TestNewValidatorOptionsStored(t *testing.T) {
	v := New(WithProfile(EAI), AllowQuotedLocal(), AllowIPLiteral(), AllowNoTLD(), AllowMixedScript())
	c := v.cfg
	if c.profile != EAI || !c.quotedLocal || !c.ipLiteral || !c.noTLD || !c.mixedScript {
		t.Errorf("options not stored: %+v", c)
	}
	if err := v.Validate("zhangsan@localhost"); err != nil {
		t.Errorf("AllowNoTLD: %v", err)
	}
}

func TestValidateNoAllocs(t *testing.T) {
	for _, in := range []string{"zhang.san+tag@mail.example.org", "ZhangSan@Example.COM"} {
		n := testing.AllocsPerRun(100, func() { _ = Validate(in) })
		if n != 0 {
			t.Errorf("Validate(%q) allocs = %v, want 0", in, n)
		}
	}
}
