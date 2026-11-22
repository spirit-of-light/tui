package tui

import (
	"reflect"
	"testing"

	"github.com/ZenNotes/tui/internal/vim"
)

const reloadTwoFiles = "## meet\n\n```\nM\n```\n\n## meet1\n\n```\nM1\n```\n"

const reloadThreeFiles = reloadTwoFiles + "\n## meet2\n\n```\nM2\n```\n"

func TestSnippetReloadKeepsTheOpenPopupList(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		a, buf, key := triggerApp(t, vimOn, map[string]string{"a.md": reloadTwoFiles})
		typeKeys(a, "mee")
		feed(a, key)
		if got, want := popupRows(a), []string{"meet", "meet1"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("rows %v, want %v", got, want)
		}
		p := writeSnippetFile(t, a, "a.md", reloadThreeFiles)
		runCmd(a, a.handleVaultChange([]string{p}))
		if got, want := popupRows(a), []string{"meet", "meet1"}; !reflect.DeepEqual(got, want) {
			t.Errorf("rows after the reload %v, want %v", got, want)
		}
		if got := buf.ed.Text(); got != "meet" {
			t.Errorf("text %q, want meet", got)
		}
	})
}

func TestSnippetReloadNextTriggerKeyUsesTheNewIndex(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		a, buf, key := triggerApp(t, vimOn, map[string]string{"a.md": reloadTwoFiles})
		typeKeys(a, "mee")
		feed(a, key)
		p := writeSnippetFile(t, a, "a.md", reloadThreeFiles)
		runCmd(a, a.handleVaultChange([]string{p}))
		feed(a, vim.Ctrl('e'))
		if a.liveSnippetPopup() != nil || buf.ed.Text() != "mee" {
			t.Fatalf("popup %v, text %q, want no popup and mee", a.liveSnippetPopup() != nil, buf.ed.Text())
		}
		feed(a, key)
		if got, want := popupRows(a), []string{"meet", "meet1", "meet2"}; !reflect.DeepEqual(got, want) {
			t.Errorf("rows %v, want %v", got, want)
		}
	})
}

func TestSnippetReloadKeepsTheSession(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		a, buf, key := triggerApp(t, vimOn, map[string]string{"a.md": "## box\n\n```\n${1:a} ${2:b}\n```\n"})
		typeKeys(a, "box")
		feed(a, key)
		if got := sessionStop(buf); got != 1 {
			t.Fatalf("stop %d, want 1", got)
		}
		cur, text := buf.ed.Cursor(), buf.ed.Text()
		p := writeSnippetFile(t, a, "a.md", "## box\n\n```\nX\n```\n")
		runCmd(a, a.handleVaultChange([]string{p}))
		if got := sessionStop(buf); got != 1 {
			t.Errorf("stop %d after the reload, want 1", got)
		}
		if got := buf.ed.Cursor(); got != cur {
			t.Errorf("cursor %v after the reload, want %v", got, cur)
		}
		if got := buf.ed.Text(); got != text {
			t.Errorf("text %q after the reload, want %q", got, text)
		}
	})
}
