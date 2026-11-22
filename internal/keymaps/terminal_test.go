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
	if len(entries) != 2 || entries[0].Title != "Next snippet tabstop" || entries[1].Title != "Previous snippet tabstop" {
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
