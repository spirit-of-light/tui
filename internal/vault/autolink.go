package vault

import (
	"sort"
	"strings"
)

// AutolinkKind is the type of one planned change.
type AutolinkKind string

// AutolinkAdd turns the first plain reference to a note into a link.
const AutolinkAdd AutolinkKind = "add"

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
	// Target is the path of the note that the change links to.
	Target string
	// Line is the 1-based line of the change, and Context is that line trimmed.
	Line    int
	Context string
}

// AutolinkOptions holds the vault settings that decide which notes take part.
type AutolinkOptions struct {
	// TemplateFolder and PreambleFolder name the folders whose notes neither
	// get links nor receive them. An empty name excludes nothing.
	TemplateFolder string
	PreambleFolder string
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
	conflict   bool
}

// PlanAutolinks lists the changes that link the first reference to each other
// live note in body. self is the path of the note that holds body.
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

	var matches []autolinkMatch
	for ti, n := range targets {
		matches = append(matches, targetMatches(body, mask, n, ti)...)
	}
	if len(matches) == 0 {
		return nil
	}
	markAutolinkConflicts(matches)

	linked := linkedTargets(body, self, notes)
	chosen := make(map[int]autolinkMatch)
	for _, m := range matches {
		if m.conflict || linked[targets[m.target].Path] {
			continue
		}
		if cur, ok := chosen[m.target]; !ok || m.start < cur.start {
			chosen[m.target] = m
		}
	}

	lineStarts := []int{0}
	for i := 0; i < len(body); i++ {
		if body[i] == '\n' {
			lineStarts = append(lineStarts, i+1)
		}
	}
	var changes []AutolinkChange
	for ti, m := range chosen {
		n := targets[ti]
		link, ok := autolinkText(notes, n, body[m.start:m.end])
		if !ok {
			continue
		}
		line := sort.Search(len(lineStarts), func(i int) bool { return lineStarts[i] > m.start })
		from := lineStarts[line-1]
		to := len(body)
		if line < len(lineStarts) {
			to = lineStarts[line] - 1
		}
		changes = append(changes, AutolinkChange{
			Kind:    AutolinkAdd,
			Edits:   []AutolinkEdit{{Start: m.start, End: m.end, Text: link}},
			Target:  n.Path,
			Line:    line,
			Context: strings.TrimSpace(body[from:to]),
		})
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Edits[0].Start < changes[j].Edits[0].Start })
	return changes
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

// markAutolinkConflicts flags each match that overlaps a match of a different
// note: the plan does not choose between them.
func markAutolinkConflicts(matches []autolinkMatch) {
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].start != matches[j].start {
			return matches[i].start < matches[j].start
		}
		return matches[i].end > matches[j].end
	})
	for i := range matches {
		for j := i + 1; j < len(matches) && matches[j].start < matches[i].end; j++ {
			if matches[j].target != matches[i].target {
				matches[i].conflict = true
				matches[j].conflict = true
			}
		}
	}
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
// the note by title when that resolves to n, else by path. It reports false
// when no form of the link resolves to n.
func autolinkText(notes []NoteMeta, n NoteMeta, text string) (string, bool) {
	if strings.ContainsAny(text, "|[]\n") {
		return "", false
	}
	for _, target := range []string{n.Title, strings.TrimSuffix(n.Path, ".md")} {
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
