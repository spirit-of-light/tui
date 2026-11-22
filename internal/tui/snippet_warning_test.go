package tui

import (
	"context"
	"strings"
	"testing"
)

func warningToasts(a *App) []string {
	var out []string
	for _, t := range a.toasts {
		out = append(out, t.text)
	}
	return out
}

func TestTerminalKeymapWarningNoTable(t *testing.T) {
	a := newApp(context.Background(), Options{}, overridePrefs(t, true, ""), true)
	if len(a.toasts) != 0 || a.message != "" {
		t.Fatalf("a default config warned: toasts %q, message %q", warningToasts(a), a.message)
	}
}

func TestTerminalKeymapWarningValidTableStaysQuiet(t *testing.T) {
	a := newApp(context.Background(), Options{}, overridePrefs(t, true, "[terminal.keymaps]\n\"snippet.nextTabstop\" = \"Alt+N\"\n"), true)
	if len(a.toasts) != 0 || a.message != "" {
		t.Fatalf("a valid table warned: toasts %q", warningToasts(a))
	}
}

func TestTerminalKeymapWarningUnknownAction(t *testing.T) {
	a := newApp(context.Background(), Options{}, overridePrefs(t, true, "[terminal.keymaps]\n\"snippet.nope\" = \"Alt+N\"\n\"snippet.prevTabstop\" = \"Alt+P\"\n"), true)
	got := warningToasts(a)
	if len(got) != 1 || !strings.Contains(got[0], "snippet.nope") || !a.toasts[0].isErr {
		t.Fatalf("want one error toast that names snippet.nope, got %q", got)
	}
	if strings.Contains(got[0], "snippet.prevTabstop") {
		t.Fatalf("the warning names a valid entry: %q", got[0])
	}
}

func TestTerminalKeymapWarningBadKeyAndBadType(t *testing.T) {
	for name, toml := range map[string]string{
		"snippet.nextTabstop": "[terminal.keymaps]\n\"snippet.nextTabstop\" = \"Ctrl+Foo\"\n",
		"snippet.prevTabstop": "[terminal.keymaps]\n\"snippet.prevTabstop\" = 5\n",
	} {
		t.Run(name, func(t *testing.T) {
			a := newApp(context.Background(), Options{}, overridePrefs(t, true, toml), true)
			got := warningToasts(a)
			if len(got) != 1 || !strings.Contains(got[0], name) {
				t.Fatalf("want one toast that names %s, got %q", name, got)
			}
		})
	}
}

func TestTerminalKeymapWarningTwoEntriesOneToast(t *testing.T) {
	a := newApp(context.Background(), Options{}, overridePrefs(t, true, "[terminal.keymaps]\n\"snippet.nope\" = \"Alt+N\"\n\"snippet.nextTabstop\" = 5\n"), true)
	got := warningToasts(a)
	if len(got) != 1 || !strings.Contains(got[0], "snippet.nope") || !strings.Contains(got[0], "snippet.nextTabstop") {
		t.Fatalf("want one toast with both entries, got %q", got)
	}
}

func TestTerminalKeymapWarningReload(t *testing.T) {
	toml := "[terminal.keymaps]\n\"snippet.nope\" = \"Alt+N\"\n"
	p := overridePrefs(t, true, toml)
	a := newApp(context.Background(), Options{}, p, true)
	if len(a.toasts) != 1 {
		t.Fatalf("want one toast at start, got %q", warningToasts(a))
	}
	a.applyPreferences(p)
	if len(a.toasts) != 1 {
		t.Fatalf("a reload with the same problems warned again: %q", warningToasts(a))
	}
	p2 := overridePrefs(t, true, toml+"\"snippet.other\" = \"Alt+O\"\n")
	a.applyPreferences(p2)
	if got := warningToasts(a); len(got) != 2 || !strings.Contains(got[1], "snippet.other") {
		t.Fatalf("a changed set of problems must warn once: %q", got)
	}
}
