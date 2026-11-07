package snippets

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// Snippet is a user-defined snippet, expanded by typing its trigger and
// pressing Ctrl+N in insert mode.
type Snippet struct {
	Trigger     string
	Description string
	Body        string
}

// Library maps triggers to snippets.
type Library map[string]Snippet

// Lookup returns the snippet for a trigger word.
func (l Library) Lookup(trigger string) (Snippet, bool) {
	s, ok := l[trigger]
	return s, ok
}

// Triggers lists the trigger words in order.
func (l Library) Triggers() []string {
	out := make([]string, 0, len(l))
	for t := range l {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// IsWordRune reports whether r may appear in a trigger: letters, digits,
// underscore and hyphen. A trigger is the run of such runes before the cursor.
func IsWordRune(r rune) bool {
	return r == '_' || r == '-' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

type fileDoc struct {
	Snippet []struct {
		Trigger     string `toml:"trigger"`
		Description string `toml:"description"`
		Body        string `toml:"body"`
	} `toml:"snippet"`
}

// ParseLibrary reads the TOML form:
//
//	[[snippet]]
//	trigger = "snip"
//	description = "A snippet"
//	body = '''
//	${1:Title}
//	$0
//	'''
//
// Invalid entries are skipped and reported; the rest still load.
func ParseLibrary(raw string) (Library, []error) {
	var doc fileDoc
	if _, err := toml.Decode(raw, &doc); err != nil {
		return Library{}, []error{fmt.Errorf("snippets: %w", err)}
	}
	lib := Library{}
	var errs []error
	for i, s := range doc.Snippet {
		trigger := strings.TrimSpace(s.Trigger)
		switch {
		case trigger == "":
			errs = append(errs, fmt.Errorf("snippet %d: trigger is empty", i+1))
			continue
		case !allWordRunes(trigger):
			errs = append(errs, fmt.Errorf("snippet %q: trigger may only use letters, digits, _ and -", trigger))
			continue
		case s.Body == "":
			errs = append(errs, fmt.Errorf("snippet %q: body is empty", trigger))
			continue
		}
		if _, dup := lib[trigger]; dup {
			errs = append(errs, fmt.Errorf("snippet %q: defined twice; the later one is used", trigger))
		}
		lib[trigger] = Snippet{Trigger: trigger, Description: s.Description, Body: strings.TrimSuffix(s.Body, "\n")}
	}
	return lib, errs
}

// LoadLibrary reads path. A missing file is an empty library, not an error.
func LoadLibrary(path string) (Library, []error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Library{}, nil
	}
	if err != nil {
		return Library{}, []error{err}
	}
	return ParseLibrary(string(raw))
}

func allWordRunes(s string) bool {
	for _, r := range s {
		if !IsWordRune(r) {
			return false
		}
	}
	return s != ""
}
