package tui

import (
	"time"

	"github.com/ZenNotes/tui/internal/templates"
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
