package tui

import (
	"testing"

	"github.com/ZenNotes/tui/internal/keymaps"
	"github.com/ZenNotes/tui/internal/vim"
)

// acceptFiles has three triggers that start with "me".
const acceptFiles = "## meet\n\n```\nM\n```\n\n" +
	"## meet1\n\n```\nM1 $1\n```\n\n" +
	"## meet2\n\n```\nM2\n```\n"

// selectRow cycles the popup until the row with the text want is selected.
func selectRow(t *testing.T, a *App, want string) {
	t.Helper()
	for i := 0; i < 8 && a.snippetPopup != nil; i++ {
		if a.snippetPopup.items[a.snippetPopup.sel].text == want {
			return
		}
		feed(a, vim.Ctrl('n'))
	}
	t.Fatalf("row %q not reached", want)
}

// openPopupOn types "me", opens the popup and selects the row want.
func openPopupOn(t *testing.T, a *App, key vim.Key, want string) {
	t.Helper()
	typeKeys(a, "me")
	feed(a, key)
	selectRow(t, a, want)
}

func TestSnippetPopupAcceptExpandsATrigger(t *testing.T) {
	for _, accept := range []vim.Key{vim.KeyEnter, vim.Ctrl('y')} {
		bothModes(t, func(t *testing.T, vimOn bool) {
			a, buf, key := popupApp(t, vimOn, map[string]string{"a.md": acceptFiles})
			openPopupOn(t, a, key, "meet1")
			feed(a, accept)
			if a.snippetPopup != nil {
				t.Error("popup still open")
			}
			if got := buf.ed.Text(); got != "M1 " {
				t.Errorf("text %q, want %q with no line break", got, "M1 ")
			}
			if cur, _, _, ok := buf.ed.SnippetStatus(); !ok || cur != 1 {
				t.Errorf("session status %d %v, want stop 1", cur, ok)
			}
		})
	}
}

func TestSnippetPopupAcceptFirstRow(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		a, buf, key := popupApp(t, vimOn, map[string]string{"a.md": acceptFiles})
		typeKeys(a, "me")
		feed(a, key, vim.KeyEnter)
		if got := buf.ed.Text(); got != "M" {
			t.Errorf("text %q, want M", got)
		}
	})
}

func TestSnippetPopupAcceptKeepsABufferWord(t *testing.T) {
	for _, accept := range []vim.Key{vim.KeyEnter, vim.Ctrl('y')} {
		bothModes(t, func(t *testing.T, vimOn bool) {
			a, buf, key := popupApp(t, vimOn, map[string]string{"a.md": acceptFiles})
			typeKeys(a, "meeting")
			feed(a, vim.KeyEnter)
			openPopupOn(t, a, key, "meeting")
			feed(a, accept)
			if a.snippetPopup != nil || buf.ed.SnippetActive() {
				t.Error("popup open or session started")
			}
			if got := buf.ed.Text(); got != "meeting\nmeeting" {
				t.Errorf("text %q, want meeting twice and no extra line break", got)
			}
		})
	}
}

func TestSnippetPopupAcceptRestoreWithCtrlE(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		a, buf, key := popupApp(t, vimOn, map[string]string{"a.md": acceptFiles})
		openPopupOn(t, a, key, "meet1")
		feed(a, vim.Ctrl('e'))
		if a.snippetPopup != nil || buf.ed.Text() != "me" {
			t.Errorf("popup %v, text %q, want closed and me", a.snippetPopup != nil, buf.ed.Text())
		}
	})
}

func TestSnippetPopupAcceptOtherKeysKeepTheText(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		for typed, want := range map[rune]string{'x': "meet1x", ' ': "meet1 "} {
			a, buf, key := popupApp(t, vimOn, map[string]string{"a.md": acceptFiles})
			openPopupOn(t, a, key, "meet1")
			feed(a, vim.R(typed))
			if a.snippetPopup != nil || buf.ed.Text() != want {
				t.Errorf("popup %v, text %q, want closed and %q", a.snippetPopup != nil, buf.ed.Text(), want)
			}
		}
	})
}

func TestSnippetPopupAcceptEscVimOn(t *testing.T) {
	a, buf, key := popupApp(t, true, map[string]string{"a.md": acceptFiles})
	openPopupOn(t, a, key, "meet1")
	feed(a, vim.KeyEsc)
	if a.snippetPopup != nil || buf.ed.Text() != "meet1" || buf.ed.Mode() != vim.ModeNormal {
		t.Errorf("popup %v, text %q, mode %v, want closed, meet1 and normal", a.snippetPopup != nil, buf.ed.Text(), buf.ed.Mode())
	}
}

func TestSnippetPopupAcceptEscVimOff(t *testing.T) {
	a, buf, key := popupApp(t, false, map[string]string{"a.md": acceptFiles})
	openPopupOn(t, a, key, "meet1")
	feed(a, vim.KeyEsc)
	if a.snippetPopup != nil || buf.ed.Text() != "meet1" {
		t.Errorf("popup %v, text %q, want closed and meet1", a.snippetPopup != nil, buf.ed.Text())
	}
}

func TestSnippetPopupAcceptEscKeepsTheSession(t *testing.T) {
	files := "## box\n\n```\n${1:a} ${2:b}\n```\n\n" + acceptFiles
	a, buf, key := popupApp(t, false, map[string]string{"a.md": files})
	a.terminalKeys = keymaps.NewTerminalResolver(map[string][]string{"snippet.expand": {"Alt+S"}})
	key = vim.Key{Rune: 's', Alt: true}
	typeKeys(a, "box")
	feed(a, key)
	if !buf.ed.SnippetActive() {
		t.Fatal("no session")
	}
	typeKeys(a, " me")
	feed(a, key)
	if a.snippetPopup == nil {
		t.Fatal("no popup in the session")
	}
	feed(a, vim.KeyEsc)
	if a.snippetPopup != nil || !buf.ed.SnippetActive() {
		t.Errorf("popup %v, session %v, want closed popup and a live session", a.snippetPopup != nil, buf.ed.SnippetActive())
	}
}

func TestSnippetPopupAcceptUndoVimOff(t *testing.T) {
	a, buf, key := popupApp(t, false, map[string]string{"a.md": "## meet\n\n```\nM\n```\n\n## meet1\n\n```\nM1\n```\n"})
	typeKeys(a, "x me")
	feed(a, key, vim.Ctrl('n'), vim.KeyEnter)
	if got := buf.ed.Text(); got != "x M1" {
		t.Fatalf("text %q, want x M1", got)
	}
	feed(a, vim.Ctrl('z'))
	if got := buf.ed.Text(); got != "x me" {
		t.Errorf("after one undo %q, want x me", got)
	}
}

func TestSnippetPopupAcceptWordThenTypeUndoVimOff(t *testing.T) {
	a, buf, key := popupApp(t, false, map[string]string{"a.md": acceptFiles})
	typeKeys(a, "x meeting")
	feed(a, vim.KeyEnter)
	typeKeys(a, "x me")
	feed(a, key)
	selectRow(t, a, "meeting")
	feed(a, vim.KeyEnter)
	typeKeys(a, "z")
	feed(a, vim.Ctrl('z'))
	if got := buf.ed.Text(); got != "x meeting\nx meeting" {
		t.Errorf("after one undo %q, want the word kept and z removed", got)
	}
}
