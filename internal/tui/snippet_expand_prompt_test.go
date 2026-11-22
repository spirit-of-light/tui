package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/ZenNotes/tui/internal/vim"
)

// promptUnchangedApp gives an App whose :keymaps prompt for snippet.expand
// was submitted with its text unchanged.
func promptUnchangedApp(t *testing.T, vimOn bool) (*App, *noteBuffer, string) {
	t.Helper()
	toml := ""
	if !vimOn {
		toml = "[vim]\nenabled = false\n"
	}
	p := overridePrefs(t, vimOn, toml)
	dir := os.Getenv("ZENNOTES_CONFIG_DIR")
	a, buf := newTagTestApp(t, p, "")
	// The test app points the config dir at its own temporary dir, so
	// point it back at the dir that holds the file under test.
	t.Setenv("ZENNOTES_CONFIG_DIR", dir)
	a.prefsLoaded = true
	writeSnippetFile(t, a, "a.md", expandSnip)
	loadSnippets(a)
	before := bindConfig(t)

	pal := keymapPalette(t, a)
	for _, ch := range "snippet.expand" {
		pal.handleKey(a, vim.Key{Rune: ch})
	}
	pal.handleKey(a, vim.Key{Name: "enter"})
	pr, ok := a.overlay.(*prompt)
	if !ok {
		t.Fatalf("overlay after Enter: %T", a.overlay)
	}
	if got := pr.input.String(); got != "Ctrl+N, Tab" {
		t.Fatalf("prompt text %q", got)
	}
	pr.handleKey(a, vim.Key{Name: "enter"})
	if vimOn {
		feed(a, vim.R('i'))
	}
	return a, buf, before
}

// An unchanged prompt of :keymaps is no change: nothing is written and the
// default key of each mode keeps its mode.
func TestSnippetExpandOverridePromptUnchanged(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		own, other := vim.Ctrl('n'), vim.KeyTab
		if !vimOn {
			own, other = other, own
		}

		a, buf, before := promptUnchangedApp(t, vimOn)
		if got := bindConfig(t); got != before || strings.Contains(got, "snippet.expand") {
			t.Fatalf("config.toml changed:\n%s", got)
		}
		if a.terminalKeys.Overridden("snippet.expand") {
			t.Fatal("the action is overridden")
		}
		typeKeys(a, "snip")
		feed(a, own)
		if got := buf.ed.Text(); got != "S" {
			t.Fatalf("key of this mode: text %q, want S", got)
		}

		a, buf, _ = promptUnchangedApp(t, vimOn)
		typeKeys(a, "snip")
		feed(a, other)
		if got := buf.ed.Text(); got == "S" {
			t.Fatalf("the key of the other mode expanded: %q", got)
		}
	})
}
