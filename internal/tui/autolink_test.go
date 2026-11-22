package tui

import (
	"errors"
	"os"
	"path/filepath"
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

const moveBody = "Plain x here.\n\nLater [[X|the x]] and [[X]].\n"

func reviewRows(t *testing.T, a *App) []pickItem {
	t.Helper()
	p, ok := a.overlay.(*pickOverlay)
	if !ok {
		t.Fatalf("overlay = %T, want the review", a.overlay)
	}
	return p.items
}

func requireOldLink(t *testing.T, rows []pickItem) {
	t.Helper()
	for _, it := range rows {
		if strings.HasPrefix(it.label, "move") {
			if !strings.Contains(it.detail, "Old link: line 3  [[X|the x]] → the x") || !strings.Contains(it.detail, "Plain x here.") {
				t.Errorf("move detail = %q, want the old link and the context", it.detail)
			}
			return
		}
	}
	t.Fatalf("no move row in %+v", rows)
}

func TestAutolinkExMoveRowShowsTheOldLink(t *testing.T) {
	a, _ := autolinkApp(t, map[string]string{"X.md": "# X\n", "Y.md": moveBody}, "Y.md")
	if err := runEx(t, a, "autolink", ""); err != nil {
		t.Fatal(err)
	}
	rows := reviewRows(t, a)
	requireOldLink(t, rows)
	for _, it := range rows {
		if !strings.HasPrefix(it.label, "move") && strings.Contains(it.detail, "Old link") {
			t.Errorf("row %q shows an old link, but it is not a move", it.label)
		}
	}
}

func TestAutolinkAllMoveRowShowsTheOldLink(t *testing.T) {
	a, _ := linkTestApp(t, map[string]string{"X.md": "# X\n", "Y.md": moveBody})
	if err := runEx(t, a, "autolink", "--all"); err != nil {
		t.Fatal(err)
	}
	rows := reviewRows(t, a)
	requireOldLink(t, rows)
	for _, it := range rows {
		if !strings.Contains(it.label, "Y.md:") {
			t.Errorf("label %q has no note path", it.label)
		}
	}
}

func TestAutolinkAllDirtyBufferStopsTheRun(t *testing.T) {
	a, root := linkTestApp(t, map[string]string{"X.md": "# X\n", "Y.md": "I read X today.\n", "Z.md": "Z reads X.\n"})
	a.openNote("Y.md", true)
	buf := a.activeBuffer()
	buf.ed.ReplaceText("I read X today. More.\n")
	err := runEx(t, a, "autolink", "--all")
	if err == nil || !strings.Contains(err.Error(), "Y.md") || strings.Contains(err.Error(), "Z.md") {
		t.Fatalf("error = %v, want it to name Y.md only", err)
	}
	if a.overlay != nil {
		t.Errorf("overlay = %T, want none", a.overlay)
	}
	for name, want := range map[string]string{"Y.md": "I read X today.\n", "Z.md": "Z reads X.\n"} {
		got, _ := os.ReadFile(filepath.Join(root, name))
		if string(got) != want {
			t.Errorf("%s on disk = %q, want %q", name, got, want)
		}
	}
}

func TestAutolinkAllWritesBufferAndDisk(t *testing.T) {
	a, root := linkTestApp(t, map[string]string{"X.md": "# X\n", "Y.md": "I read X today.\n", "Z.md": "Z reads X.\n"})
	a.openNote("Y.md", true)
	buf := a.activeBuffer()
	if err := runEx(t, a, "autolink", "--all"); err != nil {
		t.Fatal(err)
	}
	pressEnter(a)
	if got, want := buf.ed.Text(), "I read [[X]] today.\n"; got != want {
		t.Errorf("buffer = %q, want %q", got, want)
	}
	if buf.ed.Dirty() || buf.dirty() {
		t.Error("the buffer of Y must be saved")
	}
	if got, _ := os.ReadFile(filepath.Join(root, "Y.md")); string(got) != "I read [[X]] today.\n" {
		t.Errorf("Y on disk = %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "Z.md")); string(got) != "Z reads [[X]].\n" {
		t.Errorf("Z on disk = %q", got)
	}
	a.handleKey(vim.R('u'))
	if got := buf.ed.Text(); got != "I read X today.\n" {
		t.Errorf("after u, buffer = %q", got)
	}
}

func TestAutolinkAllServerVaultGivesAnError(t *testing.T) {
	a, root := linkTestApp(t, map[string]string{"X.md": "# X\n", "Y.md": "I read X today.\n"})
	a.opts.Target.Kind = backend.KindRemote
	err := runEx(t, a, "autolink", "--all")
	if err == nil || !errors.Is(err, backend.ErrAutolinkRemote) {
		t.Errorf("error = %v, want ErrAutolinkRemote", err)
	}
	if a.overlay != nil {
		t.Errorf("overlay = %T, want none", a.overlay)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "Y.md")); string(got) != "I read X today.\n" {
		t.Errorf("Y on disk = %q", got)
	}
}
