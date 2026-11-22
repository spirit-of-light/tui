package vault

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Snippet group files are plain `.md` files directly in the flat
// `.zennotes/snippets/` directory. Like templates, they are not notes: the
// note layer refuses `.zennotes/`, so this layer reads them. It moves raw
// text only; the snippet package owns the file format.

const snippetsRelDir = ".zennotes/snippets"

// SnippetFile is one snippet group file. When Err is set, zn could not read
// the file and Raw is empty.
type SnippetFile struct {
	// Name is the file name, for example "My Notes.md".
	Name string
	// Path is the vault-relative path with forward slashes.
	Path string
	// Raw is the text of the file.
	Raw string
	// Err is the error from the read.
	Err error
}

// ListSnippetFiles returns the regular `.md` files directly in the snippets
// directory, in byte order of their names. Hidden files, other extensions and
// subdirectories are not listed. A file that cannot be read stays in the list
// with its error, so the caller can report it. A path is resolved inside the
// vault, so a link that leaves the vault is never read. A missing directory
// gives an empty list and no error.
func (v *Vault) ListSnippetFiles() ([]SnippetFile, error) {
	entries, err := os.ReadDir(filepath.Join(v.root, ".zennotes", "snippets"))
	if errors.Is(err, os.ErrNotExist) {
		return []SnippetFile{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]SnippetFile, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || strings.HasPrefix(name, ".") || !strings.EqualFold(filepath.Ext(name), ".md") {
			continue
		}
		file := SnippetFile{Name: name, Path: snippetsRelDir + "/" + name}
		abs, err := SafeJoin(v.root, file.Path)
		if err != nil {
			file.Err = err
			out = append(out, file)
			continue
		}
		if info, statErr := os.Stat(abs); statErr == nil && !info.Mode().IsRegular() {
			continue
		}
		raw, err := os.ReadFile(abs)
		if err != nil {
			file.Err = err
			out = append(out, file)
			continue
		}
		file.Raw = string(raw)
		out = append(out, file)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
