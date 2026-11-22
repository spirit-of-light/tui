package tui

import (
	"testing"
	"time"

	"github.com/ZenNotes/tui/internal/config"
	"github.com/ZenNotes/tui/internal/templates"
	"github.com/ZenNotes/tui/internal/vim"
)

func snippetFixNow(t *testing.T, now time.Time) {
	t.Helper()
	prev := snippetNow
	snippetNow = func() time.Time { return now }
	t.Cleanup(func() { snippetNow = prev })
}

func vimOffPrefs() config.Prefs {
	p := vimPrefs()
	p.VimMode = false
	return p
}

func TestAppSnippetExpandTitle(t *testing.T) {
	snippetFixNow(t, time.Date(2026, 3, 5, 14, 7, 0, 0, time.UTC))
	a, buf := newTagTestApp(t, vimPrefs(), "")
	meta, ok := a.noteMeta(buf.path)
	if !ok || meta.Title == "" {
		t.Fatalf("test note has no title in the index: %+v", meta)
	}
	a.expandSnippet(buf, "# ${title}")
	if got, want := buf.ed.Text(), "# "+meta.Title; got != want {
		t.Fatalf("text %q, want %q", got, want)
	}
}

func TestAppSnippetExpandNoTitle(t *testing.T) {
	snippetFixNow(t, time.Date(2026, 3, 5, 14, 7, 0, 0, time.UTC))
	a, buf := newTagTestApp(t, vimPrefs(), "")
	delete(a.idx.byPath, buf.path)
	a.expandSnippet(buf, "[${title}]")
	if got := buf.ed.Text(); got != "[]" {
		t.Fatalf("text %q, want %q", got, "[]")
	}
}

func TestAppSnippetExpandDateTimeWeek(t *testing.T) {
	now := time.Date(2026, 3, 5, 14, 7, 0, 0, time.UTC)
	snippetFixNow(t, now)
	a, buf := newTagTestApp(t, vimPrefs(), "")
	a.expandSnippet(buf, "${date} ${time} ${week}")
	want := templates.Render("{{date}} {{time}} {{week}}", "", now).Body
	if got := buf.ed.Text(); got != "2026-03-05 14:07 10" || got != want {
		t.Fatalf("text %q, template %q", got, want)
	}
}

func TestAppSnippetExpandDateFormatsMatchTemplates(t *testing.T) {
	now := time.Date(2026, 3, 5, 14, 7, 0, 0, time.UTC)
	snippetFixNow(t, now)
	for _, format := range []string{"YYYY/MM/DD", "dddd, D MMMM", "HH.mm"} {
		a, buf := newTagTestApp(t, vimPrefs(), "")
		a.expandSnippet(buf, "${date:"+format+"}")
		want := templates.Render("{{date:"+format+"}}", "", now).Body
		if want == "" || want == "{{date:"+format+"}}" {
			t.Fatalf("template gave no value for %q: %q", format, want)
		}
		if got := buf.ed.Text(); got != want {
			t.Errorf("format %q: text %q, template %q", format, got, want)
		}
	}
}

func TestAppSnippetExpandVariableInPlaceholder(t *testing.T) {
	snippetFixNow(t, time.Date(2026, 3, 5, 14, 7, 0, 0, time.UTC))
	a, buf := newTagTestApp(t, vimPrefs(), "")
	a.expandSnippet(buf, "${1:due ${date}}")
	if got := buf.ed.Text(); got != "due 2026-03-05" {
		t.Fatalf("text %q", got)
	}
	if !buf.ed.SnippetActive() {
		t.Fatal("no session for a body with a placeholder")
	}
}

func TestAppSnippetExpandLiteralBodies(t *testing.T) {
	snippetFixNow(t, time.Date(2026, 3, 5, 14, 7, 0, 0, time.UTC))
	cases := map[string]string{
		`${foo}`:               `${foo}`,
		`a}b`:                  `a}b`,
		`$x$ and $ 5`:          `$x$ and $ 5`,
		`a ${ b`:               `a ${ b`,
		`\$1`:                  `$1`,
		`\frac{a}{b} \\ x`:     `\frac{a}{b} \\ x`,
		`${title:x} ${time:y}`: `${title:x} ${time:y}`,
	}
	for body, want := range cases {
		a, buf := newTagTestApp(t, vimPrefs(), "")
		a.expandSnippet(buf, body)
		if got := buf.ed.Text(); got != want {
			t.Errorf("body %q: text %q, want %q", body, got, want)
		}
		if a.message != "" {
			t.Errorf("body %q: message %q", body, a.message)
		}
	}
}

func TestAppSnippetExpandGoesToInsertMode(t *testing.T) {
	snippetFixNow(t, time.Date(2026, 3, 5, 14, 7, 0, 0, time.UTC))
	a, buf := newTagTestApp(t, vimPrefs(), "")
	if buf.ed.Mode() != vim.ModeNormal {
		t.Fatalf("start mode %v", buf.ed.Mode())
	}
	a.expandSnippet(buf, "a $1 b")
	if buf.ed.Mode() != vim.ModeInsert {
		t.Fatalf("mode after expansion %v, want insert", buf.ed.Mode())
	}
}

func TestAppSnippetExpandVimOff(t *testing.T) {
	snippetFixNow(t, time.Date(2026, 3, 5, 14, 7, 0, 0, time.UTC))
	a, buf := newTagTestApp(t, vimOffPrefs(), "")
	a.expandSnippet(buf, "x ${1:y} z")
	if got := buf.ed.Text(); got != "x y z" {
		t.Fatalf("text %q", got)
	}
	if !buf.ed.SnippetActive() {
		t.Fatal("no session with Vim off")
	}
}

func TestAppSnippetExpandRecordsOwner(t *testing.T) {
	snippetFixNow(t, time.Date(2026, 3, 5, 14, 7, 0, 0, time.UTC))
	a, buf := newTagTestApp(t, vimPrefs(), "")
	a.expandSnippet(buf, "abc")
	if a.snippetOwner.buf != nil {
		t.Fatalf("owner kept for a body with no session: %+v", a.snippetOwner)
	}
	a.expandSnippet(buf, "$1 $2")
	o := a.snippetOwner
	if o.buf != buf || o.pane != a.activePane || o.tab != a.activeTab() {
		t.Fatalf("owner %+v does not match the focused buffer", o)
	}
}
