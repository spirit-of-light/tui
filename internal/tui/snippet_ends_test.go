package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ZenNotes/tui/internal/vault"
	"github.com/ZenNotes/tui/internal/vim"
)

// sendMsg sends one message through Update, as Bubble Tea does.
func sendMsg(a *App, msg tea.Msg) {
	a.Update(msg)
}

func sendRunes(a *App, s string) {
	for _, r := range s {
		sendMsg(a, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

func wantSession(t *testing.T, buf *noteBuffer, active bool, what string) {
	t.Helper()
	if got := buf.ed.SnippetActive(); got != active {
		t.Fatalf("%s: session active = %v, want %v", what, got, active)
	}
}

func TestSnippetSessionEndsOnBufferChange(t *testing.T) {
	for _, m := range snippetKeyModes {
		t.Run(m.name, func(t *testing.T) {
			a, buf := snippetKeyApp(t, m.prefs(), "", "$1 $2")
			a.openNote("Plan.md", true)
			sendMsg(a, tea.WindowSizeMsg{Width: 100, Height: 30})
			wantSession(t, buf, false, "after the focus moved to another buffer")
		})
	}
}

func TestSnippetSessionEndsOnBufferCommand(t *testing.T) {
	for _, m := range snippetKeyModes {
		t.Run(m.name, func(t *testing.T) {
			a, buf := snippetKeyApp(t, m.prefs(), "", "$1 $2")
			a.runCommandLine("bn")
			sendMsg(a, tea.WindowSizeMsg{Width: 100, Height: 30})
			wantSession(t, buf, false, "after :bn")
		})
	}
}

func TestSnippetSessionEndsOnTabChange(t *testing.T) {
	for _, m := range snippetKeyModes {
		t.Run(m.name, func(t *testing.T) {
			a, buf := snippetKeyApp(t, m.prefs(), "", "$1 $2")
			a.openNote("Plan.md", true)
			a.activePane.active = a.activePane.findTab("Draft.md")
			sendMsg(a, tea.WindowSizeMsg{Width: 100, Height: 30})
			wantSession(t, buf, true, "back in the note")
			a.cycleTab(1)
			if a.activeBuffer() == buf {
				t.Fatal("the tab change did not leave the note")
			}
			sendMsg(a, tea.WindowSizeMsg{Width: 100, Height: 30})
			wantSession(t, buf, false, "after the tab change")
		})
	}
}

func TestSnippetSessionEndsOnTabKey(t *testing.T) {
	for _, m := range snippetKeyModes {
		t.Run(m.name, func(t *testing.T) {
			a, buf := snippetKeyApp(t, m.prefs(), "", "$1 $2")
			a.openNote("Plan.md", true)
			a.activePane.active = a.activePane.findTab("Draft.md")
			sendMsg(a, tea.WindowSizeMsg{Width: 100, Height: 30})
			wantSession(t, buf, true, "back in the note")
			sendMsg(a, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}, Alt: true})
			if a.activeBuffer() == buf {
				t.Fatal("Alt+3 did not leave the note")
			}
			wantSession(t, buf, false, "after the tab key")
		})
	}
}

func TestSnippetSessionEndsOnPaneChange(t *testing.T) {
	for _, m := range snippetKeyModes {
		t.Run(m.name+" split", func(t *testing.T) {
			a, buf := snippetKeyApp(t, m.prefs(), "", "$1 $2")
			first := a.activePane
			a.splitPane(true)
			sendMsg(a, tea.WindowSizeMsg{Width: 100, Height: 30})
			if a.activePane == first {
				t.Fatal("the split did not move focus")
			}
			wantSession(t, buf, false, "after the split")
		})
		t.Run(m.name+" same note", func(t *testing.T) {
			a, _ := newTagTestApp(t, m.prefs(), "")
			a.splitPane(true)
			second := a.activeBuffer()
			a.expandSnippet(second, "$1 $2")
			wantSession(t, second, true, "start")
			sendMsg(a, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}, Alt: true})
			wantSession(t, second, false, "after Alt+H to the other pane")
		})
		t.Run(m.name+" other note", func(t *testing.T) {
			a, _ := newTagTestApp(t, m.prefs(), "")
			a.splitPane(true)
			a.openNote("Plan.md", true)
			plan := a.activeBuffer()
			a.expandSnippet(plan, "$1 $2")
			wantSession(t, plan, true, "start")
			sendMsg(a, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}, Alt: true})
			if a.activeBuffer() == plan {
				t.Fatal("Alt+H did not move to the other pane")
			}
			wantSession(t, plan, false, "after Alt+H to the other note")
		})
	}
}

func TestSnippetSessionKeepsOnSidebarFocus(t *testing.T) {
	for _, m := range snippetKeyModes {
		t.Run(m.name, func(t *testing.T) {
			a, buf := snippetKeyApp(t, m.prefs(), "", "$1 $2")
			a.sidebarOpen = true
			a.focus = focusSidebar
			sendMsg(a, tea.WindowSizeMsg{Width: 100, Height: 30})
			wantSession(t, buf, true, "after the focus moved to the sidebar")
		})
	}
}

func TestSnippetSessionEndsOnUndo(t *testing.T) {
	t.Run("vim off Ctrl+Z", func(t *testing.T) {
		a, buf := snippetKeyApp(t, vimOffPrefs(), "", "$1 $2")
		sendMsg(a, tea.KeyMsg{Type: tea.KeyCtrlZ})
		wantSession(t, buf, false, "after Ctrl+Z")
	})
	t.Run("vim on Ctrl+O u", func(t *testing.T) {
		a, buf := snippetKeyApp(t, vimPrefs(), "", "$1 $2")
		buf.ed.HandleKey(vim.Ctrl('o'))
		buf.ed.HandleKey(vim.R('u'))
		sendMsg(a, tea.WindowSizeMsg{Width: 100, Height: 30})
		wantSession(t, buf, false, "after Ctrl+O u")
	})
}

func TestSnippetSessionEndsOnReload(t *testing.T) {
	disk := func(t *testing.T, a *App, buf *noteBuffer, body string) vault.NoteContent {
		t.Helper()
		if err := os.WriteFile(filepath.Join(a.opts.Target.Root, buf.path), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		content, err := readSnapshot(a.ctx, a.backend, buf.path)
		if err != nil {
			t.Fatal(err)
		}
		return content
	}
	for _, m := range snippetKeyModes {
		t.Run(m.name+" reconcile", func(t *testing.T) {
			a, buf := snippetKeyApp(t, m.prefs(), "", "$1 $2")
			buf.savedText = buf.ed.Text()
			buf.ed.MarkSaved()
			content := disk(t, a, buf, "changed outside\n")
			a.reconcileBuffer(buf, content, nil)
			if buf.ed.Text() != "changed outside\n" {
				t.Fatalf("the note was not reloaded: %q", buf.ed.Text())
			}
			wantSession(t, buf, false, "after the reload")
		})
		t.Run(m.name+" conflict menu", func(t *testing.T) {
			a, buf := snippetKeyApp(t, m.prefs(), "", "$1 $2")
			disk(t, a, buf, "changed outside\n")
			buf.diskChanged = true
			a.conflictMenu()
			sendRunes(a, "r")
			sendRunes(a, "y")
			if buf.ed.Text() != "changed outside\n" {
				t.Fatalf("the note was not reloaded: %q", buf.ed.Text())
			}
			wantSession(t, buf, false, "after the conflict reload")
		})
	}
}

func TestSnippetSessionKeepsOnSave(t *testing.T) {
	for _, m := range snippetKeyModes {
		t.Run(m.name, func(t *testing.T) {
			a, buf := snippetKeyApp(t, m.prefs(), "", "a $1 b $2")
			sendTea(a, tea.KeyCtrlL)
			cur := buf.ed.Cursor()
			sendMsg(a, tea.KeyMsg{Type: tea.KeyCtrlS})
			wantSession(t, buf, true, "after Ctrl+S")
			wantCursor(t, buf, cur.Line, cur.Col, "after Ctrl+S")
			sendMsg(a, autosaveMsg{})
			wantSession(t, buf, true, "after the autosave tick")
			wantCursor(t, buf, cur.Line, cur.Col, "after the autosave tick")
		})
	}
}
