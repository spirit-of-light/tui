package tui

import (
	"reflect"
	"testing"
)

// A note that another note links to by one of its aliases lists that note as a
// backlink, and the link is not unresolved on the other side.
func TestAutolinkAliasBacklink(t *testing.T) {
	files := map[string]string{
		"X.md": "---\naliases: [LA]\n---\n# X\n",
		"Y.md": "see [[LA]]\n",
	}
	a, _ := linkTestApp(t, files)
	a.openNote("X.md", true)
	p := &connectionsPanel{}
	p.refresh(a)
	got := []string{}
	for _, n := range p.backlinks {
		got = append(got, n.Path)
	}
	if want := []string{"Y.md"}; !reflect.DeepEqual(got, want) {
		t.Errorf("backlinks = %q, want %q", got, want)
	}

	a, _ = linkTestApp(t, files)
	a.openNote("Y.md", true)
	p = &connectionsPanel{}
	p.refresh(a)
	if len(p.missing) != 0 {
		t.Errorf("missing = %q, want none", p.missing)
	}
}
