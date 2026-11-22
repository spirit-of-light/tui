package vim

import (
	"strings"

	"github.com/ZenNotes/tui/internal/snippet"
)

// snippetStop is one tabstop of a running snippet session.
type snippetStop struct {
	number int
	// ranges holds each occurrence, in document order.
	ranges [][2]Pos
	// parent is the index of the stop that encloses the first occurrence,
	// or -1.
	parent int
	// unchanged is true while the placeholder text is the text that the
	// snippet put there.
	unchanged bool
}

// snippetSession is one expanded snippet that the user still fills in.
type snippetSession struct {
	stops []snippetStop
	// cur is the index of the current stop.
	cur int
	// span is the range of the whole snippet text.
	span [2]Pos
}

// ExpandSnippet inserts the expansion of body at the cursor and goes to
// insert mode. The vars function gives the values of the template
// variables. A session starts when the body has a tabstop to visit.
func (e *Editor) ExpandSnippet(body string, vars snippet.Resolver) {
	line := e.buf.Line(e.cursor.Line)
	res := snippet.Expand(body, snippet.Options{
		Indent:   string(leadingWhitespace(line)),
		Unit:     strings.Repeat(" ", e.opts.TabSize),
		Resolver: vars,
	})

	if e.mode != ModeInsert && e.mode != ModeReplace {
		e.startInsert(e.cursor)
	} else {
		e.beginChange()
	}
	start := e.buf.clampInsert(e.cursor)
	end := e.buf.InsertText(start, res.Text)
	e.desiredCol = -1

	runes := []rune(res.Text)
	at := func(off int) Pos {
		p := start
		for _, r := range runes[:min(off, len(runes))] {
			if r == '\n' {
				p = Pos{p.Line + 1, 0}
			} else {
				p.Col++
			}
		}
		return p
	}

	s := &snippetSession{span: [2]Pos{start, end}}
	numbered := 0
	finals := 0
	for _, st := range res.Stops {
		ss := snippetStop{number: st.Number, parent: st.Parent}
		for _, r := range st.Ranges {
			ss.ranges = append(ss.ranges, [2]Pos{at(r[0]), at(r[1])})
		}
		first := ss.ranges[0]
		ss.unchanged = first[0] != first[1]
		s.stops = append(s.stops, ss)
		if st.Number == 0 {
			if !res.ImplicitEnd {
				finals = len(st.Ranges)
			}
		} else {
			numbered++
		}
	}

	// The cursor goes to the lowest numbered stop, else the first $0, else
	// the end of the text.
	target := end
	if numbered > 0 {
		s.cur = 0
		target = s.stops[0].ranges[0][0]
	} else {
		s.cur = len(s.stops) - 1
		if finals > 0 {
			target = s.stops[s.cur].ranges[0][0]
		}
	}
	e.cursor = e.buf.clampInsert(target)

	e.sessions = nil
	if numbered > 0 || finals > 1 {
		e.sessions = []*snippetSession{s}
	}
}

// SnippetActive reports whether a snippet session is running.
func (e *Editor) SnippetActive() bool { return len(e.sessions) > 0 }

func (e *Editor) topSession() *snippetSession {
	if len(e.sessions) == 0 {
		return nil
	}
	return e.sessions[len(e.sessions)-1]
}

// SnippetStatus gives the position for the status line: the number of the
// current tabstop and the count of numbered tabstops. AtEnd is true at the
// final tabstop. Ok is false when no session runs.
func (e *Editor) SnippetStatus() (current, total int, atEnd bool, ok bool) {
	s := e.topSession()
	if s == nil {
		return 0, 0, false, false
	}
	for _, st := range s.stops {
		if st.number != 0 {
			total++
		}
	}
	if s.stops[s.cur].number == 0 {
		return total, total, true, true
	}
	return s.cur + 1, total, false, true
}

// SnippetPlaceholder gives the column ranges [start, end) on a line of the
// current placeholder, while its text is unchanged.
func (e *Editor) SnippetPlaceholder(line int) [][2]int {
	s := e.topSession()
	if s == nil {
		return nil
	}
	cur := s.stops[s.cur]
	if !cur.unchanged {
		return nil
	}
	var out [][2]int
	for _, r := range cur.ranges {
		if r[0] == r[1] || line < r[0].Line || line > r[1].Line {
			continue
		}
		from, to := 0, e.buf.LineLen(line)
		if line == r[0].Line {
			from = r[0].Col
		}
		if line == r[1].Line {
			to = r[1].Col
		}
		if from < to {
			out = append(out, [2]int{from, to})
		}
	}
	return out
}
