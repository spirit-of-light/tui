package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZenNotes/tui/internal/config"
)

func TestSnippetToolsCreateListAndRefuseSilentReplace(t *testing.T) {
	t.Setenv("ZENNOTES_CONFIG_DIR", t.TempDir())
	cs := runScoped(t, seedScopedVault(t), nil)

	created := mustCall(t, cs, "snippet_create", map[string]any{
		"trigger":     "snip",
		"body":        "[${1:label}](${2:url})$0",
		"description": "Markdown link",
	})
	var out struct {
		Expands  string   `json:"expands"`
		Tabstops []string `json:"tabstops"`
	}
	if err := json.Unmarshal([]byte(created), &out); err != nil {
		t.Fatalf("%v: %s", err, created)
	}
	if out.Expands != "[label](url)" || len(out.Tabstops) != 3 {
		t.Fatalf("check result %+v", out)
	}
	if _, err := os.Stat(config.SnippetsPath()); err != nil {
		t.Fatalf("file not written: %v", err)
	}

	listed := mustCall(t, cs, "snippets_list", map[string]any{})
	if !strings.Contains(listed, `"trigger": "snip"`) || !strings.Contains(listed, "Markdown link") {
		t.Fatalf("list %s", listed)
	}

	mustFail(t, cs, "snippet_create", map[string]any{"trigger": "snip", "body": "x"})
	mustCall(t, cs, "snippet_create", map[string]any{"trigger": "snip", "body": "y$0", "replace": true})
	if raw, _ := os.ReadFile(config.SnippetsPath()); !strings.Contains(string(raw), `"y$0"`) && !strings.Contains(string(raw), "y$0") {
		t.Fatalf("replace did not take: %s", raw)
	}
	mustFail(t, cs, "snippet_create", map[string]any{"trigger": "bad trigger", "body": "x"})
}

func TestSnippetCreateRefusesFileWithErrors(t *testing.T) {
	// The session sets its own config directory, so the file is written to the
	// path the server reads, after the session exists.
	cs := runScoped(t, seedScopedVault(t), nil)
	bad := config.SnippetsPath()
	if err := os.MkdirAll(filepath.Dir(bad), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("[[snippet]]\ntrigger = \"x y\"\nbody = \"z\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustFail(t, cs, "snippet_create", map[string]any{"trigger": "ok", "body": "z"})
	raw, _ := os.ReadFile(bad)
	if !strings.Contains(string(raw), "x y") {
		t.Fatal("the broken file was overwritten")
	}
}

func TestLocalServerDescribesSnippetsToAgents(t *testing.T) {
	if !strings.Contains(snippetInstructionsFor(), "$0") || !strings.Contains(snippetInstructionsFor(), "Ctrl+N") {
		t.Fatal("snippet syntax missing from instructions")
	}
}
