package templates

import (
	"strings"
	"testing"
	"time"
)

var fixed = time.Date(2026, 11, 7, 9, 30, 0, 0, time.UTC)

// Every built-in renders exactly as it did when its cursor was `{{cursor}}`:
// same text, same cursor offset, and the only tabstop is the cursor.
func TestBuiltinsRenderAsBeforeSnippets(t *testing.T) {
	for _, b := range Builtins() {
		if strings.Count(b.Body, "$") != 1 || !strings.Contains(b.Body, "$1") {
			t.Fatalf("%s: want exactly one $1 tabstop, body %q", b.ID, b.Body)
		}
		legacy := Render(strings.ReplaceAll(b.Body, "$1", "{{cursor}}"), "Title", fixed)
		now := Render(b.Body, "Title", fixed)
		if now.Body != legacy.Body || now.CursorOffset != legacy.CursorOffset {
			t.Fatalf("%s: rendered %q@%d, legacy %q@%d", b.ID, now.Body, now.CursorOffset, legacy.Body, legacy.CursorOffset)
		}
		if now.CursorOffset < 0 {
			t.Fatalf("%s: cursor not placed", b.ID)
		}
		if strings.Contains(now.Body, "$") {
			t.Fatalf("%s: tabstop marker leaked into the note: %q", b.ID, now.Body)
		}
	}
}

func TestLegacyCursorTokenStillPlacesCursor(t *testing.T) {
	r := Render("ab{{cursor}}cd", "T", fixed)
	if r.Body != "abcd" || r.CursorOffset != 2 {
		t.Fatalf("%q@%d", r.Body, r.CursorOffset)
	}
}

func TestBodyWithoutStopsHasNoCursor(t *testing.T) {
	r := Render("# {{title}}\n", "Note", fixed)
	if r.Body != "# Note\n" || r.CursorOffset != -1 {
		t.Fatalf("%q@%d", r.Body, r.CursorOffset)
	}
}

func TestTitleWithDollarStaysLiteral(t *testing.T) {
	r := Render("# {{title}}\n$1", "a$1b", fixed)
	if r.Body != "# a$1b\n" || r.CursorOffset != 7 {
		t.Fatalf("%q@%d", r.Body, r.CursorOffset)
	}
}

func TestRenderTitleDropsCursorAndExpandsDates(t *testing.T) {
	if got := RenderTitle("{{date:YYYY-MM-DD}} {{cursor}}", "", fixed); got != "2026-11-07" {
		t.Fatalf("title %q", got)
	}
}
