package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/ZenNotes/tui/internal/vim"
)

var altS = vim.Key{Rune: 's', Alt: true}

const (
	expandOverrideToml = "[terminal.keymaps]\n\"snippet.expand\" = \"Alt+S\"\n"
	expandUnboundToml  = "[terminal.keymaps]\n\"snippet.expand\" = \"\"\n"
	expandSnip         = "## snip\n\n```\nS\n```\n"
)

// expandKeyApp gives an App in edit mode with the keys of config.toml and
// the snippet files loaded. With Vim on, the editor is in insert mode.
func expandKeyApp(t *testing.T, vimOn bool, toml string, files map[string]string) (*App, *noteBuffer) {
	t.Helper()
	p := overridePrefs(t, vimOn, toml)
	a, buf := newTagTestApp(t, p, "")
	for name, text := range files {
		writeSnippetFile(t, a, name, text)
	}
	loadSnippets(a)
	if vimOn {
		feed(a, vim.R('i'))
	}
	return a, buf
}

func TestSnippetExpandOverrideKeyExpands(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		a, buf := expandKeyApp(t, vimOn, expandOverrideToml, map[string]string{"a.md": expandSnip})
		typeKeys(a, "snip")
		feed(a, altS)
		if got := buf.ed.Text(); got != "S" {
			t.Fatalf("text %q, want S", got)
		}
	})
}

// expandAfterKey types a buffer word, a new line and a prefix, presses the
// key, and gives the text and the cursor.
func expandAfterKey(t *testing.T, vimOn bool, toml string, files map[string]string, key vim.Key) (string, vim.Pos) {
	t.Helper()
	a, buf := expandKeyApp(t, vimOn, toml, files)
	typeKeys(a, "snippets")
	feed(a, vim.KeyEnter)
	typeKeys(a, "snip")
	feed(a, key)
	return buf.ed.Text(), buf.ed.Cursor()
}

func TestSnippetExpandOverrideFreesTheDefaultKey(t *testing.T) {
	cases := []struct {
		name string
		vim  bool
		key  vim.Key
		toml string
	}{
		{"vim on Ctrl+N with Alt+S", true, vim.Ctrl('n'), expandOverrideToml},
		{"vim off Tab with Alt+S", false, vim.KeyTab, expandOverrideToml},
		{"vim on Ctrl+N unbound", true, vim.Ctrl('n'), expandUnboundToml},
		{"vim off Tab unbound", false, vim.KeyTab, expandUnboundToml},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wantText, wantCur := expandAfterKey(t, c.vim, c.toml, nil, c.key)
			gotText, gotCur := expandAfterKey(t, c.vim, c.toml, map[string]string{"a.md": expandSnip}, c.key)
			if gotText != wantText || gotCur != wantCur {
				t.Fatalf("got %q %v, baseline %q %v", gotText, gotCur, wantText, wantCur)
			}
		})
	}
}

func TestSnippetExpandOverridePopupKeys(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		a, buf := expandKeyApp(t, vimOn, expandOverrideToml, map[string]string{"a.md": popupFiles})
		typeKeys(a, "me")
		feed(a, altS)
		if a.liveSnippetPopup() == nil || buf.ed.Text() != "meet" {
			t.Fatalf("popup %v, text %q", a.liveSnippetPopup() != nil, buf.ed.Text())
		}
		steps := []struct {
			key  vim.Key
			want string
		}{
			{vim.Ctrl('n'), "meet1"},
			{vim.KeyShiftTab, "meet"},
			{altS, "meet1"},
			{vim.Ctrl('p'), "meet"},
		}
		for i, s := range steps {
			feed(a, s.key)
			if got := buf.ed.Text(); got != s.want {
				t.Fatalf("step %d: text %q, want %q", i, got, s.want)
			}
			if a.liveSnippetPopup() == nil {
				t.Fatalf("step %d: popup closed", i)
			}
		}
	})
}

func TestSnippetExpandOverrideKeymapList(t *testing.T) {
	a, _ := expandKeyApp(t, true, "", nil)
	it := terminalItem(t, keymapPalette(t, a), "snippet.expand")
	if it.hint != "Ctrl+N (Vim), Tab (no Vim)" {
		t.Fatalf("hint %q", it.hint)
	}
	if it.section != "Terminal" || !strings.Contains(it.detail, "zn only") {
		t.Fatalf("section %q, detail %q", it.section, it.detail)
	}
	a, _ = expandKeyApp(t, true, expandOverrideToml, nil)
	if got := terminalItem(t, keymapPalette(t, a), "snippet.expand").hint; got != "Alt+S" {
		t.Fatalf("override hint %q", got)
	}
}

func TestSnippetExpandOverrideBind(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		toml := ""
		if !vimOn {
			toml = "[vim]\nenabled = false\n"
		}
		p := overridePrefs(t, vimOn, toml)
		dir := os.Getenv("ZENNOTES_CONFIG_DIR")
		a, buf := newTagTestApp(t, p, "")
		// The test app points the config dir at its own temporary dir, so
		// point it back at the dir that holds the file under test.
		t.Setenv("ZENNOTES_CONFIG_DIR", dir)
		a.prefsLoaded = true
		writeSnippetFile(t, a, "a.md", expandSnip)
		loadSnippets(a)
		if vimOn {
			feed(a, vim.R('i'))
		}
		a.runCommandLine("bind snippet.expand Alt+S")
		if got := bindConfig(t); !strings.Contains(got, "[terminal.keymaps]") || !strings.Contains(got, "Alt+S") {
			t.Fatalf("config.toml has no terminal table:\n%s", got)
		}
		typeKeys(a, "snip")
		feed(a, altS)
		if got := buf.ed.Text(); got != "S" {
			t.Fatalf("text %q, want S", got)
		}
	})
}
