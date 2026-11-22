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

// foldRune returns the same rune for all runes that match when the case is
// ignored.
func foldRune(r rune) rune {
	if r < 'A' {
		return r
	}
	m := r
	for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
		if f < m {
			m = f
		}
	}
	return m
}

func foldKey(s string) string {
	var b strings.Builder
	for _, r := range s {
		b.WriteRune(foldRune(r))
	}
	return b.String()
}

// wordPrefix returns the longest prefix of w that has only word runes.
func wordPrefix(w string) string {
	for i, r := range w {
		if !isAutolinkWordRune(r) {
			return w[:i]
		}
	}
	return w
}

// autolinkIndex holds the candidate start positions of a text. A candidate
// is a position where the rune before it is not a word rune. The positions
// are in the order of the text.
type autolinkIndex struct {
	text string
	// words maps a case-folded run of word runes to the positions where
	// that run starts.
	words map[string][]int
	// other holds the candidate positions that start with a rune that is
	// not a word rune.
	other []int
	// runs holds the start and the end of each run in words.
	runs [][2]int
	// pairs maps two case-folded runs of word runes, with only spaces and
	// tabs between them, to the positions where the first run starts. It is
	// built on the first use.
	pairs map[string][]int
}

// pairKey is the key of a pair of folded runs in autolinkIndex.pairs.
func pairKey(a, b string) string {
	return a + "\x00" + b
}

// buildPairs fills ix.pairs.
func (ix *autolinkIndex) buildPairs() {
	ix.pairs = map[string][]int{}
	text := ix.text
	for _, run := range ix.runs {
		next := run[1]
		for next < len(text) && (text[next] == ' ' || text[next] == '\t') {
			next++
		}
		if next == run[1] || next == len(text) {
			continue
		}
		end := next
		for end < len(text) {
			r, size := utf8.DecodeRuneInString(text[end:])
			if !isAutolinkWordRune(r) {
				break
			}
			end += size
		}
		if end == next {
			continue
		}
		key := pairKey(foldKey(text[run[0]:run[1]]), foldKey(text[next:end]))
		ix.pairs[key] = append(ix.pairs[key], run[0])
	}
}

// startsWithPair returns the positions where the first word, then a gap, then
// one of the second words can start. It reports false when the words do not
// allow a lookup by pair.
func (ix *autolinkIndex) startsWithPair(first string, second []string) ([]int, bool) {
	if wordPrefix(first) != first || first == "" {
		return nil, false
	}
	for _, w := range second {
		if wordPrefix(w) == "" {
			return nil, false
		}
	}
	if ix.pairs == nil {
		ix.buildPairs()
	}
	key := foldKey(first)
	lists := make([][]int, 0, len(second))
	for _, w := range second {
		lists = append(lists, ix.pairs[pairKey(key, foldKey(wordPrefix(w)))])
	}
	return mergePositions(lists), true
}

// mergePositions joins sorted lists of positions into one sorted list
// without duplicates.
func mergePositions(lists [][]int) []int {
	var out []int
	for _, list := range lists {
		if out == nil {
			out = list
			continue
		}
		merged := make([]int, 0, len(out)+len(list))
		i, j := 0, 0
		for i < len(out) || j < len(list) {
			switch {
			case j == len(list) || (i < len(out) && out[i] < list[j]):
				merged = append(merged, out[i])
				i++
			case i == len(out) || list[j] < out[i]:
				merged = append(merged, list[j])
				j++
			default:
				merged = append(merged, out[i])
				i++
				j++
			}
		}
		out = merged
	}
	return out
}

// newAutolinkIndex reads text once and records its candidate positions.
func newAutolinkIndex(text string) *autolinkIndex {
	ix := &autolinkIndex{text: text, words: map[string][]int{}}
	prevWord := false
	for pos := 0; pos < len(text); {
		r, size := utf8.DecodeRuneInString(text[pos:])
		if isAutolinkWordRune(r) {
			end := pos + size
			for end < len(text) {
				nr, ns := utf8.DecodeRuneInString(text[end:])
				if !isAutolinkWordRune(nr) {
					break
				}
				end += ns
			}
			if !prevWord {
				key := foldKey(text[pos:end])
				ix.words[key] = append(ix.words[key], pos)
				ix.runs = append(ix.runs, [2]int{pos, end})
			}
			pos, prevWord = end, true
			continue
		}
		if !prevWord {
			ix.other = append(ix.other, pos)
		}
		pos, prevWord = pos+size, false
	}
	return ix
}

// startsWith returns, in text order, the positions where one of the words can
// start. The part of a word before its first rune that is not a word rune
// must equal the run of word runes at the position.
func (ix *autolinkIndex) startsWith(words []string) []int {
	lists := make([][]int, 0, len(words))
	for _, w := range words {
		p := wordPrefix(w)
		if p == "" {
			return ix.other
		}
		lists = append(lists, ix.words[foldKey(p)])
	}
	return mergePositions(lists)
}

// autolinkMatches returns the byte span of each whole-word,
// case-insensitive match of the phrase in text. Words before the last word
// must match as written. The last word also matches its word forms.
// Matches do not overlap.
func autolinkMatches(text, phrase string) [][2]int {
	return newAutolinkIndex(text).matches(phrase)
}

// matches is autolinkMatches for the text of the index.
func (ix *autolinkIndex) matches(phrase string) [][2]int {
	words := strings.Fields(phrase)
	if len(words) == 0 {
		return nil
	}
	head := words[:len(words)-1]
	lastForms := autolinkWordForms(words[len(words)-1])

	// A phrase of two words or more looks up its first two words together, so
	// a common first word does not give many positions to verify.
	var candidates []int
	switch {
	case len(head) == 0:
		candidates = ix.startsWith(lastForms)
	default:
		second := lastForms
		if len(head) > 1 {
			second = head[1:2]
		}
		var ok bool
		if candidates, ok = ix.startsWithPair(head[0], second); !ok {
			candidates = ix.startsWith(head[:1])
		}
	}
	var spans [][2]int
	reach := 0
	for _, pos := range candidates {
		if pos < reach {
			continue
		}
		if end := matchPhraseAt(ix.text, pos, head, lastForms); end > 0 {
			spans = append(spans, [2]int{pos, end})
			reach = end
		}
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
