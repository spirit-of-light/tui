// Package snippets is the snippet engine: a parser for VS Code style
// snippet bodies (tabstops `$1`, `${1:default}`, final stop `$0`, mirrors)
// with `{{name}}` variables, and a Session that keeps the stops in place
// while the user types, moves between them and mirrors edits.
//
// Offsets are rune offsets into text, the unit the vim engine uses.
package snippets

import (
	"sort"
	"strings"
)

// Span is a half-open range of rune offsets.
type Span struct{ Start, End int }

// Stop is one tabstop. At is the primary location, where the cursor goes and
// where the user types; Mirrors are the other places with the same index, which
// repeat whatever is typed at the primary location.
type Stop struct {
	Index   int
	At      Span
	Mirrors []Span
}

// Expansion is a parsed snippet: the plain text to insert and its stops in
// navigation order (1 upward, then 0).
type Expansion struct {
	Text string
	// Stops is never empty. Without an explicit `$0` the final stop sits at the
	// end of the text.
	Stops []Stop
	// Explicit is false when the body declared no tabstop at all.
	Explicit bool
	// Declared counts the distinct tabstop numbers the body wrote, `$0` included.
	Declared int
}

// Parse expands body. resolve supplies the value of a `{{name}}` variable; a
// name it does not know is kept as written. Variable values are inserted as
// literal text and never parsed for stops. Parse never fails: malformed
// syntax is kept as literal text.
//
// Escapes: `\$` and `\}` produce a literal `$` and `}`. Any other backslash is
// kept as written, so LaTeX and Windows paths survive.
func Parse(body string, resolve func(name string) (string, bool)) Expansion {
	p := &parser{src: []rune(body), resolve: resolve}
	p.seq(false)
	return p.finish()
}

type occurrence struct {
	index       int
	at          Span
	placeholder bool
}

type parser struct {
	src     []rune
	pos     int
	out     []rune
	occs    []occurrence
	resolve func(string) (string, bool)
}

// seq copies text until the input ends or, when nested inside a placeholder,
// an unescaped `}`.
func (p *parser) seq(nested bool) {
	for p.pos < len(p.src) {
		r := p.src[p.pos]
		switch {
		case r == '\\' && p.pos+1 < len(p.src) && (p.src[p.pos+1] == '$' || p.src[p.pos+1] == '}'):
			p.out = append(p.out, p.src[p.pos+1])
			p.pos += 2
		case r == '}' && nested:
			return
		case r == '$':
			p.dollar()
		case r == '{' && p.pos+1 < len(p.src) && p.src[p.pos+1] == '{':
			p.variable()
		default:
			p.out = append(p.out, r)
			p.pos++
		}
	}
}

// dollar handles a `$` at p.pos: `$N`, `${N}`, `${N:default}`, or a literal `$`.
func (p *parser) dollar() {
	p.pos++ // the '$'
	if p.pos < len(p.src) && isDigit(p.src[p.pos]) {
		n := p.number()
		p.stopAt(n, false)
		return
	}
	if p.pos < len(p.src) && p.src[p.pos] == '{' {
		save := p.pos
		p.pos++
		if p.pos < len(p.src) && isDigit(p.src[p.pos]) {
			n := p.number()
			if p.pos < len(p.src) && p.src[p.pos] == '}' {
				p.pos++
				p.stopAt(n, false)
				return
			}
			if p.pos < len(p.src) && p.src[p.pos] == ':' {
				p.pos++
				start := len(p.out)
				p.seq(true)
				if p.pos < len(p.src) {
					p.pos++ // the closing '}'; an unclosed placeholder runs to the end
				}
				p.occs = append(p.occs, occurrence{index: n, at: Span{start, len(p.out)}, placeholder: true})
				return
			}
		}
		p.pos = save
	}
	p.out = append(p.out, '$')
}

func (p *parser) stopAt(index int, placeholder bool) {
	at := len(p.out)
	p.occs = append(p.occs, occurrence{index: index, at: Span{at, at}, placeholder: placeholder})
}

func (p *parser) number() int {
	n := 0
	for p.pos < len(p.src) && isDigit(p.src[p.pos]) {
		if n < 1_000_000 {
			n = n*10 + int(p.src[p.pos]-'0')
		}
		p.pos++
	}
	return n
}

// variable handles `{{name}}`. Unknown names and unclosed braces stay literal.
func (p *parser) variable() {
	end := -1
	for i := p.pos + 2; i+1 < len(p.src); i++ {
		if p.src[i] == '}' && p.src[i+1] == '}' {
			end = i
			break
		}
	}
	if end < 0 {
		p.out = append(p.out, '{')
		p.pos++
		return
	}
	name := strings.TrimSpace(string(p.src[p.pos+2 : end]))
	raw := p.src[p.pos : end+2]
	p.pos = end + 2
	if p.resolve != nil {
		if value, ok := p.resolve(name); ok {
			p.out = append(p.out, []rune(value)...)
			return
		}
	}
	p.out = append(p.out, raw...)
}

func (p *parser) finish() Expansion {
	sort.SliceStable(p.occs, func(i, j int) bool { return p.occs[i].at.Start < p.occs[j].at.Start })

	// Mirrors start out holding their primary's default text.
	byIndex := map[int][]int{}
	var order []int
	for i, o := range p.occs {
		if _, seen := byIndex[o.index]; !seen {
			order = append(order, o.index)
		}
		byIndex[o.index] = append(byIndex[o.index], i)
	}
	type edit struct {
		at   Span
		text []rune
	}
	var edits []edit
	for _, idx := range order {
		list := byIndex[idx]
		if len(list) < 2 {
			continue
		}
		prim := primaryOf(p.occs, list)
		content := append([]rune(nil), p.out[p.occs[prim].at.Start:p.occs[prim].at.End]...)
		for _, i := range list {
			if i != prim {
				edits = append(edits, edit{at: p.occs[i].at, text: content})
			}
		}
	}
	sort.SliceStable(edits, func(i, j int) bool { return edits[i].at.Start > edits[j].at.Start })
	for _, e := range edits {
		spans := make([]*Span, 0, len(p.occs))
		for i := range p.occs {
			spans = append(spans, &p.occs[i].at)
		}
		p.out = splice(p.out, spans, e.at.Start, e.at.End, e.text)
	}

	var stops []Stop
	hasFinal := false
	for _, idx := range order {
		list := byIndex[idx]
		prim := primaryOf(p.occs, list)
		st := Stop{Index: idx, At: p.occs[prim].at}
		for _, i := range list {
			if i != prim {
				st.Mirrors = append(st.Mirrors, p.occs[i].at)
			}
		}
		stops = append(stops, st)
		hasFinal = hasFinal || idx == 0
	}
	if !hasFinal {
		end := len(p.out)
		stops = append(stops, Stop{Index: 0, At: Span{end, end}})
	}
	sort.SliceStable(stops, func(i, j int) bool { return navRank(stops[i].Index) < navRank(stops[j].Index) })
	return Expansion{Text: string(p.out), Stops: stops, Explicit: len(p.occs) > 0, Declared: len(order)}
}

// navRank orders stops for navigation: 1 upward, with 0 last.
func navRank(index int) int {
	if index == 0 {
		return 1 << 30
	}
	return index
}

// primaryOf picks the occurrence that receives typing: the first placeholder
// with default text, otherwise the first occurrence.
func primaryOf(occs []occurrence, list []int) int {
	for _, i := range list {
		if occs[i].placeholder {
			return i
		}
	}
	return list[0]
}

// Slice keeps the runes between offsets from and to of the expansion text,
// shifting the stops to match. Used to trim a title line or a trailing newline
// before the body is inserted.
func (e Expansion) Slice(from, to int) Expansion {
	runes := []rune(e.Text)
	from = clamp(from, 0, len(runes))
	to = clamp(to, from, len(runes))
	out := Expansion{Text: string(runes[from:to]), Explicit: e.Explicit, Declared: e.Declared}
	width := to - from
	shift := func(s Span) Span {
		return Span{clamp(s.Start-from, 0, width), clamp(s.End-from, 0, width)}
	}
	for _, st := range e.Stops {
		ns := Stop{Index: st.Index, At: shift(st.At)}
		for _, m := range st.Mirrors {
			ns.Mirrors = append(ns.Mirrors, shift(m))
		}
		out.Stops = append(out.Stops, ns)
	}
	return out
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// splice replaces text[a:b] with ins and moves every span boundary to match.
func splice(text []rune, spans []*Span, a, b int, ins []rune) []rune {
	delta := len(ins) - (b - a)
	for _, s := range spans {
		s.Start = moveBoundary(s.Start, a, b, len(ins), delta, false)
		s.End = moveBoundary(s.End, a, b, len(ins), delta, true)
	}
	out := make([]rune, 0, len(text)+delta)
	out = append(out, text[:a]...)
	out = append(out, ins...)
	return append(out, text[b:]...)
}

// moveBoundary is where an offset ends up after text[a:b] becomes n runes.
// Offsets before a stay put. An offset inside the replaced range lands at its
// new end, and offsets after it move with the edit. The one exception is an
// end boundary at a pure insertion point: it moves past the insertion, so
// typing at the end of a stop extends the stop, while a start boundary stays
// put and text typed before a stop stays outside it.
func moveBoundary(x, a, b, n, delta int, end bool) int {
	switch {
	case x < a:
		return x
	case x == a && !(end && a == b):
		return a
	case x == a:
		return a + n
	case x < b:
		return a + n
	default:
		return x + delta
	}
}
