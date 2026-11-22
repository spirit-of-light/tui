package tui

import (
	"strings"
	"testing"
)

// helpRow returns the help row whose key part starts with id.
func helpRow(t *testing.T, a *App, id string) string {
	t.Helper()
	for _, l := range helpLines(a, "") {
		if key, _, ok := strings.Cut(l, "\t"); ok && strings.HasPrefix(key, id) {
			return l
		}
	}
	t.Fatalf("help has no row for %s", id)
	return ""
}

func TestHelpSnippetKeysDefaults(t *testing.T) {
	a, _ := snippetKeyApp(t, vimPrefs(), "", "$1 $2")
	next := helpRow(t, a, "snippet.nextTabstop")
	for _, want := range []string{"Ctrl+L", "Tab"} {
		if !strings.Contains(next, want) {
			t.Errorf("next row %q lacks %q", next, want)
		}
	}
	prev := helpRow(t, a, "snippet.prevTabstop")
	for _, want := range []string{"Ctrl+H", "Shift+Tab"} {
		if !strings.Contains(prev, want) {
			t.Errorf("previous row %q lacks %q", prev, want)
		}
	}
}

func TestHelpSnippetKeysOverride(t *testing.T) {
	p := overridePrefs(t, true, "[terminal.keymaps]\n\"snippet.nextTabstop\" = \"Alt+N\"\n\"snippet.prevTabstop\" = \"\"\n")
	a, _ := snippetKeyApp(t, p, "", "$1 $2")
	next := helpRow(t, a, "snippet.nextTabstop")
	key, _, _ := strings.Cut(next, "\t")
	if !strings.Contains(key, "Alt+N") || strings.Contains(key, "Ctrl+L") {
		t.Errorf("next key part after override: %q", key)
	}
	prev := helpRow(t, a, "snippet.prevTabstop")
	if key, _, _ := strings.Cut(prev, "\t"); !strings.Contains(key, "unbound") {
		t.Errorf("previous key part when unbound: %q", key)
	}
}
