package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ZenNotes/tui/internal/vim"
)

// The snippet keys must arrive from the terminal as the chords the session
// checks for, and Backspace must stay a different key.
func TestSnippetChordsFromTerminal(t *testing.T) {
	cases := map[string]tea.KeyMsg{
		"ctrl+h": {Type: tea.KeyCtrlH},
		"ctrl+l": {Type: tea.KeyCtrlL},
		"ctrl+n": {Type: tea.KeyCtrlN},
	}
	for name, msg := range cases {
		k := keyFromTea(msg)
		want := map[string]vim.Key{"ctrl+h": vim.Ctrl('h'), "ctrl+l": vim.Ctrl('l'), "ctrl+n": vim.Ctrl('n')}[name]
		if k != want {
			t.Fatalf("%s arrived as %+v", name, k)
		}
	}
	if k := keyFromTea(tea.KeyMsg{Type: tea.KeyBackspace}); k.IsCtrl('h') {
		t.Fatal("Backspace is mapped to Ctrl+H")
	}
}
