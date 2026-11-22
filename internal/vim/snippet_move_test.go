package vim

import (
	"fmt"
	"testing"
)

func TestEditorSnippetMove(t *testing.T) {
	status := func(e *Editor) string {
		c, n, end, ok := e.SnippetStatus()
		return fmt.Sprintf("%d/%d end=%v ok=%v", c, n, end, ok)
	}
	expand := func(t *testing.T, vimOn bool, text string, cur Pos, body string) *Editor {
		e := snippetEditor(t, vimOn, text, cur, "i")
		e.ExpandSnippet(body, fakeVars)
		return e
	}

	t.Run("next goes by number", func(t *testing.T) {
		eachVimMode(t, func(t *testing.T, vimOn bool) {
			e := expand(t, vimOn, "", Pos{}, "$3 $1 $2")
			if e.Cursor() != (Pos{0, 1}) {
				t.Fatalf("start: %+v", e.Cursor())
			}
			e.SnippetNext()
			if e.Cursor() != (Pos{0, 2}) {
				t.Fatalf("stop 2: %+v", e.Cursor())
			}
			e.SnippetNext()
			if e.Cursor() != (Pos{0, 0}) {
				t.Fatalf("stop 3: %+v", e.Cursor())
			}
			if !e.SnippetActive() {
				t.Fatal("session ended at stop 3")
			}
		})
	})

	t.Run("prev goes back", func(t *testing.T) {
		eachVimMode(t, func(t *testing.T, vimOn bool) {
			e := expand(t, vimOn, "", Pos{}, "$3 $1 $2")
			e.SnippetNext()
			e.SnippetPrev()
			if e.Cursor() != (Pos{0, 1}) {
				t.Fatalf("stop 1: %+v", e.Cursor())
			}
		})
	})

	t.Run("prev at first stop does nothing", func(t *testing.T) {
		eachVimMode(t, func(t *testing.T, vimOn bool) {
			e := expand(t, vimOn, "", Pos{}, "$3 $1 $2")
			e.SnippetPrev()
			if e.Cursor() != (Pos{0, 1}) || e.Text() != "  " || !e.SnippetActive() {
				t.Fatalf("changed: %+v %q %v", e.Cursor(), e.Text(), e.SnippetActive())
			}
		})
	})

	t.Run("next to a single final stop ends the session", func(t *testing.T) {
		eachVimMode(t, func(t *testing.T, vimOn bool) {
			e := expand(t, vimOn, "", Pos{}, "$1 x$0y")
			e.SnippetNext()
			if e.Cursor() != (Pos{0, 2}) || e.Text() != " xy" {
				t.Fatalf("got %+v %q", e.Cursor(), e.Text())
			}
			if e.SnippetActive() {
				t.Fatal("session still active")
			}
		})
	})

	t.Run("next to the implicit end ends the session", func(t *testing.T) {
		eachVimMode(t, func(t *testing.T, vimOn bool) {
			e := expand(t, vimOn, "", Pos{}, "$1 $2 end")
			e.SnippetNext()
			e.SnippetNext()
			if e.Cursor() != (Pos{0, 5}) || e.SnippetActive() {
				t.Fatalf("got %+v active=%v", e.Cursor(), e.SnippetActive())
			}
		})
	})

	t.Run("final stop with copies keeps the session until the next key", func(t *testing.T) {
		eachVimMode(t, func(t *testing.T, vimOn bool) {
			e := expand(t, vimOn, "", Pos{}, "$1 $0 x $0")
			e.SnippetNext()
			if e.Cursor() != (Pos{0, 1}) || !e.SnippetActive() {
				t.Fatalf("got %+v active=%v", e.Cursor(), e.SnippetActive())
			}
			if _, _, atEnd, _ := e.SnippetStatus(); !atEnd {
				t.Fatal("status is not at the end")
			}
			before := e.Text()
			e.SnippetNext()
			if e.SnippetActive() || e.Cursor() != (Pos{0, 1}) || e.Text() != before {
				t.Fatalf("got %+v %q active=%v", e.Cursor(), e.Text(), e.SnippetActive())
			}
		})
	})

	t.Run("next at a final pair on an indented line", func(t *testing.T) {
		eachVimMode(t, func(t *testing.T, vimOn bool) {
			e := expand(t, vimOn, "  ", Pos{0, 2}, "$0 x $0")
			cur, text := e.Cursor(), e.Text()
			e.SnippetNext()
			if e.SnippetActive() || e.Cursor() != cur || e.Text() != text {
				t.Fatalf("got %+v %q active=%v", e.Cursor(), e.Text(), e.SnippetActive())
			}
		})
	})

	t.Run("status", func(t *testing.T) {
		eachVimMode(t, func(t *testing.T, vimOn bool) {
			e := expand(t, vimOn, "", Pos{}, "$1 $2 $3 $4")
			e.SnippetNext()
			if got := status(e); got != "2/4 end=false ok=true" {
				t.Fatalf("got %s", got)
			}
			e = expand(t, vimOn, "", Pos{}, "$1 $2$0")
			e.SnippetNext()
			if got := status(e); got != "2/2 end=false ok=true" {
				t.Fatalf("got %s", got)
			}
			e = expand(t, vimOn, "", Pos{}, "$1 $0 x $0")
			e.SnippetNext()
			if got := status(e); got != "1/1 end=true ok=true" {
				t.Fatalf("got %s", got)
			}
		})
	})

	t.Run("no session does nothing", func(t *testing.T) {
		e := snippetEditor(t, true, "abc", Pos{0, 1}, "i")
		e.SnippetNext()
		e.SnippetPrev()
		if e.Cursor() != (Pos{0, 1}) || e.Text() != "abc" {
			t.Fatalf("changed: %+v %q", e.Cursor(), e.Text())
		}
	})
}
