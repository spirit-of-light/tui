package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/ZenNotes/tui/internal/vim"
)

// These tests set the colour profile of the process, so they do not run in
// parallel.
func trueColor(t *testing.T) {
	t.Helper()
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(profile) })
}

func screenRows(a *App) [][]rune {
	rows := [][]rune{}
	for _, line := range strings.Split(ansi.Strip(a.View()), "\n") {
		rows = append(rows, []rune(line))
	}
	return rows
}

// boxAt finds the box whose top-left corner is in column col, and gives the
// row of that corner, the row of the bottom-left corner and the text of each
// row between them.
func boxAt(t *testing.T, a *App, col int) (top, bottom int, inner []string) {
	t.Helper()
	rows := screenRows(a)
	tops := []int{}
	for r, row := range rows {
		if col < len(row) && row[col] == '╭' {
			tops = append(tops, r)
		}
	}
	if len(tops) != 1 {
		t.Fatalf("%d box corners in column %d, want 1\n%s", len(tops), col, ansi.Strip(a.View()))
	}
	top = tops[0]
	for r := top + 1; r < len(rows); r++ {
		row := rows[r]
		if col >= len(row) {
			break
		}
		if row[col] == '╰' {
			return top, r, inner
		}
		text := string(row[col+1:])
		if end := strings.Index(text, "│"); end >= 0 {
			text = text[:end]
		}
		inner = append(inner, strings.TrimSpace(text))
	}
	t.Fatalf("the box has no bottom edge\n%s", ansi.Strip(a.View()))
	return 0, 0, nil
}

// popupBox is the box of the open snippet popup.
func popupBox(t *testing.T, a *App) (top, bottom int, inner []string) {
	t.Helper()
	p := a.liveSnippetPopup()
	if p == nil {
		t.Fatal("no snippet popup is open")
	}
	a.View() // the rows of the buffer come from a render
	x, _, ok := a.anchorAt(p.buf, p.at.Line, p.start)
	if !ok {
		t.Fatal("the popup has no anchor on screen")
	}
	return boxAt(t, a, x-2)
}

func manyTriggers(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "## t%02d\n\n```\nB%d\n```\n\n", i, i)
	}
	return b.String()
}

func TestSnippetPopupRenderRows(t *testing.T) {
	trueColor(t)
	files := map[string]string{"meetings.md": popupFiles}
	a, _, key := triggerAppDraft(t, true, "meeting\nxxxxxxxx ", files)
	typeKeys(a, "me")
	feed(a, key)
	_, _, inner := popupBox(t, a)
	var trigger, word string
	for _, row := range inner {
		if strings.Contains(row, "Plan a meeting.") {
			trigger = row
		}
		if strings.Contains(row, "word") {
			word = row
		}
	}
	if !strings.Contains(trigger, "meet") || !strings.Contains(trigger, "meetings") {
		t.Errorf("the meet row %q must show the trigger, the description and the group\n%v", trigger, inner)
	}
	if !strings.Contains(word, "meeting") {
		t.Errorf("the word row %q must show the word and the label word\n%v", word, inner)
	}
}

func TestSnippetPopupRenderRowsAreLimitedAndScroll(t *testing.T) {
	trueColor(t)
	a, _, key := triggerAppDraft(t, true, "xxxxxxxx ", map[string]string{"a.md": manyTriggers(12)})
	typeKeys(a, "t")
	feed(a, key)
	_, _, inner := popupBox(t, a)
	if len(inner) != 11 || inner[10] != "1 of 12" {
		t.Fatalf("12 candidates give %d rows %v, want 10 rows and the count row", len(inner), inner)
	}
	for i := 0; i < 10; i++ {
		feed(a, key)
	}
	_, _, inner = popupBox(t, a)
	found := false
	for _, row := range inner {
		found = found || strings.HasPrefix(row, "t11")
	}
	if !found || inner[len(inner)-1] != "11 of 12" {
		t.Errorf("the 11th candidate is not in view: %v", inner)
	}
}

func TestSnippetPopupRenderShowsTheTriggerNotTheBody(t *testing.T) {
	trueColor(t)
	a, _, key := triggerAppDraft(t, false, "xxxxxxxx ", map[string]string{
		"a.md": "## meet\n\n```\nBODYTEXT\n```\n\n## meet1\n\n```\nOTHERBODY\n```\n",
	})
	typeKeys(a, "me")
	feed(a, key)
	view := ansi.Strip(a.View())
	if !strings.Contains(view, "xxxxxxxx meet") {
		t.Errorf("the buffer does not show the trigger:\n%s", view)
	}
	if strings.Contains(view, "BODYTEXT") || strings.Contains(view, "OTHERBODY") {
		t.Errorf("the screen shows a body:\n%s", view)
	}
}

func TestSnippetPopupRenderSharesPlaceWithTagMenu(t *testing.T) {
	trueColor(t)
	const draft = "xxxxxxxx "
	pa, _, key := triggerAppDraft(t, true, draft, map[string]string{
		"a.md": "## prox\n\n```\nX\n```\n\n## proy\n\n```\nY\n```\n",
	})
	typeKeys(pa, "pro")
	feed(pa, key)
	popupTop, _, _ := popupBox(t, pa)

	ta, _, _ := triggerAppDraft(t, true, draft, nil)
	typeKeys(ta, "pro")
	feed(ta, vim.Ctrl('x'), vim.Ctrl('o'))
	m := ta.liveTagMenu()
	if m == nil {
		t.Fatal("the tag menu did not open")
	}
	ta.View()
	x, _, _ := ta.anchorAt(ta.activeBuffer(), m.tok.line, m.tok.start)
	menuTop, _, _ := boxAt(t, ta, x-2)
	if popupTop != menuTop {
		t.Errorf("the popup starts in row %d and the tag menu in row %d", popupTop, menuTop)
	}
	// Both boxes use the same border cells.
	for _, a := range []*App{pa, ta} {
		rows := screenRows(a)
		if rows[popupTop][x-2] != '╭' || rows[popupTop+1][x-2] != '│' {
			t.Errorf("the box in row %d has another border", popupTop)
		}
	}
}

func TestSnippetPopupRenderGoesAboveTheLineNearTheBottom(t *testing.T) {
	trueColor(t)
	draft := strings.Repeat("line\n", 40) + "xxxxxxxx "
	a, _, key := triggerAppDraft(t, true, draft, map[string]string{
		"a.md": "## prox\n\n```\nX\n```\n\n## proy\n\n```\nY\n```\n",
	})
	typeKeys(a, "pro")
	feed(a, key)
	p := a.liveSnippetPopup()
	a.View()
	_, y, ok := a.anchorAt(p.buf, p.at.Line, p.start)
	if !ok {
		t.Fatal("the popup has no anchor on screen")
	}
	_, bottom, _ := popupBox(t, a)
	if bottom != y-1 {
		t.Errorf("the box ends in row %d, want the row above the line (row %d)", bottom, y-1)
	}
}
