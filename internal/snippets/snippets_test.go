package snippets

import (
	"reflect"
	"strings"
	"testing"
)

func noVars(string) (string, bool) { return "", false }

func stopsOf(e Expansion) []int {
	var out []int
	for _, s := range e.Stops {
		out = append(out, s.Index)
	}
	return out
}

func TestParseTabstopsAndFinalStop(t *testing.T) {
	e := Parse("${1:foo} and ${2:bar}$0", noVars)
	if e.Text != "foo and bar" {
		t.Fatalf("text %q", e.Text)
	}
	want := []Stop{
		{Index: 1, At: Span{0, 3}},
		{Index: 2, At: Span{8, 11}},
		{Index: 0, At: Span{11, 11}},
	}
	if !reflect.DeepEqual(e.Stops, want) {
		t.Fatalf("stops %+v", e.Stops)
	}
	if !e.Explicit {
		t.Fatal("explicit stops not reported")
	}
}

func TestParseOrdersNumericallyNotByPosition(t *testing.T) {
	e := Parse("$3 $2 $1 $0", noVars)
	if got := stopsOf(e); !reflect.DeepEqual(got, []int{1, 2, 3, 0}) {
		t.Fatalf("order %v", got)
	}
}

func TestImplicitFinalStopAtEnd(t *testing.T) {
	e := Parse("x$1y", noVars)
	if e.Text != "xy" {
		t.Fatalf("text %q", e.Text)
	}
	last := e.Stops[len(e.Stops)-1]
	if last.Index != 0 || last.At != (Span{2, 2}) {
		t.Fatalf("final stop %+v", last)
	}
}

func TestMirrorsShareAnIndex(t *testing.T) {
	e := Parse("$1 and $1$0", noVars)
	if len(e.Stops) != 2 {
		t.Fatalf("stops %+v", e.Stops)
	}
	st := e.Stops[0]
	if st.At != (Span{0, 0}) || !reflect.DeepEqual(st.Mirrors, []Span{{5, 5}}) {
		t.Fatalf("stop %+v", st)
	}
}

func TestMirrorTakesPrimaryDefault(t *testing.T) {
	e := Parse("${1:foo} $1", noVars)
	if e.Text != "foo foo" {
		t.Fatalf("text %q", e.Text)
	}
	st := e.Stops[0]
	if st.At != (Span{0, 3}) || !reflect.DeepEqual(st.Mirrors, []Span{{4, 7}}) {
		t.Fatalf("stop %+v", st)
	}
}

func TestMirrorBeforePlaceholderIsFilledAndShifted(t *testing.T) {
	e := Parse("$1 ${1:foo}$0", noVars)
	if e.Text != "foo foo" {
		t.Fatalf("text %q", e.Text)
	}
	st := e.Stops[0]
	if st.At != (Span{4, 7}) || !reflect.DeepEqual(st.Mirrors, []Span{{0, 3}}) {
		t.Fatalf("stop %+v", st)
	}
}

func TestEscapesAndLiteralDollars(t *testing.T) {
	cases := map[string]string{
		`\$5 and \}`:           "$5 and }",
		`a\\b`:                 `a\\b`,
		`cost $ and ${x} $$`:   `cost $ and ${x} $$`,
		`$`:                    `$`,
		`\latex{x}`:            `\latex{x}`,
		"${1:a \\} b}":         "a } b",
		"${1:unclosed":         "unclosed",
		"{{ unclosed and $1x}": "{{ unclosed and x}",
	}
	for in, want := range cases {
		if got := Parse(in, noVars).Text; got != want {
			t.Errorf("Parse(%q) = %q, want %q", in, got, want)
		}
	}
	if Parse(`\$5`, noVars).Explicit {
		t.Fatal("escaped dollar counted as a stop")
	}
}

func TestVariablesAreLiteralAndNeverParsed(t *testing.T) {
	vars := func(name string) (string, bool) {
		if name == "title" {
			return "T$1}", true
		}
		return "", false
	}
	e := Parse("# {{title}}\n$1{{nope}}", vars)
	if e.Text != "# T$1}\n{{nope}}" {
		t.Fatalf("text %q", e.Text)
	}
	if got := e.Stops[0].At; got != (Span{len([]rune("# T$1}\n")), len([]rune("# T$1}\n"))}) {
		t.Fatalf("stop at %+v", got)
	}
}

func TestNestedPlaceholders(t *testing.T) {
	e := Parse("${1:a ${2:b} c}$0", noVars)
	if e.Text != "a b c" {
		t.Fatalf("text %q", e.Text)
	}
	want := map[int]Span{1: {0, 5}, 2: {2, 3}, 0: {5, 5}}
	for _, s := range e.Stops {
		if want[s.Index] != s.At {
			t.Fatalf("stop %d at %+v, want %+v", s.Index, s.At, want[s.Index])
		}
	}
}

func TestNoStopsIsNotExplicit(t *testing.T) {
	e := Parse("plain text", noVars)
	if e.Explicit || e.Text != "plain text" {
		t.Fatalf("%+v", e)
	}
}

func TestSliceTrimsAndShifts(t *testing.T) {
	e := Parse("# T\n\n$1\n", noVars)
	trimmed := e.Slice(len("# T\n"), len(e.Text)-1)
	if trimmed.Text != "\n" {
		t.Fatalf("text %q", trimmed.Text)
	}
	if got := trimmed.Stops[0].At; got != (Span{1, 1}) {
		t.Fatalf("stop %+v", got)
	}
}

func TestSessionNavigatesStopsInOrder(t *testing.T) {
	e := Parse("x$1y ${2:z}$0", noVars)
	s := NewSession(e, 10, "0123456789"+e.Text)
	if !s.Active() || s.Cursor() != 11 {
		t.Fatalf("start at %d active=%v", s.Cursor(), s.Active())
	}
	if off, ok := s.Next(); !ok || off != 13 {
		t.Fatalf("next %d %v", off, ok)
	}
	if off, ok := s.Next(); !ok || off != 14 {
		t.Fatalf("second next %d %v", off, ok)
	}
	if _, ok := s.Next(); ok {
		t.Fatal("moved past the final stop")
	}
	if off, ok := s.Prev(); !ok || off != 13 {
		t.Fatalf("prev %d %v", off, ok)
	}
}

func TestSessionWithOnlyFinalStopIsInactive(t *testing.T) {
	s := NewSession(Parse("done$0", noVars), 0, "done")
	if s.Active() {
		t.Fatal("single final stop should not be an active session")
	}
}

func TestSessionMirrorsFollowPrimary(t *testing.T) {
	e := Parse("${1:foo} and $1", noVars)
	s := NewSession(e, 0, e.Text)
	// Replace the primary "foo" with "bar".
	reps := s.Sync("bar and foo")
	if len(reps) != 1 || reps[0] != (Replacement{Start: 8, End: 11, Text: "bar"}) {
		t.Fatalf("reps %+v", reps)
	}
	if string(s.text) != "bar and bar" {
		t.Fatalf("session text %q", string(s.text))
	}
}

func TestSessionTypingAtStopEndExtendsIt(t *testing.T) {
	e := Parse("${1:foo} and $1", noVars)
	s := NewSession(e, 0, e.Text)
	reps := s.Sync("fooX and foo")
	if len(reps) != 1 || reps[0].Text != "fooX" || reps[0].Start != 9 {
		t.Fatalf("reps %+v", reps)
	}
	if got := MapOffset(12, reps); got != 13 {
		t.Fatalf("cursor after mirror %d", got)
	}
}

func TestSessionEditOutsideStopsLeavesMirrorsAlone(t *testing.T) {
	e := Parse("${1:foo} and $1 tail", noVars)
	s := NewSession(e, 0, e.Text)
	if reps := s.Sync(e.Text + "!"); reps != nil {
		t.Fatalf("reps %+v", reps)
	}
	if off := s.Cursor(); off != 0 {
		t.Fatalf("cursor moved to %d", off)
	}
}

func TestSessionStopsFollowEditsBeforeThem(t *testing.T) {
	e := Parse("abc$1$0", noVars)
	s := NewSession(e, 0, e.Text)
	s.Sync("XXabc")
	if got := s.Cursor(); got != 5 {
		t.Fatalf("stop moved to %d, want 5", got)
	}
}

func TestDiffIsASingleRegion(t *testing.T) {
	a, b, ins := diff([]rune("hello world"), []rune("hello big world"))
	if a != 6 || b != 6 || string(ins) != "big " {
		t.Fatalf("%d %d %q", a, b, string(ins))
	}
}

func TestParseLibrary(t *testing.T) {
	raw := `
[[snippet]]
trigger = "snip"
description = "Demo"
body = '''
${1:Title}
$0
'''

[[snippet]]
trigger = "bad trigger"
body = "x"

[[snippet]]
trigger = "empty"
body = ""
`
	lib, errs := ParseLibrary(raw)
	if len(errs) != 2 {
		t.Fatalf("errors %v", errs)
	}
	s, ok := lib.Lookup("snip")
	if !ok || s.Body != "${1:Title}\n$0" || s.Description != "Demo" {
		t.Fatalf("snippet %+v", s)
	}
	if !strings.Contains(errs[0].Error(), "bad trigger") {
		t.Fatalf("first error %v", errs[0])
	}
}

func TestParseLibraryRejectsBadTOML(t *testing.T) {
	if _, errs := ParseLibrary("[[snippet"); len(errs) == 0 {
		t.Fatal("bad TOML accepted")
	}
}
