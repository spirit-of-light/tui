package vim

import (
	"strings"
	"testing"
)

func TestEditorSnippetMirrorCopiesTypedText(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := expandIn(t, vimOn, "", Pos{}, "${1:x} = $1")
		typeKeys(e, "a")
		if got := e.Text(); got != "a = a" {
			t.Fatalf("after one key: got %q want %q", got, "a = a")
		}
		typeKeys(e, "bc")
		if got := e.Text(); got != "abc = abc" {
			t.Fatalf("after three keys: got %q want %q", got, "abc = abc")
		}
		if e.Cursor() != (Pos{0, 3}) {
			t.Fatalf("cursor: got %+v want {0 3}", e.Cursor())
		}
		if !e.SnippetActive() {
			t.Fatal("session ended")
		}
	})
}

func TestEditorSnippetMirrorBackspaceCopies(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := expandIn(t, vimOn, "", Pos{}, "${1:x} = $1")
		typeKeys(e, "abc")
		e.HandleKey(KeyBackspace)
		if got := e.Text(); got != "ab = ab" {
			t.Fatalf("text: got %q want %q", got, "ab = ab")
		}
	})
}

func TestEditorSnippetMirrorFinalStop(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := expandIn(t, vimOn, "", Pos{}, "$0 x $0")
		typeKeys(e, "z")
		if got := e.Text(); got != "z x z" {
			t.Fatalf("text: got %q want %q", got, "z x z")
		}
		if !e.SnippetActive() {
			t.Fatal("session ended")
		}
	})
}

func TestEditorSnippetMirrorReplacedPlaceholderDropsNestedStops(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := expandIn(t, vimOn, "", Pos{}, "${1:a ${2:b} c} $3")
		typeKeys(e, "z")
		if got := e.Text(); got != "z " {
			t.Fatalf("text: got %q want %q", got, "z ")
		}
		e.SnippetNext()
		if e.Cursor() != (Pos{0, 2}) {
			t.Fatalf("next: cursor got %+v want {0 2}", e.Cursor())
		}
		if cur, total, _, ok := e.SnippetStatus(); !ok || cur != 2 || total != 2 {
			t.Fatalf("status: got %d/%d ok=%v want 2/2", cur, total, ok)
		}
	})
}

func TestEditorSnippetMirrorFromSecondOccurrence(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := expandIn(t, vimOn, "", Pos{}, "${1:x} = $1")
		typeKeys(e, "ab")
		e.HandleKey(KeyEnd)
		if e.Cursor() != (Pos{0, 7}) {
			t.Fatalf("cursor after End: got %+v want {0 7}", e.Cursor())
		}
		typeKeys(e, "c")
		if got := e.Text(); got != "abc = abc" {
			t.Fatalf("text: got %q want %q", got, "abc = abc")
		}
		if e.Cursor() != (Pos{0, 9}) {
			t.Fatalf("cursor: got %+v want {0 9}", e.Cursor())
		}
		if !e.SnippetActive() {
			t.Fatal("session ended")
		}
	})
}

func TestEditorSnippetMirrorMultiLineText(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		e := expandIn(t, vimOn, "", Pos{}, "${1:x} | $1")
		typeKeys(e, "a")
		e.HandleKey(KeyEnter)
		typeKeys(e, "b")
		if got := e.Text(); got != "a\nb | a\nb" {
			t.Fatalf("text: got %q", got)
		}
		if e.Cursor() != (Pos{1, 1}) {
			t.Fatalf("cursor: got %+v want {1 1}", e.Cursor())
		}
	})
}

func TestEditorSnippetMirrorLargeNote(t *testing.T) {
	eachVimMode(t, func(t *testing.T, vimOn bool) {
		lines := make([]string, 10000)
		for i := range lines {
			lines[i] = "line " + strings.Repeat("x", i%7)
		}
		text := strings.Join(lines, "\n")
		opts := DefaultOptions()
		opts.VimEnabled = vimOn
		e := New(text, opts, Hooks{})
		e.SetViewport(10)
		e.SetCursor(Pos{5000, 0})
		if vimOn {
			e.Feed("i")
		}
		e.ExpandSnippet("${1:x} = $1", fakeVars)
		typeKeys(e, "ab")
		lines[5000] = "ab = ab" + lines[5000]
		if got := e.Text(); got != strings.Join(lines, "\n") {
			t.Fatalf("text differs near line 5000: %q", e.buf.LineString(5000))
		}
	})
}
