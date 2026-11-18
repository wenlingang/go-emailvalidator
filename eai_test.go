package emailvalidator

import (
	"errors"
	"strings"
	"testing"
)

var eaiCases = []struct {
	in  string
	err error
}{
	{"用户@例子.中国", nil},
	{"张三abc@example.com", nil},
	{"用户123@example.com", nil},
	{"ユーザー@例え.テスト", nil},
	{"사용자@example.com", nil},
	{"zhangsan@example.com", nil},
	{"张.三@example.com", nil},
	{"zh\u200Bangsan@example.com", ErrLocalChar}, // zero-width space (Cf)
	{"zhang\u3000san@example.com", ErrLocalChar}, // ideographic space
	{"zhang\u0007san@example.com", ErrLocalChar}, // control character
	{"zhang\uE000san@example.com", ErrLocalChar}, // private use
	{"zhang\xffsan@example.com", ErrLocalChar},   // invalid UTF-8
	{"张..三@example.com", ErrDot},
	{"cafe\u0301@example.com", ErrNotNFC},                       // decomposed e + acute
	{"pаypal@example.com", ErrMixedScript},                      // Latin + Cyrillic a
	{strings.Repeat("用", 22) + "@example.com", ErrLocalTooLong}, // 66 bytes
	{strings.Repeat("用", 21) + "@example.com", nil},             // 63 bytes
}

func TestEAI(t *testing.T) {
	v := New(WithProfile(EAI))
	for _, tc := range eaiCases {
		err := v.Validate(tc.in)
		if !errors.Is(err, tc.err) || (tc.err == nil) != (err == nil) {
			t.Errorf("Validate(%q) = %v, want %v", tc.in, err, tc.err)
		}
	}
}

func TestEAIOptions(t *testing.T) {
	if err := New(WithProfile(EAI), AllowMixedScript()).Validate("pаypal@example.com"); err != nil {
		t.Errorf("AllowMixedScript: %v", err)
	}
	// Non-EAI profiles still reject non-ASCII local parts.
	if err := New(WithProfile(IDN)).Validate("用户@example.com"); !errors.Is(err, ErrNonASCIILocal) {
		t.Errorf("IDN: %v", err)
	}
}

func TestEAINormalize(t *testing.T) {
	v := New(WithProfile(EAI))
	got, err := v.Normalize("cafe\u0301@example.com")
	if err != nil || got != "café@example.com" {
		t.Errorf("Normalize = %q, %v", got, err)
	}
	got, err = v.Normalize("  用户@例子.中国 ")
	if err != nil || got != "用户@xn--fsqu00a.xn--fiqs8s" {
		t.Errorf("Normalize = %q, %v", got, err)
	}
}

var eaiQuotedCases = []struct {
	v   *Validator
	in  string
	err error
}{
	{New(WithProfile(EAI), AllowQuotedLocal()), `"张 三"@example.com`, nil},
	{New(WithProfile(EAI), AllowQuotedLocal()), `"张\三"@example.com`, nil},
	{New(WithProfile(EAI), AllowQuotedLocal()), "\"张\u200B三\"@example.com", ErrLocalChar},
	{New(WithProfile(EAI), AllowQuotedLocal()), "\"张\u3000三\"@example.com", ErrLocalChar},
	{New(AllowQuotedLocal()), `"张 三"@example.com`, ErrNonASCIILocal},
	{New(WithProfile(IDN), AllowQuotedLocal()), `"张 三"@example.com`, ErrNonASCIILocal},
}

func TestEAIQuoted(t *testing.T) {
	for _, tc := range eaiQuotedCases {
		err := tc.v.Validate(tc.in)
		if !errors.Is(err, tc.err) || (tc.err == nil) != (err == nil) {
			t.Errorf("Validate(%q) = %v, want %v", tc.in, err, tc.err)
		}
	}
}

func TestNormalizePosIndexesInput(t *testing.T) {
	in := "  张三 <zhangsan@example.com>"
	_, err := New().Normalize(in)
	var e *Error
	if !errors.As(err, &e) || e.Pos != strings.Index(in, "<") {
		t.Fatalf("got %#v, want Pos %d", err, strings.Index(in, "<"))
	}
}

func TestEAINormalizePosAfterNFC(t *testing.T) {
	// The decomposed local shrinks by one byte under NFC; Pos must still
	// index the offending byte of the original Input.
	in := "  cafe\u0301@example..com"
	_, err := New(WithProfile(EAI)).Normalize(in)
	var e *Error
	if !errors.As(err, &e) || e.Pos < 0 || e.Pos >= len(in) || in[e.Pos] != '.' {
		t.Fatalf("got %#v for %q", err, in)
	}
}
