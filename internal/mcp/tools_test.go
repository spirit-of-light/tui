package mcp

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ZenNotes/tui/internal/backend"
)

// The desktop MCP server's tool names; the Go server must offer them all.
var desktopTools = []string{
	"add_comment", "append_to_note", "archive_note", "backlinks", "create_folder", "create_note", "delete_folder", "delete_note",
	"duplicate_note", "empty_trash", "insert_at_line", "list_assets", "list_comments", "list_folders", "list_notes", "list_tags",
	"list_tasks", "move_note", "move_to_trash", "prepend_to_note", "read_note", "rename_folder", "rename_note", "replace_in_note",
	"reply_to_comment", "resolve_comment", "restore_from_trash", "search_by_tag", "search_by_title", "search_text", "toggle_task",
	"unarchive_note", "vault_info", "write_note",
}

func TestToolsMatchTheDesktopServer(t *testing.T) {
	have := map[string]bool{}
	for _, tool := range tools() {
		have[tool.name] = true
	}
	for _, name := range desktopTools {
		if !have[name] {
			t.Errorf("missing tool %s", name)
		}
	}
}

// toggle_task rolls a repeating task the way `zn task toggle` does: the open
// copy for the next date goes above the ticked line.
func TestToggleTaskToolInsertsNextOccurrence(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "inbox", "plants.md")
	note := "intro\n- [ ] Water plants due:2026-11-02 repeat:mon,thu\noutro\n"
	if err := os.WriteFile(path, []byte(note), 0o644); err != nil {
		t.Fatal(err)
	}
	today := time.Date(2026, 11, 2, 12, 0, 0, 0, time.Local)
	cs := runServerWith(t, backend.Options{Now: func() time.Time { return today }}, func() (backend.Target, error) {
		return backend.Target{Kind: backend.KindLocal, Root: root}, nil
	})
	text, isErr := callTool(t, cs, "toggle_task", map[string]any{"id": "inbox/plants.md#0"})
	if isErr {
		t.Fatalf("toggle_task: %s", text)
	}
	want := "intro\n- [ ] Water plants due:2026-11-05 repeat:mon,thu\n" +
		"- [x] Water plants due:2026-11-02 repeat:mon,thu\noutro\n"
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Fatalf("note (%v):\n%s\nwant:\n%s", err, got, want)
	}
}
