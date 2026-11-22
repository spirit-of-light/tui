package tui

import (
	"testing"

	"github.com/ZenNotes/tui/internal/keymaps"
	"github.com/ZenNotes/tui/internal/vim"
)

// precedenceFiles has a snippet with two tabstops, a snippet with a nested
// body, and two triggers that start with "pro" like the tags of the test
// vault.
const precedenceFiles = "## box\n\n```\n${1:a} ${2:b}\n```\n\n" +
	"## sn\n\n```\n(${1:b})\n```\n\n" +
	"## pro1\n\n```\nP1\n```\n\n" +
	"## pro2\n\n```\nP2\n```\n"

// sessionStop gives the current stop of the running session, or 0 with no
// session.
func sessionStop(buf *noteBuffer) int {
	cur, _, _, ok := buf.ed.SnippetStatus()
	if !ok {
		return 0
	}
	return cur
}

func TestSnippetTriggerPrecedenceSessionKeyWinsWithVimOff(t *testing.T) {
	a, buf, key := triggerApp(t, false, map[string]string{"a.md": precedenceFiles})
	typeKeys(a, "box")
	feed(a, key)
	if got := sessionStop(buf); got != 1 {
		t.Fatalf("stop %d, want 1", got)
	}
	// Tab is also the next-tabstop key of the session, so it wins over the
	// trigger and does not expand the word before the cursor.
	typeKeys(a, "sn")
	text := buf.ed.Text()
	feed(a, key)
	if got := sessionStop(buf); got != 2 {
		t.Errorf("stop %d, want 2", got)
	}
	if got := buf.ed.Text(); got != text || a.liveSnippetPopup() != nil {
		t.Errorf("text %q popup %v, want %q and no popup", got, a.liveSnippetPopup() != nil, text)
	}
}

func TestSnippetTriggerPrecedenceNestedSessionWithVimOn(t *testing.T) {
	a, buf, key := triggerApp(t, true, map[string]string{"a.md": precedenceFiles})
	typeKeys(a, "box")
	feed(a, key)
	if got := sessionStop(buf); got != 1 {
		t.Fatalf("stop %d, want 1", got)
	}
	// Ctrl+N is not a session key, so the trigger expands inside the session.
	typeKeys(a, "sn")
	feed(a, key)
	if got := buf.ed.Text(); got != "(b) b" {
		t.Fatalf("text %q, want (b) b", got)
	}
	if !buf.ed.SnippetActive() {
		t.Error("the nested session did not start")
	}
	if a.liveSnippetPopup() != nil {
		t.Error("a popup opened")
	}
}

func TestSnippetTriggerPrecedenceTagMenuKeepsCtrlN(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		a, buf, _ := triggerApp(t, vimOn, map[string]string{"a.md": precedenceFiles})
		typeKeys(a, "see #pro")
		m := a.liveTagMenu()
		if m == nil {
			t.Fatal("no tag menu")
		}
		before := m.cursor
		feed(a, vim.Ctrl('n'))
		if a.liveSnippetPopup() != nil {
			t.Error("a snippet popup opened")
		}
		if m = a.liveTagMenu(); m == nil || m.cursor != before+1 {
			t.Errorf("the tag menu did not select the next item: %+v", m)
		}
		if got := buf.ed.Text(); got != "see #pro" {
			t.Errorf("text %q, want it unchanged", got)
		}
	})
}

func TestSnippetTriggerPrecedenceTagMenuKeepsTab(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		a, buf, _ := triggerApp(t, vimOn, map[string]string{"a.md": precedenceFiles})
		typeKeys(a, "see #pro")
		if a.liveTagMenu() == nil {
			t.Fatal("no tag menu")
		}
		feed(a, vim.KeyTab)
		if a.liveSnippetPopup() != nil {
			t.Error("a snippet popup opened")
		}
		if got := buf.ed.Text(); got != "see #project" {
			t.Errorf("text %q, want the tag accepted", got)
		}
	})
}

func TestSnippetTriggerPrecedencePopupTabKeepsTheStop(t *testing.T) {
	a, buf, _ := triggerApp(t, false, map[string]string{"a.md": precedenceFiles})
	a.terminalKeys = keymaps.NewTerminalResolver(map[string][]string{"snippet.expand": {"Alt+S"}})
	key := vim.Key{Rune: 's', Alt: true}
	typeKeys(a, "box")
	feed(a, key)
	if got := sessionStop(buf); got != 1 {
		t.Fatalf("stop %d, want 1", got)
	}
	typeKeys(a, " pro")
	feed(a, key)
	if a.liveSnippetPopup() == nil {
		t.Fatal("no popup in the session")
	}
	first := buf.ed.Text()
	feed(a, vim.KeyTab)
	if a.liveSnippetPopup() == nil || a.snippetPopup.sel != 1 {
		t.Errorf("the popup did not select the next row")
	}
	if got := sessionStop(buf); got != 1 {
		t.Errorf("stop %d, want 1", got)
	}
	if got := buf.ed.Text(); got == first {
		t.Errorf("text %q did not change to the next row", got)
	}
}
