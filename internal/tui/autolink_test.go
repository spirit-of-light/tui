package tui

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/ZenNotes/tui/internal/backend"
	"github.com/ZenNotes/tui/internal/vim"
)

func autolinkApp(t *testing.T, files map[string]string, open string) (*App, *noteBuffer) {
	t.Helper()
	a, _ := linkTestApp(t, files)
	a.openNote(open, true)
	buf := a.activeBuffer()
	if buf == nil || buf.path != open {
		t.Fatalf("did not open %s", open)
	}
	return a, buf
}

func pressEnter(a *App) { a.handleKey(vim.Key{Name: "enter"}) }

func TestAutolinkExAddsLinksFromTheEx(t *testing.T) {
	a, buf := autolinkApp(t, map[string]string{"X.md": "# X\n", "Y.md": "I read X today.\n"}, "Y.md")
	if err := runEx(t, a, "autolink", ""); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.overlay.(*pickOverlay); !ok {
		t.Fatalf("overlay = %T, want the review", a.overlay)
	}
	pressEnter(a)
	if got, want := buf.ed.Text(), "I read [[X]] today.\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
	if !buf.ed.Dirty() {
		t.Error("autolink must not save the buffer")
	}
}

func TestAutolinkExPaletteEntryGivesTheSameBody(t *testing.T) {
	a, buf := autolinkApp(t, map[string]string{"X.md": "# X\n", "Y.md": "I read X today.\n"}, "Y.md")
	a.openCommandPalette()
	pal, ok := a.overlay.(*palette)
	if !ok {
		t.Fatalf("overlay = %T", a.overlay)
	}
	var item *paletteItem
	for i := range pal.items {
		if pal.items[i].label == "Autolink" {
			item = &pal.items[i]
		}
	}
	if item == nil {
		t.Fatal("no Autolink entry in the palette")
	}
	a.overlay = nil
	a.runCommandLine(item.id)
	pressEnter(a)
	if got, want := buf.ed.Text(), "I read [[X]] today.\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestAutolinkExOneUndoStep(t *testing.T) {
	orig := "X and Z are here.\n"
	a, buf := autolinkApp(t, map[string]string{"X.md": "# X\n", "Z.md": "# Z\n", "Y.md": orig}, "Y.md")
	if err := runEx(t, a, "autolink", ""); err != nil {
		t.Fatal(err)
	}
	pressEnter(a)
	if got, want := buf.ed.Text(), "[[X]] and [[Z]] are here.\n"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
	a.handleKey(vim.R('u'))
	if got := buf.ed.Text(); got != orig {
		t.Errorf("after u, body = %q, want %q", got, orig)
	}
}

func TestAutolinkExTabOmitsAChange(t *testing.T) {
	a, buf := autolinkApp(t, map[string]string{"X.md": "# X\n", "Z.md": "# Z\n", "Y.md": "X and Z are here.\n"}, "Y.md")
	if err := runEx(t, a, "autolink", ""); err != nil {
		t.Fatal(err)
	}
	a.handleKey(vim.Key{Name: "tab"})
	pressEnter(a)
	if got, want := buf.ed.Text(), "X and [[Z]] are here.\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestAutolinkExEscChangesNothing(t *testing.T) {
	a, buf := autolinkApp(t, map[string]string{"X.md": "# X\n", "Y.md": "I read X today.\n"}, "Y.md")
	if err := runEx(t, a, "autolink", ""); err != nil {
		t.Fatal(err)
	}
	a.handleKey(vim.Key{Name: "esc"})
	if a.overlay != nil {
		t.Errorf("overlay = %T, want none", a.overlay)
	}
	if got := buf.ed.Text(); got != "I read X today.\n" {
		t.Errorf("body = %q", got)
	}
	if buf.ed.Dirty() {
		t.Error("the buffer is dirty")
	}
}

func TestAutolinkExConflictMenu(t *testing.T) {
	files := map[string]string{
		"Linear Algebra.md": "# Linear Algebra\n",
		"Algebra.md":        "# Algebra\n",
		"Y.md":              "Linear Algebra is here. Later Linear Algebra again.\n",
	}
	a, buf := autolinkApp(t, files, "Y.md")
	if err := runEx(t, a, "autolink", ""); err != nil {
		t.Fatal(err)
	}
	m, ok := a.overlay.(*menu)
	if !ok {
		t.Fatalf("overlay = %T, want the conflict menu", a.overlay)
	}
	if len(m.items) != 3 || !strings.HasPrefix(m.items[0].label, "Skip") {
		t.Fatalf("menu items = %+v, want skip and two notes", m.items)
	}
	for i, it := range m.items {
		if it.label == "Link to Algebra.md" {
			m.cursor = i
		}
	}
	pressEnter(a)
	if _, ok := a.overlay.(*pickOverlay); !ok {
		t.Fatalf("overlay = %T, want the review", a.overlay)
	}
	pressEnter(a)
	want := "Linear [[Algebra]] is here. Later [[Linear Algebra]] again.\n"
	if got := buf.ed.Text(); got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestAutolinkExSkipLeavesTextPlain(t *testing.T) {
	files := map[string]string{
		"Linear Algebra.md": "# Linear Algebra\n",
		"Algebra.md":        "# Algebra\n",
		"Y.md":              "Linear Algebra is here.\n",
	}
	a, buf := autolinkApp(t, files, "Y.md")
	if err := runEx(t, a, "autolink", ""); err != nil {
		t.Fatal(err)
	}
	pressEnter(a) // the first item is skip
	if a.overlay != nil {
		t.Errorf("overlay = %T, want none", a.overlay)
	}
	if got := buf.ed.Text(); got != "Linear Algebra is here.\n" {
		t.Errorf("body = %q", got)
	}
}

func TestAutolinkExServerVaultGivesAnError(t *testing.T) {
	a, buf := autolinkApp(t, map[string]string{"X.md": "# X\n", "Y.md": "I read X today.\n"}, "Y.md")
	a.opts.Target.Kind = backend.KindRemote
	err := runEx(t, a, "autolink", "")
	if err == nil || !errors.Is(err, backend.ErrAutolinkRemote) {
		t.Errorf("error = %v, want ErrAutolinkRemote", err)
	}
	if a.overlay != nil {
		t.Errorf("overlay = %T, want none", a.overlay)
	}
	if got := buf.ed.Text(); got != "I read X today.\n" {
		t.Errorf("body = %q", got)
	}
}

func TestAutolinkExNoChangeShowsAMessage(t *testing.T) {
	a, buf := autolinkApp(t, map[string]string{"X.md": "# X\n", "Y.md": "Nothing here.\n"}, "Y.md")
	if err := runEx(t, a, "autolink", ""); err != nil {
		t.Fatal(err)
	}
	if a.overlay != nil {
		t.Errorf("overlay = %T, want none", a.overlay)
	}
	if !strings.Contains(a.message, "No change") || a.messageErr {
		t.Errorf("message = %q (error %v)", a.message, a.messageErr)
	}
	if buf.ed.Dirty() {
		t.Error("the buffer is dirty")
	}
}

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
