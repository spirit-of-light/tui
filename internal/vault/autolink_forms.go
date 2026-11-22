package vault

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// irregularPlurals pairs a singular with its plural. A pair works in both
// directions.
var irregularPlurals = [][2]string{
	{"matrix", "matrices"},
	{"index", "indices"},
	{"vertex", "vertices"},
	{"appendix", "appendices"},
	{"analysis", "analyses"},
	{"axis", "axes"},
	{"basis", "bases"},
	{"crisis", "crises"},
	{"hypothesis", "hypotheses"},
	{"thesis", "theses"},
	{"criterion", "criteria"},
	{"phenomenon", "phenomena"},
	{"radius", "radii"},
	{"nucleus", "nuclei"},
	{"focus", "foci"},
	{"formula", "formulae"},
	{"child", "children"},
	{"person", "people"},
	{"man", "men"},
	{"woman", "women"},
	{"mouse", "mice"},
	{"foot", "feet"},
	{"tooth", "teeth"},
}

var irregularForms = func() map[string]string {
	m := make(map[string]string, len(irregularPlurals)*2)
	for _, p := range irregularPlurals {
		m[p[0]] = p[1]
		m[p[1]] = p[0]
	}
	return m
}()

func isAutolinkWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func isAllLetters(w string) bool {
	if w == "" {
		return false
	}
	for _, r := range w {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

func isVowel(r rune) bool {
	switch r {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	}
	return false
}

func endsSibilant(w string) bool {
	for _, s := range []string{"s", "x", "z", "ch", "sh"} {
		if strings.HasSuffix(w, s) {
			return true
		}
	}
	return false
}

// autolinkWordForms returns the lower-case forms of a word that count as
// the same word: the word itself, its plural and singular, and the
// irregular pair. A word with a character that is not a letter has only
// its own form.
func autolinkWordForms(word string) []string {
	w := strings.ToLower(word)
	forms := []string{w}
	if !isAllLetters(w) {
		return forms
	}
	add := func(f string) {
		for _, e := range forms {
			if e == f {
				return
			}
		}
		forms = append(forms, f)
	}
	add(w + "s")
	if strings.HasSuffix(w, "s") && len(w) > 1 {
		add(strings.TrimSuffix(w, "s"))
	}
	if endsSibilant(w) {
		add(w + "es")
	}
	if strings.HasSuffix(w, "es") {
		if stem := strings.TrimSuffix(w, "es"); stem != "" && endsSibilant(stem) {
			add(stem)
		}
	}
	if strings.HasSuffix(w, "y") && len(w) > 1 {
		stem := strings.TrimSuffix(w, "y")
		if last, _ := utf8.DecodeLastRuneInString(stem); !isVowel(last) {
			add(stem + "ies")
		}
	}
	if strings.HasSuffix(w, "ies") && len(w) > 3 {
		add(strings.TrimSuffix(w, "ies") + "y")
	}
	if other, ok := irregularForms[w]; ok {
		add(other)
	}
	return forms
}

// prefixFold reports the byte length of the prefix of s that equals p when
// the case is ignored, or -1 when s does not start with p.
func prefixFold(s, p string) int {
	i := 0
	for _, pr := range p {
		if i >= len(s) {
			return -1
		}
		sr, size := utf8.DecodeRuneInString(s[i:])
		if sr != pr && !strings.EqualFold(string(sr), string(pr)) {
			return -1
		}
		i += size
	}
	return i
}

// autolinkMatches returns the byte span of each whole-word,
// case-insensitive match of the phrase in text. Words before the last word
// must match as written. The last word also matches its word forms.
// Matches do not overlap.
func autolinkMatches(text, phrase string) [][2]int {
	words := strings.Fields(phrase)
	if len(words) == 0 {
		return nil
	}
	head := words[:len(words)-1]
	lastForms := autolinkWordForms(words[len(words)-1])

	var spans [][2]int
	pos := 0
	for pos < len(text) {
		_, size := utf8.DecodeRuneInString(text[pos:])
		if pos > 0 {
			prev, _ := utf8.DecodeLastRuneInString(text[:pos])
			if isAutolinkWordRune(prev) {
				pos += size
				continue
			}
		}
		if end := matchPhraseAt(text, pos, head, lastForms); end > 0 {
			spans = append(spans, [2]int{pos, end})
			pos = end
			continue
		}
		pos += size
	}
	return spans
}

func matchPhraseAt(text string, start int, head []string, lastForms []string) int {
	pos := start
	for _, w := range head {
		n := prefixFold(text[pos:], w)
		if n < 0 {
			return -1
		}
		pos += n
		gap := 0
		for pos+gap < len(text) && (text[pos+gap] == ' ' || text[pos+gap] == '\t') {
			gap++
		}
		if gap == 0 {
			return -1
		}
		pos += gap
	}
	best := -1
	for _, f := range lastForms {
		n := prefixFold(text[pos:], f)
		if n < 0 {
			continue
		}
		end := pos + n
		if end < len(text) {
			next, _ := utf8.DecodeRuneInString(text[end:])
			if isAutolinkWordRune(next) {
				continue
			}
		}
		if end > best {
			best = end
		}
	}
	return best
}
