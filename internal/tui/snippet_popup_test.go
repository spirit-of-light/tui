package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ZenNotes/tui/internal/vim"
)

// popupFiles has three triggers that start with "me" and one that does not.
const popupFiles = "## meet\n\nPlan a meeting.\n\n```\nM\n```\n\n" +
	"## meet1\n\n```\nM1\n```\n\n" +
	"## meet2\n\n```\nM2\n```\n\n" +
	"## link\n\n```\nL\n```\n"

func popupRows(a *App) []string {
	p := a.liveSnippetPopup()
	if p == nil {
		return nil
	}
	rows := []string{}
	for _, it := range p.items {
		rows = append(rows, it.text)
	}
	return rows
}

func popupApp(t *testing.T, vimOn bool, files map[string]string) (*App, *noteBuffer, vim.Key) {
	t.Helper()
	return triggerApp(t, vimOn, files)
}

func TestSnippetPopupCycleOpensOnTwoMatches(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		a, buf, key := popupApp(t, vimOn, map[string]string{"a.md": popupFiles})
		typeKeys(a, "me")
		feed(a, key)
		if got, want := popupRows(a), []string{"meet", "meet1", "meet2"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("rows %v, want %v", got, want)
		}
		if got := buf.ed.Text(); got != "meet" {
			t.Errorf("text %q, want the first row meet and no expansion", got)
		}
		if a.snippetPopup.sel != 0 {
			t.Errorf("selected row %d, want 0", a.snippetPopup.sel)
		}
	})
}

func TestSnippetPopupCycleOneMatchOpensNoPopup(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		a, buf, key := popupApp(t, vimOn, map[string]string{"a.md": popupFiles})
		typeKeys(a, "li")
		feed(a, key)
		if a.liveSnippetPopup() != nil || buf.ed.Text() != "L" {
			t.Errorf("popup %v, text %q, want no popup and L", a.liveSnippetPopup() != nil, buf.ed.Text())
		}
	})
}

func TestSnippetPopupCycleTriggersThenBufferWords(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		a, _, key := popupApp(t, vimOn, map[string]string{"a.md": popupFiles})
		// A lower-case prefix also matches Meeting; meet is a trigger and has one row.
		typeKeys(a, "meeting meet Meeting meetup")
		feed(a, vim.KeyEnter)
		typeKeys(a, "me")
		feed(a, key)
		want := []string{"meet", "meet1", "meet2", "meeting", "Meeting", "meetup"}
		if got := popupRows(a); !reflect.DeepEqual(got, want) {
			t.Fatalf("rows %v, want %v", got, want)
		}
		if a.snippetPopup.items[3].snip != nil || a.snippetPopup.items[0].snip == nil {
			t.Error("trigger rows must have a snippet and word rows none")
		}
	})
}

func TestSnippetPopupCycleSmartCase(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		a, _, key := popupApp(t, vimOn, map[string]string{"a.md": popupFiles})
		typeKeys(a, "Meeting meeting")
		feed(a, vim.KeyEnter)
		typeKeys(a, "Me")
		feed(a, key)
		// No trigger starts with Me, so one word matches and nothing opens.
		if got := popupRows(a); got != nil {
			t.Fatalf("rows %v, want no popup", got)
		}
		a, _, key = popupApp(t, vimOn, map[string]string{"a.md": "## Meet1\n\n```\nA\n```\n\n## Meet2\n\n```\nB\n```\n"})
		typeKeys(a, "Meeting meeting")
		feed(a, vim.KeyEnter)
		typeKeys(a, "Me")
		feed(a, key)
		want := []string{"Meet1", "Meet2", "Meeting"}
		if got := popupRows(a); !reflect.DeepEqual(got, want) {
			t.Errorf("rows %v, want %v", got, want)
		}
	})
}

func TestSnippetPopupCycleSkipsPrefixAndWordsOfTriggers(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		a, _, key := popupApp(t, vimOn, map[string]string{"a.md": popupFiles})
		typeKeys(a, "mee meet1 meet1")
		feed(a, vim.KeyEnter)
		typeKeys(a, "mee")
		feed(a, key)
		for _, r := range popupRows(a) {
			if r == "mee" {
				t.Errorf("rows list the typed prefix: %v", popupRows(a))
			}
		}
		count := 0
		for _, r := range popupRows(a) {
			if r == "meet1" {
				count++
			}
		}
		if count != 1 {
			t.Errorf("meet1 is in %d rows, want 1: %v", count, popupRows(a))
		}
	})
}

func TestSnippetPopupCycleNextAndPreviousWrap(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		a, buf, key := popupApp(t, vimOn, map[string]string{"a.md": popupFiles})
		typeKeys(a, "me")
		feed(a, key)
		steps := []struct {
			key  vim.Key
			want string
		}{
			{vim.Ctrl('n'), "meet1"},
			{vim.KeyTab, "meet2"},
			{vim.Ctrl('n'), "meet"},
			{vim.Ctrl('p'), "meet2"},
			{vim.KeyShiftTab, "meet1"},
			{vim.Ctrl('p'), "meet"},
			{vim.Ctrl('p'), "meet2"},
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

func TestSnippetPopupCycleNeverShowsThePrefix(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		a, buf, key := popupApp(t, vimOn, map[string]string{"a.md": popupFiles})
		typeKeys(a, "me")
		feed(a, key)
		var seen []string
		for range 3 {
			feed(a, vim.Ctrl('n'))
			seen = append(seen, buf.ed.Text())
		}
		if want := []string{"meet1", "meet2", "meet"}; !reflect.DeepEqual(seen, want) {
			t.Errorf("texts %v, want %v", seen, want)
		}
	})
}

func TestSnippetPopupCycleVimOffKeepsNoteKeys(t *testing.T) {
	a, buf, key := popupApp(t, false, map[string]string{"a.md": popupFiles})
	typeKeys(a, "me")
	feed(a, key)
	tabs, overlay := len(a.activePane.tabs), a.overlay
	feed(a, vim.Ctrl('n'))
	feed(a, vim.Ctrl('p'))
	if a.overlay != overlay || len(a.activePane.tabs) != tabs {
		t.Errorf("Ctrl+N and Ctrl+P in the popup opened an overlay or a tab")
	}
	if got := buf.ed.Text(); got != "meet" {
		t.Errorf("text %q, want meet", got)
	}
}

func TestSnippetPopupCycleDuplicateTriggerListedOnce(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		a, _, key := popupApp(t, vimOn, map[string]string{
			"a.md": "## meet\n\n```\nA\n```\n\n## meet1\n\n```\nA1\n```\n",
			"b.md": "## meet\n\n```\nB\n```\n",
		})
		typeKeys(a, "me")
		feed(a, key)
		if got, want := popupRows(a), []string{"meet", "meet1"}; !reflect.DeepEqual(got, want) {
			t.Errorf("rows %v, want %v", got, want)
		}
	})
}

func TestSnippetPopupCycleOtherKeyClosesAndGoesOn(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		a, buf, key := popupApp(t, vimOn, map[string]string{"a.md": popupFiles})
		typeKeys(a, "me")
		feed(a, key, vim.Ctrl('n'))
		feed(a, vim.R('x'))
		if a.snippetPopup != nil {
			t.Error("popup stays open after another key")
		}
		if got := buf.ed.Text(); got != "meet1x" {
			t.Errorf("text %q, want meet1x", got)
		}
	})
}

func TestSnippetPopupCycleStaleAfterCursorMove(t *testing.T) {
	a, buf, key := popupApp(t, false, map[string]string{"a.md": popupFiles})
	typeKeys(a, "me")
	feed(a, key)
	buf.ed.SetCursor(vim.Pos{Line: 0, Col: 1})
	if a.liveSnippetPopup() != nil {
		t.Error("popup is live after the cursor moved")
	}
	feed(a, vim.Ctrl('n'))
	if strings.Contains(buf.ed.Text(), "meet1") {
		t.Errorf("a stale popup took a key: %q", buf.ed.Text())
	}
}

func TestSnippetPopupCycleOpenClosesTagMenu(t *testing.T) {
	a, _, key := popupApp(t, false, map[string]string{"a.md": popupFiles})
	typeKeys(a, "me")
	feed(a, key)
	if a.liveTagMenu() != nil {
		t.Error("the tag menu is open with the popup")
	}
}

func TestSnippetPopupCycleIgnoresWordsOfOtherBuffers(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		a, buf, key := popupApp(t, vimOn, map[string]string{"a.md": popupFiles})
		other := filepath.Join(a.opts.Target.Root, "Other.md")
		if err := os.WriteFile(other, []byte("meetzebra\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		a.openNote("Other.md", true)
		if ob := a.activeBuffer(); ob == nil || ob == buf || !strings.Contains(ob.ed.Text(), "meetzebra") {
			t.Fatalf("the second note did not open in its own buffer")
		}
		a.openNote("Draft.md", true)
		if a.activeBuffer() != buf {
			t.Fatal("Draft.md is not the active buffer again")
		}
		if vimOn && buf.ed.Mode() != vim.ModeInsert {
			feed(a, vim.R('i'))
		}
		typeKeys(a, "meetcat ")
		typeKeys(a, "mee")
		feed(a, key)
		rows := popupRows(a)
		if len(rows) == 0 {
			t.Fatalf("no popup, text %q", buf.ed.Text())
		}
		if slices.Contains(rows, "meetzebra") {
			t.Errorf("rows list a word of another buffer: %v", rows)
		}
		if !slices.Contains(rows, "meetcat") {
			t.Errorf("rows %v lack the word of the current buffer", rows)
		}
	})
}
