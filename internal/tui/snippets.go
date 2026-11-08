package tui

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ZenNotes/tui/internal/snippets"
	"github.com/ZenNotes/tui/internal/templates"
	"github.com/ZenNotes/tui/internal/vim"
)

// snippetKey handles the snippet keys in insert mode. Ctrl+N expands the
// trigger word before the cursor; while a session is open, Ctrl+L and Ctrl+H
// step to the next and previous stop. A key that has nothing to do here is
// left to the usual handling, so outside a session these keys keep their
// normal meaning.
func (a *App) snippetKey(buf *noteBuffer, k vim.Key) bool {
	if buf.ed.Mode() != vim.ModeInsert {
		return false
	}
	nextKey := a.keymap.Binding("editor.snippetNext")
	prevKey := a.keymap.Binding("editor.snippetPrev")
	expandKey := a.keymap.Binding("editor.snippetExpand")

	// Handle active snippet preview navigation
	if buf.snippetPreview != nil && buf.snippetPreview.active() {
		nextKeys := bindingKeys(nextKey)
		prevKeys := bindingKeys(prevKey)
		expandKeys := bindingKeys(expandKey)
		if len(nextKeys) == 1 && sameKey(k, nextKeys[0]) {
			buf.snippetPreview.moveDown()
			return true
		}
		if len(prevKeys) == 1 && sameKey(k, prevKeys[0]) {
			buf.snippetPreview.moveUp()
			return true
		}
		if k.Is("esc") {
			buf.snippetPreview = nil
			return true
		}
		if k.Is("enter") || (len(expandKeys) == 1 && sameKey(k, expandKeys[0])) {
			// Expand the selected preview
			trigger := buf.snippetPreview.currentTrigger()
			return a.expandTrigger(buf, trigger)
		}
		// Any other key dismisses the preview and is processed normally
		buf.snippetPreview = nil
	}

	if buf.snip.Active() {
		if span, ok := buf.snip.Replaces(); ok && k.Printable() && buf.ed.OffsetOf(buf.ed.Cursor()) == span.Start {
			// Typing over untouched default text replaces it. The deletion is synced
			// now, so mirrors of the stop are cleared with it; the key then types
			// into the empty stop.
			buf.ed.ReplaceSpan(span.Start, span.End, "")
			applySnippetReps(buf, buf.snip.Sync(buf.ed.Text()))
			return false
		}
		nextKeys := bindingKeys(nextKey)
		prevKeys := bindingKeys(prevKey)
		if len(nextKeys) == 1 && sameKey(k, nextKeys[0]) {
			if off, ok := buf.snip.Next(); ok {
				buf.ed.SetCursor(buf.ed.PosOf(off))
				return true
			}
			// Past the final stop the session is over and Ctrl+L does its usual job.
			buf.snip = nil
			return false
		}
		if len(prevKeys) == 1 && sameKey(k, prevKeys[0]) {
			if off, ok := buf.snip.Prev(); ok {
				buf.ed.SetCursor(buf.ed.PosOf(off))
				return true
			}
			return false
		}
	}
	expandKeys := bindingKeys(expandKey)
	if len(expandKeys) == 1 && sameKey(k, expandKeys[0]) {
		return a.expandSnippetAtCursor(buf)
	}
	return false
}

// expandTrigger expands a specific trigger word. It's used when the user
// confirms a snippet from the preview popup or via the normal expand key.
func (a *App) expandTrigger(buf *noteBuffer, trigger string) bool {
	sn, ok := a.snippetLib.Lookup(trigger)
	if !ok {
		return false
	}

	cur := buf.ed.Cursor()
	line := buf.ed.LineRunes(cur.Line)
	end := min(cur.Col, len(line))
	start := end
	for start > 0 && snippets.IsWordRune(line[start-1]) {
		start--
	}

	title := ""
	if meta, ok := a.noteMeta(buf.path); ok {
		title = meta.Title
	}
	exp := snippets.Parse(sn.Body, templates.Vars(title, time.Now(), "en"))
	from := buf.ed.OffsetOf(vim.Pos{Line: cur.Line, Col: start})
	to := buf.ed.OffsetOf(cur)
	buf.ed.ReplaceSpan(from, to, exp.Text)
	a.beginSnippetSession(buf, exp, from, true)
	buf.snippetPreview = nil
	return true
}

// expandSnippetAtCursor expands the trigger word that ends at the cursor. It
// reports false, changing nothing, when there is no such trigger, so Ctrl+N
// keeps its word completion everywhere else.
func (a *App) expandSnippetAtCursor(buf *noteBuffer) bool {
	cur := buf.ed.Cursor()
	line := buf.ed.LineRunes(cur.Line)
	end := min(cur.Col, len(line))
	start := end
	for start > 0 && snippets.IsWordRune(line[start-1]) {
		start--
	}
	if start == end {
		return false
	}
	sn, ok := a.snippetLib.Lookup(string(line[start:end]))
	if !ok {
		return false
	}
	title := ""
	if meta, ok := a.noteMeta(buf.path); ok {
		title = meta.Title
	}
	exp := snippets.Parse(sn.Body, templates.Vars(title, time.Now(), "en"))
	from := buf.ed.OffsetOf(vim.Pos{Line: cur.Line, Col: start})
	to := buf.ed.OffsetOf(cur)
	buf.ed.ReplaceSpan(from, to, exp.Text)
	a.beginSnippetSession(buf, exp, from, true)
	return true
}

// beginSnippetSession opens the session for exp, which was inserted at offset
// base of the buffer. The cursor goes to the first stop, or to the end of the
// text when the body has none and moveCursor is set. A session is kept only
// when there is something to step through.
func (a *App) beginSnippetSession(buf *noteBuffer, exp snippets.Expansion, base int, moveCursor bool) {
	sess := snippets.NewSession(exp, base, buf.ed.Text())
	if moveCursor {
		buf.ed.SetCursor(buf.ed.PosOf(sess.Cursor()))
	}
	if sess.Active() {
		buf.snip = sess
	} else {
		buf.snip = nil
	}
}

// syncSnippets keeps the open session in step with the text after each key.
// Mirrors of an edited stop are updated here. A session ends when its note is
// no longer the active one.
func (a *App) syncSnippets() {
	cur := a.activeBuffer()
	for _, b := range a.buffers {
		if b != cur {
			b.snip = nil
			b.snippetPreview = nil
		}
	}
	if cur == nil || cur.snip == nil {
		return
	}
	applySnippetReps(cur, cur.snip.Sync(cur.ed.Text()))
}

// applySnippetReps writes the mirror updates from a session sync into the
// buffer and keeps the cursor on the same text.
func applySnippetReps(buf *noteBuffer, reps []snippets.Replacement) {
	if len(reps) == 0 {
		return
	}
	at := buf.ed.OffsetOf(buf.ed.Cursor())
	for _, r := range reps {
		buf.ed.ReplaceSpan(r.Start, r.End, r.Text)
	}
	buf.ed.SetCursor(buf.ed.PosOf(snippets.MapOffset(at, reps)))
}

// updateSnippetPreview refreshes the snippet preview popup based on the word
// at the cursor. It shows matching snippet triggers as the user types.
func (a *App) updateSnippetPreview(buf *noteBuffer) {
	if buf.ed.Mode() != vim.ModeInsert || buf.snip.Active() {
		buf.snippetPreview = nil
		return
	}

	cur := buf.ed.Cursor()
	line := buf.ed.LineRunes(cur.Line)
	end := min(cur.Col, len(line))
	start := end

	// Collect the word being typed
	for start > 0 && snippets.IsWordRune(line[start-1]) {
		start--
	}

	if start == end {
		buf.snippetPreview = nil
		return
	}

	trigger := string(line[start:end])
	prefix := strings.ToLower(trigger)
	var matches []snippetMatch
	for _, t := range a.snippetLib.Triggers() {
		if strings.HasPrefix(strings.ToLower(t), prefix) && len(prefix) >= 1 {
			sn, ok := a.snippetLib.Lookup(t)
			if ok {
				matches = append(matches, snippetMatch{trigger: t, snippet: sn})
			}
		}
	}

	if len(matches) == 0 {
		buf.snippetPreview = nil
		return
	}

	buf.snippetPreview = &snippetPreview{matches: matches, sel: 0}
}

// templateBodyAfterTitle drops the `# Title` line a template body starts with
// and any trailing newlines, so the body can be inserted into a note that
// already has its title.
func templateBodyAfterTitle(exp snippets.Expansion, title string) snippets.Expansion {
	from := 0
	if prefix := "# " + title + "\n"; strings.HasPrefix(exp.Text, prefix) {
		from = utf8.RuneCountInString(prefix)
	}
	to := utf8.RuneCountInString(strings.TrimRight(exp.Text, "\n"))
	return exp.Slice(from, max(from, to))
}

// paintSnippetPreview renders the snippet preview popup at the cursor position.
func (a *App) paintSnippetPreview(screen string) string {
	buf := a.activeBuffer()
	if buf == nil || buf.snippetPreview == nil || !buf.snippetPreview.active() {
		return screen
	}

	cur := buf.ed.Cursor()
	line := buf.ed.LineRunes(cur.Line)
	end := min(cur.Col, len(line))
	start := end
	for start > 0 && snippets.IsWordRune(line[start-1]) {
		start--
	}

	// Find the screen position of the trigger word
	beforeCursor := buf.ed.OffsetOf(vim.Pos{Line: cur.Line, Col: start})
	curPos := buf.ed.OffsetOf(cur)

	// Get the rendered pane to calculate screen position
	// For now, use a simple heuristic: the popup appears below the cursor
	x := start % 80 // approximate column
	y := cur.Line % 24 // approximate row

	block := buf.snippetPreview.render(x, y, a.width, a.theme)
	h := strings.Count(block, "\n") + 1

	// Adjust position to be below the current line
	top := y + 1
	if top+h > a.height-2 {
		top = max(0, y-h)
	}

	screen, _ = overlayPlaceAt(screen, block, x, top, a.width, a.height)
	return screen
}
