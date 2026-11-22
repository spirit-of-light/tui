package keymaps

import (
	"strings"

	"github.com/ZenNotes/tui/internal/vim"
)

// TerminalEntry is one zn-only action. The desktop app has no such action,
// so these ids stay out of Catalog and live in the `[terminal.keymaps]`
// table of config.toml. An action can have more than one key.
type TerminalEntry struct {
	ID       string
	Title    string
	Bindings []string
}

// TerminalCatalog lists the zn-only actions with their default keys.
var TerminalCatalog = []TerminalEntry{
	{"snippet.nextTabstop", "Next snippet tabstop", []string{"Ctrl+L", "Tab"}},
	{"snippet.prevTabstop", "Previous snippet tabstop", []string{"Ctrl+H", "Shift+Tab"}},
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
