package tui

import (
	"unicode/utf8"

	"github.com/ZenNotes/tui/internal/snippet"
	"github.com/ZenNotes/tui/internal/vim"
)

// snippetCandidate is one row of the snippet popup. A trigger row has the
// snippet. A buffer-word row has none.
type snippetCandidate struct {
	text string
	snip *snippet.Snippet
}

// snippetPopup is the list that opens when two or more triggers match the
// word before the cursor. The text of the selected row shows in the buffer in
// place of the typed prefix.
type snippetPopup struct {
	buf    *noteBuffer
	path   string
	prefix string
	// start is the column of the first rune of the shown text.
	start int
	items []snippetCandidate
	sel   int
	// shown is the number of runes of the selected text in the buffer.
	shown int
	// at is where the cursor stood after the last change of the popup. A
	// cursor that moved by other means makes the popup stale.
	at vim.Pos
}

// snippetCandidates lists the triggers that match, in index order, then the
// words of the buffer that match, each one time. It leaves out the typed
// prefix and every word that is the exact text of a listed trigger.
func snippetCandidates(matches []snippet.Snippet, words []string, prefix string) []snippetCandidate {
	items := make([]snippetCandidate, 0, len(matches)+len(words))
	listed := map[string]bool{prefix: true}
	for i := range matches {
		items = append(items, snippetCandidate{text: matches[i].Trigger, snip: &matches[i]})
		listed[matches[i].Trigger] = true
	}
	for _, w := range words {
		if listed[w] || !snippet.MatchPrefix(w, prefix) {
			continue
		}
		listed[w] = true
		items = append(items, snippetCandidate{text: w})
	}
	return items
}

// openSnippetPopup opens the popup over the typed prefix and shows the first
// row in the text. The tag menu never stays open with it.
func (a *App) openSnippetPopup(buf *noteBuffer, prefix string, matches []snippet.Snippet) {
	n := utf8.RuneCountInString(prefix)
	cur := buf.ed.Cursor()
	p := &snippetPopup{
		buf:    buf,
		path:   buf.path,
		prefix: prefix,
		start:  cur.Col - n,
		items:  snippetCandidates(matches, buf.ed.BufferWords(), prefix),
	}
	a.tagMenu = nil
	a.snippetPopup = p
	p.show(n, 0)
}

// show puts the text of row i in the buffer in place of the n runes before
// the cursor.
func (p *snippetPopup) show(n, i int) {
	p.sel = i
	text := p.items[i].text
	p.buf.ed.ReplaceBeforeCursor(n, text)
	p.shown = utf8.RuneCountInString(text)
	p.at = p.buf.ed.Cursor()
}

// closeSnippetPopup closes the popup and keeps the text in the buffer. With
// Vim off, the change of the popup commits.
func (a *App) closeSnippetPopup(p *snippetPopup) {
	a.snippetPopup = nil
	p.buf.ed.EndTriggerChange()
}

// liveSnippetPopup gives the popup if it still belongs to what is on screen.
// A popup whose buffer, cursor, mode, focus or overlay changed is closed.
func (a *App) liveSnippetPopup() *snippetPopup {
	p := a.snippetPopup
	if p == nil {
		return nil
	}
	buf := a.activeBuffer()
	if buf == nil || buf != p.buf || a.overlay != nil || a.focus != focusPane || buf.path != p.path ||
		buf.ed.Mode() != vim.ModeInsert || buf.ed.Cursor() != p.at {
		a.closeSnippetPopup(p)
		return nil
	}
	return p
}

// snippetPopupKey takes the keys of an open popup. The keys that select the
// next and the previous row move the selection and wrap. Enter and Ctrl+Y use
// the selected row, and Ctrl+E puts back the typed prefix. Esc closes the popup
// and keeps the text; with Vim on the key then goes on, and without Vim the
// popup takes it. Any other key closes the popup, keeps the text and goes on.
func (a *App) snippetPopupKey(buf *noteBuffer, k vim.Key) bool {
	p := a.liveSnippetPopup()
	if p == nil {
		return false
	}
	n := len(p.items)
	switch {
	case k.IsCtrl('n') || k.Is("tab") || a.snippetExpandKey(k):
		p.show(p.shown, (p.sel+1)%n)
	case k.IsCtrl('p') || k.Is("shift+tab"):
		p.show(p.shown, (p.sel-1+n)%n)
	case k.Is("enter") || k.IsCtrl('y'):
		a.acceptSnippetPopup(buf, p)
	case k.IsCtrl('e'):
		a.snippetPopup = nil
		buf.ed.ReplaceBeforeCursor(p.shown, p.prefix)
		buf.ed.EndTriggerChange()
	case k.Is("esc"):
		a.closeSnippetPopup(p)
		return !a.prefs.VimMode
	default:
		a.closeSnippetPopup(p)
		return false
	}
	return true
}

// acceptSnippetPopup uses the selected row and closes the popup. A trigger
// expands over the text that shows. A word stays as text.
func (a *App) acceptSnippetPopup(buf *noteBuffer, p *snippetPopup) {
	it := p.items[p.sel]
	if it.snip == nil {
		a.closeSnippetPopup(p)
		return
	}
	a.snippetPopup = nil
	buf.ed.ExpandSnippetOver(p.shown, it.snip.Body, a.snippetVars(buf))
	a.recordSnippetOwner(buf)
	a.refreshTagMenu(buf)
}
