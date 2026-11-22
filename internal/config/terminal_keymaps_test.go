package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// loadTerminalKeymaps writes a config.toml with the given text and loads it.
func loadTerminalKeymaps(t *testing.T, text string) Prefs {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("ZENNOTES_CONFIG_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _, err := LoadPrefs()
	if err != nil {
		t.Fatalf("LoadPrefs returned an error: %v", err)
	}
	return p
}

func TestTerminalKeymapsPrefsNoTable(t *testing.T) {
	t.Setenv("ZENNOTES_CONFIG_DIR", t.TempDir())
	p, _, err := LoadPrefs()
	if err != nil {
		t.Fatal(err)
	}
	if len(p.TerminalKeymapOverrides) != 0 || len(p.TerminalKeymapProblems) != 0 {
		t.Fatalf("no file: got %v and %v", p.TerminalKeymapOverrides, p.TerminalKeymapProblems)
	}
	p = loadTerminalKeymaps(t, "[terminal]\nmouse = true\n")
	if len(p.TerminalKeymapOverrides) != 0 || len(p.TerminalKeymapProblems) != 0 {
		t.Fatalf("no table: got %v and %v", p.TerminalKeymapOverrides, p.TerminalKeymapProblems)
	}
}

func TestTerminalKeymapsPrefsValues(t *testing.T) {
	p := loadTerminalKeymaps(t, `[terminal.keymaps]
"snippet.nextTabstop" = "Alt+N"
"snippet.prevTabstop" = ["Alt+P", "Space p"]
`)
	want := map[string][]string{
		"snippet.nextTabstop": {"Alt+N"},
		"snippet.prevTabstop": {"Alt+P", "Space p"},
	}
	if !reflect.DeepEqual(p.TerminalKeymapOverrides, want) {
		t.Fatalf("overrides = %v, want %v", p.TerminalKeymapOverrides, want)
	}
	if len(p.TerminalKeymapProblems) != 0 {
		t.Fatalf("problems = %v", p.TerminalKeymapProblems)
	}

	p = loadTerminalKeymaps(t, `[terminal.keymaps]
"snippet.nextTabstop" = ""
"snippet.prevTabstop" = []
`)
	for _, id := range []string{"snippet.nextTabstop", "snippet.prevTabstop"} {
		keys, ok := p.TerminalKeymapOverrides[id]
		if !ok || len(keys) != 0 {
			t.Fatalf("%s: keys = %v, present = %v; want an empty list", id, keys, ok)
		}
	}
	if len(p.TerminalKeymapProblems) != 0 {
		t.Fatalf("problems = %v", p.TerminalKeymapProblems)
	}
}

func TestTerminalKeymapsPrefsUnknownID(t *testing.T) {
	p := loadTerminalKeymaps(t, `[terminal.keymaps]
"snippet.nope" = "Alt+N"
"snippet.prevTabstop" = "Alt+P"
`)
	if got := p.TerminalKeymapOverrides["snippet.prevTabstop"]; !reflect.DeepEqual(got, []string{"Alt+P"}) {
		t.Fatalf("valid entry = %v", got)
	}
	if _, bad := p.TerminalKeymapOverrides["snippet.nope"]; bad {
		t.Fatal("an unknown action must not become an override")
	}
	if len(p.TerminalKeymapProblems) != 1 || !strings.Contains(p.TerminalKeymapProblems[0], "snippet.nope") {
		t.Fatalf("problems = %v", p.TerminalKeymapProblems)
	}
}

func TestTerminalKeymapsPrefsBadKeyString(t *testing.T) {
	p := loadTerminalKeymaps(t, `[terminal.keymaps]
"snippet.nextTabstop" = "Ctrl+Foo"
`)
	if _, bad := p.TerminalKeymapOverrides["snippet.nextTabstop"]; bad {
		t.Fatal("a key string that zn cannot read must not become an override")
	}
	if len(p.TerminalKeymapProblems) != 1 ||
		!strings.Contains(p.TerminalKeymapProblems[0], "snippet.nextTabstop") ||
		!strings.Contains(p.TerminalKeymapProblems[0], "Ctrl+Foo") {
		t.Fatalf("problems = %v", p.TerminalKeymapProblems)
	}

	p = loadTerminalKeymaps(t, `[terminal.keymaps]
"snippet.nextTabstop" = ["Alt+N", "Ctrl+Foo"]
`)
	if _, bad := p.TerminalKeymapOverrides["snippet.nextTabstop"]; bad || len(p.TerminalKeymapProblems) != 1 {
		t.Fatalf("a list with one bad key: overrides %v, problems %v", p.TerminalKeymapOverrides, p.TerminalKeymapProblems)
	}
}

func TestTerminalKeymapsPrefsWrongType(t *testing.T) {
	for name, text := range map[string]string{
		"number":       "\"snippet.nextTabstop\" = 5",
		"list number":  "\"snippet.nextTabstop\" = [\"Alt+N\", 5]",
		"boolean":      "\"snippet.nextTabstop\" = true",
		"inline table": "\"snippet.nextTabstop\" = { key = \"Alt+N\" }",
	} {
		p := loadTerminalKeymaps(t, "[terminal.keymaps]\n"+text+"\n")
		if _, bad := p.TerminalKeymapOverrides["snippet.nextTabstop"]; bad {
			t.Fatalf("%s: a wrong type must not become an override", name)
		}
		if len(p.TerminalKeymapProblems) != 1 || !strings.Contains(p.TerminalKeymapProblems[0], "snippet.nextTabstop") {
			t.Fatalf("%s: problems = %v", name, p.TerminalKeymapProblems)
		}
	}
}

func TestTerminalKeymapsPrefsStableOrder(t *testing.T) {
	text := `[terminal.keymaps]
"snippet.zzz" = "Alt+Z"
"snippet.nextTabstop" = 5
"snippet.aaa" = "Alt+A"
"snippet.prevTabstop" = "Ctrl+Foo"
`
	first := loadTerminalKeymaps(t, text).TerminalKeymapProblems
	if len(first) != 4 {
		t.Fatalf("problems = %v", first)
	}
	for i := 1; i < len(first); i++ {
		if first[i-1] > first[i] {
			t.Fatalf("problems are not sorted: %v", first)
		}
	}
	for i := 0; i < 5; i++ {
		if again := loadTerminalKeymaps(t, text).TerminalKeymapProblems; !reflect.DeepEqual(first, again) {
			t.Fatalf("order changed: %v then %v", first, again)
		}
	}
}

func TestTerminalKeymapsPrefsTableOfWrongType(t *testing.T) {
	p := loadTerminalKeymaps(t, "[terminal]\nkeymaps = \"Alt+N\"\n")
	if len(p.TerminalKeymapOverrides) != 0 {
		t.Fatalf("overrides = %v", p.TerminalKeymapOverrides)
	}
	want := []string{"keymaps: the value must be a table of actions"}
	if !reflect.DeepEqual(p.TerminalKeymapProblems, want) {
		t.Fatalf("problems = %v, want %v", p.TerminalKeymapProblems, want)
	}
}

func TestTerminalKeymapsPrefsKeepOtherTables(t *testing.T) {
	p := loadTerminalKeymaps(t, `[keymaps]
"global.toggleSidebar" = "Alt+B"

[terminal.keymaps]
"snippet.nextTabstop" = "Alt+N"
`)
	if p.KeymapOverrides["global.toggleSidebar"] != "Alt+B" {
		t.Fatalf("[keymaps] = %v", p.KeymapOverrides)
	}
	if _, leaked := p.KeymapOverrides["snippet.nextTabstop"]; leaked {
		t.Fatal("a terminal action must stay out of [keymaps]")
	}
}
