package emailvalidator

import (
	"math/bits"
	"unicode"
)

// Script bits for the scripts we recognise. Any other script maps to scriptOther.
const (
	scriptLatin = 1 << iota
	scriptHan
	scriptHiragana
	scriptKatakana
	scriptBopomofo
	scriptHangul
	scriptCyrillic
	scriptGreek
	scriptArabic
	scriptHebrew
	scriptThai
	scriptOther
)

var scriptTables = []struct {
	bit uint
	tab *unicode.RangeTable
}{
	{scriptLatin, unicode.Latin},
	{scriptHan, unicode.Han},
	{scriptHiragana, unicode.Hiragana},
	{scriptKatakana, unicode.Katakana},
	{scriptBopomofo, unicode.Bopomofo},
	{scriptHangul, unicode.Hangul},
	{scriptCyrillic, unicode.Cyrillic},
	{scriptGreek, unicode.Greek},
	{scriptArabic, unicode.Arabic},
	{scriptHebrew, unicode.Hebrew},
	{scriptThai, unicode.Thai},
}

// allowedScriptSets lists the multi-script combinations permitted by a
// simplified UTS #39 "Highly Restrictive" level (a single script is always ok).
var allowedScriptSets = []uint{
	scriptLatin | scriptHan | scriptHiragana | scriptKatakana, // Japanese
	scriptLatin | scriptHan | scriptBopomofo,                  // Chinese with Bopomofo
	scriptLatin | scriptHan | scriptHangul,                    // Korean
}

// mixedScript reports whether s mixes scripts in a disallowed way.
// Common and Inherited characters are ignored. Characters of scripts not in
// the list above all count as one "other" script, so two different unlisted
// scripts are not told apart (they are still mixed with any listed script).
func mixedScript(s string) bool {
	var seen uint
	for _, r := range s {
		if unicode.Is(unicode.Common, r) || unicode.Is(unicode.Inherited, r) {
			continue
		}
		bit := uint(scriptOther)
		for _, t := range scriptTables {
			if unicode.Is(t.tab, r) {
				bit = t.bit
				break
			}
		}
		seen |= bit
	}
	if bits.OnesCount(seen) <= 1 {
		return false
	}
	for _, set := range allowedScriptSets {
		if seen&^set == 0 {
			return false
		}
	}
	return true
}
