package snippet

import (
	"reflect"
	"strings"
	"testing"
)

func TestSnippetGroupParse(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		defs     []Def
		problems []Problem
	}{
		{
			name: "heading, description and body",
			raw:  "## link\nInsert a link.\n\n```\n[$1]($2)\n```\n",
			defs: []Def{{Trigger: "link", Description: "Insert a link.", Body: "[$1]($2)", Line: 1}},
		},
		{
			name: "no description",
			raw:  "## link\n```\n[$1]($2)\n```\n",
			defs: []Def{{Trigger: "link", Body: "[$1]($2)", Line: 1}},
		},
		{
			name: "frontmatter is skipped",
			raw:  "---\ntitle: Links\n---\n## link\n```\nx\n```\n",
			defs: []Def{{Trigger: "link", Body: "x", Line: 4}},
		},
		{
			name: "frontmatter with no closing line is not frontmatter",
			raw:  "---\n## link\n```\nx\n```\n",
			defs: []Def{{Trigger: "link", Body: "x", Line: 2}},
		},
		{
			name: "text and a fence before the first heading",
			raw:  "Intro text.\n\n```\n## fake\n```\n\n## link\n```\nx\n```\n",
			defs: []Def{{Trigger: "link", Body: "x", Line: 7}},
		},
		{
			name: "two-line description is joined by one space",
			raw:  "## link\n  Insert a\n  link.  \n```\nx\n```\n",
			defs: []Def{{Trigger: "link", Description: "Insert a link.", Body: "x", Line: 1}},
		},
		{
			name: "only the first paragraph is the description",
			raw:  "## link\nFirst.\n\nSecond.\n```\nx\n```\n",
			defs: []Def{{Trigger: "link", Description: "First.", Body: "x", Line: 1}},
		},
		{
			name: "body has no final line break",
			raw:  "## two\n```\nline one\nline two\n```\n",
			defs: []Def{{Trigger: "two", Body: "line one\nline two", Line: 1}},
		},
		{
			name: "empty body",
			raw:  "## none\n```\n```\n",
			defs: []Def{{Trigger: "none", Body: "", Line: 1}},
		},
		{
			name: "a second fence is ignored",
			raw:  "## one\n```\nfirst\n```\n\n```\nsecond\n```\n",
			defs: []Def{{Trigger: "one", Body: "first", Line: 1}},
		},
		{
			name: "long fence holds a short fence",
			raw:  "## code\n````\n```go\nx := 1\n```\n````\n",
			defs: []Def{{Trigger: "code", Body: "```go\nx := 1\n```", Line: 1}},
		},
		{
			name: "tilde fence",
			raw:  "## tilde\n~~~\nx\n~~~\n",
			defs: []Def{{Trigger: "tilde", Body: "x", Line: 1}},
		},
		{
			name: "info string is ignored",
			raw:  "## info\n```markdown title=x\nbody\n```\n",
			defs: []Def{{Trigger: "info", Body: "body", Line: 1}},
		},
		{
			name: "a tilde line does not close a backtick fence",
			raw:  "## mix\n```\n~~~\n```\n",
			defs: []Def{{Trigger: "mix", Body: "~~~", Line: 1}},
		},
		{
			name: "a closing line with text does not close",
			raw:  "## text\n```\n``` not closed\n```\n",
			defs: []Def{{Trigger: "text", Body: "``` not closed", Line: 1}},
		},
		{
			name: "level-3 heading is plain text",
			raw:  "## link\n### meet\n```\nx\n```\n",
			defs: []Def{{Trigger: "link", Description: "### meet", Body: "x", Line: 1}},
		},
		{
			name: "heading with a space is a problem and others stay",
			raw:  "## link\n```\nx\n```\n\n## meet now\n```\ny\n```\n",
			defs: []Def{{Trigger: "link", Body: "x", Line: 1}},
			problems: []Problem{
				{Line: 6, Kind: ProblemNotOneWord, Heading: "meet now"},
			},
		},
		{
			name: "heading with a hyphen is a problem",
			raw:  "## meet-now\n```\nx\n```\n",
			problems: []Problem{
				{Line: 1, Kind: ProblemNotOneWord, Heading: "meet-now"},
			},
		},
		{
			name: "empty heading is a problem",
			raw:  "##\n```\nx\n```\n",
			problems: []Problem{
				{Line: 1, Kind: ProblemNotOneWord, Heading: ""},
			},
		},
		{
			name: "heading with no fenced block is a problem and others stay",
			raw:  "## link\n```\nx\n```\n## empty\nOnly a description.\n\n## later\n```\nz\n```\n",
			defs: []Def{
				{Trigger: "link", Body: "x", Line: 1},
				{Trigger: "later", Body: "z", Line: 8},
			},
			problems: []Problem{
				{Line: 5, Kind: ProblemNoFence, Heading: "empty"},
			},
		},
		{
			name: "heading at the end of the file has no fenced block",
			raw:  "## lonely",
			problems: []Problem{
				{Line: 1, Kind: ProblemNoFence, Heading: "lonely"},
			},
		},
		{
			name: "a heading line inside the body fence is body text",
			raw:  "## doc\n```\n## x\ntext\n```\n",
			defs: []Def{{Trigger: "doc", Body: "## x\ntext", Line: 1}},
		},
		{
			name: "a heading line inside an ignored fence is not a heading",
			raw:  "## one\n```\na\n```\n```\n## x\n```\n",
			defs: []Def{{Trigger: "one", Body: "a", Line: 1}},
		},
		{
			name: "an unclosed fence is one problem",
			raw:  "## open\n```\nbody\n## later\nz\n",
			problems: []Problem{
				{Line: 1, Kind: ProblemUnclosedFence, Heading: "open"},
			},
		},
		{
			name: "heading may have up to three leading spaces",
			raw:  "   ## link\n```\nx\n```\n",
			defs: []Def{{Trigger: "link", Body: "x", Line: 1}},
		},
		{
			name: "four leading spaces is not a heading",
			raw:  "    ## link\n",
		},
		{
			name: "non-ASCII trigger and underscore",
			raw:  "## café_1\n```\nx\n```\n",
			defs: []Def{{Trigger: "café_1", Body: "x", Line: 1}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defs, problems := ParseGroup(tt.raw)
			if !reflect.DeepEqual(defs, tt.defs) {
				t.Errorf("defs = %#v, want %#v", defs, tt.defs)
			}
			if !reflect.DeepEqual(problems, tt.problems) {
				t.Errorf("problems = %#v, want %#v", problems, tt.problems)
			}
			// A file with CRLF line ends gives the same result.
			crlf := strings.ReplaceAll(tt.raw, "\n", "\r\n")
			cd, cp := ParseGroup(crlf)
			if !reflect.DeepEqual(cd, tt.defs) || !reflect.DeepEqual(cp, tt.problems) {
				t.Errorf("CRLF result differs: defs %#v problems %#v", cd, cp)
			}
		})
	}
}
