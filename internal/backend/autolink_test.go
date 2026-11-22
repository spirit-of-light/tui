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

func autolinkVault(t *testing.T, files map[string]string) (Backend, string) {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	b, err := New(Target{Kind: KindLocal, Root: root}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return b, root
}

func autolinkFiles() map[string]string {
	return map[string]string{
		".zennotes/vault.json":   `{"templates":{"folder":"inbox/Templates"}}`,
		"inbox/X.md":             "# X\n",
		"inbox/Y.md":             "Y mentions X here.\n",
		"inbox/Z.md":             "Nothing to link.\n",
		"inbox/Templates/T.md":   "T mentions X.\n",
		"inbox/typst/P.md":       "P mentions X.\n",
		"archive/B.md":           "B mentions X.\n",
		"inbox/Templates/Tpl.md": "# Tpl\n",
		"inbox/typst/Pre.md":     "# Pre\n",
		"archive/Old.md":         "# Old\n",
	}
}

func TestAutolinkRunPlansLiveNotesOnly(t *testing.T) {
	files := autolinkFiles()
	files["inbox/Y.md"] = "Y mentions X, Tpl, Pre and Old here.\n"
	b, _ := autolinkVault(t, files)

	run, err := PlanAutolinkRun(context.Background(), b, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Plans) != 1 || run.Plans[0].Path != "inbox/Y.md" {
		t.Fatalf("plans: %+v", run.Plans)
	}
	changes := run.Plans[0].Changes
	if len(changes) != 1 || changes[0].Target != "inbox/X.md" {
		t.Fatalf("changes: %+v", changes)
	}
}

func TestAutolinkRunOnePath(t *testing.T) {
	b, _ := autolinkVault(t, autolinkFiles())
	ctx := context.Background()
	run, err := PlanAutolinkRun(ctx, b, []string{"inbox/Y.md"}, true)
	if err != nil || len(run.Plans) != 1 {
		t.Fatalf("%+v %v", run.Plans, err)
	}
	if _, err := PlanAutolinkRun(ctx, b, []string{"archive/B.md"}, true); err == nil {
		t.Fatal("an archived note must not be a source")
	}
	if _, err := PlanAutolinkRun(ctx, b, []string{"inbox/Templates/T.md"}, true); err == nil {
		t.Fatal("a template note must not be a source")
	}
	if _, err := PlanAutolinkRun(ctx, b, []string{"inbox/typst/P.md"}, true); err == nil {
		t.Fatal("a preamble note must not be a source")
	}
	if _, err := PlanAutolinkRun(ctx, b, []string{"inbox/None.md"}, true); err == nil {
		t.Fatal("a missing note must give an error")
	}
}

func TestAutolinkRunWritesOnlyAcceptedNotes(t *testing.T) {
	files := autolinkFiles()
	files["inbox/W.md"] = "W mentions X too.\n"
	b, root := autolinkVault(t, files)
	ctx := context.Background()

	run, err := PlanAutolinkRun(ctx, b, nil, true)
	if err != nil || len(run.Plans) != 2 {
		t.Fatalf("%+v %v", run.Plans, err)
	}
	skip := run.Plans[1].Path
	written, err := WriteAutolinks(ctx, b, run.Plans, func(plan, _ int) bool { return plan == 0 })
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 1 || written[0] != run.Plans[0].Path {
		t.Fatalf("written: %v", written)
	}
	read := func(rel string) string {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	if got := read(run.Plans[0].Path); got == files[run.Plans[0].Path] {
		t.Fatalf("accepted note did not change: %q", got)
	}
	if got := read(skip); got != files[skip] {
		t.Fatalf("omitted note changed: %q", got)
	}
	if got := read("inbox/Z.md"); got != files["inbox/Z.md"] {
		t.Fatalf("a note with no change was written: %q", got)
	}
	if got := read("archive/B.md"); got != files["archive/B.md"] {
		t.Fatalf("an archived note changed: %q", got)
	}
}

func TestAutolinkRunRefusesAServer(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		http.Error(w, "no", http.StatusInternalServerError)
	}))
	defer srv.Close()
	b, err := New(Target{Kind: KindRemote, BaseURL: srv.URL}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := PlanAutolinkRun(ctx, b, nil, true); !errors.Is(err, ErrAutolinkRemote) {
		t.Fatalf("plan error: %v", err)
	}
	if _, err := WriteAutolinks(ctx, b, nil, nil); !errors.Is(err, ErrAutolinkRemote) {
		t.Fatalf("write error: %v", err)
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("the server got %d requests", n)
	}
}
