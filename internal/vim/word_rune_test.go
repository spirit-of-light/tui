package vim

import (
	"testing"

	"github.com/ZenNotes/tui/internal/snippet"
)

// The snippet package decides which headings are valid triggers, and the
// editor decides which text before the cursor is a trigger prefix. The two
// word rules must agree.
func TestSnippetWordRuleMatchesEditorWordRule(t *testing.T) {
	check := func(r rune) {
		if got, want := snippet.IsWordRune(r), isWordRune(r); got != want {
			t.Errorf("rune %U %q: snippet.IsWordRune = %v, isWordRune = %v", r, r, got, want)
		}
	}
	for r := rune(0); r <= 0x24F; r++ { // ASCII, Latin-1, Latin Extended
		check(r)
	}
	ranges := [][2]rune{
		{0x0300, 0x036F}, // combining marks
		{0x0590, 0x06FF}, // Hebrew, Arabic and their digits
		{0x0900, 0x097F}, // Devanagari and its digits
		{0x2000, 0x206F}, // general punctuation and spaces
		{0x3000, 0x30FF}, // CJK punctuation, kana
		{0x4E00, 0x4E20}, // a few CJK ideographs
		{0xFF00, 0xFFEF}, // full-width forms and digits
	}
	for _, rg := range ranges {
		for r := rg[0]; r <= rg[1]; r++ {
			check(r)
		}
	}
	for _, r := range "_- \t\n" {
		check(r)
	}
}
