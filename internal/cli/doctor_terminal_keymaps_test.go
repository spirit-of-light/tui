package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZenNotes/tui/internal/config"
)

func terminalKeymapsCheck(checks []diagnosticCheck) (diagnosticCheck, bool) {
	for _, c := range checks {
		if c.Name == "terminal keymaps" {
			return c, true
		}
	}
	return diagnosticCheck{}, false
}

// A bad entry gives one failed check that names the entry and the problem.
func TestDoctorTerminalKeymapsReportsBadEntry(t *testing.T) {
	out, checks := doctorWithTools(t, "[terminal.keymaps]\n\"snippet.nope\" = \"Alt+N\"\n\"snippet.prevTabstop\" = \"Alt+P\"")
	check, ok := terminalKeymapsCheck(checks)
	if !ok {
		t.Fatalf("no \"terminal keymaps\" check for a bad entry:\n%s", out)
	}
	if check.OK {
		t.Errorf("the check must fail: %+v", check)
	}
	if !strings.Contains(check.Message, "snippet.nope") || !strings.Contains(check.Message, "does not know") {
		t.Errorf("the message must name the entry and the problem: %q", check.Message)
	}
	if strings.Contains(check.Message, "snippet.prevTabstop") {
		t.Errorf("a valid entry must not appear in the message: %q", check.Message)
	}
}

// The text output shows the same line.
func TestDoctorTerminalKeymapsTextOutput(t *testing.T) {
	isolatedCLI(t)
	if err := os.MkdirAll(filepath.Dir(config.ConfigTomlPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "[terminal.keymaps]\n\"snippet.nextTabstop\" = 5\n"
	if err := os.WriteFile(config.ConfigTomlPath(), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	text := captureOutput(t, func() { _ = Main([]string{"doctor"}) })
	if !strings.Contains(text, "FAIL  terminal keymaps:") || !strings.Contains(text, "snippet.nextTabstop") {
		t.Errorf("doctor text has no terminal keymaps line:\n%s", text)
	}
}

// With no table, or a table with no problem, doctor adds no line.
func TestDoctorTerminalKeymapsQuietWhenValid(t *testing.T) {
	for _, terminal := range []string{"", "[terminal.keymaps]\n\"snippet.nextTabstop\" = \"Alt+N\""} {
		out, checks := doctorWithTools(t, terminal)
		if _, ok := terminalKeymapsCheck(checks); ok {
			t.Errorf("%q: unexpected terminal keymaps check:\n%s", terminal, out)
		}
	}
}
