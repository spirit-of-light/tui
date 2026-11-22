package tui

import (
	"context"
	"errors"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ZenNotes/tui/internal/backend"
	"github.com/ZenNotes/tui/internal/snippet"
)

// snippetsDir is the vault folder that holds the snippet group files.
const snippetsDir = ".zennotes/snippets"

// snippetsLoadedMsg carries the result of one snippet load. The epoch ties it
// to the vault that started the load.
type snippetsLoadedMsg struct {
	epoch  *int
	index  *snippet.Index
	remote bool
	err    error
}

// loadSnippetsCmd reads the group files through the backend and builds the
// index. A bad snippet is a problem of the index and never an error here.
func (a *App) loadSnippetsCmd() tea.Cmd {
	b, epoch := a.backend, a.epoch
	return func() tea.Msg {
		files, err := b.ListSnippetFiles(context.Background())
		if errors.Is(err, backend.ErrSnippetsNotSupported) {
			return snippetsLoadedMsg{epoch: epoch, remote: true}
		}
		if err != nil {
			return snippetsLoadedMsg{epoch: epoch, err: err}
		}
		groups := make([]snippet.GroupFile, 0, len(files))
		for _, f := range files {
			groups = append(groups, snippet.GroupFile{Name: f.Name, Path: f.Path, Raw: f.Raw, Err: f.Err})
		}
		return snippetsLoadedMsg{epoch: epoch, index: snippet.BuildIndex(groups)}
	}
}

// applySnippets stores a finished load. It shows nothing in the editor: the
// problems of the files are for `zn doctor`. A load that failed keeps the
// index that zn already has.
func (a *App) applySnippets(m snippetsLoadedMsg) {
	if m.epoch != a.epoch || m.err != nil {
		return
	}
	a.snippetIndex = m.index
	a.snippetsRemote = m.remote
}

// resetSnippets drops the snippets of the vault that zn leaves.
func (a *App) resetSnippets() {
	a.snippetIndex = nil
	a.snippetsRemote = false
}

// snippetPathChanged reports whether a watcher batch has the snippets folder
// or a path under it.
func snippetPathChanged(paths []string) bool {
	for _, p := range paths {
		if p == snippetsDir || strings.HasPrefix(p, snippetsDir+"/") {
			return true
		}
	}
	return false
}
