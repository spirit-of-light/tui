package snippets

import "sort"

// Replacement is an edit the caller must apply to the editor to keep mirrors
// in step with their primary: replace the runes in [Start, End) with Text. A
// slice of replacements is ordered so each one is valid in the text left by
// the ones before it.
type Replacement struct {
	Start, End int
	Text       string
}

// Session tracks one expansion while the user works in it. It holds the text
// as of its last sync and the stops' positions in that text.
type Session struct {
	stops []Stop
	// navigable says there is something to do in the expansion: two or more
	// tabstops to step through, or a mirror to keep in step. A lone tabstop
	// with no mirror is only placed, not a session.
	navigable bool
	cur       int
	text      []rune
	// fresh says the current stop still holds the default text it arrived with,
	// so the next typed character replaces it. Any edit clears it.
	fresh bool
}

// NewSession starts a session for exp inserted at offset base of text, the
// full buffer text after the expansion has been inserted. The cursor starts at
// the first stop.
func NewSession(exp Expansion, base int, text string) *Session {
	s := &Session{text: []rune(text), navigable: exp.Declared >= 2 || hasMirrors(exp)}
	for _, st := range exp.Stops {
		ns := Stop{Index: st.Index, At: Span{st.At.Start + base, st.At.End + base}}
		for _, m := range st.Mirrors {
			ns.Mirrors = append(ns.Mirrors, Span{m.Start + base, m.End + base})
		}
		s.stops = append(s.stops, ns)
	}
	s.fresh = s.hasDefault()
	return s
}

// hasMirrors reports whether any stop is repeated in the expansion.
func hasMirrors(exp Expansion) bool {
	for _, st := range exp.Stops {
		if len(st.Mirrors) > 0 {
			return true
		}
	}
	return false
}

// Active reports whether the session has something to do. A body with one
// tabstop and nothing mirrored, such as a template's cursor, does not.
func (s *Session) Active() bool { return s != nil && s.navigable && len(s.stops) > 1 }

// Cursor is the offset of the current stop.
func (s *Session) Cursor() int { return s.stops[s.cur].At.Start }

// Next moves to the following stop. It reports false, and does not move, when
// the current stop is the last: the session is then over.
func (s *Session) Next() (int, bool) {
	if s.cur+1 >= len(s.stops) {
		return 0, false
	}
	s.cur++
	s.fresh = s.hasDefault()
	return s.Cursor(), true
}

// Prev moves to the preceding stop. It reports false at the first stop.
func (s *Session) Prev() (int, bool) {
	if s.cur == 0 {
		return 0, false
	}
	s.cur--
	s.fresh = s.hasDefault()
	return s.Cursor(), true
}

// Replaces returns the current stop's default text when typing should replace
// it: the stop has not been edited since the cursor reached it.
func (s *Session) Replaces() (Span, bool) {
	at := s.stops[s.cur].At
	return at, s.fresh && at.End > at.Start
}

func (s *Session) hasDefault() bool {
	at := s.stops[s.cur].At
	return at.End > at.Start
}

// Sync brings the session up to date with newText, the buffer text after the
// user's last change. It moves every stop to follow the edit. If the edit
// changed a stop with mirrors, it returns the replacements that copy the
// stop's text to its mirrors; the caller applies them in order and then maps
// the cursor with MapOffset. It returns nil when nothing changed.
func (s *Session) Sync(newText string) []Replacement {
	nt := []rune(newText)
	if string(nt) == string(s.text) {
		return nil
	}
	s.fresh = false
	a, b, ins := diff(s.text, nt)

	// The edit belongs to the innermost stop that contains it.
	owner := -1
	for i, st := range s.stops {
		if len(st.Mirrors) == 0 || st.At.Start > a || b > st.At.End {
			continue
		}
		if owner < 0 || st.At.End-st.At.Start < s.stops[owner].At.End-s.stops[owner].At.Start {
			owner = i
		}
	}
	s.apply(a, b, ins)
	if owner < 0 {
		return nil
	}

	content := append([]rune(nil), s.text[s.stops[owner].At.Start:s.stops[owner].At.End]...)
	mirrors := append([]Span(nil), s.stops[owner].Mirrors...)
	sort.SliceStable(mirrors, func(i, j int) bool { return mirrors[i].Start > mirrors[j].Start })
	var reps []Replacement
	// Right to left: each replacement only moves spans after it, and those are
	// already done, so the remaining mirror spans stay valid.
	for _, m := range mirrors {
		if string(s.text[m.Start:m.End]) == string(content) {
			continue
		}
		reps = append(reps, Replacement{Start: m.Start, End: m.End, Text: string(content)})
		s.apply(m.Start, m.End, content)
	}
	return reps
}

// MapOffset maps an offset in the text before reps to the text after them.
func MapOffset(x int, reps []Replacement) int {
	for _, r := range reps {
		n := len([]rune(r.Text))
		x = moveBoundary(x, r.Start, r.End, n, n-(r.End-r.Start), false)
	}
	return x
}

// apply replaces text[a:b] with ins, moving every stop boundary.
func (s *Session) apply(a, b int, ins []rune) {
	var spans []*Span
	for i := range s.stops {
		spans = append(spans, &s.stops[i].At)
		for j := range s.stops[i].Mirrors {
			spans = append(spans, &s.stops[i].Mirrors[j])
		}
	}
	s.text = splice(s.text, spans, a, b, ins)
}

// diff finds the one region that differs between old and nw: old[a:b] became
// ins. Common prefix and suffix are kept, which is exact for a single edit.
func diff(old, nw []rune) (a, b int, ins []rune) {
	p := 0
	for p < len(old) && p < len(nw) && old[p] == nw[p] {
		p++
	}
	suffix := 0
	for suffix < len(old)-p && suffix < len(nw)-p && old[len(old)-1-suffix] == nw[len(nw)-1-suffix] {
		suffix++
	}
	return p, len(old) - suffix, nw[p : len(nw)-suffix]
}
