package snippet

import "testing"

// The example in docs/snippets.md must parse as the guide says.
func TestSnippetGroupParseGuideExample(t *testing.T) {
	raw := "---\ntitle: Markdown snippets\n---\n\nText before the first heading is ignored.\n\n" +
		"## link\n\nInsert a link.\n\n```\n[${1:text}](${2:url})$0\n```\n\n" +
		"## callout\n\nInsert a callout with a title.\n\n```\n> [!note] ${1:Title ${2:subtitle}}\n> $0\n```\n\n" +
		"## price\n\nInsert a price. The `\\$` is a literal dollar sign.\n\n```\nPrice: \\$${1:10}\n```\n\n" +
		"## today\n\nInsert the date and the time.\n\n```\n${date} ${time}\n```\n\n" +
		"## fence\n\nInsert a code block. The longer fence holds the fence of the body.\n\n" +
		"````\n```${1:go}\n$0\n```\n````\n"
	defs, problems := ParseGroup(raw)
	if len(problems) != 0 {
		t.Fatalf("problems = %#v, want none", problems)
	}
	want := []struct{ trigger, body string }{
		{"link", "[${1:text}](${2:url})$0"},
		{"callout", "> [!note] ${1:Title ${2:subtitle}}\n> $0"},
		{"price", "Price: \\$${1:10}"},
		{"today", "${date} ${time}"},
		{"fence", "```${1:go}\n$0\n```"},
	}
	if len(defs) != len(want) {
		t.Fatalf("got %d snippets, want %d: %#v", len(defs), len(want), defs)
	}
	for i, w := range want {
		if defs[i].Trigger != w.trigger || defs[i].Body != w.body || defs[i].Description == "" {
			t.Errorf("snippet %d = %#v, want trigger %q body %q", i, defs[i], w.trigger, w.body)
		}
	}
}
