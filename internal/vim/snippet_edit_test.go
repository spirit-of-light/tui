package vim

import "testing"

// typeKeys sends each rune as a key.
func typeKeys(e *Editor, s string) {
	for _, r := range s {
		e.HandleKey(R(r))
	}
}

// expandIn expands body in insert mode with the cursor at cur. With Vim on,
// a cursor past the last rune of the line is reached with A.
func expandIn(t *testing.T, vimOn bool, text string, cur Pos, body string) *Editor {
	t.Helper()
	keys := "i"
	if cur.Col > 0 {
		keys = "A"
	}
	e := snippetEditor(t, vimOn, text, cur, keys)
	e.ExpandSnippet(body, fakeVars)
	return e
}

func TestEditorSnippetEditTypeReplacesPlaceholder(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := expandIn(t, vimOn, "", Pos{}, "${1:name}")
		typeKeys(e, "z")
		if got := e.Text(); got != "z" {
			t.Fatalf("text: got %q want %q", got, "z")
		}
		if got := e.SnippetPlaceholder(0); len(got) != 0 {
			t.Fatalf("placeholder still drawn: %v", got)
		}
		if !e.SnippetActive() {
			t.Fatal("session ended")
		}
	})
}

func TestEditorSnippetEditBackspaceDeletesPlaceholder(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		for _, k := range []Key{KeyBackspace, Ctrl('h')} {
			e := expandIn(t, vimOn, "ab ", Pos{0, 3}, "${1:name}")
			e.HandleKey(k)
			if got := e.Text(); got != "ab " {
				t.Fatalf("%v: text: got %q want %q", k, got, "ab ")
			}
			if e.Cursor() != (Pos{0, 3}) {
				t.Fatalf("%v: cursor: got %+v", k, e.Cursor())
			}
			if !e.SnippetActive() {
				t.Fatalf("%v: session ended", k)
			}
			typeKeys(e, "xy")
			if got := e.Text(); got != "ab xy" {
				t.Fatalf("%v: typed: got %q", k, got)
			}
		}
	})
}

func TestEditorSnippetEditBackspaceAfterTyping(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := expandIn(t, vimOn, "", Pos{}, "${1:name} $2")
		typeKeys(e, "ab")
		e.HandleKey(KeyBackspace)
		if got := e.Text(); got != "a " {
			t.Fatalf("text: got %q want %q", got, "a ")
		}
		if e.Cursor() != (Pos{0, 1}) {
			t.Fatalf("cursor: got %+v", e.Cursor())
		}
		e.SnippetNext()
		if e.Cursor() != (Pos{0, 2}) {
			t.Fatalf("next: got %+v", e.Cursor())
		}
	})
}

func TestEditorSnippetEditDeleteKey(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := expandIn(t, vimOn, "", Pos{}, "${1:name} x")
		e.HandleKey(KeyDelete)
		if got := e.Text(); got != " x" {
			t.Fatalf("text: got %q want %q", got, " x")
		}
		if e.Cursor() != (Pos{0, 0}) {
			t.Fatalf("cursor: got %+v", e.Cursor())
		}
	})
}

func TestEditorSnippetEditMovingKeepsText(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := expandIn(t, vimOn, "", Pos{}, "${1:name}")
		e.HandleKey(KeyRight)
		if got := e.SnippetPlaceholder(0); len(got) != 0 {
			t.Fatalf("placeholder after a move: %v", got)
		}
		if !e.SnippetActive() {
			t.Fatal("session ended")
		}
		typeKeys(e, "z")
		if got := e.Text(); got != "nzame" {
			t.Fatalf("text: got %q want %q", got, "nzame")
		}
	})
}

func TestEditorSnippetEditAutoPairAndReplacement(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := expandIn(t, vimOn, "", Pos{}, "$1 $2")
		typeKeys(e, "(")
		if got := e.Text(); got != "() " {
			t.Fatalf("pair: got %q", got)
		}
		typeKeys(e, "->")
		if got := e.Text(); got != "(→) " {
			t.Fatalf("replacement: got %q", got)
		}
		e.SnippetNext()
		if e.Cursor() != (Pos{0, 4}) {
			t.Fatalf("stop 2: got %+v", e.Cursor())
		}
		typeKeys(e, "q")
		if got := e.Text(); got != "(→) q" {
			t.Fatalf("text: got %q", got)
		}
		e.SnippetPrev()
		if e.Cursor() != (Pos{0, 0}) {
			t.Fatalf("stop 1: got %+v", e.Cursor())
		}
	})
}

func TestEditorSnippetEditTypeBeforeSameRune(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := expandIn(t, vimOn, "", Pos{}, "$1a$2")
		typeKeys(e, "a")
		if got := e.Text(); got != "aa" {
			t.Fatalf("text: got %q", got)
		}
		e.SnippetNext()
		if e.Cursor() != (Pos{0, 2}) {
			t.Fatalf("stop 2: got %+v", e.Cursor())
		}
		e.SnippetPrev()
		if e.Cursor() != (Pos{0, 0}) {
			t.Fatalf("stop 1: got %+v", e.Cursor())
		}
		typeKeys(e, "b")
		if got := e.Text(); got != "baa" {
			t.Fatalf("text: got %q", got)
		}
	})
}

func TestEditorSnippetEditNewlineShiftsStops(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := expandIn(t, vimOn, "", Pos{}, "$1 and $2")
		e.HandleKey(KeyEnter)
		e.SnippetNext()
		// Enter keeps the indent of the line, one blank here.
		if e.Cursor() != (Pos{1, 6}) {
			t.Fatalf("stop 2: got %+v text %q", e.Cursor(), e.Text())
		}
	})
}

func TestEditorSnippetEditLeavingTheRangeEndsSession(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := expandIn(t, vimOn, "above\n", Pos{1, 0}, "${1:name}")
		e.HandleKey(KeyUp)
		if e.SnippetActive() {
			t.Fatal("session stays after the cursor left the range")
		}

		e = expandIn(t, vimOn, "above\nx", Pos{1, 1}, "${1:name}")
		e.SetCursor(Pos{0, 2})
		if e.SnippetActive() {
			t.Fatal("session stays after SetCursor outside the range")
		}
	})
}

func TestEditorSnippetEditCursorInsideRangeKeepsSession(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := expandIn(t, vimOn, "x", Pos{0, 1}, "${1:name}")
		e.SetCursor(Pos{0, 3})
		if !e.SnippetActive() {
			t.Fatal("session ended inside the range")
		}
		e.SetCursor(Pos{0, 5})
		if !e.SnippetActive() {
			t.Fatal("session ended at the end of the range")
		}
		e.HandleKey(KeyLeft)
		e.HandleKey(KeyLeft)
		e.HandleKey(KeyLeft)
		e.HandleKey(KeyLeft)
		e.HandleKey(KeyLeft)
		if e.SnippetActive() {
			t.Fatal("session stays before the start of the range")
		}
	})
}

func TestEditorSnippetEditInsertAtCursorKeepsRanges(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := expandIn(t, vimOn, "", Pos{}, "$1 $2")
		e.InsertAtCursor("[[note]]")
		if got := e.Text(); got != "[[note]] " {
			t.Fatalf("text: got %q", got)
		}
		e.SnippetNext()
		if e.Cursor() != (Pos{0, 9}) {
			t.Fatalf("stop 2: got %+v", e.Cursor())
		}
	})
}

func TestEditorSnippetEditReplaceAtCursorKeepsRanges(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := expandIn(t, vimOn, "", Pos{}, "$1 $2")
		typeKeys(e, "#ta")
		e.ReplaceAtCursor(3, 0, "#tag ")
		if got := e.Text(); got != "#tag  " {
			t.Fatalf("text: got %q", got)
		}
		e.SnippetNext()
		if e.Cursor() != (Pos{0, 6}) {
			t.Fatalf("stop 2: got %+v", e.Cursor())
		}
	})
}
