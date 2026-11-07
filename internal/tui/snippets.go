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
	if buf.snip.Active() {
		if span, ok := buf.snip.Replaces(); ok && k.Printable() && buf.ed.OffsetOf(buf.ed.Cursor()) == span.Start {
			// Typing over untouched default text replaces it. The deletion is synced
			// now, so mirrors of the stop are cleared with it; the key then types
			// into the empty stop.
			buf.ed.ReplaceSpan(span.Start, span.End, "")
			applySnippetReps(buf, buf.snip.Sync(buf.ed.Text()))
			return false
		}
		switch {
		case k.IsCtrl('l'):
			if off, ok := buf.snip.Next(); ok {
				buf.ed.SetCursor(buf.ed.PosOf(off))
				return true
			}
			// Past the final stop the session is over and Ctrl+L does its usual job.
			buf.snip = nil
			return false
		case k.IsCtrl('h'):
			if off, ok := buf.snip.Prev(); ok {
				buf.ed.SetCursor(buf.ed.PosOf(off))
				return true
			}
			return false
		}
	}
	if k.IsCtrl('n') {
		return a.expandSnippetAtCursor(buf)
	}
	return false
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
