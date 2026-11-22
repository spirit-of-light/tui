package tui

import (
	"strings"
	"testing"

	"github.com/ZenNotes/tui/internal/vim"
)

// keymapPalette runs :keymaps and returns the open palette.
func keymapPalette(t *testing.T, a *App) *palette {
	t.Helper()
	a.runCommandLine("keymaps")
	p, ok := a.overlay.(*palette)
	if !ok {
		t.Fatalf("overlay after :keymaps: %T", a.overlay)
	}
	return p
}

func terminalItem(t *testing.T, p *palette, id string) paletteItem {
	t.Helper()
	for _, it := range p.items {
		if it.id == id {
			return it
		}
	}
	t.Fatalf("no item for %s", id)
	return paletteItem{}
}

func TestKeymapListTerminalDefaults(t *testing.T) {
	a, _ := snippetKeyApp(t, vimPrefs(), "", "$1 $2")
	p := keymapPalette(t, a)
	next := terminalItem(t, p, "snippet.nextTabstop")
	prev := terminalItem(t, p, "snippet.prevTabstop")
	if next.hint != "Ctrl+L, Tab" {
		t.Fatalf("next hint: %q", next.hint)
	}
	if prev.hint != "Ctrl+H, Shift+Tab" {
		t.Fatalf("previous hint: %q", prev.hint)
	}
}

func TestKeymapListTerminalSection(t *testing.T) {
	a, _ := snippetKeyApp(t, vimPrefs(), "", "$1 $2")
	p := keymapPalette(t, a)
	terminal := 0
	for _, it := range p.items {
		isTerminal := it.id == "snippet.nextTabstop" || it.id == "snippet.prevTabstop"
		if isTerminal {
			terminal++
			if it.section != "Terminal" || !strings.Contains(it.detail, "zn only") {
				t.Fatalf("item %s: section %q, detail %q", it.id, it.section, it.detail)
			}
		} else if it.section == "Terminal" {
			t.Fatalf("catalog id %s is in the Terminal section", it.id)
		}
	}
	if terminal != 2 {
		t.Fatalf("%d terminal items, want 2", terminal)
	}
	for _, ch := range "tabstop" {
		p.handleKey(a, vim.Key{Rune: ch})
	}
	out := stripAnsi(p.render(a, 120, 40))
	header := strings.Index(out, "Terminal")
	if header < 0 {
		t.Fatalf("no Terminal header in the overlay:\n%s", out)
	}
	if !strings.Contains(out[header:], "snippet.nextTabstop") || !strings.Contains(out[header:], "zn only") {
		t.Fatalf("terminal rows are not under the header:\n%s", out)
	}
	for _, it := range p.items {
		if it.section == "" && strings.Contains(out[header:], it.id) {
			t.Fatalf("catalog id %s is under the Terminal header:\n%s", it.id, out)
		}
	}
}

func TestKeymapListTerminalOverride(t *testing.T) {
	p := overridePrefs(t, true, "[terminal.keymaps]\n\"snippet.nextTabstop\" = \"Alt+N\"\n\"snippet.prevTabstop\" = \"\"\n")
	a, _ := snippetKeyApp(t, p, "", "$1 $2")
	pal := keymapPalette(t, a)
	if got := terminalItem(t, pal, "snippet.nextTabstop").hint; got != "Alt+N" {
		t.Fatalf("next hint: %q", got)
	}
	if got := terminalItem(t, pal, "snippet.prevTabstop").hint; got != "(unbound)" {
		t.Fatalf("previous hint: %q", got)
	}
}

func TestKeymapListTerminalBindPrompt(t *testing.T) {
	a, _ := bindApp(t, true, "")
	p := keymapPalette(t, a)
	for _, ch := range "snippet.nextTab" {
		p.handleKey(a, vim.Key{Rune: ch})
	}
	p.handleKey(a, vim.Key{Name: "enter"})
	pr, ok := a.overlay.(*prompt)
	if !ok {
		t.Fatalf("overlay after Enter: %T", a.overlay)
	}
	if got := pr.input.String(); got != "Ctrl+L, Tab" {
		t.Fatalf("prompt text: %q", got)
	}
	pr.input = newTextInput("Alt+N, Alt+M")
	pr.handleKey(a, vim.Key{Name: "enter"})
	if got := a.terminalKeys.Bindings("snippet.nextTabstop"); len(got) != 2 || got[0] != "Alt+N" || got[1] != "Alt+M" {
		t.Fatalf("keys after the prompt: %v", got)
	}
}

func TestKeymapListTerminalCompletion(t *testing.T) {
	a, _ := snippetKeyApp(t, vimPrefs(), "", "$1 $2")
	for _, cmd := range []string{"bind", "unbind"} {
		_, cands := a.completeEx(cmd + " snip")
		got := strings.Join(cands, " ")
		if !strings.Contains(got, "snippet.nextTabstop") || !strings.Contains(got, "snippet.prevTabstop") {
			t.Fatalf(":%s snip candidates: %v", cmd, cands)
		}
	}
}
