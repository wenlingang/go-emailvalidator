package emailvalidator

import (
	"errors"
	"strings"
	"testing"
)

// distinctHan returns n distinct CJK ideographs; punycode compresses repeats,
// so distinct runes are needed to exceed the 63-octet label limit.
func distinctHan(n int) string {
	r := make([]rune, n)
	for i := range r {
		r[i] = rune(0x4E00 + i*211)
	}
	return string(r)
}

// idnCases is a package-level var so the fuzz task can reuse it as seed corpus.
var idnCases = []validateCase{
	{"zhangsan@例子.中国", nil},
	{"zhangsan@例え.テスト", nil},
	{"zhangsan@xn--fsqu00a.xn--fiqs8s", nil},
	{"zhangsan@Example.COM", nil},
	{"用户@例子.中国", ErrNonASCIILocal},
	{"zhangsan@例子..中国", ErrLabel},
	{"zhangsan@例子.中国.", ErrLabel},
	{"zhangsan@例子.中国。", ErrLabel}, // ideographic full stop maps to a trailing dot
	{"zhangsan@例子", ErrNoTLD},
	{"zhangsan@例子.123", ErrNumericTLD},
	{"zhangsan@例 子.中国", ErrIDN},
	{"zhangsan@-例子.中国", ErrIDN},
	{"zhangsan@" + distinctHan(30) + ".中国", ErrLabelTooLong},
	// Raw input is 154 bytes (within the 254 limit); the punycode form exceeds 253.
	{"a@" + strings.Repeat("例.", 50) + "中国", ErrDomainTooLong},
	// Domain alone fits in 253 octets as punycode, but local@domain exceeds 254.
	{strings.Repeat("a", 64) + "@" + strings.Repeat("例.", 24) + "中国", ErrTooLong},
	// Invalid UTF-8 in a domain must not be silently mapped to U+FFFD.
	{"zhangsan@\xbe\x8b.例.中国", ErrIDN},
}

func runIDN(t *testing.T, v *Validator, cases []validateCase) {
	t.Helper()
	for _, tc := range cases {
		err := v.Validate(tc.in)
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

func TestIDN(t *testing.T) { runIDN(t, New(WithProfile(IDN)), idnCases) }

// EAI handles the domain like IDN (Unicode local parts are covered in eai_test.go).
func TestEAIDomainLikeIDN(t *testing.T) {
	runIDN(t, New(WithProfile(EAI)), []validateCase{
		{"zhangsan@例子.中国", nil},
		{"用户@例子.中国", nil},
	})
}

func TestIDNRejectedByDefaultProfile(t *testing.T) {
	if err := New().Validate("zhangsan@例子.中国"); !errors.Is(err, ErrNonASCIIDomain) {
		t.Errorf("got %v", err)
	}
}

func TestIDNInvalidIsErrIDN(t *testing.T) {
	err := New(WithProfile(IDN)).Validate("zhangsan@例 子.中国")
	var e *Error
	if !errors.As(err, &e) || e.Pos != 9 || e.Input != "zhangsan@例 子.中国" {
		t.Errorf("got %#v", err)
	}
}

func TestParse(t *testing.T) {
	a, err := New(WithProfile(IDN)).Parse("zhangsan@例子.中国")
	if err != nil {
		t.Fatal(err)
	}
	if a.Local != "zhangsan" || a.Domain != "例子.中国" || a.ASCIIDomain != "xn--fsqu00a.xn--fiqs8s" {
		t.Errorf("got %+v", a)
	}
	a, err = New().Parse("ZhangSan@Example.COM")
	if err != nil {
		t.Fatal(err)
	}
	if a.ASCIIDomain != "example.com" || a.String() != "ZhangSan@Example.COM" {
		t.Errorf("got %+v %q", a, a.String())
	}
	if _, err := New().Parse("zhangsan"); !errors.Is(err, ErrMissingAt) {
		t.Errorf("got %v", err)
	}
	if a, err := New().Parse("zhangsan@example.com\n"); err == nil || a != (Address{}) {
		t.Errorf("want zero Address and error, got %+v %v", a, err)
	}
	// IP literals are kept verbatim (the IPv6 tag is case-sensitive here).
	a, err = New(AllowIPLiteral()).Parse("zhangsan@[IPv6:2001:db8::1]")
	if err != nil || a.ASCIIDomain != "[IPv6:2001:db8::1]" {
		t.Errorf("got %+v %v", a, err)
	}
}

func TestNormalize(t *testing.T) {
	cases := []struct {
		v    *Validator
		in   string
		want string
		err  error
	}{
		{New(), "  ZhangSan@Example.COM ", "ZhangSan@example.com", nil},
		{New(WithProfile(IDN)), "zhangsan@例子.中国", "zhangsan@xn--fsqu00a.xn--fiqs8s", nil},
		{New(WithProfile(IDN)), "zhangsan@例え.テスト", "zhangsan@xn--r8jz45g.xn--zckzah", nil},
		{New(WithProfile(IDN)), "zhangsan@例子。中国", "zhangsan@xn--fsqu00a.xn--fiqs8s", nil},
		{New(), "张三 <zhangsan@example.com>", "", ErrDisplayName},
	}
	for _, tc := range cases {
		got, err := tc.v.Normalize(tc.in)
		if got != tc.want || !errors.Is(err, tc.err) {
			t.Errorf("Normalize(%q) = %q, %v; want %q, %v", tc.in, got, err, tc.want, tc.err)
		}
	}
	var e *Error
	_, err := New().Normalize("  张三 <zhangsan@example.com>")
	if !errors.As(err, &e) || e.Input != "  张三 <zhangsan@example.com>" {
		t.Errorf("Input not original: %#v", err)
	}
}

func TestIDNNoAllocsForASCIIDomain(t *testing.T) {
	v := New(WithProfile(IDN))
	n := testing.AllocsPerRun(100, func() { _ = v.Validate("ZhangSan@Example.COM") })
	if n != 0 {
		t.Errorf("allocs = %v, want 0", n)
	}
}
