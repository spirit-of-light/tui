package vim

import "github.com/ZenNotes/tui/internal/snippet"

// WordBeforeCursor gives the keyword runes directly before the cursor. It is
// empty after a space, at the start of a line and on an empty line.
func (e *Editor) WordBeforeCursor() string {
	line := e.buf.Line(e.cursor.Line)
	end := min(e.cursor.Col, len(line))
	start := end
	for start > 0 && isWordRune(line[start-1]) {
		start--
	}
	return string(line[start:end])
}

// BufferWords gives the distinct words of the buffer in order from the top,
// each with its exact text.
func (e *Editor) BufferWords() []string {
	var out []string
	seen := map[string]bool{}
	for l := 0; l < e.buf.LineCount(); l++ {
		text := e.buf.Line(l)
		for i := 0; i < len(text); {
			if !isWordRune(text[i]) {
				i++
				continue
			}
			j := i
			for j < len(text) && isWordRune(text[j]) {
				j++
			}
			if w := string(text[i:j]); !seen[w] {
				seen[w] = true
				out = append(out, w)
			}
			i = j
		}
	}
	return out
}

// KeywordCompletionActive reports whether a keyword completion cycle (Ctrl-N
// or Ctrl-P in insert mode) is running.
func (e *Editor) KeywordCompletionActive() bool {
	return e.opts.VimEnabled && e.ins.completion != nil
}

// openTriggerChange prepares the undo change for a trigger or a popup. With
// Vim on, the text joins the insert in progress. Without Vim, the typing that
// is open commits first, so one undo gives back the typed prefix, and the
// edits of the trigger then form one change.
func (e *Editor) openTriggerChange() {
	if e.opts.VimEnabled {
		return
	}
	if !e.triggerOpen || e.active == nil {
		if len(e.sessions) == 0 {
			e.commitChange()
		}
		e.triggerOpen = true
	}
	e.beginChange()
}

// deleteRunesBefore deletes up to n runes before the cursor on its line.
func (e *Editor) deleteRunesBefore(n int) {
	n = min(n, e.cursor.Col)
	if n <= 0 {
		return
	}
	e.tracked(func() { e.deleteBefore(n) })
}

// ReplaceBeforeCursor replaces the n runes before the cursor with text. The
// popup uses it to show the selected candidate. Without Vim, all the replaces
// of one popup are one undo change.
func (e *Editor) ReplaceBeforeCursor(n int, text string) {
	e.openTriggerChange()
	e.deleteRunesBefore(n)
	e.tracked(func() { e.insertRaw(text) })
	e.desiredCol = -1
	e.ensureCursorVisible()
}

// ExpandSnippetOver deletes the n runes before the cursor and expands body
// there, as ExpandSnippet does. The deletion and the expansion are one undo
// change. Without Vim, the change commits when no session starts.
func (e *Editor) ExpandSnippetOver(n int, body string, vars snippet.Resolver) {
	e.openTriggerChange()
	e.deleteRunesBefore(n)
	e.ExpandSnippet(body, vars)
	e.EndTriggerChange()
	e.ensureCursorVisible()
}

// EndTriggerChange commits the change that a trigger or a popup keeps open
// without Vim. It does nothing while a snippet session runs, because the
// change then stays open until the last session ends.
func (e *Editor) EndTriggerChange() {
	if !e.opts.VimEnabled && e.triggerOpen {
		e.commitUnlessSnippet()
	}
}
