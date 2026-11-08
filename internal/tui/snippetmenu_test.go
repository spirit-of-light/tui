package tui

import (
	"strings"
	"testing"

	"github.com/ZenNotes/tui/internal/keymaps"
	"github.com/ZenNotes/tui/internal/vim"
)

const menuLibrary = `
[[snippet]]
trigger = "snip"
description = "Markdown link"
body = '''[${1:label}](${2:url})$0'''

[[snippet]]
trigger = "snippet"
description = "Second"
body = "two $0"

[[snippet]]
trigger = "todo"
description = "Checklist"
body = "- [ ] $0"
`

func TestSnippetMenuOpensOnPartialTriggerAndPreviews(t *testing.T) {
	a, buf := snippetTestApp(t, menuLibrary)
	snipTypeKeys(a, "sn")
	m := a.liveSnippetMenu()
	if m == nil || len(m.items) != 2 {
		t.Fatalf("menu %+v", m)
	}
	block := stripAnsi(m.render(a))
	if !strings.Contains(block, "snip") || !strings.Contains(block, "[label](url)") {
		t.Fatalf("menu shows:\n%s", block)
	}
	if strings.Contains(block, "todo") {
		t.Fatal("non-matching trigger listed")
	}
	if buf.ed.Text() != "sn" {
		t.Fatalf("typing changed text to %q", buf.ed.Text())
	}
}

func TestSnippetMenuTabAcceptsSelectedItem(t *testing.T) {
	a, buf := snippetTestApp(t, menuLibrary)
	snipTypeKeys(a, "sn")
	a.handleKey(vim.Ctrl('j')) // next: "snippet"
	a.handleKey(vim.Special("tab"))
	if buf.ed.Text() != "two " {
		t.Fatalf("Tab expanded to %q", buf.ed.Text())
	}
	if a.liveSnippetMenu() != nil {
		t.Fatal("menu still open after accepting")
	}
}

func TestSnippetMenuCtrlNExpandsExactTriggerFirst(t *testing.T) {
	a, buf := snippetTestApp(t, menuLibrary)
	snipTypeKeys(a, "snip")
	if m := a.liveSnippetMenu(); m == nil || m.items[0].Trigger != "snip" {
		t.Fatalf("exact trigger not first: %+v", m)
	}
	a.handleKey(vim.Ctrl('n'))
	if buf.ed.Text() != "[label](url)" {
		t.Fatalf("expanded to %q", buf.ed.Text())
	}
}

func TestSnippetMenuDismissStaysClosedUntilWordChanges(t *testing.T) {
	a, _ := snippetTestApp(t, menuLibrary)
	snipTypeKeys(a, "to")
	a.handleKey(vim.Ctrl('e'))
	if a.liveSnippetMenu() != nil {
		t.Fatal("dismiss did not close the menu")
	}
	snipTypeKeys(a, "d")
	if a.liveSnippetMenu() != nil {
		t.Fatal("dismissed word reopened the menu")
	}
	a.handleKey(vim.Special("backspace"))
	a.handleKey(vim.Special("backspace"))
	a.handleKey(vim.Special("backspace"))
	snipTypeKeys(a, "to")
	if a.liveSnippetMenu() == nil {
		t.Fatal("a new word did not open the menu")
	}
}

func TestSnippetMenuNotOpenForTags(t *testing.T) {
	a, _ := snippetTestApp(t, menuLibrary)
	snipTypeKeys(a, "#to")
	if a.liveSnippetMenu() != nil {
		t.Fatal("snippet menu opened on a tag")
	}
}

func TestSnippetKeysFollowKeymapOverrides(t *testing.T) {
	a, buf := snippetTestApp(t, menuLibrary)
	a.keymap = keymaps.NewResolver(map[string]string{"editor.snippetExpand": "Alt+X"})
	snipTypeKeys(a, "snip")
	a.handleKey(vim.Ctrl('n'))
	if buf.ed.Text() == "[label](url)" {
		t.Fatal("Ctrl+N expanded after the expand key was rebound")
	}
	a.handleKey(vim.Alt('x'))
	if buf.ed.Text() != "[label](url)" {
		t.Fatalf("rebound key did not expand: %q", buf.ed.Text())
	}
}
