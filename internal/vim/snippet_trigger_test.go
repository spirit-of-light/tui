package vim

import (
	"reflect"
	"testing"
)

func TestEditorSnippetTriggerWordBeforeCursor(t *testing.T) {
	cases := []struct {
		name string
		text string
		cur  Pos
		keys string
		want string
	}{
		{"after a word", "x sn", Pos{0, 4}, "A", "sn"},
		{"inside a word", "snippet", Pos{0, 2}, "i", "sn"},
		{"after a space", "x ", Pos{0, 2}, "A", ""},
		{"empty line", "", Pos{}, "i", ""},
		{"start of line", "snip", Pos{}, "i", ""},
		{"underscore and digit", "a_b2", Pos{0, 4}, "A", "a_b2"},
	}
	for _, c := range cases {
		eachVimMode(t, func(t *testing.T, vimOn bool) {
			t.Run(c.name, func(t *testing.T) {
				e := snippetEditor(t, vimOn, c.text, c.cur, c.keys)
				if got := e.WordBeforeCursor(); got != c.want {
					t.Fatalf("got %q want %q", got, c.want)
				}
			})
		})
	}
}

func TestEditorSnippetTriggerBufferWords(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := snippetEditor(t, vimOn, "Meeting meeting, a_1\nmeeting x", Pos{}, "i")
		want := []string{"Meeting", "meeting", "a_1", "x"}
		if got := e.BufferWords(); !reflect.DeepEqual(got, want) {
			t.Fatalf("got %q want %q", got, want)
		}
	})
}

func TestEditorSnippetTriggerKeywordCompletionActive(t *testing.T) {
	e := snippetEditor(t, true, "world\nwor", Pos{1, 3}, "A")
	if e.KeywordCompletionActive() {
		t.Fatal("active before a key")
	}
	e.HandleKey(Ctrl('n'))
	if !e.KeywordCompletionActive() {
		t.Fatal("not active after Ctrl-N that found a word")
	}
	e.HandleKey(R('x'))
	if e.KeywordCompletionActive() {
		t.Fatal("active after another key")
	}

	off := snippetEditor(t, false, "world\nwor", Pos{1, 3}, "")
	if off.KeywordCompletionActive() {
		t.Fatal("active without Vim")
	}
}

func TestEditorSnippetTriggerExpandOver(t *testing.T) {
	cases := []struct {
		name   string
		text   string
		cur    Pos
		keys   string
		n      int
		body   string
		want   string
		cursor Pos
		sess   bool
	}{
		{"after a word", "x sn", Pos{0, 4}, "A", 2, "S", "x S", Pos{0, 3}, false},
		{"inside a word", "snippet", Pos{0, 2}, "i", 2, "S", "Sippet", Pos{0, 1}, false},
		{"session at stop 1", "x sn", Pos{0, 4}, "A", 2, "[${1:a}]", "x [a]", Pos{0, 3}, true},
		{"indent after the first line", "  sn", Pos{0, 4}, "A", 2, "a\nb", "  a\n  b", Pos{1, 3}, false},
	}
	for _, c := range cases {
		eachVimMode(t, func(t *testing.T, vimOn bool) {
			t.Run(c.name, func(t *testing.T) {
				e := snippetEditor(t, vimOn, c.text, c.cur, c.keys)
				e.ExpandSnippetOver(c.n, c.body, fakeVars)
				if got := e.Text(); got != c.want {
					t.Fatalf("text: got %q want %q", got, c.want)
				}
				if e.Cursor() != c.cursor {
					t.Fatalf("cursor: got %+v want %+v", e.Cursor(), c.cursor)
				}
				if e.SnippetActive() != c.sess {
					t.Fatalf("session: got %v want %v", e.SnippetActive(), c.sess)
				}
				if e.Mode() != ModeInsert {
					t.Fatalf("mode: got %v", e.Mode())
				}
			})
		})
	}
}

func TestEditorSnippetTriggerUndoWithoutVim(t *testing.T) {
	e := snippetEditor(t, false, "", Pos{}, "")
	typeKeys(e, "x sn")
	e.ExpandSnippetOver(2, "S", fakeVars)
	if got := e.Text(); got != "x S" {
		t.Fatalf("text: got %q", got)
	}
	e.HandleKey(Ctrl('z'))
	if got := e.Text(); got != "x sn" {
		t.Fatalf("after undo: got %q want %q", got, "x sn")
	}
}

func TestEditorSnippetTriggerUndoWithVim(t *testing.T) {
	e := snippetEditor(t, true, "x sn", Pos{0, 4}, "A")
	e.ExpandSnippetOver(2, "S", fakeVars)
	e.Feed("<Esc>u")
	if got := e.Text(); got != "x sn" {
		t.Fatalf("after undo: got %q want %q", got, "x sn")
	}
}

func TestEditorSnippetTriggerPopupUndoWithoutVim(t *testing.T) {
	e := snippetEditor(t, false, "", Pos{}, "")
	typeKeys(e, "x me")
	e.ReplaceBeforeCursor(2, "meet")
	if got := e.Text(); got != "x meet" {
		t.Fatalf("first replace: got %q", got)
	}
	e.ReplaceBeforeCursor(4, "meet1")
	e.ExpandSnippetOver(5, "M1", fakeVars)
	if got := e.Text(); got != "x M1" {
		t.Fatalf("expand: got %q", got)
	}
	e.HandleKey(Ctrl('z'))
	if got := e.Text(); got != "x me" {
		t.Fatalf("after undo: got %q want %q", got, "x me")
	}
}

func TestEditorSnippetTriggerPopupCloseCommitsWithoutVim(t *testing.T) {
	e := snippetEditor(t, false, "", Pos{}, "")
	typeKeys(e, "x me")
	e.ReplaceBeforeCursor(2, "meeting")
	e.EndTriggerChange()
	typeKeys(e, "z")
	e.HandleKey(Ctrl('z'))
	if got := e.Text(); got != "x meeting" {
		t.Fatalf("after undo: got %q want %q", got, "x meeting")
	}
}
