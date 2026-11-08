package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"

	"github.com/ZenNotes/tui/internal/snippets"
	"github.com/ZenNotes/tui/internal/vim"
)

// snippetPreview displays matching snippet previews above the cursor.
type snippetPreview struct {
	matches []snippetMatch
	sel     int // selected index into matches
}

type snippetMatch struct {
	trigger string
	snippet snippets.Snippet
}

// newSnippetPreview finds snippet triggers matching the word at the cursor.
func newSnippetPreview(lib snippets.Library, wordStart, wordEnd string) *snippetPreview {
	prefix := strings.ToLower(wordStart + wordEnd)
	if prefix == "" {
		return nil
	}

	var matches []snippetMatch
	for _, t := range lib.Triggers() {
		if strings.HasPrefix(strings.ToLower(t), prefix) {
			sn, ok := lib.Lookup(t)
			if ok {
				matches = append(matches, snippetMatch{trigger: t, snippet: sn})
			}
		}
	}
	if len(matches) == 0 {
		return nil
	}
	return &snippetPreview{matches: matches, sel: 0}
}

// active reports whether there are any matches.
func (p *snippetPreview) active() bool { return p != nil && len(p.matches) > 0 }

// currentTrigger returns the selected trigger.
func (p *snippetPreview) currentTrigger() string {
	if !p.active() || p.sel >= len(p.matches) {
		return ""
	}
	return p.matches[p.sel].trigger
}

// moveDown selects the next match, cycling if needed.
func (p *snippetPreview) moveDown() {
	if !p.active() {
		return
	}
	p.sel = (p.sel + 1) % len(p.matches)
}

// moveUp selects the previous match, cycling if needed.
func (p *snippetPreview) moveUp() {
	if !p.active() {
		return
	}
	p.sel = (p.sel - 1 + len(p.matches)) % len(p.matches)
}

// render draws the preview popup. x, y is the cursor position in screen coordinates.
func (p *snippetPreview) render(x, y int, width int, st Theme) string {
	if !p.active() {
		return ""
	}
	m := p.matches[p.sel]
	preview := p.previewBody(m.snippet.Body, width-4)
	desc := m.snippet.Description
	if desc == "" {
		desc = m.trigger
	}
	lines := []string{
		st.accent.Render(m.trigger) + " — " + desc,
		"",
	}
	for _, line := range strings.Split(preview, "\n") {
		if utf8.RuneCountInString(line) > width-4 {
			line = line[:width-4]
		}
		lines = append(lines, "  "+line)
	}
	content := strings.Join(lines, "\n")
	border := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	return border.Render(content)
}

// previewBody returns a shortened preview of the snippet body (first few lines).
func (p *snippetPreview) previewBody(body string, maxWidth int) string {
	lines := strings.Split(body, "\n")
	if len(lines) > 3 {
		lines = lines[:3]
	}
	preview := strings.Join(lines, "\n")
	if utf8.RuneCountInString(preview) > maxWidth {
		runes := []rune(preview)
		if maxWidth > 3 {
			return string(runes[:maxWidth-3]) + "…"
		}
		return "…"
	}
	return preview
}

// currentSnippet returns the selected snippet.
func (p *snippetPreview) currentSnippet() snippets.Snippet {
	if !p.active() || p.sel >= len(p.matches) {
		return snippets.Snippet{}
	}
	return p.matches[p.sel].snippet
}

// count returns the number of matches.
func (p *snippetPreview) count() int {
	if p == nil {
		return 0
	}
	return len(p.matches)
}

// hint returns a help string.
func (p *snippetPreview) hint() string {
	if !p.active() {
		return ""
	}
	return fmt.Sprintf("↑↓ to select (%d/%d) | Enter to expand | Esc to dismiss", p.sel+1, len(p.matches))
}
