package vault

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func writeNoteFile(t *testing.T, v *Vault, rel, body string) {
	t.Helper()
	abs := filepath.Join(v.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAliasesAreReadFromFrontmatter(t *testing.T) {
	v := newTestVault(t)
	for _, c := range []struct {
		name, body string
		want       []string
	}{
		{"block list", "---\naliases:\n  - LA\n  - \"Lin Alg\"\n---\n# X\n", []string{"LA", "Lin Alg"}},
		{"inline list", "---\naliases: [LA, 'Lin Alg']\n---\n# X\n", []string{"LA", "Lin Alg"}},
		{"no frontmatter", "# X\naliases:\n  - LA\n", nil},
		{"no aliases key", "---\ntags: [a]\n---\n# X\n", nil},
	} {
		writeNoteFile(t, v, "inbox/X.md", c.body)
		meta, err := v.MetaFor("inbox/X.md")
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(meta.Aliases, c.want) {
			t.Errorf("%s: Aliases = %q, want %q", c.name, meta.Aliases, c.want)
		}
	}
}

func aliasNotes() []NoteMeta {
	return []NoteMeta{
		{Path: "inbox/X.md", Title: "X", Folder: FolderInbox, Aliases: []string{"LA"}},
		{Path: "inbox/Y.md", Title: "Y", Folder: FolderInbox, Wikilinks: []string{"LA"}},
		{Path: "inbox/Z.md", Title: "Z", Folder: FolderInbox},
	}
}

func TestAliasesResolveAWikilink(t *testing.T) {
	notes := aliasNotes()
	for _, target := range []string{"LA", "la", "LA#Part", "LA|text"} {
		if n, ok := ResolveWikilink(notes, target); !ok || n.Path != "inbox/X.md" {
			t.Errorf("ResolveWikilink(%q) = %q, %v; want inbox/X.md", target, n.Path, ok)
		}
	}
}

func TestAliasesLoseToATitle(t *testing.T) {
	notes := aliasNotes()
	notes[2].Title = "LA"
	n, ok := ResolveWikilink(notes, "LA")
	if !ok || n.Path != "inbox/Z.md" {
		t.Errorf("ResolveWikilink(LA) = %q, %v; want the note titled LA", n.Path, ok)
	}
}

func TestAliasesOfATrashedNoteDoNotResolve(t *testing.T) {
	notes := aliasNotes()
	notes[0].Folder = FolderTrash
	if n, ok := ResolveWikilink(notes, "LA"); ok {
		t.Errorf("ResolveWikilink(LA) = %q, want no match", n.Path)
	}
}

func TestAliasesCountAsBacklinks(t *testing.T) {
	got := []string{}
	for _, n := range BacklinksIn(aliasNotes(), "inbox/X.md") {
		got = append(got, n.Path)
	}
	if want := []string{"inbox/Y.md"}; !reflect.DeepEqual(got, want) {
		t.Errorf("BacklinksIn = %q, want %q", got, want)
	}
}

func TestAliasesShadowedByATitleAreNotBacklinks(t *testing.T) {
	notes := aliasNotes()
	notes[2] = NoteMeta{Path: "inbox/LA.md", Title: "LA", Folder: FolderInbox}
	if got := BacklinksIn(notes, "inbox/X.md"); len(got) != 0 {
		t.Errorf("BacklinksIn(X) = %+v, want none: [[LA]] resolves to the note titled LA", got)
	}
	got := BacklinksIn(notes, "inbox/LA.md")
	if len(got) != 1 || got[0].Path != "inbox/Y.md" {
		t.Errorf("BacklinksIn(LA) = %+v, want inbox/Y.md", got)
	}
}

func TestAliasesSurviveARename(t *testing.T) {
	notes := aliasNotes()
	body := "[[LA]] and [[X]] and [[x|text]]"
	got, changed := RewriteWikilinksForRename(body, notes, "inbox/X.md", "Renamed")
	if want := "[[LA]] and [[Renamed]] and [[Renamed|text]]"; got != want || changed != 2 {
		t.Errorf("RewriteWikilinksForRename = %q (%d), want %q (2)", got, changed, want)
	}
}

func TestAliasesRenameThroughTheVault(t *testing.T) {
	v := newTestVault(t)
	writeNoteFile(t, v, "inbox/X.md", "---\naliases: [LA]\n---\n# X\n")
	writeNoteFile(t, v, "inbox/Y.md", "[[LA]] [[X]]\n")
	if _, err := v.RenameNote("inbox/X.md", "Renamed"); err != nil {
		t.Fatal(err)
	}
	content, err := v.ReadNote("inbox/Y.md")
	if err != nil {
		t.Fatal(err)
	}
	if want := "[[LA]] [[Renamed]]\n"; content.Body != want {
		t.Errorf("Y body = %q, want %q", content.Body, want)
	}
}

func TestAliasesStayOutOfTheJSON(t *testing.T) {
	with := NoteMeta{Path: "a.md", Aliases: []string{"LA"}}
	without := NoteMeta{Path: "a.md"}
	keys := func(m NoteMeta) []string {
		raw, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			t.Fatal(err)
		}
		out := []string{}
		for k := range obj {
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	}
	if got, want := keys(with), keys(without); !reflect.DeepEqual(got, want) {
		t.Errorf("JSON keys = %q, want %q", got, want)
	}
	for _, k := range keys(with) {
		if k == "aliases" || k == "Aliases" {
			t.Error("aliases leaked into the JSON")
		}
	}
}
