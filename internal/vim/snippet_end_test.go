package vim

import "testing"

func TestEditorSnippetEndEscapeKeys(t *testing.T) {
	keys := map[string]Key{"esc": KeyEsc, "ctrl+c": Ctrl('c'), "ctrl+[": Ctrl('[')}
	for name, k := range keys {
		t.Run(name, func(t *testing.T) {
			e := expandIn(t, true, "", Pos{}, "${1:x} $2")
			e.HandleKey(k)
			if e.SnippetActive() {
				t.Fatal("session still active")
			}
			if e.Mode() != ModeNormal {
				t.Fatalf("mode: got %v want normal", e.Mode())
			}
		})
	}
}

func TestEditorSnippetEndEscapeEndsAllSessions(t *testing.T) {
	e := expandIn(t, true, "", Pos{}, "${1:x} $2")
	e.HandleKey(KeyEsc)
	if _, _, _, ok := e.SnippetStatus(); ok {
		t.Fatal("status still reports a session")
	}
}

func TestEditorSnippetEndUndoEndsSession(t *testing.T) {
	t.Run("vim on: Ctrl+O u", func(t *testing.T) {
		e := expandIn(t, true, "", Pos{}, "${1:x} $2")
		typeKeys(e, "a")
		e.HandleKey(Ctrl('o'))
		e.HandleKey(R('u'))
		if e.SnippetActive() {
			t.Fatal("session still active")
		}
	})
	t.Run("vim off: Ctrl+Z", func(t *testing.T) {
		e := expandIn(t, false, "", Pos{}, "${1:x} $2")
		typeKeys(e, "a")
		e.HandleKey(Ctrl('z'))
		if e.SnippetActive() {
			t.Fatal("session still active")
		}
	})
	t.Run("Undo and Redo calls", func(t *testing.T) {
		for _, redo := range []bool{false, true} {
			e := expandIn(t, true, "", Pos{}, "${1:x} $2")
			if redo {
				e.Redo()
			} else {
				e.Undo()
			}
			if e.SnippetActive() {
				t.Fatalf("redo=%v: session still active", redo)
			}
		}
	})
}

func TestEditorSnippetEndTextReplacementEndsSession(t *testing.T) {
	calls := map[string]func(e *Editor){
		"ReplaceText": func(e *Editor) { e.ReplaceText("new text") },
		"SetText":     func(e *Editor) { e.SetText("new text") },
		"LoadText":    func(e *Editor) { e.LoadText("new text") },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			eachVimMode(t, func(t *testing.T, vimOn bool) {
				e := expandIn(t, vimOn, "", Pos{}, "${1:x} $2")
				call(e)
				if e.SnippetActive() {
					t.Fatal("session still active")
				}
			})
		})
	}
}

func TestEditorSnippetEndEndSnippets(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := expandIn(t, vimOn, "", Pos{}, "${1:x} $2")
		e.EndSnippets()
		if e.SnippetActive() {
			t.Fatal("session still active")
		}
		if e.Mode() != ModeInsert {
			t.Fatalf("mode: got %v want insert", e.Mode())
		}
	})
}

func TestEditorSnippetEndSaveKeepsSession(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := expandIn(t, vimOn, "", Pos{}, "${1:x} $2")
		typeKeys(e, "ab")
		cur := e.Cursor()
		e.MarkSaved()
		if !e.SnippetActive() {
			t.Fatal("session ended by a save")
		}
		if e.Cursor() != cur {
			t.Fatalf("cursor: got %+v want %+v", e.Cursor(), cur)
		}
	})
}

func TestEditorSnippetEndOneUndoVimOff(t *testing.T) {
	e := snippetEditor(t, false, "foobar", Pos{0, 3}, "")
	e.ExpandSnippet("-${1:x}-", fakeVars)
	typeKeys(e, "a")
	// A cursor key commits the change in plain editing when no session runs.
	e.HandleKey(Special("left"))
	e.HandleKey(Special("right"))
	typeKeys(e, "b")
	if got := e.Text(); got != "foo-ab-bar" {
		t.Fatalf("text: got %q want %q", got, "foo-ab-bar")
	}
	e.HandleKey(Ctrl('z'))
	if got := e.Text(); got != "foobar" {
		t.Fatalf("after one Ctrl+Z: got %q want %q", got, "foobar")
	}
}

func TestEditorSnippetEndOneUndoVimOffAfterEnd(t *testing.T) {
	e := snippetEditor(t, false, "foobar", Pos{0, 3}, "")
	e.ExpandSnippet("-${1:x}-$0", fakeVars)
	typeKeys(e, "a")
	e.SnippetNext()
	if e.SnippetActive() {
		t.Fatal("session still active")
	}
	typeKeys(e, "c")
	e.HandleKey(Special("left"))
	e.HandleKey(Ctrl('z'))
	if got := e.Text(); got != "foo-a-bar" {
		t.Fatalf("typing after the end: got %q want %q", got, "foo-a-bar")
	}
	e.HandleKey(Ctrl('z'))
	if got := e.Text(); got != "foobar" {
		t.Fatalf("second Ctrl+Z: got %q want %q", got, "foobar")
	}
}

func TestEditorSnippetEndOneUndoVimOn(t *testing.T) {
	e := snippetEditor(t, true, "foobar", Pos{0, 3}, "i")
	e.ExpandSnippet("-${1:x}-", fakeVars)
	typeKeys(e, "a")
	e.HandleKey(KeyEsc)
	e.HandleKey(R('u'))
	if got := e.Text(); got != "foobar" {
		t.Fatalf("after Esc u: got %q want %q", got, "foobar")
	}
}

func TestEditorSnippetEndOneUndoAcrossStops(t *testing.T) {
	e := snippetEditor(t, true, "foobar", Pos{0, 3}, "i")
	e.ExpandSnippet("-${1:x}- $2", fakeVars)
	typeKeys(e, "ab")
	e.SnippetNext()
	typeKeys(e, "c")
	e.HandleKey(KeyEsc)
	e.HandleKey(R('u'))
	if got := e.Text(); got != "foobar" {
		t.Fatalf("after Esc u: got %q want %q", got, "foobar")
	}
}
