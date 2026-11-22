package vim

// This file keeps the ranges of the snippet sessions in step with the
// buffer. The editor does not know which feature changed the text (auto
// pairs, a text replacement, list continuation, a completion), so it
// compares the lines before and after each key and moves every range by the
// one region that differs.

// textEdit says that the old text in [a, b) became the new text in [a, c).
type textEdit struct {
	a, b, c Pos
}

// owners names, for each session, the occurrence of its current stop that
// holds the cursor. Text typed there belongs to that stop.
type snippetOwners map[*snippetSession][2]int

// snippetKind tells what a key does at an unchanged placeholder.
type snippetKind int

const (
	snippetKeyOther snippetKind = iota
	// snippetKeyAdds is a key that adds text.
	snippetKeyAdds
	// snippetKeyDeletes is Backspace or Delete.
	snippetKeyDeletes
)

// tracked runs an edit and then moves the session ranges by what it changed.
func (e *Editor) tracked(run func()) {
	if len(e.sessions) == 0 {
		run()
		return
	}
	owners := e.snippetOwners()
	before := e.buf.Snapshot()
	cursor := e.cursor
	run()
	e.snippetTrack(before, cursor, owners)
	e.snippetCheckRange()
}

// snippetKey runs one key while a session is active. At an unchanged
// placeholder, a key that adds text first deletes the placeholder, and
// Backspace or Delete delete it and do nothing else.
func (e *Editor) snippetKey(k Key) bool {
	if s := e.topSession(); s != nil && e.snippetTyping() {
		st := &s.stops[s.cur]
		if st.unchanged && e.cursorInStop(st) {
			switch e.snippetKeyKind(k) {
			case snippetKeyDeletes:
				e.ClearMessage()
				e.deleteSnippetPlaceholder(s)
				e.ensureCursorVisible()
				e.snippetCheckRange()
				return true
			case snippetKeyAdds:
				e.deleteSnippetPlaceholder(s)
			}
		}
	}
	handled := true
	e.tracked(func() { handled = e.dispatchKey(k) })
	return handled
}

// snippetTyping is true when the next key goes to the text, not to a Vim
// command.
func (e *Editor) snippetTyping() bool {
	return (e.mode == ModeInsert || e.mode == ModeReplace) && !e.ins.ctrlO
}

func (e *Editor) snippetKeyKind(k Key) snippetKind {
	if e.ins.ctrlR {
		if k.Printable() {
			return snippetKeyAdds
		}
		return snippetKeyOther
	}
	if e.ins.ctrlV {
		if k.Printable() || k.Is("tab") || k.Is("enter") {
			return snippetKeyAdds
		}
		return snippetKeyOther
	}
	switch {
	case k.Is("backspace"), k.IsCtrl('h'), k.Is("delete"):
		return snippetKeyDeletes
	case k.Printable(), k.Is("enter"), k.Is("tab"):
		return snippetKeyAdds
	case e.opts.VimEnabled && (k.IsCtrl('j') || k.IsCtrl('m') || k.IsCtrl('a')):
		return snippetKeyAdds
	}
	return snippetKeyOther
}

func (e *Editor) cursorInStop(st *snippetStop) bool {
	return occurrenceAt(st, e.cursor) >= 0
}

// occurrenceAt gives the index of the first occurrence of a stop that holds
// p, the ends included, or -1.
func occurrenceAt(st *snippetStop, p Pos) int {
	for j, r := range st.ranges {
		if !p.Less(r[0]) && !r[1].Less(p) {
			return j
		}
	}
	return -1
}

func (e *Editor) snippetOwners() snippetOwners {
	out := snippetOwners{}
	for _, s := range e.sessions {
		if j := occurrenceAt(&s.stops[s.cur], e.cursor); j >= 0 {
			out[s] = [2]int{s.cur, j}
		}
	}
	return out
}

// deleteSnippetPlaceholder deletes the text of the current stop in all of
// its occurrences. The cursor goes to the start of the occurrence that held
// it.
func (e *Editor) deleteSnippetPlaceholder(s *snippetSession) {
	e.beginChange()
	st := &s.stops[s.cur]
	at := max(occurrenceAt(st, e.cursor), 0)
	for j := len(st.ranges) - 1; j >= 0; j-- {
		r := st.ranges[j]
		if r[0] == r[1] {
			continue
		}
		e.buf.DeleteRange(r[0], r[1])
		e.shiftSnippets(textEdit{a: r[0], b: r[1], c: r[0]}, nil)
	}
	st.unchanged = false
	pos := st.ranges[at][0]
	e.dropNestedStops(s)
	e.cursor = e.buf.clampInsert(pos)
	e.desiredCol = -1
}

// dropNestedStops removes the stops that lie inside the current stop. The
// text that holds them is gone, so they leave the session.
func (e *Editor) dropNestedStops(s *snippetSession) {
	inside := make([]bool, len(s.stops))
	found := false
	for i := range s.stops {
		for p := s.stops[i].parent; p >= 0 && p < len(s.stops); p = s.stops[p].parent {
			if p == s.cur {
				inside[i] = true
				found = true
				break
			}
		}
	}
	if !found {
		return
	}
	index := make([]int, len(s.stops))
	kept := s.stops[:0:0]
	for i, st := range s.stops {
		if inside[i] {
			index[i] = -1
			continue
		}
		index[i] = len(kept)
		kept = append(kept, st)
	}
	for i := range kept {
		if kept[i].parent >= 0 {
			kept[i].parent = index[kept[i].parent]
		}
	}
	s.cur = index[s.cur]
	s.stops = kept
}

// snippetTrack moves the session ranges after the buffer changed from
// before. A key that moved the cursor and left the text alone turns the
// current placeholder into usual text.
func (e *Editor) snippetTrack(before [][]rune, cursorBefore Pos, owners snippetOwners) {
	ed, changed := diffLines(before, e.buf.lines, e.cursor)
	if !changed {
		if e.cursor != cursorBefore {
			if s := e.topSession(); s != nil {
				s.stops[s.cur].unchanged = false
			}
		}
		return
	}
	e.shiftSnippets(ed, owners)
	e.mirrorSnippets(owners)
}

// mirrorSnippets copies the text of the occurrence that the user edited to
// the other occurrences of the same stop. The copies are plain edits, so they
// join the change that is open.
func (e *Editor) mirrorSnippets(owners snippetOwners) {
	for _, s := range e.sessions {
		own, ok := owners[s]
		if !ok {
			continue
		}
		st := &s.stops[own[0]]
		if len(st.ranges) < 2 {
			continue
		}
		src := st.ranges[own[1]]
		text := e.buf.Range(src[0], src[1])
		for k := range st.ranges {
			if k == own[1] {
				continue
			}
			r := st.ranges[k]
			if e.buf.Range(r[0], r[1]) == text {
				continue
			}
			e.buf.DeleteRange(r[0], r[1])
			end := e.buf.InsertText(r[0], text)
			ed := textEdit{a: r[0], b: r[1], c: end}
			e.shiftSnippets(ed, snippetOwners{s: {own[0], k}})
			if k < own[1] && !e.cursor.Less(ed.b) {
				e.cursor = ed.shift(e.cursor)
			}
		}
	}
}

// shiftSnippets moves every range of every session by one edit.
func (e *Editor) shiftSnippets(ed textEdit, owners snippetOwners) {
	for _, s := range e.sessions {
		s.span = shiftRange(s.span, ed, true, false)
		own, hasOwner := owners[s]
		for i := range s.stops {
			st := &s.stops[i]
			for j, r := range st.ranges {
				mine := hasOwner && own == [2]int{i, j}
				if touchesEdit(r, ed, mine) {
					st.unchanged = false
				}
				st.ranges[j] = shiftRange(r, ed, mine, i < s.cur)
			}
		}
	}
}

// touchesEdit says that an edit changes the text inside a range.
func touchesEdit(r [2]Pos, ed textEdit, own bool) bool {
	if own {
		return true
	}
	if ed.a == ed.b {
		return r[0].Less(ed.a) && ed.a.Less(r[1])
	}
	return r[0].Less(ed.b) && ed.a.Less(r[1])
}

// shiftRange gives the range after an edit. The owner range grows when the
// text is typed at either end of it. Another range does not grow at its end,
// and the text typed at its start goes before it. An empty range at the
// point of an insertion stays before the text when stayEmpty is true.
func shiftRange(r [2]Pos, ed textEdit, own, stayEmpty bool) [2]Pos {
	a, b := ed.a, ed.b
	if a == b {
		switch {
		case own:
			if a.Less(r[0]) {
				r[0] = ed.shift(r[0])
			}
			if !r[1].Less(a) {
				r[1] = ed.shift(r[1])
			}
		case r[0] == a:
			if r[0] == r[1] && stayEmpty {
				return r
			}
			r[0], r[1] = ed.shift(r[0]), ed.shift(r[1])
		default:
			if a.Less(r[0]) {
				r[0] = ed.shift(r[0])
			}
			if a.Less(r[1]) {
				r[1] = ed.shift(r[1])
			}
		}
		return r
	}
	switch {
	case !r[0].Less(b):
		r[0] = ed.shift(r[0])
	case a.Less(r[0]):
		r[0] = a
	}
	switch {
	case !r[1].Less(b):
		r[1] = ed.shift(r[1])
	case a.Less(r[1]):
		r[1] = ed.c
	}
	return r
}

// shift moves a position that lies at or after the end of the old text.
func (ed textEdit) shift(p Pos) Pos {
	if p.Line == ed.b.Line {
		return Pos{ed.c.Line, ed.c.Col + p.Col - ed.b.Col}
	}
	return Pos{p.Line + ed.c.Line - ed.b.Line, p.Col}
}

// snippetCheckRange ends each session whose range does not hold the cursor
// any more. The end of the range counts as inside.
func (e *Editor) snippetCheckRange() {
	for s := e.topSession(); s != nil; s = e.topSession() {
		if !e.cursor.Less(s.span[0]) && !s.span[1].Less(e.cursor) {
			return
		}
		e.popSession()
	}
}

// diffLines finds the one region that differs between two line lists. Lines
// that no edit touched share their slice, so most lines compare at once.
// Among regions that describe the change equally well, it takes the one
// that ends at the cursor.
func diffLines(old, cur [][]rune, cursor Pos) (textEdit, bool) {
	n, m := len(old), len(cur)
	i := 0
	for i < n && i < m && sameRunes(old[i], cur[i]) {
		i++
	}
	if i == n && i == m {
		return textEdit{}, false
	}
	j := 0
	for j < n-i && j < m-i && sameRunes(old[n-1-j], cur[m-1-j]) {
		j++
	}
	// A region needs a line on each side, so that the text joined by
	// newlines describes the change.
	if i+j == n || i+j == m {
		if i > 0 {
			i--
		} else {
			j--
		}
	}
	oldT := joinLines(old[i : n-j])
	newT := joinLines(cur[i : m-j])
	lo, ln := len(oldT), len(newT)
	shortest := min(lo, ln)

	p0 := 0
	for p0 < shortest && oldT[p0] == newT[p0] {
		p0++
	}
	s0 := 0
	for s0 < shortest && oldT[lo-1-s0] == newT[ln-1-s0] {
		s0++
	}
	s := min(s0, shortest-min(p0, shortest))
	if cursor.Line >= i && cursor.Line < m-j {
		off := cursor.Col
		for l := i; l < cursor.Line; l++ {
			off += len(cur[l]) + 1
		}
		off = min(off, ln)
		s = max(0, min(ln-off, min(s0, shortest)))
	}
	p := min(p0, shortest-s)
	return textEdit{
		a: textPos(i, oldT, p),
		b: textPos(i, oldT, lo-s),
		c: textPos(i, newT, ln-s),
	}, true
}

func sameRunes(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 || &a[0] == &b[0] {
		return true
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func joinLines(lines [][]rune) []rune {
	var out []rune
	for i, l := range lines {
		if i > 0 {
			out = append(out, '\n')
		}
		out = append(out, l...)
	}
	return out
}

// textPos gives the position of an offset in text that starts at column 0
// of a line.
func textPos(line int, text []rune, off int) Pos {
	p := Pos{Line: line}
	for _, r := range text[:off] {
		if r == '\n' {
			p = Pos{p.Line + 1, 0}
		} else {
			p.Col++
		}
	}
	return p
}
