package snippet

import (
	"reflect"
	"testing"
)

func fakeResolver(name, arg string, hasArg bool) (string, bool) {
	switch name {
	case "title":
		if hasArg {
			return "", false
		}
		return "Plan", true
	case "date":
		if hasArg {
			return "D<" + arg + ">", true
		}
		return "2026-03-05", true
	}
	return "", false
}

func rg(s, e int) [2]int { return [2]int{s, e} }

func TestSnippetExpand(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		opt      Options
		text     string
		stops    []Stop // nil means only the implicit end
		implicit bool
	}{
		{"bare tabstop", "a$1b", Options{}, "ab",
			[]Stop{{1, [][2]int{rg(1, 1)}, -1}, {0, [][2]int{rg(2, 2)}, -1}}, true},
		{"braced tabstop", "a${1}b", Options{}, "ab",
			[]Stop{{1, [][2]int{rg(1, 1)}, -1}, {0, [][2]int{rg(2, 2)}, -1}}, true},
		{"placeholder", "${1:name}", Options{}, "name",
			[]Stop{{1, [][2]int{rg(0, 4)}, -1}, {0, [][2]int{rg(4, 4)}, -1}}, true},
		{"two digit number", "$10x", Options{}, "x",
			[]Stop{{10, [][2]int{rg(0, 0)}, -1}, {0, [][2]int{rg(1, 1)}, -1}}, true},
		{"nested placeholder", "${1:a ${2:b} c}", Options{}, "a b c",
			[]Stop{{1, [][2]int{rg(0, 5)}, -1}, {2, [][2]int{rg(2, 3)}, 0}, {0, [][2]int{rg(5, 5)}, -1}}, true},
		{"mirror", "${1:x} = $1", Options{}, "x = x",
			[]Stop{{1, [][2]int{rg(0, 1), rg(4, 5)}, -1}, {0, [][2]int{rg(5, 5)}, -1}}, true},
		{"mirror before source", "$1 = ${1:x}", Options{}, "x = x",
			[]Stop{{1, [][2]int{rg(0, 1), rg(4, 5)}, -1}, {0, [][2]int{rg(5, 5)}, -1}}, true},
		{"mirror copies nested text", "${1:a ${2:b}} $1", Options{}, "a b a b",
			[]Stop{{1, [][2]int{rg(0, 3), rg(4, 7)}, -1}, {2, [][2]int{rg(2, 3)}, 0}, {0, [][2]int{rg(7, 7)}, -1}}, true},
		{"final stop", "a$0b", Options{}, "ab",
			[]Stop{{0, [][2]int{rg(1, 1)}, -1}}, false},
		{"empty placeholder", "a${1:}b", Options{}, "ab",
			[]Stop{{1, [][2]int{rg(1, 1)}, -1}, {0, [][2]int{rg(2, 2)}, -1}}, true},
		{"leading zeros", "a${01}b", Options{}, "ab",
			[]Stop{{1, [][2]int{rg(1, 1)}, -1}, {0, [][2]int{rg(2, 2)}, -1}}, true},
		{"stops sorted by number", "$3 $1 $2", Options{}, "  ",
			[]Stop{{1, [][2]int{rg(1, 1)}, -1}, {2, [][2]int{rg(2, 2)}, -1}, {3, [][2]int{rg(0, 0)}, -1}, {0, [][2]int{rg(2, 2)}, -1}}, true},
		{"variable inside placeholder", "${1:due ${date}}", Options{Resolver: fakeResolver}, "due 2026-03-05",
			[]Stop{{1, [][2]int{rg(0, 14)}, -1}, {0, [][2]int{rg(14, 14)}, -1}}, true},
		{"variables", "${title} ${date:Y}", Options{Resolver: fakeResolver}, "Plan D<Y>", nil, true},
		{"variable with no value stays literal", "${title:x}|${week}", Options{Resolver: fakeResolver}, "${title:x}|${week}", nil, true},
		{"no resolver keeps variables literal", "${date}", Options{}, "${date}", nil, true},
		{"unknown name", "${foo}", Options{Resolver: fakeResolver}, "${foo}", nil, true},
		{"escaped tabstop", `\$1`, Options{}, "$1", nil, true},
		{"escaped brace", "${1:a\\}b}", Options{}, "a}b",
			[]Stop{{1, [][2]int{rg(0, 3)}, -1}, {0, [][2]int{rg(3, 3)}, -1}}, true},
		{"backslash then tabstop", `\\$1`, Options{}, `\`,
			[]Stop{{1, [][2]int{rg(1, 1)}, -1}, {0, [][2]int{rg(1, 1)}, -1}}, true},
		{"latex stays", `\frac{a}{b} \\ x`, Options{}, `\frac{a}{b} \\ x`, nil, true},
		{"unclosed placeholder", "x ${1:foo", Options{}, "x ${1:foo", nil, true},
		{"open brace", "a ${ b", Options{}, "a ${ b", nil, true},
		{"dollar signs", "$x$ and $ 5", Options{}, "$x$ and $ 5", nil, true},
		{"lone close brace", "a}b", Options{}, "a}b", nil, true},
		{"later valid part", "${1:foo $2", Options{}, "${1:foo ",
			[]Stop{{2, [][2]int{rg(8, 8)}, -1}, {0, [][2]int{rg(8, 8)}, -1}}, true},
		{"indent", "a\nb\nc", Options{Indent: "  "}, "a\n  b\n  c", nil, true},
		{"tab is one level", "x\n\ty", Options{Unit: "    "}, "x\n    y", nil, true},
		{"tab with indent", "x\n\ty", Options{Indent: "  ", Unit: "    "}, "x\n      y", nil, true},
		{"implicit end after text", "$1 $2 end", Options{}, "  end",
			[]Stop{{1, [][2]int{rg(0, 0)}, -1}, {2, [][2]int{rg(1, 1)}, -1}, {0, [][2]int{rg(5, 5)}, -1}}, true},
		{"two final stops", "$0 x $0", Options{}, " x ",
			[]Stop{{0, [][2]int{rg(0, 0), rg(3, 3)}, -1}}, false},
		{"rune offsets", "é${1:ü}$1", Options{}, "éüü",
			[]Stop{{1, [][2]int{rg(1, 2), rg(2, 3)}, -1}, {0, [][2]int{rg(3, 3)}, -1}}, true},
		{"empty body", "", Options{}, "", nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Expand(c.body, c.opt)
			if got.Text != c.text {
				t.Errorf("text = %q, want %q", got.Text, c.text)
			}
			if got.ImplicitEnd != c.implicit {
				t.Errorf("ImplicitEnd = %v, want %v", got.ImplicitEnd, c.implicit)
			}
			if c.stops == nil {
				if len(got.Stops) != 1 || got.Stops[0].Number != 0 {
					t.Errorf("stops = %+v, want only the implicit end", got.Stops)
				}
				return
			}
			if !reflect.DeepEqual(got.Stops, c.stops) {
				t.Errorf("stops = %+v, want %+v", got.Stops, c.stops)
			}
		})
	}
}

func TestSnippetExpandRangesMatchText(t *testing.T) {
	got := Expand("${1:a ${2:b} c} = $1", Options{})
	runes := []rune(got.Text)
	if got.Text != "a b c = a b c" {
		t.Errorf("text = %q", got.Text)
	}
	for _, s := range got.Stops {
		for _, r := range s.Ranges {
			if r[0] < 0 || r[1] > len(runes) || r[0] > r[1] {
				t.Errorf("stop %d range %v outside the text %q", s.Number, r, got.Text)
			}
		}
	}
}
