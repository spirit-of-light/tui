package vim

import (
	"fmt"
	"testing"
)

// snippetEditor builds an editor that has text, with the cursor at cur and
// in insert mode. With Vim on, keys moves from normal mode to insert mode.
func snippetEditor(t *testing.T, vimOn bool, text string, cur Pos, keys string) *Editor {
	t.Helper()
	opts := DefaultOptions()
	opts.VimEnabled = vimOn
	e := New(text, opts, Hooks{})
	e.SetViewport(10)
	e.SetCursor(cur)
	if vimOn {
		e.Feed(keys)
	}
	if e.Mode() != ModeInsert {
		t.Fatalf("setup: mode %v, want insert", e.Mode())
	}
	return e
}

func fakeVars(name, arg string, hasArg bool) (string, bool) {
	if name == "title" {
		return "Plan", true
	}
	return "", false
}

func eachVimMode(t *testing.T, f func(t *testing.T, vimOn bool)) {
	t.Helper()
	for _, vimOn := range []bool{true, false} {
		t.Run(fmt.Sprintf("vim=%v", vimOn), func(t *testing.T) { f(t, vimOn) })
	}
}

func TestEditorSnippetExpand(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		text    string
		cursor  Pos
		session bool
	}{
		{"bare tabstop", "a$1b", "ab", Pos{0, 1}, true},
		{"braced tabstop", "a${1}b", "ab", Pos{0, 1}, true},
		{"placeholder", "${1:name}", "name", Pos{0, 0}, true},
		{"two digit tabstop", "$10x", "x", Pos{0, 0}, true},
		{"nested placeholder", "${1:a ${2:b} c}", "a b c", Pos{0, 0}, true},
		{"mirror", "${1:x} = $1", "x = x", Pos{0, 0}, true},
		{"final only", "a$0b", "ab", Pos{0, 1}, false},
		{"empty placeholder", "a${1:}b", "ab", Pos{0, 1}, true},
		{"leading zero", "a${01}b", "ab", Pos{0, 1}, true},
		{"plain text", "abc", "abc", Pos{0, 3}, false},
		{"order by number", "$3 $1 $2", "  ", Pos{0, 1}, true},
		{"two finals", "$0 x $0", " x ", Pos{0, 0}, true},
		{"variable", "# ${title}", "# Plan", Pos{0, 6}, false},
	}
	for _, c := range cases {
		eachVimMode(t, func(t *testing.T, vimOn bool) {
			t.Run(c.name, func(t *testing.T) {
				e := snippetEditor(t, vimOn, "", Pos{}, "i")
				e.ExpandSnippet(c.body, fakeVars)
				if got := e.Text(); got != c.text {
					t.Fatalf("text: got %q want %q", got, c.text)
				}
				if e.Cursor() != c.cursor {
					t.Fatalf("cursor: got %+v want %+v", e.Cursor(), c.cursor)
				}
				if e.SnippetActive() != c.session {
					t.Fatalf("session: got %v want %v", e.SnippetActive(), c.session)
				}
				if e.Mode() != ModeInsert {
					t.Fatalf("mode: got %v want insert", e.Mode())
				}
			})
		})
	}
}

func TestEditorSnippetExpandInText(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := snippetEditor(t, vimOn, "foobar", Pos{0, 3}, "i")
		e.ExpandSnippet("-$1-", fakeVars)
		if got := e.Text(); got != "foo--bar" {
			t.Fatalf("text: got %q", got)
		}
		if e.Cursor() != (Pos{0, 4}) {
			t.Fatalf("cursor: got %+v", e.Cursor())
		}
		if !e.SnippetActive() {
			t.Fatal("no session")
		}
	})
}

func TestEditorSnippetExpandFromNormalMode(t *testing.T) {
	opts := DefaultOptions()
	e := New("foobar", opts, Hooks{})
	e.SetCursor(Pos{0, 3})
	if e.Mode() != ModeNormal {
		t.Fatalf("setup: mode %v", e.Mode())
	}
	e.ExpandSnippet("-$1-", fakeVars)
	if e.Mode() != ModeInsert {
		t.Fatalf("mode: got %v want insert", e.Mode())
	}
	if got := e.Text(); got != "foo--bar" {
		t.Fatalf("text: got %q", got)
	}
	e.Feed("<Esc>u")
	if got := e.Text(); got != "foobar" {
		t.Fatalf("undo: got %q", got)
	}
}

func TestEditorSnippetExpandIndent(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := snippetEditor(t, vimOn, "  - item", Pos{0, 8}, "A")
		e.ExpandSnippet("a\nb\nc", fakeVars)
		if got, want := e.Text(), "  - itema\n  b\n  c"; got != want {
			t.Fatalf("text: got %q want %q", got, want)
		}
		if e.SnippetActive() {
			t.Fatal("session for a body with no stop")
		}
		if e.Cursor() != (Pos{2, 3}) {
			t.Fatalf("cursor: got %+v", e.Cursor())
		}
	})
}

func TestEditorSnippetExpandTabIsIndentLevel(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := snippetEditor(t, vimOn, "", Pos{}, "i")
		e.ExpandSnippet("x\n\ty", fakeVars)
		if got, want := e.Text(), "x\n    y"; got != want {
			t.Fatalf("text: got %q want %q", got, want)
		}
	})
}

func TestEditorSnippetExpandSecondLineStop(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := snippetEditor(t, vimOn, "  x", Pos{0, 3}, "A")
		e.ExpandSnippet("a\n${1:bc}", fakeVars)
		if got, want := e.Text(), "  xa\n  bc"; got != want {
			t.Fatalf("text: got %q want %q", got, want)
		}
		if e.Cursor() != (Pos{1, 2}) {
			t.Fatalf("cursor: got %+v", e.Cursor())
		}
		if got := e.SnippetPlaceholder(1); len(got) != 1 || got[0] != [2]int{2, 4} {
			t.Fatalf("placeholder: got %v", got)
		}
	})
}

func TestEditorSnippetExpandPlaceholderAndStatus(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := snippetEditor(t, vimOn, "", Pos{}, "i")
		e.ExpandSnippet("${1:name}", fakeVars)
		got := e.SnippetPlaceholder(0)
		if len(got) != 1 || got[0] != [2]int{0, 4} {
			t.Fatalf("placeholder: got %v", got)
		}
		if got := e.SnippetPlaceholder(1); len(got) != 0 {
			t.Fatalf("placeholder on another line: got %v", got)
		}

		e = snippetEditor(t, vimOn, "", Pos{}, "i")
		e.ExpandSnippet("$1 $2 $3 $4", fakeVars)
		cur, total, atEnd, ok := e.SnippetStatus()
		if !ok || cur != 1 || total != 4 || atEnd {
			t.Fatalf("status: got %d/%d end=%v ok=%v", cur, total, atEnd, ok)
		}

		e = snippetEditor(t, vimOn, "", Pos{}, "i")
		e.ExpandSnippet("abc", fakeVars)
		if _, _, _, ok := e.SnippetStatus(); ok {
			t.Fatal("status with no session")
		}
	})
}

func TestEditorSnippetExpandMirrorCursor(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := snippetEditor(t, vimOn, "", Pos{}, "i")
		e.ExpandSnippet("${1:x} = $1", fakeVars)
		if e.Cursor() != (Pos{0, 0}) {
			t.Fatalf("cursor: got %+v", e.Cursor())
		}
		if got := e.Text(); got != "x = x" {
			t.Fatalf("text: got %q", got)
		}
	})
}
