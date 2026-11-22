package templates

import (
	"testing"
	"time"

	"github.com/ZenNotes/tui/internal/periodic"
)

func variableNow() time.Time {
	return time.Date(2026, 3, 5, 14, 7, 0, 0, time.UTC)
}

func TestTemplateVariableValues(t *testing.T) {
	now := variableNow()
	cases := []struct{ name, want string }{
		{"title", "Plan"},
		{"date", "2026-03-05"},
		{"time", "14:07"},
		{"week", "10"},
	}
	for _, c := range cases {
		got, ok := Variable(c.name, "", false, "Plan", now, "en")
		if !ok || got != c.want {
			t.Errorf("Variable(%q) = %q, %v; want %q, true", c.name, got, ok, c.want)
		}
	}
}

func TestTemplateVariableDateFormat(t *testing.T) {
	now := variableNow()
	for _, locale := range []string{"en", "de"} {
		for _, format := range []string{"", "YYYY-MM-DD", "dddd, MMMM D, YYYY", "[W]ww"} {
			want := periodic.FormatTemplateDateLocale(now, format, locale)
			got, ok := Variable("date", format, true, "", now, locale)
			if !ok || got != want {
				t.Errorf("Variable(date, %q, %s) = %q, %v; want %q", format, locale, got, ok, want)
			}
		}
	}
	if got, _ := Variable("date", "", true, "", now, "en"); got != "" {
		t.Errorf("empty format gave %q; want empty", got)
	}
}

func TestTemplateVariableUnknown(t *testing.T) {
	for _, name := range []string{"foo", "cursor", "Title", "", "date "} {
		if got, ok := Variable(name, "", false, "Plan", variableNow(), "en"); ok || got != "" {
			t.Errorf("Variable(%q) = %q, %v; want empty, false", name, got, ok)
		}
	}
	for _, name := range []string{"title", "time", "week", "foo"} {
		if got, ok := Variable(name, "x", true, "Plan", variableNow(), "en"); ok || got != "" {
			t.Errorf("Variable(%q, with arg) = %q, %v; want empty, false", name, got, ok)
		}
	}
}

func TestTemplateVariableRenderUnchanged(t *testing.T) {
	body := "# {{title}} | {{ date }} | {{time}} | {{week}} | {{date:YYYY}}-W{{week}} | {{date:}} | {{foo}} | a{{cursor}}b"
	got := Render(body, "Plan", variableNow())
	want := "# Plan | 2026-03-05 | 14:07 | 10 | 2026-W10 |  | {{foo}} | ab"
	if got.Body != want {
		t.Errorf("Render body = %q; want %q", got.Body, want)
	}
	if got.CursorOffset != len([]rune("# Plan | 2026-03-05 | 14:07 | 10 | 2026-W10 |  | {{foo}} | a")) {
		t.Errorf("CursorOffset = %d", got.CursorOffset)
	}
}

func TestTemplateVariableLiteralForms(t *testing.T) {
	body := "{{title:x}} {{time:HH}} {{week:x}} {{date :x}} {{cursor:x}}"
	got := Render(body, "Plan", variableNow())
	if got.Body != body || got.CursorOffset != -1 {
		t.Errorf("Render = %q, %d; want the body unchanged and no cursor", got.Body, got.CursorOffset)
	}
}
