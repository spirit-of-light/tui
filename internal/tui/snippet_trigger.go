package tui

import (
	"unicode/utf8"

	"github.com/ZenNotes/tui/internal/keymaps"
	"github.com/ZenNotes/tui/internal/vim"
)

// snippetExpandKey reports whether k is a single-key binding of the snippet
// trigger action for the current Vim mode.
func (a *App) snippetExpandKey(k vim.Key) bool {
	if a.terminalKeys == nil {
		a.terminalKeys = keymaps.NewTerminalResolver(nil)
	}
	for _, keys := range a.terminalKeys.KeysFor("snippet.expand", a.prefs.VimMode) {
		if len(keys) == 1 && sameKey(k, keys[0]) {
			return true
		}
	}
	return false
}

// snippetTriggerKey expands the snippet whose trigger the typed word starts.
// It acts only when the key is the trigger key of the current mode, the
// buffer takes text and one trigger matches. In every other case the key goes
// on with its usual function.
func (a *App) snippetTriggerKey(buf *noteBuffer, k vim.Key) bool {
	if buf.loading {
		return false
	}
	if a.prefs.VimMode && buf.ed.Mode() != vim.ModeInsert {
		return false
	}
	if !a.snippetExpandKey(k) {
		return false
	}
	// A running Vim keyword completion cycle keeps its keys.
	if a.prefs.VimMode && buf.ed.KeywordCompletionActive() {
		return false
	}
	prefix := buf.ed.WordBeforeCursor()
	if prefix == "" {
		return false
	}
	matches := a.snippetIndex.Match(prefix)
	if len(matches) >= 2 {
		a.openSnippetPopup(buf, prefix, matches)
		return true
	}
	if len(matches) != 1 {
		return false
	}
	buf.ed.ExpandSnippetOver(utf8.RuneCountInString(prefix), matches[0].Body, a.snippetVars(buf))
	a.recordSnippetOwner(buf)
	a.refreshTagMenu(buf)
	return true
}
