package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/ZenNotes/tui/internal/config"
)

func bindConfig(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(config.ConfigTomlPath())
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func bindApp(t *testing.T, vimOn bool, toml string) (*App, *noteBuffer) {
	t.Helper()
	if !vimOn {
		// :bind reloads the file, so Vim mode must be off in the file too.
		toml = "[vim]\nenabled = false\n" + toml
	}
	p := overridePrefs(t, vimOn, toml)
	dir := os.Getenv("ZENNOTES_CONFIG_DIR")
	a, buf := snippetKeyApp(t, p, "", "$1 $2 $3")
	// The test app points the config dir at its own temporary dir, so
	// point it back at the dir that holds the file under test.
	t.Setenv("ZENNOTES_CONFIG_DIR", dir)
	a.prefsLoaded = true
	return a, buf
}

func TestBindTerminalKeymapOneKey(t *testing.T) {
	for _, m := range snippetKeyModes {
		t.Run(m.name, func(t *testing.T) {
			a, buf := bindApp(t, m.name == "vim on", "[keymaps]\n\"global.toggleSidebar\" = \"Alt+B\"\n")
			a.runCommandLine("bind snippet.nextTabstop Alt+N")
			got := bindConfig(t)
			if !strings.Contains(got, `"snippet.nextTabstop" = "Alt+N"`) && !strings.Contains(got, `snippet.nextTabstop = "Alt+N"`) {
				t.Fatalf("config.toml has no string for the action:\n%s", got)
			}
			if !strings.Contains(got, "[keymaps]") || !strings.Contains(got, "Alt+B") || strings.Contains(got, "[keymaps]\n\"snippet") {
				t.Fatalf("[keymaps] changed:\n%s", got)
			}
			if a.message != "" {
				t.Fatalf("message: %q", a.message)
			}
			a.handleKey(altN)
			wantCursor(t, buf, 0, 1, "Alt+N after :bind")
		})
	}
}

func TestBindTerminalKeymapUnbind(t *testing.T) {
	a, _ := bindApp(t, true, "")
	a.runCommandLine("unbind snippet.nextTabstop")
	if got := bindConfig(t); !strings.Contains(got, `""`) {
		t.Fatalf("no empty string written:\n%s", got)
	}
	if k := a.terminalKeys.Bindings("snippet.nextTabstop"); len(k) != 0 {
		t.Fatalf("next keys after :unbind: %v", k)
	}
}

func TestBindTerminalKeymapList(t *testing.T) {
	cases := []struct{ cmd, want string }{
		{"bind snippet.nextTabstop Alt+N, Space n", `["Alt+N", "Space n"]`},
		{"bind snippet.nextTabstop Ctrl+,", `"Ctrl+,"`},
		{"bind snippet.nextTabstop Ctrl+,, Tab", `["Ctrl+,", "Tab"]`},
		{"bind snippet.nextTabstop Alt+N,Alt+M", `["Alt+N", "Alt+M"]`},
	}
	for _, c := range cases {
		t.Run(c.cmd, func(t *testing.T) {
			a, _ := bindApp(t, true, "")
			a.runCommandLine(c.cmd)
			if got := bindConfig(t); !strings.Contains(got, c.want) {
				t.Fatalf("config.toml lacks %s:\n%s", c.want, got)
			}
		})
	}
}

func TestBindTerminalKeymapBadKey(t *testing.T) {
	a, _ := bindApp(t, true, "")
	a.runCommandLine("bind snippet.nextTabstop Alt+N, Ctrl+Foo")
	if _, err := os.Stat(config.ConfigTomlPath()); err == nil {
		if got := bindConfig(t); strings.Contains(got, "snippet.nextTabstop") {
			t.Fatalf("a bad key was written:\n%s", got)
		}
	}
	if a.message == "" {
		t.Fatal("no error for a key that zn cannot read")
	}
}

func TestBindTerminalKeymapCatalogIDKeepsOldRule(t *testing.T) {
	a, _ := bindApp(t, true, "")
	a.runCommandLine("bind global.toggleSidebar Alt+B, Alt+C")
	got := bindConfig(t)
	if !strings.Contains(got, `"Alt+B, Alt+C"`) {
		t.Fatalf("[keymaps] lacks the joined string:\n%s", got)
	}
	if strings.Contains(got, "terminal.keymaps") {
		t.Fatalf("[terminal.keymaps] changed:\n%s", got)
	}
}

func TestBindTerminalKeymapStringReplacesList(t *testing.T) {
	for _, m := range snippetKeyModes {
		t.Run(m.name, func(t *testing.T) {
			a, buf := bindApp(t, m.name == "vim on", "[terminal.keymaps]\n\"snippet.nextTabstop\" = [\"Alt+N\", \"Alt+M\"]\n")
			a.runCommandLine("bind snippet.nextTabstop Alt+N")
			if got := bindConfig(t); strings.Contains(got, "Alt+M") {
				t.Fatalf("the list stays:\n%s", got)
			}
			a.handleKey(altM)
			wantCursor(t, buf, 0, 0, "Alt+M after the string")
			a.handleKey(altN)
			wantCursor(t, buf, 0, 1, "Alt+N")
		})
	}
}
