package backend

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestBackendSnippetFilesLocal(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".zennotes", "snippets")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw := "## link\n\n```\n[$1]($2)\n```\n"
	if err := os.WriteFile(filepath.Join(dir, "markdown.md"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := New(Target{Kind: KindLocal, Root: root}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	files, err := b.ListSnippetFiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Name != "markdown.md" || files[0].Raw != raw || files[0].Err != nil {
		t.Fatalf("%+v", files)
	}
}

func TestBackendSnippetFilesRemote(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer srv.Close()
	b, err := New(Target{Kind: KindRemote, BaseURL: srv.URL, AuthToken: "tok"}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	files, err := b.ListSnippetFiles(context.Background())
	if !errors.Is(err, ErrSnippetsNotSupported) || !errors.Is(err, errors.ErrUnsupported) || files != nil {
		t.Fatalf("%v %v", files, err)
	}
	if n := requests.Load(); n != 0 {
		t.Fatalf("the remote backend sent %d requests", n)
	}
}
