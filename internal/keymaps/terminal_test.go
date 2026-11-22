package keymaps

import (
	"reflect"
	"testing"

	"github.com/ZenNotes/tui/internal/vim"
)

func TestTerminalKeymapDefaults(t *testing.T) {
	r := NewTerminalResolver(nil)
	if got, want := r.Bindings("snippet.nextTabstop"), []string{"Ctrl+L", "Tab"}; !reflect.DeepEqual(got, want) {
		t.Errorf("next = %v, want %v", got, want)
	}
	if got, want := r.Bindings("snippet.prevTabstop"), []string{"Ctrl+H", "Shift+Tab"}; !reflect.DeepEqual(got, want) {
		t.Errorf("prev = %v, want %v", got, want)
	}
	if !r.Known("snippet.nextTabstop") || r.Known("snippet.nope") || r.Known("global.toggleSidebar") {
		t.Error("Known gives the wrong answer")
	}
	entries := r.Entries()
	titles := map[string]string{}
	for _, e := range entries {
		titles[e.ID] = e.Title
	}
	if titles["snippet.nextTabstop"] != "Next snippet tabstop" || titles["snippet.prevTabstop"] != "Previous snippet tabstop" || titles["snippet.expand"] != "Expand a snippet" {
		t.Errorf("Entries = %+v", entries)
	}
}

func TestTerminalKeymapOverride(t *testing.T) {
	r := NewTerminalResolver(map[string][]string{"snippet.nextTabstop": {"Alt+N"}, "snippet.nope": {"Alt+X"}})
	if got, want := r.Bindings("snippet.nextTabstop"), []string{"Alt+N"}; !reflect.DeepEqual(got, want) {
		t.Errorf("override = %v, want %v", got, want)
	}
	if got := r.Bindings("snippet.prevTabstop"); len(got) != 2 {
		t.Errorf("an untouched action lost its defaults: %v", got)
	}
	if r.Bindings("snippet.nope") != nil {
		t.Error("an unknown id has keys")
	}
}

func TestTerminalKeymapUnbind(t *testing.T) {
	r := NewTerminalResolver(map[string][]string{"snippet.prevTabstop": {}})
	if len(r.Bindings("snippet.prevTabstop")) != 0 || len(r.Keys("snippet.prevTabstop")) != 0 {
		t.Error("an empty list must unbind the action")
	}
	r.Set("snippet.nextTabstop", []string{""})
	if len(r.Keys("snippet.nextTabstop")) != 0 {
		t.Error("a blank key must unbind the action")
	}
	if r.Set("snippet.nope", []string{"Alt+N"}) {
		t.Error("Set accepted an unknown id")
	}
}

func TestTerminalKeymapKeys(t *testing.T) {
	r := NewTerminalResolver(nil)
	next := r.Keys("snippet.nextTabstop")
	if want := [][]vim.Key{{{Rune: 'l', Ctrl: true}}, {{Name: "tab"}}}; !reflect.DeepEqual(next, want) {
		t.Errorf("next keys = %+v, want %+v", next, want)
	}
	prev := r.Keys("snippet.prevTabstop")
	if want := [][]vim.Key{{{Rune: 'h', Ctrl: true}}, {{Name: "tab", Shift: true}}}; !reflect.DeepEqual(prev, want) {
		t.Errorf("prev keys = %+v, want %+v", prev, want)
	}
}

func TestSplitTerminalKeys(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"Alt+N", []string{"Alt+N"}},
		{"Alt+N, Space n", []string{"Alt+N", "Space n"}},
		{"Ctrl+,", []string{"Ctrl+,"}},
		{"Ctrl+,, Tab", []string{"Ctrl+,", "Tab"}},
		{"Alt+N ,Alt+M", []string{"Alt+N", "Alt+M"}},
		{"Alt+N,Alt+M", []string{"Alt+N", "Alt+M"}},
		{"  Alt+N  ", []string{"Alt+N"}},
		{"  Space   n ,  Alt+M  ", []string{"Space n", "Alt+M"}},
		{"Alt+N,", []string{"Alt+N"}},
		{"", nil},
		{"   ", nil},
	}
	for _, c := range cases {
		got := SplitTerminalKeys(c.in)
		if len(got) == 0 && len(c.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("SplitTerminalKeys(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTerminalKeymapParseKeys(t *testing.T) {
	if keys, ok := ParseKeys(""); !ok || keys != nil {
		t.Errorf("empty = %v %v", keys, ok)
	}
	if _, ok := ParseKeys("Ctrl+Foo"); ok {
		t.Error("Ctrl+Foo must not parse")
	}
	if keys, ok := ParseKeys("g t"); !ok || len(keys) != 2 {
		t.Errorf("g t = %v %v", keys, ok)
	}
}

func TestTerminalKeymapExpandDefaults(t *testing.T) {
	r := NewTerminalResolver(nil)
	ctrlN := [][]vim.Key{{{Rune: 'n', Ctrl: true}}}
	tab := [][]vim.Key{{{Name: "tab"}}}
	if got := r.KeysFor("snippet.expand", true); !reflect.DeepEqual(got, ctrlN) {
		t.Errorf("Vim on = %+v, want Ctrl+N only", got)
	}
	if got := r.KeysFor("snippet.expand", false); !reflect.DeepEqual(got, tab) {
		t.Errorf("Vim off = %+v, want Tab only", got)
	}
	if r.Overridden("snippet.expand") {
		t.Error("a default action is not overridden")
	}
	if got, want := r.Labels("snippet.expand"), []string{"Ctrl+N (Vim)", "Tab (no Vim)"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Labels = %v, want %v", got, want)
	}
}

func TestTerminalKeymapExpandOverride(t *testing.T) {
	r := NewTerminalResolver(map[string][]string{"snippet.expand": {"Alt+S"}})
	want := [][]vim.Key{{{Rune: 's', Alt: true}}}
	for _, vimOn := range []bool{true, false} {
		if got := r.KeysFor("snippet.expand", vimOn); !reflect.DeepEqual(got, want) {
			t.Errorf("vim=%v: %+v, want Alt+S", vimOn, got)
		}
	}
	if !r.Overridden("snippet.expand") {
		t.Error("an override must show as overridden")
	}
	if got, want := r.Labels("snippet.expand"), []string{"Alt+S"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Labels = %v, want %v", got, want)
	}
	r = NewTerminalResolver(map[string][]string{"snippet.expand": {""}})
	for _, vimOn := range []bool{true, false} {
		if got := r.KeysFor("snippet.expand", vimOn); len(got) != 0 {
			t.Errorf("vim=%v: an empty override left keys %+v", vimOn, got)
		}
	}
	if !r.Overridden("snippet.expand") {
		t.Error("an empty override is still an override")
	}
}

func TestTerminalKeymapExpandSessionKeysKeepModes(t *testing.T) {
	r := NewTerminalResolver(nil)
	for _, vimOn := range []bool{true, false} {
		if got := r.KeysFor("snippet.nextTabstop", vimOn); !reflect.DeepEqual(got, r.Keys("snippet.nextTabstop")) {
			t.Errorf("vim=%v: next keys changed: %+v", vimOn, got)
		}
		if got := r.KeysFor("snippet.prevTabstop", vimOn); !reflect.DeepEqual(got, r.Keys("snippet.prevTabstop")) {
			t.Errorf("vim=%v: prev keys changed: %+v", vimOn, got)
		}
	}
	if got, want := r.Labels("snippet.nextTabstop"), []string{"Ctrl+L", "Tab"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Labels = %v, want %v", got, want)
	}
}
