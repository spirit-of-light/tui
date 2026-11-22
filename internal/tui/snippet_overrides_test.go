package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ZenNotes/tui/internal/config"
	"github.com/ZenNotes/tui/internal/vim"
)

// overridePrefs loads the preferences of a config.toml with the given
// content, with Vim mode on or off.
func overridePrefs(t *testing.T, vimOn bool, toml string) config.Prefs {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("ZENNOTES_CONFIG_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _, err := config.LoadPrefs()
	if err != nil {
		t.Fatal(err)
	}
	p.VimInsertEscape = ""
	p.VimMode = vimOn
	return p
}

var (
	altN = vim.Key{Rune: 'n', Alt: true}
	altM = vim.Key{Rune: 'm', Alt: true}
	altP = vim.Key{Rune: 'p', Alt: true}
	altE = vim.Key{Rune: 'e', Alt: true}
)

func TestSnippetKeyOverridesNextAndPrevious(t *testing.T) {
	for _, m := range snippetKeyModes {
		t.Run(m.name, func(t *testing.T) {
			p := overridePrefs(t, m.name == "vim on", "[terminal.keymaps]\n\"snippet.nextTabstop\" = \"Alt+N\"\n\"snippet.prevTabstop\" = \"Alt+P\"\n")
			a, buf := snippetKeyApp(t, p, "", "$1 $2 $3")
			a.handleKey(altN)
			wantCursor(t, buf, 0, 1, "Alt+N")
			a.handleKey(altN)
			wantCursor(t, buf, 0, 2, "Alt+N again")
			a.handleKey(altP)
			wantCursor(t, buf, 0, 1, "Alt+P")
			if got := buf.ed.Text(); got != "  " {
				t.Fatalf("override keys changed the text: %q", got)
			}
		})
	}
}

func TestSnippetKeyOverridesList(t *testing.T) {
	for _, m := range snippetKeyModes {
		t.Run(m.name, func(t *testing.T) {
			p := overridePrefs(t, m.name == "vim on", "[terminal.keymaps]\n\"snippet.nextTabstop\" = [\"Alt+N\", \"Alt+M\"]\n")
			a, buf := snippetKeyApp(t, p, "", "$1 $2 $3")
			a.handleKey(altN)
			wantCursor(t, buf, 0, 1, "Alt+N")
			a.handleKey(altM)
			wantCursor(t, buf, 0, 2, "Alt+M")
		})
	}
}

func TestSnippetKeyOverridesReplaceDefaults(t *testing.T) {
	for _, m := range snippetKeyModes {
		t.Run(m.name, func(t *testing.T) {
			p := overridePrefs(t, m.name == "vim on", "[terminal.keymaps]\n\"snippet.nextTabstop\" = \"Alt+N\"\n")
			a, buf := snippetKeyApp(t, p, "", "- [ ] $1 $2")
			sendTea(a, tea.KeyCtrlL)
			if got := buf.ed.Text(); got != "- [x]  " {
				t.Fatalf("Ctrl+L after an override: %q, want the checkbox toggle", got)
			}
			a, buf = snippetKeyApp(t, p, "", "- a$1\n- b$2")
			a.handleKey(vim.Key{Name: "tab"})
			if got := buf.ed.Text(); got == "- a\n- b" {
				t.Fatal("Tab did not indent after an override")
			}
			if cur, _, _, _ := buf.ed.SnippetStatus(); cur != 1 {
				t.Fatalf("Tab moved to tabstop %d", cur)
			}
		})
	}
}

func TestSnippetKeyOverridesUnbound(t *testing.T) {
	for _, m := range snippetKeyModes {
		t.Run(m.name, func(t *testing.T) {
			p := overridePrefs(t, m.name == "vim on", "[terminal.keymaps]\n\"snippet.nextTabstop\" = \"\"\n\"snippet.prevTabstop\" = []\n")
			a, buf := snippetKeyApp(t, p, "", "- [ ] $1 $2")
			sendTea(a, tea.KeyCtrlL)
			if got := buf.ed.Text(); got != "- [x]  " {
				t.Fatalf("Ctrl+L with no next key: %q", got)
			}
			if cur, _, _, _ := buf.ed.SnippetStatus(); cur != 1 {
				t.Fatalf("Ctrl+L moved to tabstop %d", cur)
			}

			a, buf = snippetKeyApp(t, p, "", "$1")
			typeKeys(a, "ab")
			sendTea(a, tea.KeyCtrlH)
			if got := buf.ed.Text(); got != "a" {
				t.Fatalf("0x08 with no previous key: %q, want %q", got, "a")
			}
		})
	}
}

func TestSnippetKeyOverridesSharedGlobalKey(t *testing.T) {
	for _, m := range snippetKeyModes {
		t.Run(m.name, func(t *testing.T) {
			p := overridePrefs(t, m.name == "vim on", "[keymaps]\n\"global.modeEdit\" = \"Alt+E\"\n[terminal.keymaps]\n\"snippet.nextTabstop\" = \"Alt+E\"\n")
			a, buf := snippetKeyApp(t, p, "", "$1 $2")
			mode := a.activeTab().mode
			a.handleKey(altE)
			wantCursor(t, buf, 0, 1, "Alt+E in a session")
			if a.activeTab().mode != mode {
				t.Fatalf("pane mode changed in a session: %v, was %v", a.activeTab().mode, mode)
			}

			a, _ = snippetKeyApp(t, p, "", "$1")
			a.handleKey(vim.KeyEsc)
			a.setPaneMode(modePreview)
			a.handleKey(altE)
			if a.activeTab().mode != modeEdit {
				t.Fatalf("Alt+E with no session left the pane in mode %v", a.activeTab().mode)
			}
		})
	}
}

func TestSnippetKeyOverridesNoTable(t *testing.T) {
	for _, m := range snippetKeyModes {
		t.Run(m.name, func(t *testing.T) {
			p := overridePrefs(t, m.name == "vim on", "")
			a, buf := snippetKeyApp(t, p, "", "$1 $2 $3")
			sendTea(a, tea.KeyCtrlL)
			sendTea(a, tea.KeyTab)
			wantCursor(t, buf, 0, 2, "Ctrl+L and Tab")
			sendTea(a, tea.KeyCtrlH)
			sendTea(a, tea.KeyShiftTab)
			wantCursor(t, buf, 0, 0, "Ctrl+H and Shift+Tab")
		})
	}
}

func TestSnippetKeyOverridesBadEntryKeepsGoodOnes(t *testing.T) {
	for _, m := range snippetKeyModes {
		t.Run(m.name, func(t *testing.T) {
			p := overridePrefs(t, m.name == "vim on", "[terminal.keymaps]\n\"snippet.nope\" = \"Alt+N\"\n\"snippet.prevTabstop\" = \"Alt+P\"\n")
			a, buf := snippetKeyApp(t, p, "", "$1 $2")
			sendTea(a, tea.KeyCtrlL)
			wantCursor(t, buf, 0, 1, "default next key")
			a.handleKey(altP)
			wantCursor(t, buf, 0, 0, "Alt+P")
		})
	}
}

func TestSnippetKeyOverridesReload(t *testing.T) {
	a, buf := snippetKeyApp(t, vimPrefs(), "", "$1 $2")
	p := vimPrefs()
	p.TerminalKeymapOverrides = map[string][]string{"snippet.nextTabstop": {"Alt+N"}}
	a.applyPreferences(p)
	a.handleKey(altN)
	wantCursor(t, buf, 0, 1, "Alt+N after a reload")
}
