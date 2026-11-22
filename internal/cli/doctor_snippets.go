package cli

import (
	"context"
	"errors"
	"fmt"
	"path"

	"github.com/ZenNotes/tui/internal/backend"
	"github.com/ZenNotes/tui/internal/snippet"
)

// snippetProblem is the structured part of a duplicate snippet trigger.
type snippetProblem struct {
	Trigger     string `json:"trigger"`
	UsedPath    string `json:"usedPath"`
	UsedLine    int    `json:"usedLine"`
	IgnoredPath string `json:"ignoredPath"`
	IgnoredLine int    `json:"ignoredLine"`
}

// doctorSnippetChecks reads the snippet group files through the backend. It gives one
// warning check for each bad snippet, each unreadable file and each duplicate
// trigger, and no check when there is no problem. A remote vault gives one
// check that says zn skipped the snippets.
func doctorSnippetChecks(ctx context.Context, b backend.Backend) []diagnosticCheck {
	files, err := b.ListSnippetFiles(ctx)
	if errors.Is(err, backend.ErrSnippetsNotSupported) {
		return []diagnosticCheck{{
			Name:    "snippets",
			OK:      true,
			Message: "zn skipped the snippet check. Snippets are not available for a remote vault.",
		}}
	}
	warn := func(message string) diagnosticCheck {
		return diagnosticCheck{Name: "snippets", OK: true, Warning: true, Message: message}
	}
	if err != nil {
		return []diagnosticCheck{warn(".zennotes/snippets: zn cannot read this folder. zn ignores the snippets.")}
	}
	groups := make([]snippet.GroupFile, 0, len(files))
	for _, f := range files {
		groups = append(groups, snippet.GroupFile{Name: f.Name, Path: f.Path, Raw: f.Raw, Err: f.Err})
	}
	idx := snippet.BuildIndex(groups)
	var out []diagnosticCheck
	for _, p := range idx.Problems() {
		switch p.Kind {
		case snippet.ProblemUnreadable:
			out = append(out, warn(fmt.Sprintf("%s: zn cannot read this file. zn ignores this file.", p.Path)))
		case snippet.ProblemNotOneWord:
			out = append(out, warn(fmt.Sprintf("%s line %d: The heading %q is not one word. zn ignores this snippet.", p.Path, p.Line, p.Heading)))
		case snippet.ProblemNoFence:
			out = append(out, warn(fmt.Sprintf("%s line %d: The snippet %q has no fenced block. zn ignores this snippet.", p.Path, p.Line, p.Heading)))
		case snippet.ProblemUnclosedFence:
			out = append(out, warn(fmt.Sprintf("%s line %d: The fenced block of the snippet %q does not close. zn ignores this snippet.", p.Path, p.Line, p.Heading)))
		}
	}
	for _, d := range idx.Duplicates() {
		c := warn(fmt.Sprintf("The trigger %q is in %s line %d and in %s line %d. zn uses %s line %d and ignores %s line %d.",
			d.Trigger, d.UsedPath, d.UsedLine, d.IgnoredPath, d.IgnoredLine,
			path.Base(d.UsedPath), d.UsedLine, path.Base(d.IgnoredPath), d.IgnoredLine))
		c.Snippet = &snippetProblem{
			Trigger: d.Trigger, UsedPath: d.UsedPath, UsedLine: d.UsedLine,
			IgnoredPath: d.IgnoredPath, IgnoredLine: d.IgnoredLine,
		}
		out = append(out, c)
	}
	return out
}
