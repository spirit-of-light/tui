package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ZenNotes/tui/internal/config"
	"github.com/ZenNotes/tui/internal/vim"
)

// sendTea sends one key as the terminal reports it, through keyFromTea.
func sendTea(a *App, types ...tea.KeyType) {
	for _, ty := range types {
		a.handleKey(keyFromTea(tea.KeyMsg{Type: ty}))
	}
}

type snippetKeyMode struct {
	name  string
	prefs func() config.Prefs
}

var snippetKeyModes = []snippetKeyMode{
	{"vim on", vimPrefs},
	{"vim off", vimOffPrefs},
}

// snippetKeyApp opens a note with the draft and expands body at the cursor.
func snippetKeyApp(t *testing.T, prefs config.Prefs, draft, body string) (*App, *noteBuffer) {
	t.Helper()
	snippetFixNow(t, time.Date(2026, 3, 5, 14, 7, 0, 0, time.UTC))
	a, buf := newTagTestApp(t, prefs, draft)
	a.expandSnippet(buf, body)
	if !buf.ed.SnippetActive() {
		t.Fatalf("body %q started no session", body)
	}
	return a, buf
}

func wantCursor(t *testing.T, buf *noteBuffer, line, col int, what string) {
	t.Helper()
	if c := buf.ed.Cursor(); c.Line != line || c.Col != col {
		t.Fatalf("%s: cursor %d:%d, want %d:%d", what, c.Line, c.Col, line, col)
	}
}

func TestSnippetKeysNextAndPrevious(t *testing.T) {
	for _, m := range snippetKeyModes {
		t.Run(m.name, func(t *testing.T) {
			a, buf := snippetKeyApp(t, m.prefs(), "", "$1 $2 $3")
			text := buf.ed.Text()
			wantCursor(t, buf, 0, 0, "start")
			sendTea(a, tea.KeyCtrlL)
			wantCursor(t, buf, 0, 1, "Ctrl+L")
			sendTea(a, tea.KeyTab)
			wantCursor(t, buf, 0, 2, "Tab")
			sendTea(a, tea.KeyCtrlH)
			wantCursor(t, buf, 0, 1, "Ctrl+H")
			sendTea(a, tea.KeyShiftTab)
			wantCursor(t, buf, 0, 0, "Shift+Tab")
			sendTea(a, tea.KeyShiftTab)
			wantCursor(t, buf, 0, 0, "previous at the first tabstop")
			if !buf.ed.SnippetActive() {
				t.Fatal("previous at the first tabstop ended the session")
			}
			if got := buf.ed.Text(); got != text {
				t.Fatalf("session keys changed the text: %q, want %q", got, text)
			}
		})
	}
}

func TestSnippetKeysFinalTabstop(t *testing.T) {
	for _, m := range snippetKeyModes {
		t.Run(m.name, func(t *testing.T) {
			a, buf := snippetKeyApp(t, m.prefs(), "", "$1 x$0y")
			sendTea(a, tea.KeyCtrlL)
			wantCursor(t, buf, 0, 2, "single $0")
			if buf.ed.SnippetActive() {
				t.Fatal("session stays after the single $0")
			}

			a, buf = snippetKeyApp(t, m.prefs(), "", "$1 $2 end")
			sendTea(a, tea.KeyCtrlL, tea.KeyCtrlL)
			wantCursor(t, buf, 0, 5, "implicit end")
			if buf.ed.SnippetActive() {
				t.Fatal("session stays after the implicit end")
			}

			a, buf = snippetKeyApp(t, m.prefs(), "", "$0 x $0")
			text := buf.ed.Text()
			wantCursor(t, buf, 0, 0, "mirrored $0")
			sendTea(a, tea.KeyCtrlL)
			wantCursor(t, buf, 0, 0, "next at a mirrored $0")
			if buf.ed.SnippetActive() || buf.ed.Text() != text {
				t.Fatalf("next at a mirrored $0: active %v, text %q", buf.ed.SnippetActive(), buf.ed.Text())
			}
		})
	}
}

func TestSnippetKeysEscEndsSession(t *testing.T) {
	for _, m := range snippetKeyModes {
		t.Run(m.name, func(t *testing.T) {
			a, buf := snippetKeyApp(t, m.prefs(), "", "$1 $2")
			sendTea(a, tea.KeyEsc)
			if buf.ed.SnippetActive() {
				t.Fatal("Esc left the session running")
			}
			if a.prefs.VimMode && buf.ed.Mode() != vim.ModeNormal {
				t.Fatalf("mode %v after Esc, want normal", buf.ed.Mode())
			}
		})
	}
}

func TestSnippetKeysVimLeaveKeys(t *testing.T) {
	keys := map[string]vim.Key{
		"Esc":    vim.KeyEsc,
		"Ctrl+C": {Rune: 'c', Ctrl: true},
		"Ctrl+[": keyFromTea(tea.KeyMsg{Type: tea.KeyCtrlOpenBracket}),
	}
	for name, k := range keys {
		t.Run(name, func(t *testing.T) {
			a, buf := snippetKeyApp(t, vimPrefs(), "", "$1 ${2:b}")
			a.handleKey(k)
			if buf.ed.SnippetActive() {
				t.Fatal("session still runs")
			}
			if buf.ed.Mode() != vim.ModeNormal {
				t.Fatalf("mode %v, want normal", buf.ed.Mode())
			}
		})
	}
}

func TestSnippetKeysCheckboxOnlyOutsideSession(t *testing.T) {
	for _, m := range snippetKeyModes {
		t.Run(m.name, func(t *testing.T) {
			a, buf := snippetKeyApp(t, m.prefs(), "", "- [ ] $1 $2")
			sendTea(a, tea.KeyCtrlL)
			if got := buf.ed.Text(); got != "- [ ]  " {
				t.Fatalf("Ctrl+L in a session changed the text: %q", got)
			}
			sendTea(a, tea.KeyEsc)
			if a.prefs.VimMode {
				typeKeys(a, "i")
			}
			cur := buf.ed.Cursor()
			sendTea(a, tea.KeyCtrlL)
			if got := buf.ed.Text(); got != "- [x]  " {
				t.Fatalf("Ctrl+L after the session: %q", got)
			}
			if buf.ed.Cursor() != cur {
				t.Fatalf("cursor moved from %v to %v", cur, buf.ed.Cursor())
			}
		})
	}
}

func TestSnippetKeysCheckboxAfterNestedSession(t *testing.T) {
	for _, m := range snippetKeyModes {
		t.Run(m.name, func(t *testing.T) {
			a, buf := snippetKeyApp(t, m.prefs(), "", "- [ ] $1 $2")
			a.expandSnippet(buf, "(${1:a}) $3")
			if !buf.ed.SnippetActive() {
				t.Fatal("nested expansion started no session")
			}
			sendTea(a, tea.KeyEsc)
			if buf.ed.SnippetActive() {
				t.Fatal("Esc left a nested session running")
			}
			if a.prefs.VimMode {
				typeKeys(a, "i")
			}
			sendTea(a, tea.KeyCtrlL)
			if got := buf.ed.Text(); got != "- [x] (a)  " {
				t.Fatalf("Ctrl+L after Esc: %q", got)
			}
		})
	}
}

func TestSnippetKeysOutsideSessionKeepTheirFunction(t *testing.T) {
	for _, m := range snippetKeyModes {
		t.Run(m.name, func(t *testing.T) {
			// A fresh editor gives the reference text for Tab and Shift+Tab.
			fresh, freshBuf := newTagTestApp(t, m.prefs(), "- a\n- b")
			after, afterBuf := snippetKeyApp(t, m.prefs(), "- a\n- b", "$1")
			sendTea(after, tea.KeyEsc)
			for _, p := range []struct {
				a   *App
				buf *noteBuffer
			}{{fresh, freshBuf}, {after, afterBuf}} {
				if p.a.prefs.VimMode {
					feed(p.a, vim.R('j'), vim.R('I'))
				} else {
					p.buf.ed.SetCursor(vim.Pos{Line: 1, Col: 0})
				}
				sendTea(p.a, tea.KeyTab)
			}
			if got, want := afterBuf.ed.Text(), freshBuf.ed.Text(); got != want {
				t.Fatalf("Tab after a session: %q, fresh editor %q", got, want)
			}
			if afterBuf.ed.Text() == "- a\n- b" {
				t.Fatal("Tab did not indent the list item")
			}
			sendTea(after, tea.KeyShiftTab)
			if got := afterBuf.ed.Text(); got != "- a\n- b" {
				t.Fatalf("Shift+Tab after Tab: %q", got)
			}
		})
	}
}

func TestSnippetKeysBackspaceBytes(t *testing.T) {
	for _, m := range snippetKeyModes {
		for name, ty := range map[string]tea.KeyType{"0x08": tea.KeyCtrlH, "0x7f": tea.KeyBackspace} {
			t.Run(m.name+" "+name, func(t *testing.T) {
				snippetFixNow(t, time.Date(2026, 3, 5, 14, 7, 0, 0, time.UTC))
				a, buf := newTagTestApp(t, m.prefs(), "")
				if a.prefs.VimMode {
					typeKeys(a, "i")
				}
				typeKeys(a, "ab")
				sendTea(a, ty)
				if got := buf.ed.Text(); got != "a" {
					t.Fatalf("text %q, want %q", got, "a")
				}
			})
		}
	}
}

func TestSnippetKeysBackspaceByteInSession(t *testing.T) {
	for _, m := range snippetKeyModes {
		t.Run(m.name, func(t *testing.T) {
			a, buf := snippetKeyApp(t, m.prefs(), "", "$1 $2")
			sendTea(a, tea.KeyCtrlL)
			text := buf.ed.Text()
			sendTea(a, tea.KeyCtrlH)
			wantCursor(t, buf, 0, 0, "0x08 at the second tabstop")
			if got := buf.ed.Text(); got != text {
				t.Fatalf("0x08 deleted a character: %q, want %q", got, text)
			}
		})
	}
}

func TestSnippetKeysTagMenu(t *testing.T) {
	for _, m := range snippetKeyModes {
		t.Run(m.name, func(t *testing.T) {
			a, buf := snippetKeyApp(t, m.prefs(), "", "see $1\nend $2")
			typeKeys(a, "#pro")
			if menuTags(a) == "" {
				t.Fatal("the tag menu did not open")
			}
			sendTea(a, tea.KeyTab)
			if got := buf.ed.Text(); got == "see #pro\nend " {
				t.Fatalf("Tab did not accept the tag: %q", got)
			}
			if !buf.ed.SnippetActive() {
				t.Fatal("accepting a tag ended the session")
			}
			if cur, _, _, _ := buf.ed.SnippetStatus(); cur != 1 {
				t.Fatalf("tabstop %d after Tab, want 1", cur)
			}
			if c := buf.ed.Cursor(); c.Line != 0 {
				t.Fatalf("cursor left the first tabstop: %v", c)
			}

			a, buf = snippetKeyApp(t, m.prefs(), "", "see $1\nend $2")
			typeKeys(a, "#pro")
			if menuTags(a) == "" {
				t.Fatal("the tag menu did not open")
			}
			sendTea(a, tea.KeyCtrlL)
			if cur, _, _, _ := buf.ed.SnippetStatus(); cur != 2 {
				t.Fatalf("tabstop %d after Ctrl+L, want 2", cur)
			}
			wantCursor(t, buf, 1, 4, "Ctrl+L with a menu")
		})
	}
}

func TestSnippetKeysFromTea(t *testing.T) {
	if k := keyFromTea(tea.KeyMsg{Type: tea.KeyCtrlH}); !k.IsCtrl('h') {
		t.Errorf("0x08 gives %+v, want Ctrl+h", k)
	}
	if k := keyFromTea(tea.KeyMsg{Type: tea.KeyBackspace}); !k.Is("backspace") {
		t.Errorf("0x7f gives %+v, want backspace", k)
	}
	if k := keyFromTea(tea.KeyMsg{Type: tea.KeyShiftTab}); k.Name != "tab" || !k.Shift {
		t.Errorf("Shift+Tab gives %+v", k)
	}
}
