package keymaps

import (
	"strings"

	"github.com/ZenNotes/tui/internal/vim"
)

// TerminalEntry is one zn-only action. The desktop app has no such action,
// so these ids stay out of Catalog and live in the `[terminal.keymaps]`
// table of config.toml. An action can have more than one key.
//
// Modes is parallel to Bindings. A default key with the mode "vim" acts only
// with Vim on, "plain" acts only with Vim off, and "" acts in both modes.
// An empty Modes means every default key acts in both modes.
type TerminalEntry struct {
	ID       string
	Title    string
	Bindings []string
	Modes    []string
}

const (
	modeVim   = "vim"
	modePlain = "plain"
)

// TerminalCatalog lists the zn-only actions with their default keys.
var TerminalCatalog = []TerminalEntry{
	{ID: "snippet.expand", Title: "Expand a snippet", Bindings: []string{"Ctrl+N", "Tab"}, Modes: []string{modeVim, modePlain}},
	{ID: "snippet.nextTabstop", Title: "Next snippet tabstop", Bindings: []string{"Ctrl+L", "Tab"}},
	{ID: "snippet.prevTabstop", Title: "Previous snippet tabstop", Bindings: []string{"Ctrl+H", "Shift+Tab"}},
}

// TerminalResolver answers "which keys does this zn-only action have" with
// overrides applied. An override replaces all default keys of the action,
// and an empty list unbinds it.
type TerminalResolver struct {
	overrides map[string][]string
	byID      map[string]TerminalEntry
}

// NewTerminalResolver applies `[terminal.keymaps]` overrides on top of
// TerminalCatalog. It ignores an id that the catalog does not have.
func NewTerminalResolver(overrides map[string][]string) *TerminalResolver {
	r := &TerminalResolver{overrides: map[string][]string{}, byID: map[string]TerminalEntry{}}
	for _, e := range TerminalCatalog {
		r.byID[e.ID] = e
	}
	for id, keys := range overrides {
		r.Set(id, keys)
	}
	return r
}

// Bindings is the effective key strings of an action, nil when unbound.
func (r *TerminalResolver) Bindings(id string) []string {
	if keys, ok := r.overrides[id]; ok {
		return append([]string(nil), keys...)
	}
	return append([]string(nil), r.byID[id].Bindings...)
}

// Keys is the effective bindings of an action as key sequences. A key
// string that zn cannot read gives no sequence.
func (r *TerminalResolver) Keys(id string) [][]vim.Key {
	var out [][]vim.Key
	for _, b := range r.Bindings(id) {
		if keys, ok := ParseKeys(b); ok && len(keys) > 0 {
			out = append(out, keys)
		}
	}
	return out
}

// Overridden is true when config.toml or a command set the keys of the
// action, also when the list is empty.
func (r *TerminalResolver) Overridden(id string) bool {
	_, ok := r.overrides[id]
	return ok
}

// KeysFor is the key sequences of an action that act with Vim on or off. An
// override acts in both modes. A default key acts in the mode of its entry.
func (r *TerminalResolver) KeysFor(id string, vimOn bool) [][]vim.Key {
	var out [][]vim.Key
	for _, b := range r.bindingsFor(id, vimOn) {
		if keys, ok := ParseKeys(b); ok && len(keys) > 0 {
			out = append(out, keys)
		}
	}
	return out
}

func (r *TerminalResolver) bindingsFor(id string, vimOn bool) []string {
	if r.Overridden(id) {
		return r.Bindings(id)
	}
	e := r.byID[id]
	want := modePlain
	if vimOn {
		want = modeVim
	}
	var out []string
	for i, b := range e.Bindings {
		if i >= len(e.Modes) || e.Modes[i] == "" || e.Modes[i] == want {
			out = append(out, b)
		}
	}
	return out
}

// Labels is the display text of the keys of an action. A default key with a
// mode shows the mode, as in "Ctrl+N (Vim)". An override shows plain keys.
func (r *TerminalResolver) Labels(id string) []string {
	if r.Overridden(id) {
		return r.Bindings(id)
	}
	e := r.byID[id]
	out := make([]string, 0, len(e.Bindings))
	for i, b := range e.Bindings {
		switch {
		case i >= len(e.Modes):
			out = append(out, b)
		case e.Modes[i] == modeVim:
			out = append(out, b+" (Vim)")
		case e.Modes[i] == modePlain:
			out = append(out, b+" (no Vim)")
		default:
			out = append(out, b)
		}
	}
	return out
}

// Set changes the keys of an action in memory. Blank key strings are
// dropped, so no keys unbinds the action. It is false for an unknown id.
func (r *TerminalResolver) Set(id string, keys []string) bool {
	if _, known := r.byID[id]; !known {
		return false
	}
	kept := []string{}
	for _, k := range keys {
		if k = strings.TrimSpace(k); k != "" {
			kept = append(kept, k)
		}
	}
	r.overrides[id] = kept
	return true
}

// Known is true for an id of TerminalCatalog.
func (r *TerminalResolver) Known(id string) bool {
	_, ok := r.byID[id]
	return ok
}

// SplitTerminalKeys splits the text of a `:bind` command for a zn-only
// action into key strings. A comma directly after "+" is part of a key
// (`Ctrl+,`). Any other comma separates two keys, and the spaces around it
// do not count. A space inside a key still separates the keys of a
// sequence (`Space n`). Empty parts are dropped.
func SplitTerminalKeys(s string) []string {
	var out []string
	var cur []rune
	flush := func() {
		if f := strings.Fields(string(cur)); len(f) > 0 {
			out = append(out, strings.Join(f, " "))
		}
		cur = cur[:0]
	}
	for _, r := range s {
		if r == ',' && (len(cur) == 0 || cur[len(cur)-1] != '+') {
			flush()
			continue
		}
		cur = append(cur, r)
	}
	flush()
	return out
}

// Entries lists TerminalCatalog with the overrides applied.
func (r *TerminalResolver) Entries() []TerminalEntry {
	out := make([]TerminalEntry, 0, len(TerminalCatalog))
	for _, e := range TerminalCatalog {
		e.Bindings = r.Bindings(e.ID)
		out = append(out, e)
	}
	return out
}
