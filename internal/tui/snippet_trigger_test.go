package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ZenNotes/tui/internal/vim"
)

const triggerSnippets = "## snip\n\n```\nS\n```\n\n" +
	"## box\n\n```\n[${1:a}]($2)\n```\n"

// triggerApp gives an App in edit mode with the snippet files loaded and an
// empty note. With Vim on, the editor is in insert mode. The key is the
// trigger key of the mode.
func triggerApp(t *testing.T, vimOn bool, files map[string]string) (*App, *noteBuffer, vim.Key) {
	t.Helper()
	return triggerAppDraft(t, vimOn, "", files)
}

// triggerAppDraft is triggerApp with a note that has text before the first
// key. The cursor starts at the end of the text.
func triggerAppDraft(t *testing.T, vimOn bool, draft string, files map[string]string) (*App, *noteBuffer, vim.Key) {
	t.Helper()
	prefs := vimPrefs()
	key := vim.Ctrl('n')
	if !vimOn {
		prefs = vimOffPrefs()
		key = vim.KeyTab
	}
	a, buf := newTagTestApp(t, prefs, draft)
	for name, text := range files {
		writeSnippetFile(t, a, name, text)
	}
	loadSnippets(a)
	switch {
	case vimOn && draft != "":
		feed(a, vim.R('G'), vim.R('A'))
	case vimOn:
		feed(a, vim.R('i'))
	case draft != "":
		last := buf.ed.LineCount() - 1
		buf.ed.SetCursor(vim.Pos{Line: last, Col: len(buf.ed.LineRunes(last))})
	}
	return a, buf, key
}

func bothModes(t *testing.T, f func(t *testing.T, vimOn bool)) {
	t.Helper()
	for _, vimOn := range []bool{true, false} {
		name := "vim off"
		if vimOn {
			name = "vim on"
		}
		t.Run(name, func(t *testing.T) { f(t, vimOn) })
	}
}

// triggerBaseline gives the text of a buffer that gets the same keys with no
// snippet files.
func triggerBaseline(t *testing.T, vimOn bool, typed string) (string, vim.Pos) {
	t.Helper()
	a, buf, key := triggerApp(t, vimOn, nil)
	typeKeys(a, typed)
	feed(a, key)
	return buf.ed.Text(), buf.ed.Cursor()
}

func TestSnippetTriggerExpandsOneMatch(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		for _, typed := range []string{"sn", "sni", "snip"} {
			a, buf, key := triggerApp(t, vimOn, map[string]string{"a.md": "## snippet\n\n```\nS\n```\n"})
			typeKeys(a, typed)
			feed(a, key)
			if got := buf.ed.Text(); got != "S" {
				t.Errorf("%q + trigger: text %q, want S", typed, got)
			}
		}
	})
}

func TestSnippetTriggerStartsSession(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		a, buf, key := triggerApp(t, vimOn, map[string]string{"a.md": triggerSnippets})
		typeKeys(a, "box")
		feed(a, key)
		if got := buf.ed.Text(); got != "[a]()" {
			t.Fatalf("text %q", got)
		}
		if cur, total, _, ok := buf.ed.SnippetStatus(); !ok || cur != 1 || total != 2 {
			t.Fatalf("session %d/%d ok=%v", cur, total, ok)
		}
		if a.snippetOwner.buf != buf {
			t.Fatal("the App did not record the owner of the session")
		}
	})
}

func TestSnippetTriggerBufferWordMakesNoPopup(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		a, buf, key := triggerApp(t, vimOn, map[string]string{"a.md": "## snip\n\n```\nS\n```\n"})
		typeKeys(a, "snippets")
		feed(a, vim.KeyEnter)
		typeKeys(a, "sn")
		feed(a, key)
		if got := buf.ed.Line(1); got != "S" {
			t.Fatalf("line %q, want S", got)
		}
	})
}

func TestSnippetTriggerAfterTextAndBeforeSuffix(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		files := map[string]string{"a.md": "## sn\n\n```\nS\n```\n"}
		a, buf, key := triggerApp(t, vimOn, files)
		typeKeys(a, "x sn")
		feed(a, key)
		if got := buf.ed.Text(); got != "x S" {
			t.Fatalf("text %q, want %q", got, "x S")
		}

		a, buf, key = triggerApp(t, vimOn, files)
		typeKeys(a, "snippet")
		for range 5 {
			feed(a, vim.Special("left"))
		}
		feed(a, key)
		if got := buf.ed.Text(); got != "Sippet" {
			t.Fatalf("text %q, want Sippet", got)
		}
	})
}

func TestSnippetTriggerSmartCase(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		a, buf, key := triggerApp(t, vimOn, map[string]string{"a.md": "## Snip\n\n```\nU\n```\n"})
		typeKeys(a, "sn")
		feed(a, key)
		if got := buf.ed.Text(); got != "U" {
			t.Errorf("sn: text %q, want U", got)
		}

		a, buf, key = triggerApp(t, vimOn, map[string]string{"a.md": "## snip\n\n```\nL\n```\n"})
		typeKeys(a, "Sn")
		feed(a, key)
		if want, wantPos := triggerBaseline(t, vimOn, "Sn"); buf.ed.Text() != want || buf.ed.Cursor() != wantPos {
			t.Errorf("Sn with only snip: %q at %v, baseline %q at %v", buf.ed.Text(), buf.ed.Cursor(), want, wantPos)
		}

		a, buf, key = triggerApp(t, vimOn, map[string]string{"a.md": "## Snip\n\n```\nA\n```\n\n## snip\n\n```\nB\n```\n"})
		typeKeys(a, "Sn")
		feed(a, key)
		if got := buf.ed.Text(); got != "A" {
			t.Errorf("Sn with Snip and snip: text %q, want A", got)
		}
	})
}

func TestSnippetTriggerKeepsKeyWithoutMatch(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		files := map[string]string{"a.md": triggerSnippets}
		for _, typed := range []string{"xy", "xy ", "", "snip "} {
			a, buf, key := triggerApp(t, vimOn, files)
			typeKeys(a, typed)
			feed(a, key)
			if want, wantPos := triggerBaseline(t, vimOn, typed); buf.ed.Text() != want || buf.ed.Cursor() != wantPos {
				t.Errorf("%q: %q at %v, baseline %q at %v", typed, buf.ed.Text(), buf.ed.Cursor(), want, wantPos)
			}
		}
	})
}

func TestSnippetTriggerVimOffOtherKeysKeepFunction(t *testing.T) {
	for _, c := range []struct {
		typed string
		key   vim.Key
	}{{"xy", vim.Ctrl('n')}, {"me", vim.Ctrl('p')}} {
		a, _, _ := triggerApp(t, false, map[string]string{"a.md": "## meet\n\n```\nM\n```\n"})
		typeKeys(a, c.typed)
		feed(a, c.key)
		b, _, _ := triggerApp(t, false, nil)
		typeKeys(b, c.typed)
		feed(b, c.key)
		if a.activeBuffer().ed.Text() != b.activeBuffer().ed.Text() ||
			(a.overlay == nil) != (b.overlay == nil) || len(a.activeBuffer().ed.Text()) != len(b.activeBuffer().ed.Text()) {
			t.Errorf("%s: state differs from the baseline", c.key)
		}
		if c.key.IsCtrl('p') && a.overlay == nil {
			t.Error("Ctrl+P with Vim off did not open the note search")
		}
	}
}

func TestSnippetTriggerVimOnTabIsNotTheKey(t *testing.T) {
	a, buf, _ := triggerApp(t, true, map[string]string{"a.md": triggerSnippets})
	typeKeys(a, "sn")
	feed(a, vim.KeyTab)
	b, bbuf, _ := triggerApp(t, true, nil)
	typeKeys(b, "sn")
	feed(b, vim.KeyTab)
	if got, want := buf.ed.Text(), bbuf.ed.Text(); got != want || buf.ed.Cursor() != bbuf.ed.Cursor() {
		t.Fatalf("%q at %v, baseline %q at %v", got, buf.ed.Cursor(), want, bbuf.ed.Cursor())
	}
}

func TestSnippetTriggerVimOnCommandLine(t *testing.T) {
	a, buf, _ := triggerApp(t, true, map[string]string{"a.md": triggerSnippets})
	feed(a, vim.KeyEsc, vim.R(':'))
	typeKeys(a, "sn")
	feed(a, vim.Ctrl('n'))
	if got := buf.ed.Text(); got != "" {
		t.Fatalf("text %q, want empty", got)
	}
}

func TestSnippetTriggerVimOnKeywordCompletionKeepsKey(t *testing.T) {
	a, buf, key := triggerApp(t, true, map[string]string{"a.md": "## snip\n\n```\nS\n```\n"})
	b, bbuf, _ := triggerApp(t, true, nil)
	for _, app := range []*App{a, b} {
		typeKeys(app, "world")
		feed(app, vim.KeyEnter)
		typeKeys(app, "wor")
		feed(app, key)
	}
	if !buf.ed.KeywordCompletionActive() {
		t.Fatal("no keyword cycle runs")
	}
	if got, want := buf.ed.Line(1), bbuf.ed.Line(1); got != want || buf.ed.Cursor() != bbuf.ed.Cursor() {
		t.Fatalf("first key: line %q at %v, baseline %q at %v", got, buf.ed.Cursor(), want, bbuf.ed.Cursor())
	}

	// A snippet that starts with the completed word must not take the key of
	// the running cycle.
	writeSnippetFile(t, a, "b.md", "## worldmap\n\n```\nX\n```\n")
	loadSnippets(a)
	feed(a, key)
	feed(b, key)
	if got, want := buf.ed.Line(1), bbuf.ed.Line(1); got != want || buf.ed.Cursor() != bbuf.ed.Cursor() {
		t.Fatalf("second key: line %q at %v, baseline %q at %v", got, buf.ed.Cursor(), want, bbuf.ed.Cursor())
	}
}

func TestSnippetTriggerPasteDoesNotExpand(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		a, buf, _ := triggerApp(t, vimOn, map[string]string{"a.md": triggerSnippets})
		a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("sn\t"), Paste: true})
		if got := buf.ed.Text(); got == "S" {
			t.Fatalf("a paste expanded a snippet: %q", got)
		}
	})
}

func TestSnippetTriggerDuplicates(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		cases := []struct {
			name  string
			files map[string]string
			typed string
			want  string
		}{
			{"first file wins", map[string]string{
				"a.md": "## meet\n\n```\nA\n```\n", "b.md": "## meet\n\n```\nB\n```\n"}, "meet", "A"},
			{"byte order", map[string]string{
				"B.md": "## meet\n\n```\nB\n```\n", "a.md": "## meet\n\n```\nA\n```\n"}, "meet", "B"},
			{"first heading wins", map[string]string{
				"a.md": "## meet\n\n```\nA\n```\n\n## meet\n\n```\nZ\n```\n"}, "meet", "A"},
			{"case differs", map[string]string{
				"a.md": "## meet\n\n```\nA\n```\n", "b.md": "## Meet\n\n```\nM\n```\n"}, "Meet", "M"},
		}
		for _, c := range cases {
			a, buf, key := triggerApp(t, vimOn, c.files)
			typeKeys(a, c.typed)
			feed(a, key)
			if got := buf.ed.Text(); got != c.want {
				t.Errorf("%s: text %q, want %q", c.name, got, c.want)
			}
		}
	})
}

func TestSnippetTriggerAfterReload(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		a, buf, key := triggerApp(t, vimOn, map[string]string{"a.md": "## meet\n\n```\nA\n```\n"})
		p := writeSnippetFile(t, a, "a.md", "## meet\n\n```\nB\n```\n")
		runCmd(a, a.handleVaultChange([]string{p}))
		typeKeys(a, "meet")
		feed(a, key)
		if got := buf.ed.Text(); got != "B" {
			t.Fatalf("changed body: %q", got)
		}

		a, buf, key = triggerApp(t, vimOn, map[string]string{"a.md": "## meet\n\n```\nA\n```\n"})
		np := writeSnippetFile(t, a, "todo.md", "## todo\n\n```\nT\n```\n")
		runCmd(a, a.handleVaultChange([]string{np}))
		typeKeys(a, "todo")
		feed(a, key)
		if got := buf.ed.Text(); got != "T" {
			t.Fatalf("new file: %q", got)
		}
	})
}

func TestSnippetTriggerUndoGivesBackThePrefix(t *testing.T) {
	files := map[string]string{"a.md": "## sn\n\n```\nS\n```\n"}
	a, buf, key := triggerApp(t, false, files)
	typeKeys(a, "x sn")
	feed(a, key)
	if got := buf.ed.Text(); got != "x S" {
		t.Fatalf("text %q", got)
	}
	feed(a, vim.Ctrl('z'))
	if got := buf.ed.Text(); got != "x sn" {
		t.Fatalf("after undo %q, want %q", got, "x sn")
	}

	a, buf, key = triggerApp(t, true, files)
	typeKeys(a, "x sn")
	feed(a, vim.KeyEsc, vim.R('A'), key)
	if got := buf.ed.Line(0); got != "x S" {
		t.Fatalf("line %q", got)
	}
	feed(a, vim.KeyEsc, vim.R('u'))
	if got := buf.ed.Line(0); got != "x sn" {
		t.Fatalf("after undo %q, want %q", got, "x sn")
	}
}

func TestSnippetTriggerInFrontmatter(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		a, buf, key := triggerApp(t, vimOn, map[string]string{"a.md": "## sn\n\n```\nS\n```\n"})
		typeKeys(a, "---")
		feed(a, vim.KeyEnter)
		typeKeys(a, "sn")
		feed(a, key)
		if got := buf.ed.Line(1); got != "S" {
			t.Fatalf("line %q, want S", got)
		}
	})
}

func TestSnippetTriggerInPreopenedFenceAndFrontmatter(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		files := map[string]string{"a.md": "## sn\n\n```\nS\n```\n"}
		for name, draft := range map[string]string{
			"fence":       "```go\nfunc x() {}\nsn",
			"frontmatter": "---\ntitle: x\nsn",
		} {
			a, buf, key := triggerAppDraft(t, vimOn, draft, files)
			last := buf.ed.LineCount() - 1
			feed(a, key)
			if got := buf.ed.Line(last); got != "S" {
				t.Errorf("%s: line %q, want S (text %q)", name, got, buf.ed.Text())
			}
		}
	})
}
