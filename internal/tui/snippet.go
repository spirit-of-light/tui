package tui

import (
	"slices"
	"strings"
	"time"

	"github.com/ZenNotes/tui/internal/keymaps"
	"github.com/ZenNotes/tui/internal/templates"
	"github.com/ZenNotes/tui/internal/vim"
)

// snippetNow is the clock that snippet variables read, so tests can fix it.
var snippetNow = time.Now

// snippetOwner names the pane, tab and buffer that a snippet session runs
// in. A zero value means that no session runs.
type snippetOwner struct {
	pane *pane
	tab  *tab
	buf  *noteBuffer
}

// expandSnippet inserts the expansion of body at the cursor of buf. The
// variables take their values from the note and the clock, in the same way
// as a template does. When the body starts a session, the App records the
// pane, tab and buffer that own it.
func (a *App) expandSnippet(buf *noteBuffer, body string) {
	if buf == nil {
		return
	}
	meta, _ := a.noteMeta(buf.path)
	now := snippetNow()
	vars := func(name, arg string, hasArg bool) (string, bool) {
		return templates.Variable(name, arg, hasArg, meta.Title, now, "en")
	}
	buf.ed.ExpandSnippet(body, vars)
	if buf.ed.SnippetActive() {
		a.snippetOwner = snippetOwner{pane: a.activePane, tab: a.activeTab(), buf: buf}
	}
}

// snippetKey handles the keys of a running snippet session: the keys of the
// next and previous tabstop actions. It acts only while a session runs in
// the buffer and the buffer takes text, so these keys keep their usual
// function everywhere else. Only a single key matches. With Vim off, Esc
// ends the session and then goes on to the editor.
func (a *App) snippetKey(buf *noteBuffer, k vim.Key) bool {
	if !buf.ed.SnippetActive() {
		return false
	}
	if a.prefs.VimMode && buf.ed.Mode() != vim.ModeInsert {
		return false
	}
	if !a.prefs.VimMode && k.Is("esc") {
		buf.ed.EndSnippets()
		return false
	}
	switch {
	case a.terminalKeyBound(k, "snippet.nextTabstop"):
		buf.ed.SnippetNext()
	case a.terminalKeyBound(k, "snippet.prevTabstop"):
		buf.ed.SnippetPrev()
	default:
		return false
	}
	a.refreshTagMenu(buf)
	return true
}

// terminalKeyBound reports whether k is one of the single-key bindings of a
// zn-only action.
func (a *App) terminalKeyBound(k vim.Key, id string) bool {
	if a.terminalKeys == nil {
		a.terminalKeys = keymaps.NewTerminalResolver(nil)
	}
	for _, keys := range a.terminalKeys.Keys(id) {
		if len(keys) == 1 && sameKey(k, keys[0]) {
			return true
		}
	}
	return false
}

// checkSnippetOwner ends the session of the owner buffer when the focused
// editor pane, tab or buffer is not the one that started it. A focus move to
// the sidebar, a side panel or an overlay keeps the session, because the
// active pane does not change then.
func (a *App) checkSnippetOwner() {
	o := a.snippetOwner
	if o.buf == nil {
		return
	}
	if !o.buf.ed.SnippetActive() {
		a.snippetOwner = snippetOwner{}
		return
	}
	if a.activePane == o.pane && a.activeTab() == o.tab && a.activeBuffer() == o.buf {
		return
	}
	o.buf.ed.EndSnippets()
	a.snippetOwner = snippetOwner{}
}

// warnTerminalKeymaps shows one error that lists the [terminal.keymaps]
// entries zn ignored. It stays quiet when there is no problem or when the
// set of problems is the one that the last call already showed.
func (a *App) warnTerminalKeymaps(problems []string) {
	if slices.Equal(problems, a.terminalKeymapWarned) {
		return
	}
	a.terminalKeymapWarned = slices.Clone(problems)
	if len(problems) == 0 {
		return
	}
	a.notifyError("zn ignores these [terminal.keymaps] entries: " + strings.Join(problems, "; "))
}
