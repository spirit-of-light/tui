package tui

import (
	"context"
	"testing"

	"github.com/ZenNotes/tui/internal/backend"
	"github.com/ZenNotes/tui/internal/vault"
	"github.com/ZenNotes/tui/internal/vim"
)

// noSnippetsBackend answers the snippet read like a remote vault does.
type noSnippetsBackend struct{ backend.Backend }

func (noSnippetsBackend) ListSnippetFiles(context.Context) ([]vault.SnippetFile, error) {
	return nil, backend.ErrSnippetsNotSupported
}

// remoteTriggerApp gives a triggerApp whose backend has no snippets.
func remoteTriggerApp(t *testing.T, vimOn bool) (*App, *noteBuffer, vim.Key) {
	t.Helper()
	a, buf, key := triggerApp(t, vimOn, nil)
	a.backend = noSnippetsBackend{a.backend}
	loadSnippets(a)
	if !a.snippetsRemote {
		t.Fatal("the remote flag is not set")
	}
	return a, buf, key
}

func TestSnippetRemoteMessageOnce(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		a, _, key := remoteTriggerApp(t, vimOn)
		a.message, a.toasts = "", nil
		typeKeys(a, "wor")
		feed(a, key)
		if a.message != snippetsRemoteMessage {
			t.Fatalf("message = %q, want %q", a.message, snippetsRemoteMessage)
		}
		if a.snippetPopup != nil {
			t.Error("a popup opened for a remote vault")
		}
		a.message, a.toasts = "", nil
		typeKeys(a, "x")
		feed(a, key)
		if a.message != "" || len(a.toasts) != 0 {
			t.Errorf("second key: message %q, toasts %v; want nothing", a.message, a.toasts)
		}
	})
}

func TestSnippetRemoteKeepsTheKey(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		typed := "wor"
		if !vimOn {
			typed = "xy"
		}
		a, buf, key := remoteTriggerApp(t, vimOn)
		typeKeys(a, typed)
		feed(a, key)
		want, wantPos := triggerBaseline(t, vimOn, typed)
		if buf.ed.Text() != want || buf.ed.Cursor() != wantPos {
			t.Errorf("%q at %v, baseline %q at %v", buf.ed.Text(), buf.ed.Cursor(), want, wantPos)
		}
	})
}

func TestSnippetRemoteNoMessageAfterSpace(t *testing.T) {
	bothModes(t, func(t *testing.T, vimOn bool) {
		a, _, key := remoteTriggerApp(t, vimOn)
		a.message, a.toasts = "", nil
		typeKeys(a, "wor ")
		feed(a, key)
		if a.message != "" || len(a.toasts) != 0 {
			t.Errorf("message %q, toasts %v; want nothing", a.message, a.toasts)
		}
	})
}

func TestSnippetRemoteSwitchKeepsTheFlag(t *testing.T) {
	a, _, key := remoteTriggerApp(t, true)
	typeKeys(a, "wor")
	feed(a, key)
	a.resetSnippets()
	a.snippetsRemote = true
	a.message = ""
	typeKeys(a, "x")
	feed(a, key)
	if a.message != "" {
		t.Errorf("message = %q after a vault switch, want none", a.message)
	}
}
