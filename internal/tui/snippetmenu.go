package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/ZenNotes/tui/internal/snippets"
	"github.com/ZenNotes/tui/internal/templates"
	"github.com/ZenNotes/tui/internal/vim"
)

// The snippet menu opens under the cursor while a trigger word is being typed,
// as mini.completion does in Neovim. It lists the triggers that start with the
// word, and previews the selected snippet's expansion. Typing narrows it. Only
// the menu's own keys are taken from the editor: next, previous, expand the
// selected item, and dismiss.

const (
	snippetMenuRows    = 6 // triggers shown at once
	snippetPreviewRows = 5 // lines of expansion shown under the list
	snippetMenuWidth   = 52
)

type snippetMenu struct {
	path   string
	line   int
	start  int // column of the trigger word's first rune
	query  string
	title  string // the note's title, for the preview's {{title}}
	items  []snippets.Snippet
	cursor int
	scroll int
	// at is where the cursor stood when the menu was last refreshed. A cursor
	// that moved by other means makes the menu stale.
	at vim.Pos
}

// snippetMenuDismissal keeps a dismissed word from reopening the menu until the
// cursor leaves it.
type snippetMenuDismissal struct {
	path        string
	line, start int
}

// snippetMenuItems lists the triggers that start with word. The exact trigger,
// if there is one, comes first, so Ctrl+N on a complete trigger expands it.
func snippetMenuItems(lib snippets.Library, word string) []snippets.Snippet {
	var items []snippets.Snippet
	for _, t := range lib.Triggers() {
		if strings.HasPrefix(t, word) {
			s, _ := lib.Lookup(t)
			items = append(items, s)
		}
	}
	for i := range items {
		if items[i].Trigger == word {
			items[0], items[i] = items[i], items[0]
			break
		}
	}
	return items
}

// refreshSnippetMenu runs after each key the editor took in insert mode: it
// opens the menu when a trigger word is being typed, narrows it as the word
// grows, and closes it when nothing matches or the word is gone.
func (a *App) refreshSnippetMenu(buf *noteBuffer) {
	prev := a.snippetMenu
	a.snippetMenu = nil
	if buf.ed.Mode() != vim.ModeInsert || len(a.snippetLib) == 0 {
		a.snippetDismissed = nil
		return
	}
	cur := buf.ed.Cursor()
	line := buf.ed.LineRunes(cur.Line)
	word, start := snippetWordBefore(line, cur.Col)
	if word == "" {
		a.snippetDismissed = nil
		return
	}
	if start > 0 && line[start-1] == '#' {
		return // a tag is being typed, which has its own menu
	}
	if d := a.snippetDismissed; d != nil && d.path == buf.path && d.line == cur.Line && d.start == start {
		return
	}
	items := snippetMenuItems(a.snippetLib, word)
	if len(items) == 0 {
		return
	}
	title := ""
	if meta, ok := a.noteMeta(buf.path); ok {
		title = meta.Title
	}
	m := &snippetMenu{path: buf.path, line: cur.Line, start: start, query: word, title: title, items: items, at: cur}
	if prev != nil && prev.path == buf.path && prev.line == cur.Line && prev.start == start && prev.query == word && prev.cursor < len(items) {
		m.cursor, m.scroll = prev.cursor, prev.scroll
	}
	a.snippetMenu = m
}

// liveSnippetMenu is the menu, if it still belongs to what is on screen.
func (a *App) liveSnippetMenu() *snippetMenu {
	m := a.snippetMenu
	if m == nil {
		return nil
	}
	buf := a.activeBuffer()
	if buf == nil || a.overlay != nil || a.focus != focusPane || buf.path != m.path ||
		buf.ed.Mode() != vim.ModeInsert || buf.ed.Cursor() != m.at {
		a.snippetMenu = nil
		return nil
	}
	return m
}

// snippetMenuKey takes the keys that drive an open menu. Everything else,
// Enter and Esc included, goes on to the editor, which is how typing narrows
// the list.
func (a *App) snippetMenuKey(buf *noteBuffer, k vim.Key) bool {
	m := a.liveSnippetMenu()
	if m == nil {
		return false
	}
	switch {
	case a.bound(k, "editor.snippetMenuNext") || k.Is("down"):
		m.cursor = (m.cursor + 1) % len(m.items)
	case a.bound(k, "editor.snippetMenuPrev") || k.Is("up"):
		m.cursor = (m.cursor - 1 + len(m.items)) % len(m.items)
	case a.bound(k, "editor.snippetMenuAccept"):
		a.acceptSnippetMenu(buf, m)
	case a.bound(k, "editor.snippetMenuDismiss"):
		a.snippetDismissed = &snippetMenuDismissal{path: m.path, line: m.line, start: m.start}
		a.snippetMenu = nil
	default:
		return false
	}
	return true
}

// acceptSnippetMenu expands the selected item in place of the word.
func (a *App) acceptSnippetMenu(buf *noteBuffer, m *snippetMenu) {
	sn := m.items[m.cursor]
	a.snippetMenu = nil
	a.expandSnippetWord(buf, sn, m.start)
}

// snippetMenuAnchor is the screen cell of the trigger word's first rune.
func (a *App) snippetMenuAnchor(buf *noteBuffer, m *snippetMenu) (x, y int, ok bool) {
	rc := a.contentRect(a.activePane)
	line := buf.ed.LineRunes(m.line)
	for _, row := range buf.rows {
		if row.line != m.line || m.start < row.segStart || m.start > row.segEnd {
			continue
		}
		if m.start == row.segEnd && row.segEnd < len(line) {
			continue // the word starts the next wrapped row
		}
		return rc.x + row.x + cellWidth(string(line[row.segStart:m.start])), rc.y + row.y, true
	}
	return 0, 0, false
}

// paintSnippetMenu draws the menu under the word, or above it near the bottom
// of the screen.
func (a *App) paintSnippetMenu(screen string) string {
	m := a.liveSnippetMenu()
	if m == nil {
		return screen
	}
	x, y, ok := a.snippetMenuAnchor(a.activeBuffer(), m)
	if !ok {
		return screen
	}
	block := m.render(a)
	h := strings.Count(block, "\n") + 1
	x = max(0, min(x-1, a.width-blockWidth(block)))
	top := y + 1
	if top+h > a.height-2 {
		top = max(0, y-h)
	}
	screen, _ = overlayPlaceAt(screen, block, x, top, a.width, a.height)
	return screen
}

// render is the box: the visible triggers with their descriptions, then the
// preview of the selected expansion.
func (m *snippetMenu) render(a *App) string {
	th := a.theme
	w := min(snippetMenuWidth, max(24, a.width-8))
	rows := min(len(m.items), snippetMenuRows)
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+rows {
		m.scroll = m.cursor - rows + 1
	}
	labelW := 0
	for _, it := range m.items {
		labelW = max(labelW, cellWidth(it.Trigger))
	}
	labelW = min(labelW, w/2)
	lines := make([]string, 0, rows+snippetPreviewRows+2)
	for i := m.scroll; i < m.scroll+rows; i++ {
		it := m.items[i]
		label := padRight(truncateCells(it.Trigger, labelW), labelW)
		desc := truncateCells(it.Description, max(0, w-labelW-2))
		if i == m.cursor {
			lines = append(lines, th.SelectedFocus.Render(padRight(label+"  "+desc, w)))
			continue
		}
		lines = append(lines, th.Tag.Render(label)+"  "+th.Muted.Render(desc))
	}
	if len(m.items) > rows {
		lines = append(lines, th.Muted.Render(padRight(snippetCountLabel(m.cursor, len(m.items)), w)))
	}
	lines = append(lines, th.Muted.Render(strings.Repeat("─", w)))
	sel := m.items[m.cursor]
	exp := snippets.Parse(sel.Body, templates.Vars(m.title, time.Now(), "en"))
	preview := strings.Split(strings.TrimRight(exp.Text, "\n"), "\n")
	for i := 0; i < len(preview) && i < snippetPreviewRows; i++ {
		lines = append(lines, th.Base.Render(padRight(truncateCells(preview[i], w), w)))
	}
	if len(preview) > snippetPreviewRows {
		lines = append(lines, th.Muted.Render("…"))
	}
	return th.Overlay.Render(strings.Join(lines, "\n"))
}

func snippetCountLabel(cursor, total int) string {
	return fmt.Sprintf("%d of %d", cursor+1, total)
}

// blockWidth is the width in cells of the widest line of a rendered block.
func blockWidth(block string) int {
	w := 0
	for _, line := range strings.Split(block, "\n") {
		w = max(w, cellWidth(line))
	}
	return w
}
