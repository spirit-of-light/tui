package keymaps

import (
	"strings"
	"testing"
)

// TestTerminalIDsStayOutOfCatalog proves that a zn-only action never joins
// the catalog that the desktop app shares.
func TestTerminalIDsStayOutOfCatalog(t *testing.T) {
	terminal := map[string]bool{}
	for _, e := range TerminalCatalog {
		terminal[e.ID] = true
	}
	for _, e := range Catalog {
		if terminal[e.ID] {
			t.Errorf("Catalog has the zn-only id %q", e.ID)
		}
		if strings.HasPrefix(e.ID, "snippet.") {
			t.Errorf("Catalog id %q starts with snippet.", e.ID)
		}
	}
}

// TestTerminalDefaultsShareCatalogKeys proves that the catalog action that
// owns a default terminal key outside a session keeps that key.
func TestTerminalDefaultsShareCatalogKeys(t *testing.T) {
	r := NewResolver(nil)
	if got := r.Binding("editor.toggleCheckbox"); got != "Ctrl+L" {
		t.Errorf("editor.toggleCheckbox = %q, want Ctrl+L", got)
	}
}
