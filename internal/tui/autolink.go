package tui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ZenNotes/tui/internal/backend"
	"github.com/ZenNotes/tui/internal/vault"
)

// autolinkOnServer reports whether the vault is on a server. Autolink reads
// and writes notes only in a local vault.
func (a *App) autolinkOnServer() bool {
	if a.backend != nil && a.backend.Kind() == backend.KindRemote {
		return true
	}
	return a.opts.Target.Kind == backend.KindRemote
}

// autolinkCommand runs :autolink for the active note.
func (a *App) autolinkCommand(buf *noteBuffer, args string) error {
	if a.autolinkOnServer() {
		return backend.ErrAutolinkRemote
	}
	if strings.TrimSpace(args) != "" {
		return errors.New("usage: :autolink")
	}
	if buf == nil {
		buf = a.activeBuffer()
	}
	if buf == nil {
		return errors.New("autolink needs an open note")
	}
	if buf.loading || buf.seeded {
		return errors.New("wait for the note to load")
	}
	if a.idx == nil {
		return errors.New("the note list is not loaded yet")
	}
	opts := vault.AutolinkOptions{
		TemplateFolder:   a.idx.settings.TemplateFolder,
		PreambleFolder:   a.idx.settings.TypstPreambleFolder,
		RemoveDuplicates: a.prefs.AutolinkRemoveDuplicates,
		Resolved:         map[int]string{},
	}
	self, ok := a.idx.byPath[buf.path]
	if !ok {
		self = buf.meta
		self.Path = buf.path
	}
	if !vault.AutolinkSource(self, opts) {
		return errors.New("autolink does not change this note")
	}
	a.autolinkPlan(buf, buf.ed.Text(), a.idx.notes, opts)
	return nil
}

// autolinkPlan plans the body again with the answers so far. It asks for the
// first conflict that has no answer, and opens the review when none is left.
func (a *App) autolinkPlan(buf *noteBuffer, body string, notes []vault.NoteMeta, opts vault.AutolinkOptions) {
	changes := vault.PlanAutolinks(body, buf.path, notes, opts)
	for _, c := range changes {
		if c.Kind != vault.AutolinkConflict {
			continue
		}
		answer := func(path string) func(a *App) {
			return func(a *App) {
				next := opts
				next.Resolved = map[int]string{}
				for k, v := range opts.Resolved {
					next.Resolved[k] = v
				}
				next.Resolved[c.Offset] = path
				a.autolinkPlan(buf, body, notes, next)
			}
		}
		items := []menuItem{{key: "1", label: "Skip: do not link this text", run: answer("")}}
		for i, choice := range c.Choices {
			key := ""
			if i+2 <= 9 {
				key = strconv.Itoa(i + 2)
			}
			items = append(items, menuItem{key: key, label: "Link to " + choice, run: answer(choice)})
		}
		a.showMenu(fmt.Sprintf("Conflict at line %d: %s", c.Line, truncateCells(c.Context, 60)), items)
		return
	}
	if len(changes) == 0 {
		a.notify("No change. No reference needs a link.")
		return
	}
	a.autolinkReview(buf, body, changes)
}

// autolinkReview shows the changes as a list with all of them checked. Tab
// omits or keeps a change, Enter applies the kept changes and Esc cancels.
func (a *App) autolinkReview(buf *noteBuffer, body string, changes []vault.AutolinkChange) {
	items := make([]pickItem, 0, len(changes))
	ids := make([]string, 0, len(changes))
	for i, c := range changes {
		if len(c.Edits) == 0 {
			continue
		}
		e := c.Edits[0]
		id := strconv.Itoa(i)
		items = append(items, pickItem{
			id:     id,
			label:  fmt.Sprintf("%s  line %d  %s → %s", c.Kind, c.Line, body[e.Start:e.End], e.Text),
			detail: c.Context,
		})
		ids = append(ids, id)
	}
	p := newPickOverlay("Autolink: "+buf.path, "Filter the changes", items, ids)
	p.onDone = func(a *App, kept []string) {
		keep := map[int]bool{}
		for _, id := range kept {
			if n, err := strconv.Atoi(id); err == nil {
				keep[n] = true
			}
		}
		a.autolinkApply(buf, body, changes, keep)
	}
	p.refilter(a)
	a.overlay = p
}

// autolinkApply puts the kept changes into the buffer as one undo step. It
// does not save the note.
func (a *App) autolinkApply(buf *noteBuffer, body string, changes []vault.AutolinkChange, keep map[int]bool) {
	if len(keep) == 0 {
		a.notify("All changes omitted. The note did not change.")
		return
	}
	if buf.ed.Text() != body {
		a.notifyError("The note changed during the review. Run autolink again.")
		return
	}
	next := vault.ApplyAutolinks(body, changes, func(i int) bool { return keep[i] })
	if next == body {
		return
	}
	buf.ed.ReplaceText(next)
	a.invalidatePreviews()
	a.notify("Autolink changed " + strconv.Itoa(len(keep)) + " place(s). Save the note to keep them.")
}
