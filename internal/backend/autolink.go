package backend

import (
	"context"
	"errors"
	"fmt"

	"github.com/ZenNotes/tui/internal/vault"
)

// ErrAutolinkRemote is the error for a vault on a server. Autolink reads and
// writes notes only in a local vault.
var ErrAutolinkRemote = errors.New("autolink works only in a local vault, not on a server")

// AutolinkNote is one note with the changes that autolink plans for it.
type AutolinkNote struct {
	Path string
	// Body is the text that the changes were planned against.
	Body    string
	Changes []vault.AutolinkChange
}

// AutolinkRun is the result of a plan: the notes that have changes, and the
// note list and options to plan a body again after the answer to a conflict.
type AutolinkRun struct {
	Notes   []vault.NoteMeta
	Options vault.AutolinkOptions
	Plans   []AutolinkNote
}

// PlanAutolinkRun plans the links for the notes at paths, or for each note that
// autolink may change when paths is empty. It returns ErrAutolinkRemote for a
// server vault, before any call to the server. A note with no change has no
// plan in the result.
func PlanAutolinkRun(ctx context.Context, b Backend, paths []string, removeDuplicates bool) (AutolinkRun, error) {
	if b.Kind() == KindRemote {
		return AutolinkRun{}, ErrAutolinkRemote
	}
	notes, err := b.ListNotes(ctx)
	if err != nil {
		return AutolinkRun{}, err
	}
	settings, err := b.VaultSettings(ctx)
	if err != nil {
		return AutolinkRun{}, err
	}
	opts := vault.AutolinkOptions{
		TemplateFolder:   settings.TemplateFolder,
		PreambleFolder:   settings.TypstPreambleFolder,
		RemoveDuplicates: removeDuplicates,
	}
	run := AutolinkRun{Notes: notes, Options: opts}

	var sources []vault.NoteMeta
	if len(paths) == 0 {
		for _, n := range notes {
			if vault.AutolinkSource(n, opts) {
				sources = append(sources, n)
			}
		}
	} else {
		for _, p := range paths {
			found := false
			for _, n := range notes {
				if n.Path != p {
					continue
				}
				if !vault.AutolinkSource(n, opts) {
					return AutolinkRun{}, fmt.Errorf("%s is not a note that autolink changes", p)
				}
				sources = append(sources, n)
				found = true
				break
			}
			if !found {
				return AutolinkRun{}, fmt.Errorf("no note at %s", p)
			}
		}
	}

	for _, n := range sources {
		content, err := b.ReadNote(ctx, n.Path)
		if err != nil {
			return AutolinkRun{}, err
		}
		changes := vault.PlanAutolinks(content.Body, n.Path, notes, opts)
		if len(changes) == 0 {
			continue
		}
		run.Plans = append(run.Plans, AutolinkNote{Path: n.Path, Body: content.Body, Changes: changes})
	}
	return run, nil
}

// WriteAutolinks applies the changes that keep accepts and writes each note
// whose body changes. keep gets the index of the plan and of the change in it;
// a nil keep accepts every change. It returns the paths that it wrote.
func WriteAutolinks(ctx context.Context, b Backend, plans []AutolinkNote, keep func(plan, change int) bool) ([]string, error) {
	if b.Kind() == KindRemote {
		return nil, ErrAutolinkRemote
	}
	var written []string
	for i, p := range plans {
		var k func(int) bool
		if keep != nil {
			i := i
			k = func(c int) bool { return keep(i, c) }
		}
		next := vault.ApplyAutolinks(p.Body, p.Changes, k)
		if next == p.Body {
			continue
		}
		if _, err := b.WriteNote(ctx, p.Path, next); err != nil {
			return written, fmt.Errorf("%s: %w", p.Path, err)
		}
		written = append(written, p.Path)
	}
	return written, nil
}
