package vault

import (
	"strings"
	"testing"
)

func autolinkNote(path, title string, aliases ...string) NoteMeta {
	folder := FolderInbox
	if i := strings.IndexByte(path, '/'); i > 0 {
		folder = NoteFolder(path[:i])
	}
	return NoteMeta{Path: path, Title: title, Folder: folder, Aliases: aliases}
}

// autolinkRun plans body for the note inbox/Y.md and applies every change.
func autolinkRun(t *testing.T, body string, notes ...NoteMeta) string {
	t.Helper()
	all := append([]NoteMeta{autolinkNote("inbox/Y.md", "Y")}, notes...)
	changes := PlanAutolinks(body, "inbox/Y.md", all, AutolinkOptions{})
	got := ApplyAutolinks(body, changes, nil)
	for _, c := range changes {
		for _, e := range c.Edits {
			link := strings.TrimSuffix(strings.TrimPrefix(strings.Split(e.Text, "|")[0], "[["), "]]")
			n, ok := ResolveWikilink(all, link)
			if !ok || n.Path != c.Target {
				t.Errorf("link %q resolves to %q (%v), want %q", e.Text, n.Path, ok, c.Target)
			}
		}
	}
	return got
}

func TestAutolinkAddForms(t *testing.T) {
	x := autolinkNote("inbox/X.md", "X")
	cases := []struct {
		name  string
		body  string
		notes []NoteMeta
		want  string
	}{
		{"same text", "X notes", []NoteMeta{x}, "[[X]] notes"},
		{"other case", "x notes", []NoteMeta{x}, "[[X|x]] notes"},
		{"thermodynamics", "thermodynamics covers this", []NoteMeta{autolinkNote("inbox/T.md", "Thermodynamics")}, "[[Thermodynamics|thermodynamics]] covers this"},
		{"thermodynamic", "the thermodynamic process", []NoteMeta{autolinkNote("inbox/T.md", "Thermodynamics")}, "the [[Thermodynamics|thermodynamic]] process"},
		{"plural", "two notes", []NoteMeta{autolinkNote("inbox/N.md", "Note")}, "two [[Note|notes]]"},
		{"irregular plural", "two matrices", []NoteMeta{autolinkNote("inbox/M.md", "Matrix")}, "two [[Matrix|matrices]]"},
		{"ies plural", "two theories", []NoteMeta{autolinkNote("inbox/T.md", "Theory")}, "two [[Theory|theories]]"},
		{"es plural", "two boxes", []NoteMeta{autolinkNote("inbox/B.md", "Box")}, "two [[Box|boxes]]"},
		{"lower case title", "linux runs here", []NoteMeta{autolinkNote("inbox/L.md", "Linux")}, "[[Linux|linux]] runs here"},
		{"go", "go there", []NoteMeta{autolinkNote("inbox/G.md", "Go")}, "[[Go|go]] there"},
		{"alias", "LA is here", []NoteMeta{autolinkNote("inbox/X.md", "X", "LA")}, "[[X|LA]] is here"},
		{"no go inside words", "ago and Gopher", []NoteMeta{autolinkNote("inbox/G.md", "Go")}, "ago and Gopher"},
		{"german plural", "Kinder", []NoteMeta{autolinkNote("inbox/K.md", "Kind")}, "Kinder"},
		{"first of three", "X and X and X", []NoteMeta{x}, "[[X]] and X and X"},
		{"no reference", "nothing here", []NoteMeta{x}, "nothing here"},
		{"currency", "costs $5 and X costs $10", []NoteMeta{x}, "costs $5 and [[X]] costs $10"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := autolinkRun(t, c.body, c.notes...); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestAutolinkAddTargets(t *testing.T) {
	opts := AutolinkOptions{TemplateFolder: "inbox/templates", PreambleFolder: "typst"}
	tmpl := NoteMeta{Path: "inbox/templates/P.md", Title: "Template", Folder: FolderInbox}
	pre := NoteMeta{Path: "inbox/typst/Q.md", Title: "Preamble", Folder: FolderInbox}
	notes := []NoteMeta{
		autolinkNote("inbox/Y.md", "Y"),
		autolinkNote("trash/T.md", "Trashed"),
		autolinkNote("archive/A.md", "Archived"),
		tmpl,
		pre,
		autolinkNote("quick/Z.md", "Quick"),
	}
	body := "Y Trashed Archived Template Preamble Quick"
	changes := PlanAutolinks(body, "inbox/Y.md", notes, opts)
	if len(changes) != 1 || changes[0].Target != "quick/Z.md" {
		t.Fatalf("changes = %+v, want one link to quick/Z.md", changes)
	}

	for _, c := range []struct {
		n    NoteMeta
		want bool
	}{
		{notes[0], true}, {notes[5], true}, {notes[2], false}, {notes[1], false}, {tmpl, false}, {pre, false},
	} {
		if got := AutolinkSource(c.n, opts); got != c.want {
			t.Errorf("AutolinkSource(%s) = %v, want %v", c.n.Path, got, c.want)
		}
	}
}

func TestAutolinkAddKeepsExistingLinks(t *testing.T) {
	x := autolinkNote("inbox/X.md", "X")
	z := autolinkNote("inbox/Z.md", "Z")
	cases := []struct{ name, body, want string }{
		{"embed", "![[X]]", "![[X]]"},
		{"anchor", "[[X#Heading]]", "[[X#Heading]]"},
		{"embed then reference", "![[X]]\nX here", "![[X]]\n[[X]] here"},
		{"anchor then reference", "[[X#Heading]]\nX here", "[[X#Heading]]\n[[X]] here"},
		{"label of another link", "[[Z|about X]]", "[[Z|about X]]"},
		{"label of markdown link", "[about X](Z.md)", "[about X](Z.md)"},
		{"frontmatter link only", "---\nrelated: \"[[X]]\"\n---\nX here", "---\nrelated: \"[[X]]\"\n---\n[[X]] here"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := autolinkRun(t, c.body, x, z); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestAutolinkAddSkipsExcludedRegions(t *testing.T) {
	x := autolinkNote("inbox/X.md", "X")
	for name, body := range map[string]string{
		"frontmatter":   "---\ntitle: X\n---\ntext",
		"fence":         "```\nX\n```",
		"inline code":   "`X`",
		"inline math":   "$X$",
		"display math":  "$$\nX\n$$",
		"environment":   "\\begin{align}\nX\n\\end{align}",
		"heading":       "## X",
		"leading title": "# X",
		"url":           "https://example.com/X",
		"bare":          "<https://example.com/X>",
		"html":          "<b>X</b>",
		"table":         "| X |\n| - |",
		"comment":       "<!-- X -->",
		"percent":       "%% X %%",
		"task":          "- [ ] X",
		"callout":       "> [!note] X",
	} {
		t.Run(name, func(t *testing.T) {
			if got := autolinkRun(t, body, x); got != body {
				t.Errorf("got %q, want the body unchanged", got)
			}
		})
	}
}

func TestAutolinkAddDescribesEachChange(t *testing.T) {
	x := autolinkNote("inbox/X.md", "X")
	z := autolinkNote("inbox/Z.md", "Z")
	notes := []NoteMeta{autolinkNote("inbox/Y.md", "Y"), x, z}
	body := "first line\nsee  X here \nthen Z"
	changes := PlanAutolinks(body, "inbox/Y.md", notes, AutolinkOptions{})
	if len(changes) != 2 {
		t.Fatalf("changes = %+v, want 2", changes)
	}
	c := changes[0]
	if c.Kind != AutolinkAdd || c.Target != "inbox/X.md" || c.Line != 2 || c.Context != "see  X here" {
		t.Errorf("first change = %+v", c)
	}
	if c := changes[1]; c.Target != "inbox/Z.md" || c.Line != 3 || c.Context != "then Z" {
		t.Errorf("second change = %+v", c)
	}

	if got := ApplyAutolinks(body, changes, func(i int) bool { return i == 1 }); got != "first line\nsee  X here \nthen [[Z]]" {
		t.Errorf("one change omitted: got %q", got)
	}
	if got := ApplyAutolinks(body, changes, func(i int) bool { return false }); got != body {
		t.Errorf("all omitted: got %q", got)
	}
	if got := ApplyAutolinks(body, changes, nil); got != "first line\nsee  [[X]] here \nthen [[Z]]" {
		t.Errorf("all kept: got %q", got)
	}
}

func TestAutolinkAddNoReference(t *testing.T) {
	notes := []NoteMeta{autolinkNote("inbox/Y.md", "Y"), autolinkNote("inbox/X.md", "X")}
	if changes := PlanAutolinks("plain prose", "inbox/Y.md", notes, AutolinkOptions{}); len(changes) != 0 {
		t.Errorf("changes = %+v, want none", changes)
	}
}

func TestAutolinkAddSameTitleUsesPath(t *testing.T) {
	notes := []NoteMeta{autolinkNote("inbox/Y.md", "Y"), autolinkNote("inbox/a/X.md", "X"), autolinkNote("inbox/b/X.md", "X")}
	// Both notes match the same text, so the plan leaves the choice to a later
	// step and adds no link that could point at the wrong note.
	if changes := PlanAutolinks("X", "inbox/Y.md", notes, AutolinkOptions{}); len(changes) != 0 {
		t.Errorf("changes = %+v, want none", changes)
	}
}
