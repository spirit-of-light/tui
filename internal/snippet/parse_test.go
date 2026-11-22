package snippet

import (
	"reflect"
	"strings"
	"testing"
)

func text(s string) Node { return Node{Kind: KindText, Text: s} }

func stop(n int, children ...Node) Node {
	return Node{Kind: KindTabstop, Number: n, Children: children}
}

func variable(raw, name, arg string, hasArg bool) Node {
	return Node{Kind: KindVariable, Name: name, Arg: arg, HasArg: hasArg, Raw: raw}
}

func TestSnippetParse(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []Node
	}{
		{"empty body", "", nil},
		{"plain text", "abc", []Node{text("abc")}},
		{"bare tabstop", "a$1b", []Node{text("a"), stop(1), text("b")}},
		{"braced tabstop", "a${1}b", []Node{text("a"), stop(1), text("b")}},
		{"placeholder", "${1:name}", []Node{stop(1, text("name"))}},
		{"maximal digits", "$10x", []Node{stop(10), text("x")}},
		{"final stop", "a$0b", []Node{text("a"), stop(0), text("b")}},
		{"leading zeros", "a${01}b", []Node{text("a"), stop(1), text("b")}},
		{"empty placeholder", "a${1:}b", []Node{text("a"), stop(1), text("b")}},
		{"nested placeholder", "${1:a ${2:b} c}",
			[]Node{stop(1, text("a "), stop(2, text("b")), text(" c"))}},
		{"mirror", "${1:x} = $1", []Node{stop(1, text("x")), text(" = "), stop(1)}},
		{"variables", "${title}${date}${time}${week}", []Node{
			variable("${title}", "title", "", false),
			variable("${date}", "date", "", false),
			variable("${time}", "time", "", false),
			variable("${week}", "week", "", false),
		}},
		{"date format", "${date:YYYY}", []Node{variable("${date:YYYY}", "date", "YYYY", true)}},
		{"date empty format", "${date:}", []Node{variable("${date:}", "date", "", true)}},
		{"date format escape", `${date:a\}b}`, []Node{variable(`${date:a\}b}`, "date", "a}b", true)}},
		{"variable in placeholder", "${1:due ${date}}",
			[]Node{stop(1, text("due "), variable("${date}", "date", "", false))}},
		{"unknown variable", "${foo}", []Node{text("${foo}")}},
		{"variable with unsupported argument", "${title:x}", []Node{text("${title:x}")}},
		{"escaped dollar", `\$1`, []Node{text("$1")}},
		{"escaped brace in placeholder", `${1:a\}b}`, []Node{stop(1, text("a}b"))}},
		{"backslash before active dollar", `\\$1`, []Node{text(`\`), stop(1)}},
		{"other backslashes are literal", `\frac{a}{b} \\ x`, []Node{text(`\frac{a}{b} \\ x`)}},
		{"dollar before other text", "$x$ and $ 5", []Node{text("$x$ and $ 5")}},
		{"dollar at the end", "cost $", []Node{text("cost $")}},
		{"opener with no digits", "a ${ b", []Node{text("a ${ b")}},
		{"opener with digits only", "a ${1 b", []Node{text("a ${1 b")}},
		{"placeholder not closed", "x ${1:foo", []Node{text("x ${1:foo")}},
		{"lone closing brace", "a}b", []Node{text("a}b")}},
		{"valid part after an open placeholder", "${1:foo $2",
			[]Node{text("${1:foo "), stop(2)}},
		{"leading zeros in an open placeholder", "${01:foo", []Node{text("${01:foo")}},
		{"inner closes, outer stays open", "${1:a ${2:b}",
			[]Node{text("${1:a "), stop(2, text("b"))}},
		{"brace after a placeholder is literal", "${1:a}}", []Node{stop(1, text("a")), text("}")}},
		{"number too long", "$1234567890", []Node{text("$1234567890")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Parse(tc.body)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Parse(%q)\n got %#v\nwant %#v", tc.body, got, tc.want)
			}
		})
	}
}

func TestSnippetParseNeverFails(t *testing.T) {
	// Deeply nested open placeholders must parse in linear time and not panic.
	body := strings.Repeat("${1:", 5000) + "x"
	got := Parse(body)
	if len(got) != 1 || got[0].Kind != KindText || got[0].Text != body {
		t.Fatalf("an open nest must stay literal, got %d nodes", len(got))
	}
	for _, b := range []string{"$", "${", "${1", "${1:", "\\", "}", "${date:", "$$$", "${}"} {
		Parse(b)
	}
}
