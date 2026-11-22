package snippet

import (
	"sort"
	"strings"
)

// Resolver gives the value of a template variable. The name is title, date,
// time or week. HasArg is true when the source had a ":" after the name, even
// when the argument is empty. It returns false when the variable has no
// value, and the source text of the variable then stays in the output.
type Resolver func(name, arg string, hasArg bool) (string, bool)

// Options sets how Expand places the text of a body.
type Options struct {
	// Indent is added after each newline of the body. It is the leading
	// blanks of the line that holds the cursor.
	Indent string
	// Unit is the text of one indent level. A tab character in the body
	// becomes Unit.
	Unit string
	// Resolver gives the variable values. A nil Resolver gives none.
	Resolver Resolver
}

// Stop is one tabstop of an expanded body.
type Stop struct {
	// Number is the tabstop number. Number 0 is the final position.
	Number int
	// Ranges holds each occurrence as rune offsets [start, end) in the
	// expanded text, in document order. The first range is the first
	// occurrence.
	Ranges [][2]int
	// Parent is the index in Result.Stops of the stop that encloses the
	// first occurrence, or -1 when no stop encloses it.
	Parent int
}

// Result is an expanded body.
type Result struct {
	// Text is the text to insert.
	Text string
	// Stops holds the numbered stops in ascending order of number, then
	// the final stop (number 0), which is always present.
	Stops []Stop
	// ImplicitEnd is true when the body has no $0. The last Stop is then
	// an empty stop at the end of Text.
	ImplicitEnd bool
}

// Expand parses a body and expands it. The first occurrence of a tabstop
// that has a placeholder gives the text of all occurrences of that tabstop.
// A stop that is nested in a later occurrence (a copy) does not appear in
// the result.
func Expand(body string, opt Options) Result {
	return ExpandNodes(Parse(body), opt)
}

// ExpandNodes expands a parsed body. See Expand.
func ExpandNodes(nodes []Node, opt Options) Result {
	x := &expander{
		opt:     opt,
		source:  map[int]int{},
		content: map[int][]Node{},
		seen:    map[int]int{},
		parent:  map[int]int{},
		ranges:  map[int][][2]int{},
		plain:   map[int]string{},
	}
	x.findSources(nodes, map[int]int{})
	x.emit(nodes)

	res := Result{Text: string(x.out)}
	nums := make([]int, 0, len(x.ranges)+1)
	for n := range x.ranges {
		nums = append(nums, n)
	}
	sort.Slice(nums, func(i, j int) bool {
		a, b := nums[i], nums[j]
		if (a == 0) != (b == 0) {
			return b == 0
		}
		return a < b
	})
	if _, ok := x.ranges[0]; !ok {
		end := len(x.out)
		x.ranges[0] = [][2]int{{end, end}}
		x.parent[0] = -1
		nums = append(nums, 0)
		res.ImplicitEnd = true
	}
	index := make(map[int]int, len(nums))
	for i, n := range nums {
		index[n] = i
	}
	for _, n := range nums {
		parent := -1
		if p, ok := x.parent[n]; ok && p >= 0 {
			if pi, ok := index[p]; ok {
				parent = pi
			}
		}
		res.Stops = append(res.Stops, Stop{Number: n, Ranges: x.ranges[n], Parent: parent})
	}
	return res
}

type expander struct {
	opt Options
	out []rune
	// plainMode makes emit write text only and record no stop.
	plainMode bool
	// source holds, for each number, the occurrence index that gives the
	// text of all occurrences. content holds its children.
	source  map[int]int
	content map[int][]Node
	seen    map[int]int
	parent  map[int]int
	ranges  map[int][][2]int
	plain   map[int]string
	stack   []int
}

// findSources picks, for each tabstop number, the first occurrence that has
// a placeholder. It counts occurrences the same way as emit, and it enters
// only the occurrence that emit enters.
func (x *expander) findSources(nodes []Node, count map[int]int) {
	for _, n := range nodes {
		if n.Kind != KindTabstop {
			continue
		}
		idx := count[n.Number]
		count[n.Number]++
		if _, ok := x.source[n.Number]; !ok && len(n.Children) > 0 {
			x.source[n.Number] = idx
			x.content[n.Number] = n.Children
			x.findSources(n.Children, count)
		}
	}
}

func (x *expander) emit(nodes []Node) {
	for _, n := range nodes {
		switch n.Kind {
		case KindText:
			x.write(n.Text, true)
		case KindVariable:
			x.writeVariable(n)
		case KindTabstop:
			x.tabstop(n)
		}
	}
}

func (x *expander) writeVariable(n Node) {
	if x.opt.Resolver != nil {
		if v, ok := x.opt.Resolver(n.Name, n.Arg, n.HasArg); ok {
			x.write(v, false)
			return
		}
	}
	x.write(n.Raw, false)
}

// write adds text. Each newline gets the indent. When tabs is true, a tab
// character becomes one indent level.
func (x *expander) write(s string, tabs bool) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	for _, r := range s {
		switch {
		case r == '\n':
			x.out = append(x.out, '\n')
			x.out = append(x.out, []rune(x.opt.Indent)...)
		case r == '\t' && tabs:
			x.out = append(x.out, []rune(x.opt.Unit)...)
		default:
			x.out = append(x.out, r)
		}
	}
}

func (x *expander) tabstop(n Node) {
	if x.plainMode {
		x.emit(n.Children)
		return
	}
	idx := x.seen[n.Number]
	x.seen[n.Number]++
	start := len(x.out)
	src, hasSource := x.source[n.Number]
	switch {
	case !hasSource:
		// No occurrence has a placeholder, so the stop is empty.
	case idx == src:
		x.stack = append(x.stack, n.Number)
		x.emit(n.Children)
		x.stack = x.stack[:len(x.stack)-1]
	default:
		x.out = append(x.out, []rune(x.sourceText(n.Number))...)
	}
	if _, ok := x.ranges[n.Number]; !ok {
		p := -1
		if len(x.stack) > 0 {
			p = x.stack[len(x.stack)-1]
		}
		x.parent[n.Number] = p
	}
	x.ranges[n.Number] = append(x.ranges[n.Number], [2]int{start, len(x.out)})
}

// sourceText gives the plain text of the source occurrence of a stop.
func (x *expander) sourceText(number int) string {
	if s, ok := x.plain[number]; ok {
		return s
	}
	sub := &expander{opt: x.opt, plainMode: true}
	sub.emit(x.content[number])
	s := string(sub.out)
	x.plain[number] = s
	return s
}
