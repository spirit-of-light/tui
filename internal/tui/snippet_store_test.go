package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ZenNotes/tui/internal/config"
)

const snippetsDirRel = ".zennotes/snippets"

func writeSnippetFile(t *testing.T, a *App, name, text string) string {
	t.Helper()
	dir := filepath.Join(a.opts.Target.Root, filepath.FromSlash(snippetsDirRel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return snippetsDirRel + "/" + name
}

// runCmd runs a command and feeds every message, including the parts of a
// batch, back through Update.
func runCmd(a *App, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	switch m := cmd().(type) {
	case nil:
	case tea.BatchMsg:
		for _, c := range m {
			runCmd(a, c)
		}
	default:
		a.Update(m)
	}
}

func loadSnippets(a *App) {
	a.Update(a.loadSnippetsCmd()())
}

func snippetTriggers(a *App) []string {
	var out []string
	for _, s := range a.snippetIndex.Snippets() {
		out = append(out, s.Trigger)
	}
	return out
}

func TestSnippetStoreNoDirectory(t *testing.T) {
	a, _ := newTagTestApp(t, config.Prefs{}, "x\n")
	a.message, a.toasts = "", nil
	loadSnippets(a)
	if n := len(a.snippetIndex.Snippets()); n != 0 {
		t.Fatalf("snippets = %d, want 0", n)
	}
	if a.message != "" || len(a.toasts) != 0 {
		t.Fatalf("message %q, toasts %v; want nothing", a.message, a.toasts)
	}
}

func TestSnippetStoreProblemsShowNothing(t *testing.T) {
	a, _ := newTagTestApp(t, config.Prefs{}, "x\n")
	writeSnippetFile(t, a, "a.md", "## meet\n\n```\nA\n```\n\n## meet now\n\n```\nX\n```\n\n## empty\n")
	writeSnippetFile(t, a, "b.md", "## meet\n\n```\nB\n```\n")
	a.message, a.toasts = "", nil
	loadSnippets(a)
	if got := snippetTriggers(a); len(got) != 1 || got[0] != "meet" {
		t.Fatalf("triggers = %v, want [meet]", got)
	}
	if a.message != "" || len(a.toasts) != 0 {
		t.Fatalf("message %q, toasts %v; want nothing", a.message, a.toasts)
	}
}

func TestSnippetStoreReloadsOnFileChange(t *testing.T) {
	a, _ := newTagTestApp(t, config.Prefs{}, "x\n")
	p := writeSnippetFile(t, a, "a.md", "## meet\n\n```\nA\n```\n")
	loadSnippets(a)
	if got := a.snippetIndex.Match("meet"); len(got) != 1 || got[0].Body != "A" {
		t.Fatalf("first load = %+v", got)
	}

	writeSnippetFile(t, a, "a.md", "## meet\n\n```\nB\n```\n")
	runCmd(a, a.handleVaultChange([]string{p}))
	if got := a.snippetIndex.Match("meet"); len(got) != 1 || got[0].Body != "B" {
		t.Fatalf("changed body = %+v", got)
	}

	np := writeSnippetFile(t, a, "todo.md", "## todo\n\n```\n- [ ] \n```\n")
	runCmd(a, a.handleVaultChange([]string{np}))
	if got := a.snippetIndex.Match("todo"); len(got) != 1 {
		t.Fatalf("new file = %+v", got)
	}

	if err := os.Remove(filepath.Join(a.opts.Target.Root, filepath.FromSlash(np))); err != nil {
		t.Fatal(err)
	}
	runCmd(a, a.handleVaultChange([]string{np}))
	if got := a.snippetIndex.Match("todo"); len(got) != 0 {
		t.Fatalf("deleted file still matches: %+v", got)
	}
}

func TestSnippetStoreReloadsOnNewDirectory(t *testing.T) {
	a, _ := newTagTestApp(t, config.Prefs{}, "x\n")
	loadSnippets(a)
	writeSnippetFile(t, a, "a.md", "## meet\n\n```\nA\n```\n")
	runCmd(a, a.handleVaultChange([]string{snippetsDirRel}))
	if got := a.snippetIndex.Match("meet"); len(got) != 1 {
		t.Fatalf("directory batch did not reload: %+v", got)
	}
}

func TestSnippetStoreIgnoresNoteChange(t *testing.T) {
	a, _ := newTagTestApp(t, config.Prefs{}, "x\n")
	writeSnippetFile(t, a, "a.md", "## meet\n\n```\nA\n```\n")
	// The files are on disk, but no load ran. A note path must not load them.
	runCmd(a, a.handleVaultChange([]string{"Draft.md"}))
	if n := len(a.snippetIndex.Snippets()); n != 0 {
		t.Fatalf("a note change loaded %d snippets", n)
	}
	if snippetPathChanged([]string{".zennotes/snippets-other/x.md", "Draft.md", ".zennotes/x.md"}) {
		t.Fatal("a path outside the snippets directory counted")
	}
}

func TestSnippetStoreIgnoresOldEpoch(t *testing.T) {
	a, _ := newTagTestApp(t, config.Prefs{}, "x\n")
	writeSnippetFile(t, a, "a.md", "## meet\n\n```\nA\n```\n")
	msg := a.loadSnippetsCmd()()
	a.epoch = new(int) // the vault switched while the load ran
	a.Update(msg)
	if n := len(a.snippetIndex.Snippets()); n != 0 {
		t.Fatalf("old load gave %d snippets", n)
	}
}
