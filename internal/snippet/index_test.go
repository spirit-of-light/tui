package snippet

import (
	"errors"
	"testing"
)

func group(name, raw string) GroupFile {
	return GroupFile{Name: name, Path: ".zennotes/snippets/" + name, Raw: raw}
}

func triggers(list []Snippet) []string {
	var out []string
	for _, s := range list {
		out = append(out, s.Trigger)
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func snip(trigger, body string) string {
	return "## " + trigger + "\n\n```\n" + body + "\n```\n\n"
}

func TestSnippetIndexGroups(t *testing.T) {
	idx := BuildIndex([]GroupFile{
		group("a.md", snip("one", "1")),
		group("b.md", snip("two", "2")),
		group("My Notes.md", snip("three", "3")),
	})
	all := idx.Snippets()
	if len(all) != 3 {
		t.Fatalf("want 3 snippets, got %d", len(all))
	}
	for i, want := range []string{"My Notes", "a", "b"} {
		if all[i].Group != want {
			t.Errorf("snippet %d group = %q, want %q", i, all[i].Group, want)
		}
	}
	if all[0].Path != ".zennotes/snippets/My Notes.md" || all[0].Line != 1 {
		t.Errorf("source = %q line %d", all[2].Path, all[2].Line)
	}
}

func TestSnippetIndexUnreadableFile(t *testing.T) {
	idx := BuildIndex([]GroupFile{
		{Name: "bad.md", Path: "p/bad.md", Err: errors.New("denied")},
		group("good.md", snip("link", "L")),
	})
	if got := triggers(idx.Match("link")); !equalStrings(got, []string{"link"}) {
		t.Fatalf("link = %v", got)
	}
	probs := idx.Problems()
	if len(probs) != 1 || probs[0].Kind != ProblemUnreadable || probs[0].Path != "p/bad.md" {
		t.Fatalf("problems = %+v", probs)
	}
}

func TestSnippetIndexBadSnippetIsProblem(t *testing.T) {
	idx := BuildIndex([]GroupFile{group("x.md", "# t\n\n## meet now\n\n```\nx\n```\n\n## empty\n")})
	probs := idx.Problems()
	if len(probs) != 2 {
		t.Fatalf("problems = %+v", probs)
	}
	if probs[0].Kind != ProblemNotOneWord || probs[0].Line != 3 || probs[0].Path != ".zennotes/snippets/x.md" {
		t.Errorf("first = %+v", probs[0])
	}
	if probs[1].Kind != ProblemNoFence || probs[1].Heading != "empty" {
		t.Errorf("second = %+v", probs[1])
	}
}

func TestSnippetIndexByteOrderOfFiles(t *testing.T) {
	idx := BuildIndex([]GroupFile{
		group("b.md", snip("bb", "b")),
		group("B.md", snip("BB", "B")),
		group("a.md", snip("aa", "a")),
	})
	got := triggers(idx.Snippets())
	if want := []string{"BB", "aa", "bb"}; !equalStrings(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestSnippetIndexHeadingOrder(t *testing.T) {
	idx := BuildIndex([]GroupFile{group("a.md", snip("zeta", "z")+snip("alpha", "a"))})
	got := triggers(idx.Snippets())
	if want := []string{"zeta", "alpha"}; !equalStrings(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestSnippetIndexDuplicateAcrossFiles(t *testing.T) {
	idx := BuildIndex([]GroupFile{
		group("b.md", "\n\n\n\n\n\n\n\n"+snip("meet", "B")),
		group("a.md", "\n\n\n"+snip("meet", "A")),
	})
	got := idx.Match("meet")
	if len(got) != 1 || got[0].Body != "A" {
		t.Fatalf("match = %+v", got)
	}
	dups := idx.Duplicates()
	if len(dups) != 1 {
		t.Fatalf("dups = %+v", dups)
	}
	d := dups[0]
	if d.Trigger != "meet" || d.UsedPath != ".zennotes/snippets/a.md" || d.UsedLine != 4 ||
		d.IgnoredPath != ".zennotes/snippets/b.md" || d.IgnoredLine != 9 {
		t.Fatalf("dup = %+v", d)
	}
}

func TestSnippetIndexDuplicateInOneFile(t *testing.T) {
	idx := BuildIndex([]GroupFile{group("a.md", snip("meet", "first")+snip("meet", "second"))})
	got := idx.Match("meet")
	if len(got) != 1 || got[0].Body != "first" {
		t.Fatalf("match = %+v", got)
	}
	dups := idx.Duplicates()
	if len(dups) != 1 || dups[0].UsedLine != 1 || dups[0].IgnoredLine != 7 {
		t.Fatalf("dups = %+v", dups)
	}
}

func TestSnippetIndexCaseIsDistinct(t *testing.T) {
	idx := BuildIndex([]GroupFile{group("a.md", snip("meet", "m")+snip("Meet", "M"))})
	if len(idx.Snippets()) != 2 || len(idx.Duplicates()) != 0 {
		t.Fatalf("snippets = %v dups = %v", triggers(idx.Snippets()), idx.Duplicates())
	}
}

func TestSnippetIndexPrefixTriggerIsNotDuplicate(t *testing.T) {
	idx := BuildIndex([]GroupFile{
		group("a.md", snip("meet", "1")+snip("meet1", "2")),
		group("b.md", snip("meet", "3")),
	})
	if got := triggers(idx.Snippets()); !equalStrings(got, []string{"meet", "meet1"}) {
		t.Fatalf("snippets = %v", got)
	}
	if len(idx.Duplicates()) != 1 {
		t.Fatalf("dups = %+v", idx.Duplicates())
	}
}

func matchIndex() *Index {
	return BuildIndex([]GroupFile{group("a.md",
		snip("Snip", "A")+snip("snip", "B")+snip("meet", "m")+snip("Meet1", "M"))})
}

func TestSnippetMatchLowerPrefixFolds(t *testing.T) {
	if got := triggers(matchIndex().Match("sn")); !equalStrings(got, []string{"Snip", "snip"}) {
		t.Fatalf("sn = %v", got)
	}
	if got := triggers(matchIndex().Match("me")); !equalStrings(got, []string{"meet", "Meet1"}) {
		t.Fatalf("me = %v", got)
	}
}

func TestSnippetMatchUpperPrefixIsExact(t *testing.T) {
	if got := triggers(matchIndex().Match("Sn")); !equalStrings(got, []string{"Snip"}) {
		t.Fatalf("Sn = %v", got)
	}
	only := BuildIndex([]GroupFile{group("a.md", snip("snip", "B"))})
	if got := only.Match("Sn"); len(got) != 0 {
		t.Fatalf("Sn on snip = %v", triggers(got))
	}
}

func TestSnippetMatchPrefixHelper(t *testing.T) {
	cases := []struct {
		word, prefix string
		want         bool
	}{
		{"Meeting", "me", true},
		{"meeting", "Me", false},
		{"meeting", "meeting", true},
		{"me", "meet", false},
		{"Meeting", "Me", true},
		{"anything", "", false},
	}
	for _, c := range cases {
		if got := MatchPrefix(c.word, c.prefix); got != c.want {
			t.Errorf("MatchPrefix(%q, %q) = %v, want %v", c.word, c.prefix, got, c.want)
		}
	}
}

func TestSnippetMatchEmptyPrefixMatchesNothing(t *testing.T) {
	if got := matchIndex().Match(""); len(got) != 0 {
		t.Fatalf("empty prefix = %v", triggers(got))
	}
}
