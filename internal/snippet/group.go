package snippet

import (
	"strings"
	"unicode"
)

// Def is one snippet read from a group file.
type Def struct {
	// Trigger is the heading text. It is one word.
	Trigger string
	// Description is the first paragraph after the heading, or "".
	Description string
	// Body is the text of the first fenced block, with no final line break.
	Body string
	// Line is the 1-based file line of the heading.
	Line int
}

// ProblemKind tells why zn ignores a snippet.
type ProblemKind int

const (
	// ProblemNotOneWord means the heading text is not one word.
	ProblemNotOneWord ProblemKind = iota
	// ProblemNoFence means the snippet has no fenced block.
	ProblemNoFence
	// ProblemUnclosedFence means the fenced block does not close.
	ProblemUnclosedFence
	// ProblemUnreadable means zn could not read the whole file.
	ProblemUnreadable
)

// Problem is a snippet or a file that zn ignores.
type Problem struct {
	// Line is the 1-based file line of the heading. It is 0 for a file problem.
	Line int
	Kind ProblemKind
	// Heading is the heading text.
	Heading string
}

// IsWordRune reports whether r is a word character. It gives the same answer
// as the editor word rule; a test in internal/vim checks this.
func IsWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r)
}

// IsTrigger reports whether s is a valid trigger: one or more word characters.
func IsTrigger(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !IsWordRune(r) {
			return false
		}
	}
	return true
}

// ParseGroup reads the text of a group file. Each level-2 heading outside a
// fenced block starts a snippet. The first paragraph after the heading is the
// description and the first fenced block is the body. A snippet that cannot
// be used is reported as a Problem and does not stop the others.
func ParseGroup(raw string) ([]Def, []Problem) {
	lines := strings.Split(raw, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSuffix(l, "\r")
	}
	start := skipFrontmatter(lines)

	var (
		defs     []Def
		problems []Problem
		cur      *section
		fence    fenceState
	)
	finish := func() {
		if cur == nil {
			return
		}
		switch {
		case cur.bad:
			problems = append(problems, Problem{Line: cur.line, Kind: ProblemNotOneWord, Heading: cur.trigger})
		case cur.unclosed:
			problems = append(problems, Problem{Line: cur.line, Kind: ProblemUnclosedFence, Heading: cur.trigger})
		case !cur.hasBody:
			problems = append(problems, Problem{Line: cur.line, Kind: ProblemNoFence, Heading: cur.trigger})
		default:
			defs = append(defs, Def{
				Trigger:     cur.trigger,
				Description: strings.Join(cur.desc, " "),
				Body:        strings.Join(cur.body, "\n"),
				Line:        cur.line,
			})
		}
		cur = nil
	}

	for i := start; i < len(lines); i++ {
		line := lines[i]
		if fence.open {
			if fence.closes(line) {
				fence.open = false
				if fence.capture {
					cur.hasBody = true
					cur.unclosed = false
				}
			} else if fence.capture {
				cur.body = append(cur.body, line)
			}
			continue
		}
		if f, ok := openFence(line); ok {
			if cur != nil {
				cur.descDone = true
				if !cur.bad && !cur.hasBody {
					f.capture = true
					cur.unclosed = true
				}
			}
			fence = f
			continue
		}
		if text, ok := levelTwoHeading(line); ok {
			finish()
			cur = &section{trigger: text, line: i + 1, bad: !IsTrigger(text)}
			continue
		}
		if cur == nil || cur.descDone || cur.hasBody {
			continue
		}
		if strings.TrimSpace(line) == "" {
			if len(cur.desc) > 0 {
				cur.descDone = true
			}
			continue
		}
		cur.desc = append(cur.desc, strings.TrimSpace(line))
	}
	finish()
	return defs, problems
}

// section is a snippet while the parser reads it.
type section struct {
	trigger  string
	line     int
	bad      bool
	desc     []string
	descDone bool
	body     []string
	hasBody  bool
	unclosed bool
}

// skipFrontmatter returns the index of the first line after the optional
// frontmatter. Frontmatter needs a closing line; with none, the file has none.
func skipFrontmatter(lines []string) int {
	if len(lines) == 0 || strings.TrimRight(lines[0], " \t") != "---" {
		return 0
	}
	for i := 1; i < len(lines); i++ {
		t := strings.TrimRight(lines[i], " \t")
		if t == "---" || t == "..." {
			return i + 1
		}
	}
	return 0
}

// fenceState describes a fenced block that the parser is inside.
type fenceState struct {
	open    bool
	char    byte
	length  int
	capture bool
}

// leadingSpaces counts the spaces at the start of line, up to four.
func leadingSpaces(line string) int {
	n := 0
	for n < len(line) && n < 4 && line[n] == ' ' {
		n++
	}
	return n
}

// openFence tests whether line opens a fenced block.
func openFence(line string) (fenceState, bool) {
	ind := leadingSpaces(line)
	if ind > 3 || ind >= len(line) {
		return fenceState{}, false
	}
	c := line[ind]
	if c != '`' && c != '~' {
		return fenceState{}, false
	}
	n := 0
	for ind+n < len(line) && line[ind+n] == c {
		n++
	}
	if n < 3 {
		return fenceState{}, false
	}
	// The info string of a backtick fence has no backtick.
	if c == '`' && strings.Contains(line[ind+n:], "`") {
		return fenceState{}, false
	}
	return fenceState{open: true, char: c, length: n}, true
}

// closes tests whether line closes the fence f.
func (f fenceState) closes(line string) bool {
	ind := leadingSpaces(line)
	if ind > 3 || ind >= len(line) || line[ind] != f.char {
		return false
	}
	n := 0
	for ind+n < len(line) && line[ind+n] == f.char {
		n++
	}
	return n >= f.length && strings.Trim(line[ind+n:], " \t") == ""
}

// levelTwoHeading returns the trimmed text of a level-2 heading line.
func levelTwoHeading(line string) (string, bool) {
	ind := leadingSpaces(line)
	if ind > 3 || !strings.HasPrefix(line[ind:], "##") {
		return "", false
	}
	rest := line[ind+2:]
	if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
		return "", false
	}
	return strings.TrimSpace(rest), true
}
