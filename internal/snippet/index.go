package snippet

import (
	"path"
	"sort"
	"strings"
	"unicode"
)

// GroupFile is the input for one group file. The index does no I/O: the caller
// reads the file and passes its text or the read error.
type GroupFile struct {
	// Name is the file name, for example "My Notes.md".
	Name string
	// Path is where the file is, as zn shows it in messages.
	Path string
	// Raw is the text of the file.
	Raw string
	// Err is the error from the read. When it is set, Raw is not used.
	Err error
}

// Snippet is a usable snippet in the index.
type Snippet struct {
	Trigger     string
	Description string
	Body        string
	// Group is the file name without the ".md" extension.
	Group string
	// Path is the file that holds the snippet.
	Path string
	// Line is the 1-based file line of the heading.
	Line int
}

// Duplicate records a trigger that more than one heading uses. zn uses the
// first snippet and ignores the later one.
type Duplicate struct {
	Trigger     string
	UsedPath    string
	UsedLine    int
	IgnoredPath string
	IgnoredLine int
}

// IndexProblem is a snippet or a file that zn ignores.
type IndexProblem struct {
	// Path is the file with the problem.
	Path string
	// Line is the 1-based heading line. It is 0 for a file problem.
	Line    int
	Kind    ProblemKind
	Heading string
	// Err is the read error of an unreadable file.
	Err error
}

// Index holds the snippets of one vault.
type Index struct {
	snippets   []Snippet
	duplicates []Duplicate
	problems   []IndexProblem
}

// BuildIndex builds the index from the group files. It sorts the files in byte
// order of their names. The snippets keep the order of the files, then the
// order of the headings.
func BuildIndex(files []GroupFile) *Index {
	sorted := make([]GroupFile, len(files))
	copy(sorted, files)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	idx := &Index{}
	used := map[string]int{}
	for _, f := range sorted {
		if f.Err != nil {
			idx.problems = append(idx.problems, IndexProblem{Path: f.Path, Kind: ProblemUnreadable, Err: f.Err})
			continue
		}
		group := strings.TrimSuffix(f.Name, path.Ext(f.Name))
		defs, problems := ParseGroup(f.Raw)
		for _, p := range problems {
			idx.problems = append(idx.problems, IndexProblem{Path: f.Path, Line: p.Line, Kind: p.Kind, Heading: p.Heading})
		}
		for _, d := range defs {
			if at, ok := used[d.Trigger]; ok {
				first := idx.snippets[at]
				idx.duplicates = append(idx.duplicates, Duplicate{
					Trigger:     d.Trigger,
					UsedPath:    first.Path,
					UsedLine:    first.Line,
					IgnoredPath: f.Path,
					IgnoredLine: d.Line,
				})
				continue
			}
			used[d.Trigger] = len(idx.snippets)
			idx.snippets = append(idx.snippets, Snippet{
				Trigger:     d.Trigger,
				Description: d.Description,
				Body:        d.Body,
				Group:       group,
				Path:        f.Path,
				Line:        d.Line,
			})
		}
	}
	return idx
}

// Snippets returns every usable snippet in index order.
func (x *Index) Snippets() []Snippet {
	if x == nil {
		return nil
	}
	return x.snippets
}

// Match returns the snippets whose trigger starts with prefix, in index order.
// An empty prefix matches nothing.
func (x *Index) Match(prefix string) []Snippet {
	if x == nil || prefix == "" {
		return nil
	}
	var out []Snippet
	for _, s := range x.snippets {
		if MatchPrefix(s.Trigger, prefix) {
			out = append(out, s)
		}
	}
	return out
}

// Duplicates lists the triggers that zn ignores because an earlier snippet has them.
func (x *Index) Duplicates() []Duplicate {
	if x == nil {
		return nil
	}
	return x.duplicates
}

// Problems lists the snippets and files that zn ignores, in file order.
func (x *Index) Problems() []IndexProblem {
	if x == nil {
		return nil
	}
	return x.problems
}

// MatchPrefix reports whether word starts with prefix. A prefix with no
// upper-case letter matches without regard to case. A prefix with an
// upper-case letter matches with the exact case. An empty prefix matches
// nothing.
func MatchPrefix(word, prefix string) bool {
	if prefix == "" {
		return false
	}
	exact := false
	for _, r := range prefix {
		if unicode.IsUpper(r) {
			exact = true
			break
		}
	}
	w := []rune(word)
	for i, p := range []rune(prefix) {
		if i >= len(w) {
			return false
		}
		if exact {
			if w[i] != p {
				return false
			}
		} else if !foldEqual(w[i], p) {
			return false
		}
	}
	return true
}

// foldEqual reports whether a and b are equal under simple case folding.
func foldEqual(a, b rune) bool {
	if a == b {
		return true
	}
	for r := unicode.SimpleFold(a); r != a; r = unicode.SimpleFold(r) {
		if r == b {
			return true
		}
	}
	return false
}
