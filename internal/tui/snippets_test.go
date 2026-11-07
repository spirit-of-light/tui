package tui

import (
	"testing"
	"time"

	"github.com/ZenNotes/tui/internal/snippets"
	"github.com/ZenNotes/tui/internal/templates"
	"github.com/ZenNotes/tui/internal/vim"
)

func snippetTestApp(t *testing.T, raw string) (*App, *noteBuffer) {
	t.Helper()
	a, _ := newFeatureTestApp(t)
	lib, errs := snippets.ParseLibrary(raw)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	a.snippetLib = lib
	a.openNote("note.md", true)
	buf := a.activeBuffer()
	buf.ed.ReplaceText("")
	a.handleKey(vim.R('i'))
	return a, buf
}

func snipTypeKeys(a *App, text string) {
	for _, r := range text {
		a.handleKey(vim.R(r))
	}
}

func TestSnippetTriggerExpandsAndCtrlLCtrlHStepThroughStops(t *testing.T) {
	a, buf := snippetTestApp(t, `
[[snippet]]
trigger = "snip"
body = '''[${1:label}]($2)$0'''
`)
	snipTypeKeys(a, "snip")
	a.handleKey(vim.Ctrl('n'))
	if buf.ed.Text() != "[label]()" {
		t.Fatalf("expanded to %q", buf.ed.Text())
	}
	if got := buf.ed.OffsetOf(buf.ed.Cursor()); got != 1 {
		t.Fatalf("cursor at %d, want the label", got)
	}
	a.handleKey(vim.Ctrl('l'))
	if got := buf.ed.OffsetOf(buf.ed.Cursor()); got != 8 {
		t.Fatalf("Ctrl+L went to %d, want the URL slot", got)
	}
	snipTypeKeys(a, "u")
	a.handleKey(vim.Ctrl('h'))
	if got := buf.ed.OffsetOf(buf.ed.Cursor()); got != 1 {
		t.Fatalf("Ctrl+H went to %d, want the label", got)
	}
	if buf.ed.Text() != "[label](u)" {
		t.Fatalf("text %q", buf.ed.Text())
	}
}

func TestSnippetTypingReplacesDefaultAndMirrors(t *testing.T) {
	a, buf := snippetTestApp(t, `
[[snippet]]
trigger = "twice"
body = '''${1:name} / $1'''
`)
	snipTypeKeys(a, "twice")
	a.handleKey(vim.Ctrl('n'))
	if buf.ed.Text() != "name / name" {
		t.Fatalf("expanded to %q", buf.ed.Text())
	}
	snipTypeKeys(a, "bo")
	if buf.ed.Text() != "bo / bo" {
		t.Fatalf("typed %q, want both places to read bo", buf.ed.Text())
	}
}

func TestCtrlNWithoutTriggerKeepsWordCompletion(t *testing.T) {
	a, buf := snippetTestApp(t, `
[[snippet]]
trigger = "snip"
body = "x$0"
`)
	snipTypeKeys(a, "zeta zet")
	a.handleKey(vim.Ctrl('n'))
	if buf.ed.Text() != "zeta zeta" {
		t.Fatalf("word completion changed to %q", buf.ed.Text())
	}
	if buf.snip != nil {
		t.Fatal("a session opened without a trigger")
	}
}

func TestTriggerMustBeWholeWord(t *testing.T) {
	a, buf := snippetTestApp(t, `
[[snippet]]
trigger = "snip"
body = "X$0"
`)
	snipTypeKeys(a, "xsnip")
	a.handleKey(vim.Ctrl('n'))
	if buf.ed.Text() != "xsnip" {
		t.Fatalf("expanded inside a word: %q", buf.ed.Text())
	}
}

// A template with one tabstop places the cursor and leaves Ctrl+L alone, so
// the checkbox toggle keeps working in notes made from it.
func TestSingleTabstopTemplateDoesNotCaptureCtrlL(t *testing.T) {
	a, buf := snippetTestApp(t, "")
	exp := templates.Render("ab$1cd", "", time.Now()).Expansion
	buf.ed.ReplaceText(exp.Text)
	a.beginSnippetSession(buf, exp, 0, true)
	if buf.snip != nil {
		t.Fatal("one tabstop opened a session")
	}
	if got := buf.ed.OffsetOf(buf.ed.Cursor()); got != 2 {
		t.Fatalf("cursor at %d, want the tabstop", got)
	}
}
