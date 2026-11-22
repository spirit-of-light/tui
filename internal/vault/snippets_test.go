package vault

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func snippetFileNames(files []SnippetFile) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, f.Name)
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestVaultSnippetFilesByteOrder(t *testing.T) {
	v := newTestVault(t)
	dir := filepath.Join(v.root, ".zennotes", "snippets")
	writeFile(t, filepath.Join(dir, "b.md"), "B")
	writeFile(t, filepath.Join(dir, "a.md"), "A")
	writeFile(t, filepath.Join(dir, "B.md"), "UB")
	files, err := v.ListSnippetFiles()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"B.md", "a.md", "b.md"}
	if got := snippetFileNames(files); !equalStrings(got, want) {
		t.Fatalf("names = %v, want %v", got, want)
	}
	if files[1].Raw != "A" || files[1].Err != nil {
		t.Fatalf("a.md = %+v", files[1])
	}
	if files[1].Path != ".zennotes/snippets/a.md" {
		t.Fatalf("path = %q", files[1].Path)
	}
}

func TestVaultSnippetFilesNameWithSpaces(t *testing.T) {
	v := newTestVault(t)
	writeFile(t, filepath.Join(v.root, ".zennotes", "snippets", "My Notes.md"), "x")
	files, err := v.ListSnippetFiles()
	if err != nil {
		t.Fatal(err)
	}
	if got := snippetFileNames(files); !equalStrings(got, []string{"My Notes.md"}) {
		t.Fatalf("names = %v", got)
	}
}

func TestVaultSnippetFilesSkipsOtherEntries(t *testing.T) {
	v := newTestVault(t)
	dir := filepath.Join(v.root, ".zennotes", "snippets")
	writeFile(t, filepath.Join(dir, "good.md"), "x")
	writeFile(t, filepath.Join(dir, "sub", "x.md"), "x")
	writeFile(t, filepath.Join(dir, "x.txt"), "x")
	writeFile(t, filepath.Join(dir, ".hidden.md"), "x")
	if err := os.MkdirAll(filepath.Join(dir, "dir.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	files, err := v.ListSnippetFiles()
	if err != nil {
		t.Fatal(err)
	}
	if got := snippetFileNames(files); !equalStrings(got, []string{"good.md"}) {
		t.Fatalf("names = %v", got)
	}
}

func TestVaultSnippetFilesMissingDirectory(t *testing.T) {
	v := newTestVault(t)
	files, err := v.ListSnippetFiles()
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("files = %v", files)
	}
}

func TestVaultSnippetFilesUnreadableKeepsError(t *testing.T) {
	if runtime.GOOS == "windows" || os.Getuid() == 0 {
		t.Skip("file modes do not block a read here")
	}
	v := newTestVault(t)
	dir := filepath.Join(v.root, ".zennotes", "snippets")
	writeFile(t, filepath.Join(dir, "good.md"), "ok")
	bad := filepath.Join(dir, "locked.md")
	writeFile(t, bad, "secret")
	if err := os.Chmod(bad, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(bad, 0o644) })
	files, err := v.ListSnippetFiles()
	if err != nil {
		t.Fatal(err)
	}
	if got := snippetFileNames(files); !equalStrings(got, []string{"good.md", "locked.md"}) {
		t.Fatalf("names = %v", got)
	}
	if files[0].Err != nil || files[0].Raw != "ok" {
		t.Fatalf("good.md = %+v", files[0])
	}
	if files[1].Err == nil || files[1].Raw != "" {
		t.Fatalf("locked.md = %+v", files[1])
	}
}

func TestVaultSnippetFilesLinkOutOfVaultIsNotRead(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges")
	}
	v := newTestVault(t)
	dir := filepath.Join(v.root, ".zennotes", "snippets")
	outside := filepath.Join(t.TempDir(), "outside.md")
	writeFile(t, outside, "outside text")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "link.md")); err != nil {
		t.Skip("symlink not available")
	}
	files, err := v.ListSnippetFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Raw != "" || files[0].Err == nil {
		t.Fatalf("a link out of the vault was read: %+v", files)
	}
}

func TestVaultSnippetFilesAreNotNotes(t *testing.T) {
	v := newTestVault(t)
	writeFile(t, filepath.Join(v.root, ".zennotes", "snippets", "a.md"), "x")
	writeFile(t, filepath.Join(v.root, "inbox", "n.md"), "# n")
	notes, err := v.ListNotes()
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 || notes[0].Path != "inbox/n.md" {
		t.Fatalf("notes = %+v", notes)
	}
}
