package emailvalidator

import "testing"

var mixedScriptCases = []struct {
	in   string
	want bool
}{
	{"abc", false},
	{"张三abc", false},
	{"かな漢字abc", false},
	{"ㄅ中文", false},
	{"한글漢字", false},
	{"用户123", false},
	{"привет", false},
	{"cafe\u0301", false}, // Inherited mark ignored
	{"pаypal", true},
	{"中文한글かな", true},
	{"abcΩ", true},
	{"ㄅ漢かな", true}, // Bopomofo + Han + Hiragana is not an allowed set
	{"abcक", true}, // Latin + unlisted script
}

func TestMixedScript(t *testing.T) {
	for _, tc := range mixedScriptCases {
		if got := mixedScript(tc.in); got != tc.want {
			t.Errorf("mixedScript(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
