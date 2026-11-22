package tui

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/ZenNotes/tui/internal/backend"
	"github.com/ZenNotes/tui/internal/vault"
	"github.com/ZenNotes/tui/internal/vim"
)

// autolinkOnServer reports whether the vault is on a server. Autolink reads
// and writes notes only in a local vault.
func (a *App) autolinkOnServer() bool {
	if a.backend != nil && a.backend.Kind() == backend.KindRemote {
		return true
	}
	return a.opts.Target.Kind == backend.KindRemote
}

// autolinkCommand runs :autolink for the active note, or :autolink --all for
// each note that autolink may change.
func (a *App) autolinkCommand(buf *noteBuffer, args string) error {
	if a.autolinkOnServer() {
		return backend.ErrAutolinkRemote
	}
	switch strings.TrimSpace(args) {
	case "":
	case "--all":
		return a.autolinkAll()
	default:
		return errors.New("usage: :autolink or :autolink --all")
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
	body := buf.ed.Text()
	a.autolinkResolve(buf.path, body, a.idx.notes, opts, func(changes []vault.AutolinkChange) {
		if len(changes) == 0 {
			a.notify("No change. No reference needs a link.")
			return
		}
		plans := []backend.AutolinkNote{{Path: buf.path, Body: body, Changes: changes}}
		a.autolinkReview("Autolink: "+buf.path, plans, false, func(a *App, keep func(p, c int) bool) {
			a.autolinkApply(buf, plans[0], func(c int) bool { return keep(0, c) })
		})
	})
	return nil
}

// autolinkAll plans each note that autolink may change. It stops when a note
// has unsaved edits, because the plan reads the notes from the vault.
func (a *App) autolinkAll() error {
	var dirty []string
	for path, buf := range a.buffers {
		if buf.dirty() {
			dirty = append(dirty, path)
		}
	}
	if len(dirty) > 0 {
		sort.Strings(dirty)
		return errors.New("save these notes first: " + strings.Join(dirty, ", "))
	}
	run, err := backend.PlanAutolinkRun(context.Background(), a.backend, nil, a.prefs.AutolinkRemoveDuplicates)
	if err != nil {
		return err
	}
	a.autolinkNext(run, 0, nil)
	return nil
}

// autolinkNext asks for the conflicts of plan i, then goes on to the next note.
// After the last note it opens the review of all the changes.
func (a *App) autolinkNext(run backend.AutolinkRun, i int, done []backend.AutolinkNote) {
	if i >= len(run.Plans) {
		if len(done) == 0 {
			a.notify("No change. No reference needs a link.")
			return
		}
		a.autolinkReview("Autolink: all notes", done, true, func(a *App, keep func(p, c int) bool) {
			a.autolinkApplyAll(done, keep)
		})
		return
	}
	plan := run.Plans[i]
	opts := run.Options
	opts.Resolved = map[int]string{}
	a.autolinkResolve(plan.Path, plan.Body, run.Notes, opts, func(changes []vault.AutolinkChange) {
		if len(changes) > 0 {
			plan.Changes = changes
			done = append(done, plan)
		}
		a.autolinkNext(run, i+1, done)
	})
}

// autolinkResolve plans the body again with the answers so far. It asks for the
// first conflict that has no answer, and calls done when none is left.
func (a *App) autolinkResolve(path, body string, notes []vault.NoteMeta, opts vault.AutolinkOptions, done func([]vault.AutolinkChange)) {
	changes := vault.PlanAutolinks(body, path, notes, opts)
	for _, c := range changes {
		if c.Kind != vault.AutolinkConflict {
			continue
		}
		answer := func(choice string) func(a *App) {
			return func(a *App) {
				next := opts
				next.Resolved = map[int]string{}
				for k, v := range opts.Resolved {
					next.Resolved[k] = v
				}
				next.Resolved[c.Offset] = choice
				a.autolinkResolve(path, body, notes, next, done)
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
		a.showMenu(fmt.Sprintf("Conflict in %s at line %d: %s", path, c.Line, truncateCells(c.Context, 60)), items)
		return
	}
	done(changes)
}

// autolinkDetail is the detail of a review row: the line of context. A move
// also shows its old link first, so a narrow list does not cut it.
func autolinkDetail(body string, c vault.AutolinkChange) string {
	if c.Kind != vault.AutolinkMove || len(c.Edits) < 2 {
		return c.Context
	}
	old := c.Edits[1]
	line := 1 + strings.Count(body[:old.Start], "\n")
	return fmt.Sprintf("Old link: line %d  %s → %s  |  %s", line, body[old.Start:old.End], old.Text, c.Context)
}

// autolinkRows lists the changes of the plans as review rows. Each id holds the
// index of the plan and of the change.
func autolinkRows(plans []backend.AutolinkNote, withPath bool) ([]pickItem, []string) {
	var items []pickItem
	var ids []string
	for i, plan := range plans {
		for j, c := range plan.Changes {
			if len(c.Edits) == 0 {
				continue
			}
			e := c.Edits[0]
			id := strconv.Itoa(i) + ":" + strconv.Itoa(j)
			where := fmt.Sprintf("line %d", c.Line)
			if withPath {
				where = fmt.Sprintf("%s:%d", plan.Path, c.Line)
			}
			items = append(items, pickItem{
				id:     id,
				label:  fmt.Sprintf("%s  %s  %s → %s", c.Kind, where, plan.Body[e.Start:e.End], e.Text),
				detail: autolinkDetail(plan.Body, c),
			})
			ids = append(ids, id)
		}
	}
	return items, ids
}

// autolinkReview shows the changes as a list with all of them checked. Tab
// omits or keeps a change, Enter calls apply with the kept changes and Esc
// cancels.
func (a *App) autolinkReview(title string, plans []backend.AutolinkNote, withPath bool, apply func(a *App, keep func(p, c int) bool)) {
	items, ids := autolinkRows(plans, withPath)
	p := newPickOverlay(title, "Filter the changes", items, ids)
	p.onDone = func(a *App, kept []string) {
		keep := map[string]bool{}
		for _, id := range kept {
			keep[id] = true
		}
		if len(keep) == 0 {
			a.notify("All changes omitted. The note did not change.")
			return
		}
		apply(a, func(pi, ci int) bool { return keep[strconv.Itoa(pi)+":"+strconv.Itoa(ci)] })
	}
	p.refilter(a)
	a.overlay = p
}

// autolinkApply puts the kept changes into the buffer as one undo step. It
// does not save the note.
func (a *App) autolinkApply(buf *noteBuffer, plan backend.AutolinkNote, keep func(int) bool) {
	if buf.ed.Text() != plan.Body {
		a.notifyError("The note changed during the review. Run autolink again.")
		return
	}
	next := vault.ApplyAutolinks(plan.Body, plan.Changes, keep)
	if next == plan.Body {
		return
	}
	count := 0
	for i := range plan.Changes {
		if keep(i) {
			count++
		}
	}
	buf.ed.ReplaceText(next)
	a.invalidatePreviews()
	a.notify("Autolink changed " + strconv.Itoa(count) + " place(s). Save the note to keep them.")
}

// autolinkApplyAll writes the kept changes. An open note takes them as one undo
// step and is saved. Each other note is written through the backend.
func (a *App) autolinkApplyAll(plans []backend.AutolinkNote, keep func(p, c int) bool) {
	var failed []string
	notes, places := 0, 0
	for i, plan := range plans {
		k := func(c int) bool { return keep(i, c) }
		count := 0
		for c := range plan.Changes {
			if k(c) {
				count++
			}
		}
		if count == 0 {
			continue
		}
		if buf, ok := a.buffers[plan.Path]; ok && !buf.loading {
			if buf.ed.Text() != vim.NormalizedText(plan.Body) {
				failed = append(failed, plan.Path+" changed during the review")
				continue
			}
			next := vault.ApplyAutolinks(plan.Body, plan.Changes, k)
			if next == plan.Body {
				continue
			}
			buf.ed.ReplaceText(next)
			a.invalidatePreviews()
			if err := a.writeBuffer(buf); err != nil {
				failed = append(failed, plan.Path+": "+err.Error())
				continue
			}
		} else {
			written, err := backend.WriteAutolinks(context.Background(), a.backend, []backend.AutolinkNote{plan}, func(_, c int) bool { return k(c) })
			if err != nil {
				failed = append(failed, err.Error())
				continue
			}
			if len(written) == 0 {
				continue
			}
		}
		notes++
		places += count
	}
	if len(failed) > 0 {
		a.notifyError("Autolink did not write: " + strings.Join(failed, "; "))
		return
	}
	a.notify(fmt.Sprintf("Autolink changed %d place(s) in %d note(s).", places, notes))
}
