package vim

import "testing"

func snippetStatusIs(t *testing.T, e *Editor, cur, total int) {
	t.Helper()
	c, n, _, ok := e.SnippetStatus()
	if !ok || c != cur || n != total {
		t.Fatalf("status: got %d/%d ok=%v want %d/%d", c, n, ok, cur, total)
	}
}

func TestEditorSnippetNestedStartsInnerSession(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := expandIn(t, vimOn, "", Pos{}, "[$1] $2")
		e.ExpandSnippet("(${1:a}) $3", fakeVars)
		if got := e.Text(); got != "[(a) ] " {
			t.Fatalf("text: got %q", got)
		}
		if e.Cursor() != (Pos{0, 2}) {
			t.Fatalf("cursor: got %+v want {0 2}", e.Cursor())
		}
		if !e.SnippetActive() {
			t.Fatal("no session")
		}
		if got := e.SnippetPlaceholder(0); len(got) != 1 || got[0] != [2]int{2, 3} {
			t.Fatalf("placeholder: got %v want [[2 3]]", got)
		}
	})
}

func TestEditorSnippetNestedNextReturnsToOuter(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := expandIn(t, vimOn, "", Pos{}, "[$1] $2")
		e.ExpandSnippet("(${1:a}) $3", fakeVars)
		e.SnippetNext()
		if e.Cursor() != (Pos{0, 5}) {
			t.Fatalf("inner next: cursor got %+v want {0 5}", e.Cursor())
		}
		snippetStatusIs(t, e, 2, 2)
		e.SnippetNext()
		snippetStatusIs(t, e, 1, 2)
		e.SnippetNext()
		if e.Cursor() != (Pos{0, 7}) {
			t.Fatalf("outer next: cursor got %+v want {0 7}", e.Cursor())
		}
		snippetStatusIs(t, e, 2, 2)
		e.SnippetNext()
		if e.SnippetActive() {
			t.Fatal("session still active at the end")
		}
	})
}

func TestEditorSnippetNestedEscEndsAll(t *testing.T) {
	e := expandIn(t, true, "", Pos{}, "[$1] $2")
	e.ExpandSnippet("(${1:a}) $3", fakeVars)
	e.HandleKey(KeyEsc)
	if e.SnippetActive() {
		t.Fatal("a session stays after Esc")
	}
	if e.Mode() != ModeNormal {
		t.Fatalf("mode: got %v want normal", e.Mode())
	}
}

func TestEditorSnippetNestedMirroredStop(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := expandIn(t, vimOn, "", Pos{}, "${1:x} = $1")
		typeKeys(e, "k")
		e.ExpandSnippet("(${1:b})", fakeVars)
		if got := e.Text(); got != "k(b) = k(b)" {
			t.Fatalf("text: got %q want %q", got, "k(b) = k(b)")
		}
		if e.Cursor() != (Pos{0, 2}) {
			t.Fatalf("cursor: got %+v want {0 2}", e.Cursor())
		}
		typeKeys(e, "z")
		if got := e.Text(); got != "k(z) = k(z)" {
			t.Fatalf("after typing: got %q want %q", got, "k(z) = k(z)")
		}
		if !e.SnippetActive() {
			t.Fatal("session ended")
		}
	})
}

func TestEditorSnippetNestedCopyHasNoStop(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := expandIn(t, vimOn, "", Pos{}, "${1:x} = $1")
		typeKeys(e, "k")
		e.ExpandSnippet("(${1:b}) $2", fakeVars)
		e.SnippetNext()
		if e.Cursor() != (Pos{0, 5}) {
			t.Fatalf("cursor: got %+v want {0 5}", e.Cursor())
		}
		snippetStatusIs(t, e, 2, 2)
		for _, st := range e.topSession().stops {
			if len(st.ranges) != 1 {
				t.Fatalf("stop %d has %d places, want 1", st.number, len(st.ranges))
			}
		}
	})
}

func TestEditorSnippetNestedStatusReadsTopSession(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := expandIn(t, vimOn, "", Pos{}, "$1 $2 $3 $4")
		e.ExpandSnippet("$1 $2", fakeVars)
		snippetStatusIs(t, e, 1, 2)
	})
}

func TestEditorSnippetNestedExpandReplacesUnchangedPlaceholder(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := expandIn(t, vimOn, "", Pos{}, "${1:name} $2")
		e.ExpandSnippet("<$1>", fakeVars)
		if got := e.Text(); got != "<> " {
			t.Fatalf("text: got %q want %q", got, "<> ")
		}
		e.SnippetNext()
		e.SnippetNext()
		if e.Cursor() != (Pos{0, 3}) {
			t.Fatalf("cursor: got %+v want {0 3}", e.Cursor())
		}
	})
}

func TestEditorSnippetNestedBodyWithoutStopKeepsOuter(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := expandIn(t, vimOn, "", Pos{}, "[$1] $2")
		e.ExpandSnippet("abc", fakeVars)
		if got := e.Text(); got != "[abc] " {
			t.Fatalf("text: got %q", got)
		}
		snippetStatusIs(t, e, 1, 2)
		e.SnippetNext()
		if e.Cursor() != (Pos{0, 6}) {
			t.Fatalf("cursor: got %+v want {0 6}", e.Cursor())
		}
	})
}

func TestEditorSnippetNestedOneUndoWithoutVim(t *testing.T) {
	e := expandIn(t, false, "foobar", Pos{0, 3}, "-${1:x}- $2")
	e.ExpandSnippet("(${1:y})", fakeVars)
	typeKeys(e, "z")
	e.EndSnippets()
	e.Undo()
	if got := e.Text(); got != "foobar" {
		t.Fatalf("after undo: got %q want foobar", got)
	}
}
