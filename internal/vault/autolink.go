package vault

import (
	"sort"
	"strings"
)

// AutolinkKind is the type of one planned change.
type AutolinkKind string

const (
	// AutolinkAdd turns the first plain reference to a note into a link.
	AutolinkAdd AutolinkKind = "add"
	// AutolinkConflict marks a reference that more than one note can claim. It
	// has no edits: the review chooses a note, or none, with Resolved.
	AutolinkConflict AutolinkKind = "conflict"
)

// AutolinkEdit replaces body[Start:End] with Text.
type AutolinkEdit struct {
	Start, End int
	Text       string
}

// AutolinkChange is one change that a review can keep or omit.
type AutolinkChange struct {
	Kind AutolinkKind
	// Edits are the byte edits of the change, against the body that was planned.
	Edits []AutolinkEdit
	// Target is the path of the note that the change links to. A conflict has
	// no target.
	Target string
	// Offset is the byte offset of the change in the body that was planned. For a
	// conflict it is the key of AutolinkOptions.Resolved.
	Offset int
	// Line is the 1-based line of the change, and Context is that line trimmed.
	Line    int
	Context string
	// Choices are the paths of the notes that a conflict can link to, in note
	// order. Choosing no note is always possible.
	Choices []string
}

// AutolinkOptions holds the vault settings that decide which notes take part.
type AutolinkOptions struct {
	// TemplateFolder and PreambleFolder name the folders whose notes neither
	// get links nor receive them. An empty name excludes nothing.
	TemplateFolder string
	PreambleFolder string
	// Resolved holds the answer to each conflict: the byte offset of the
	// conflict maps to the path of the chosen note, or to "" for no note. A
	// conflict without an answer stays in the plan as a conflict change.
	Resolved map[int]string
}

// inAutolinkScope reports whether a note takes part in autolink at all: a note
// in the trash, the archive, the template folder or the preamble folder does
// not.
func inAutolinkScope(n NoteMeta, opts AutolinkOptions) bool {
	if n.Folder == FolderTrash || n.Folder == FolderArchive {
		return false
	}
	if IsPathInTemplateFolder(n.Path, opts.TemplateFolder) {
		return false
	}
	return !IsTypstPreamblePath(n.Path, opts.PreambleFolder)
}

// AutolinkSource reports whether autolink may change the body of the note.
func AutolinkSource(n NoteMeta, opts AutolinkOptions) bool {
	return inAutolinkScope(n, opts)
}

// AutolinkTarget reports whether a link to the note may be added to the note at
// self.
func AutolinkTarget(n NoteMeta, self string, opts AutolinkOptions) bool {
	return n.Path != self && inAutolinkScope(n, opts)
}

type autolinkMatch struct {
	start, end int
	target     int // index into the target list
}

// PlanAutolinks lists the changes that link the first reference to each other
// live note in body. self is the path of the note that holds body.
//
// A reference that overlaps the reference to another note, or that two notes
// share by title or alias, is a conflict. A conflict is a change of its own
// until opts.Resolved answers it. The answer can change the later changes, so a
// caller plans again after each answer.
func PlanAutolinks(body, self string, notes []NoteMeta, opts AutolinkOptions) []AutolinkChange {
	var targets []NoteMeta
	for _, n := range notes {
		if AutolinkTarget(n, self, opts) {
			targets = append(targets, n)
		}
	}
	if len(targets) == 0 || body == "" {
		return nil
	}
	mask := autolinkMask(body)
	linked := linkedTargets(body, self, notes)

	var matches []autolinkMatch
	for ti, n := range targets {
		if linked[n.Path] {
			continue
		}
		matches = append(matches, targetMatches(body, mask, n, ti)...)
	}
	if len(matches) == 0 {
		return nil
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].start != matches[j].start {
			return matches[i].start < matches[j].start
		}
		return matches[i].end > matches[j].end
	})

	lineStarts := []int{0}
	for i := 0; i < len(body); i++ {
		if body[i] == '\n' {
			lineStarts = append(lineStarts, i+1)
		}
	}
	describe := func(c AutolinkChange) AutolinkChange {
		line := sort.Search(len(lineStarts), func(i int) bool { return lineStarts[i] > c.Offset })
		from := lineStarts[line-1]
		to := len(body)
		if line < len(lineStarts) {
			to = lineStarts[line] - 1
		}
		c.Line = line
		c.Context = strings.TrimSpace(body[from:to])
		return c
	}

	// settled holds each target that has a link, or that waits for the answer
	// to a conflict. A later reference to it does not add a second link.
	settled := map[int]bool{}
	var changes []AutolinkChange
	for lo := 0; lo < len(matches); {
		hi, reach := lo+1, matches[lo].end
		for hi < len(matches) && matches[hi].start < reach {
			if matches[hi].end > reach {
				reach = matches[hi].end
			}
			hi++
		}
		group := matches[lo:hi]
		lo = hi

		var live []autolinkMatch
		var order []int
		seen := map[int]bool{}
		for _, m := range group {
			if settled[m.target] {
				continue
			}
			live = append(live, m)
			if !seen[m.target] {
				seen[m.target] = true
				order = append(order, m.target)
			}
		}
		if len(live) == 0 {
			continue
		}

		pick := -1
		if len(order) == 1 {
			pick = order[0]
		} else {
			offset := live[0].start
			answer, answered := opts.Resolved[offset]
			if answered && answer != "" && !seen[targetIndex(targets, answer)] {
				answered = false
			}
			if !answered {
				sort.Ints(order)
				choices := make([]string, len(order))
				for i, ti := range order {
					choices[i] = targets[ti].Path
					settled[ti] = true
				}
				changes = append(changes, describe(AutolinkChange{Kind: AutolinkConflict, Offset: offset, Choices: choices}))
				continue
			}
			if answer == "" {
				continue
			}
			pick = targetIndex(targets, answer)
		}

		for _, m := range live {
			if m.target != pick {
				continue
			}
			settled[pick] = true
			n := targets[pick]
			link, ok := autolinkText(notes, n, body[m.start:m.end])
			if !ok {
				break
			}
			changes = append(changes, describe(AutolinkChange{
				Kind:   AutolinkAdd,
				Edits:  []AutolinkEdit{{Start: m.start, End: m.end, Text: link}},
				Target: n.Path,
				Offset: m.start,
			}))
			break
		}
	}
	sort.SliceStable(changes, func(i, j int) bool { return changes[i].Offset < changes[j].Offset })
	return changes
}

func targetIndex(targets []NoteMeta, path string) int {
	for i, n := range targets {
		if n.Path == path {
			return i
		}
	}
	return -1
}

// targetMatches lists the matches of the title and the aliases of n outside
// the mask. Where a title and an alias of n overlap, the earlier and longer
// match stays.
func targetMatches(body string, mask []bool, n NoteMeta, ti int) []autolinkMatch {
	var found []autolinkMatch
	seen := map[string]bool{}
	for _, phrase := range append([]string{n.Title}, n.Aliases...) {
		key := strings.ToLower(strings.TrimSpace(phrase))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		for _, sp := range autolinkMatches(body, phrase) {
			if !spanMasked(mask, sp[0], sp[1]) {
				found = append(found, autolinkMatch{start: sp[0], end: sp[1], target: ti})
			}
		}
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i].start != found[j].start {
			return found[i].start < found[j].start
		}
		return found[i].end > found[j].end
	})
	out := found[:0]
	for _, m := range found {
		if len(out) > 0 && m.start < out[len(out)-1].end {
			continue
		}
		out = append(out, m)
	}
	return out
}

func spanMasked(mask []bool, start, end int) bool {
	for i := start; i < end; i++ {
		if mask[i] {
			return true
		}
	}
	return false
}

// linkedTargets lists the notes that body already links to with a wikilink that
// has no anchor, or with a Markdown link. An embed does not count.
func linkedTargets(body, self string, notes []NoteMeta) map[string]bool {
	linked := map[string]bool{}
	for _, l := range autolinkLinks(body) {
		if l.Embed || l.Anchor != "" {
			continue
		}
		var n NoteMeta
		var ok bool
		if l.Wiki {
			n, ok = ResolveWikilink(notes, l.Target)
		} else {
			n, ok = ResolveNoteHref(notes, self, l.Target)
		}
		if ok {
			linked[n.Path] = true
		}
	}
	return linked
}

// autolinkText builds the wikilink for text that refers to n. The link names
// the note by title when that resolves to n and no other note has the same
// title, else by path. It reports false when no form of the link resolves to n.
func autolinkText(notes []NoteMeta, n NoteMeta, text string) (string, bool) {
	if strings.ContainsAny(text, "|[]\n") {
		return "", false
	}
	byTitle, byPath := n.Title, strings.TrimSuffix(n.Path, ".md")
	forms := []string{byTitle, byPath}
	if sharesTitle(notes, n) {
		forms = []string{byPath, byTitle}
	}
	for _, target := range forms {
		if strings.ContainsAny(target, "|[]\n") {
			continue
		}
		if got, ok := ResolveWikilink(notes, target); !ok || got.Path != n.Path {
			continue
		}
		if text == target {
			return "[[" + target + "]]", true
		}
		return "[[" + target + "|" + text + "]]", true
	}
	return "", false
}

// sharesTitle reports whether another note outside the trash has the title of
// n.
func sharesTitle(notes []NoteMeta, n NoteMeta) bool {
	for _, o := range notes {
		if o.Path != n.Path && o.Folder != FolderTrash && strings.EqualFold(strings.TrimSpace(o.Title), strings.TrimSpace(n.Title)) {
			return true
		}
	}
	return false
}

// ApplyAutolinks applies the edits of the changes that keep accepts, and
// returns the new body. A nil keep accepts every change. The edits apply from
// the end of the body, so the offsets of the earlier ones stay valid.
func ApplyAutolinks(body string, changes []AutolinkChange, keep func(i int) bool) string {
	var edits []AutolinkEdit
	for i, c := range changes {
		if keep == nil || keep(i) {
			edits = append(edits, c.Edits...)
		}
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].Start > edits[j].Start })
	for _, e := range edits {
		body = body[:e.Start] + e.Text + body[e.End:]
	}
	return body
}
