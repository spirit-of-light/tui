package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/cellbuf"
	"github.com/muesli/termenv"
)

// styledCell is one drawn character with the style that paints it.
type styledCell struct {
	r     rune
	style string
}

// styledCells decodes a drawn row into its cells.
func styledCells(row string) []styledCell {
	var (
		cells []styledCell
		pen   cellbuf.Style
		state byte
	)
	parser := ansi.NewParser()
	for len(row) > 0 {
		seq, width, n, next := ansi.DecodeSequence(row, state, parser)
		if ansi.HasCsiPrefix(seq) && parser.Command() == 'm' {
			cellbuf.ReadStyle(parser.Params(), &pen)
		}
		if width > 0 {
			cells = append(cells, styledCell{
				r:     []rune(seq)[0],
				style: fmt.Sprintf("%v|%v|%v", pen.Fg, pen.Bg, pen.Attrs),
			})
		}
		row, state = row[n:], next
	}
	return cells
}

// cellStyles gives the style of each character of word in the row, which
// must hold word once.
func cellStyles(t *testing.T, row, word string) []string {
	t.Helper()
	cells := styledCells(row)
	text := make([]rune, len(cells))
	for i, c := range cells {
		text[i] = c.r
	}
	at := strings.Index(string(text), word)
	if at < 0 {
		t.Fatalf("row does not show %q: %q", word, string(text))
	}
	start := len([]rune(string(text)[:at]))
	out := make([]string, 0, len(word))
	for i := range []rune(word) {
		out = append(out, cells[start+i].style)
	}
	return out
}

func snippetRenderApp(t *testing.T, width int, body string) (*App, *noteBuffer) {
	t.Helper()
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(profile) })
	snippetFixNow(t, time.Date(2026, 3, 5, 14, 7, 0, 0, time.UTC))
	a, buf := newTagTestApp(t, vimPrefs(), "")
	a.Update(tea.WindowSizeMsg{Width: width, Height: 30})
	a.expandSnippet(buf, body)
	if !buf.ed.SnippetActive() {
		t.Fatalf("body %q started no session", body)
	}
	return a, buf
}

func editorRow(a *App, buf *noteBuffer) string {
	return strings.Split(a.renderEditor(buf, 60, 6, true), "\n")[0]
}

// highlightOf tells whether the word is drawn with a style that the text
// before it does not have.
func highlightOf(t *testing.T, row, word string) bool {
	t.Helper()
	plain := cellStyles(t, row, "x")[0]
	styles := cellStyles(t, row, word)
	// The cursor can cover the first cell, so the check reads the others.
	for _, s := range styles[1:] {
		if s == plain {
			return false
		}
	}
	return true
}

func TestSnippetRenderPlaceholderHighlight(t *testing.T) {
	a, buf := snippetRenderApp(t, 100, "x ${1:name} ${2:other} y")
	row := editorRow(a, buf)
	if !highlightOf(t, row, "name") {
		t.Fatalf("the unchanged placeholder has no highlight: %q", row)
	}
	if highlightOf(t, row, "other") {
		t.Fatalf("a placeholder of another tabstop has a highlight: %q", row)
	}
	styles := cellStyles(t, row, " y")
	if plain := cellStyles(t, row, "x")[0]; styles[1] != plain {
		t.Fatalf("the text after the placeholder has a highlight: %q", row)
	}
}

func TestSnippetRenderHighlightFollowsTheTabstop(t *testing.T) {
	a, buf := snippetRenderApp(t, 100, "x ${1:name} ${2:other} y")
	buf.ed.SnippetNext()
	row := editorRow(a, buf)
	if highlightOf(t, row, "name") {
		t.Fatalf("the placeholder of tabstop 1 keeps its highlight: %q", row)
	}
	if !highlightOf(t, row, "other") {
		t.Fatalf("the placeholder of tabstop 2 has no highlight: %q", row)
	}
}

func TestSnippetRenderTypedTextHasNoHighlight(t *testing.T) {
	a, buf := snippetRenderApp(t, 100, "x ${1:name} y")
	a.handleKey(keyFromTea(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}}))
	row := editorRow(a, buf)
	if highlightOf(t, row, "z y") {
		t.Fatalf("text that the user typed has a highlight: %q", row)
	}
}

func TestSnippetRenderStatusIndicator(t *testing.T) {
	cases := []struct {
		name string
		body string
		next int
		want string
	}{
		{"second of four", "$1 $2 $3 $4", 1, "SNIPPET 2/4"},
		{"second of two with a final stop", "$1 $2$0", 1, "SNIPPET 2/2"},
		{"mirrored final stop", "$1 $0 x $0", 1, "SNIPPET end"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, buf := snippetRenderApp(t, 100, c.body)
			for i := 0; i < c.next; i++ {
				buf.ed.SnippetNext()
			}
			if bar := ansi.Strip(a.renderStatusBar()); !strings.Contains(bar, c.want) {
				t.Fatalf("status bar %q does not show %q", bar, c.want)
			}
		})
	}
}

func TestSnippetRenderNestedStatus(t *testing.T) {
	a, buf := snippetRenderApp(t, 100, "$1 $2 $3 $4")
	a.expandSnippet(buf, "$1 $2")
	if bar := ansi.Strip(a.renderStatusBar()); !strings.Contains(bar, "SNIPPET 1/2") {
		t.Fatalf("status bar %q does not show SNIPPET 1/2", bar)
	}
}

func TestSnippetRenderStatusGoneAfterTheEnd(t *testing.T) {
	a, buf := snippetRenderApp(t, 100, "$1 $2")
	if bar := ansi.Strip(a.renderStatusBar()); !strings.Contains(bar, "SNIPPET 1/2") {
		t.Fatalf("status bar %q does not show SNIPPET 1/2", bar)
	}
	buf.ed.EndSnippets()
	if bar := ansi.Strip(a.renderStatusBar()); strings.Contains(bar, "SNIPPET") {
		t.Fatalf("status bar %q shows SNIPPET after the session ended", bar)
	}
}

func TestSnippetRenderStatusFitsNarrowWidth(t *testing.T) {
	a, _ := snippetRenderApp(t, 60, "$1 $2 $3 $4")
	bar := a.renderStatusBar()
	if lipgloss.Height(bar) != 1 || lipgloss.Width(bar) > 60 {
		t.Fatalf("status bar is %dx%d at width 60: %q", lipgloss.Width(bar), lipgloss.Height(bar), bar)
	}
	if !strings.Contains(ansi.Strip(bar), "SNIPPET 1/4") {
		t.Fatalf("status bar %q does not show SNIPPET 1/4", ansi.Strip(bar))
	}
}
