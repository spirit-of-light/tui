package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ZenNotes/tui/internal/config"
	"github.com/ZenNotes/tui/internal/snippets"
	"github.com/ZenNotes/tui/internal/templates"
)

// Snippets are the user's own, kept in snippets.toml in their config directory
// and expanded in the terminal editor, so they are not vault content: the
// tools read and write that file and never open a vault. A hosted server
// (one that serves other people's vaults) does not offer them.

// snippetSyntax is what an agent needs to write a snippet body. It is appended
// to the instructions of a local server, and the tool descriptions repeat the
// essentials.
const snippetSyntax = `## Snippets

The user's terminal editor expands snippets: in insert mode they type a trigger
word and press Ctrl+N, or pick it from the menu that opens as they type.
Snippets live in the user's own snippets.toml, not in the vault. Use
snippets_list to see them and snippet_create to add one.

A snippet body is plain text with these tabstops:
- $1, $2, ... are stops in the order the user moves through them with Ctrl+L
  (Ctrl+H goes back). Repeating a number mirrors it: whatever is typed in one
  copy appears in all.
- ${1:default} is a stop that starts with default text. Typing replaces it.
- $0 is the final stop, where the cursor ends. Without $0 it is the end of the text.
- {{title}}, {{date}}, {{time}}, {{week}} and {{date:FORMAT}} are filled in when expanded.
- Write \$ for a literal dollar sign and \} for a literal brace inside a placeholder.

Good snippets are short, use a trigger the user would type (lowercase, no
spaces), and put the cursor where typing starts. Give each a description.
`

var snippetToolNames = map[string]bool{"snippets_list": true, "snippet_create": true}

// snippetTools are the snippet tools, listed with the others by tools().
func snippetTools() []toolDef {
	return []toolDef{
		{
			name: "snippets_list",
			description: "List the user's snippets: each trigger word, its description and body, with the file they live in. " +
				"Read-only. Use it before snippet_create so you do not duplicate a trigger. " + snippetToolSyntaxHint,
			schema:  `{"type":"object","properties":{}}`,
			handler: snippetsListHandler,
		},
		{
			name: "snippet_create",
			description: "Add a snippet to the user's snippets.toml. The user types the trigger in the editor and presses Ctrl+N " +
				"(or picks it from the menu). The body uses tabstops: $1, ${1:default}, $0 for the final cursor, and repeated numbers " +
				"mirror. Refuses an existing trigger unless replace is true. Returns the expansion as the editor will insert it, " +
				"so you can check it. " + snippetToolSyntaxHint,
			schema: `{"type":"object","properties":{
"trigger":{"type":"string","description":"The word typed before Ctrl+N. Letters, digits, _ and - only, and it must be unique."},
"body":{"type":"string","description":"The snippet text with tabstops, for example [${1:label}](${2:url})$0."},
"description":{"type":"string","description":"One line shown beside the trigger in the menu."},
"replace":{"type":"boolean","description":"Replace the snippet if the trigger already exists. Default false."}},
"required":["trigger","body"]}`,
			handler: snippetCreateHandler,
		},
	}
}

const snippetToolSyntaxHint = "Tabstops: $1 and ${1:default} in order, $0 last, repeated numbers mirror, {{title}} and {{date}} variables, \\$ for a literal dollar."

// snippetInstructionsFor is the instruction text a local server appends.
func snippetInstructionsFor() string { return "\n\n" + snippetSyntax }

func snippetsListHandler(_ context.Context, _ args, _ Backend) (any, error) {
	path := config.SnippetsPath()
	lib, errs := snippets.LoadLibrary(path)
	type item struct {
		Trigger     string `json:"trigger"`
		Description string `json:"description,omitempty"`
		Body        string `json:"body"`
	}
	items := []item{}
	for _, t := range lib.Triggers() {
		s, _ := lib.Lookup(t)
		items = append(items, item{Trigger: s.Trigger, Description: s.Description, Body: s.Body})
	}
	out := map[string]any{"path": path, "snippets": items}
	if len(errs) > 0 {
		warnings := make([]string, 0, len(errs))
		for _, e := range errs {
			warnings = append(warnings, e.Error())
		}
		out["warnings"] = warnings
	}
	return out, nil
}

func snippetCreateHandler(_ context.Context, a args, _ Backend) (any, error) {
	trigger, err := a.requireString("trigger")
	if err != nil {
		return nil, err
	}
	trigger = strings.TrimSpace(trigger)
	if !snippets.ValidTrigger(trigger) {
		return nil, fmt.Errorf("trigger %q may only use letters, digits, _ and -", trigger)
	}
	body, err := a.requireString("body")
	if err != nil {
		return nil, err
	}
	description, _, err := a.optionalString("description")
	if err != nil {
		return nil, err
	}
	replace := a.boolIs("replace")

	path := config.SnippetsPath()
	lib, errs := snippets.LoadLibrary(path)
	if len(errs) > 0 {
		// Writing back a file that has errors would silently drop the bad entries.
		return nil, fmt.Errorf("snippets.toml has errors that must be fixed first: %v", errs[0])
	}
	if _, exists := lib[trigger]; exists && !replace {
		return nil, fmt.Errorf("a snippet with trigger %q already exists; pass replace true to change it", trigger)
	}
	snip := snippets.Snippet{Trigger: trigger, Description: strings.TrimSpace(description), Body: strings.TrimSuffix(body, "\n")}
	lib[trigger] = snip
	if err := snippets.SaveLibrary(path, lib); err != nil {
		return nil, err
	}
	exp := snippets.Parse(snip.Body, templates.Vars("", time.Now(), "en"))
	return map[string]any{
		"path":     path,
		"snippet":  snip,
		"expands":  exp.Text,
		"tabstops": tabstopSummary(exp),
	}, nil
}

// tabstopSummary lists the stops in navigation order, for the agent to check.
func tabstopSummary(exp snippets.Expansion) []string {
	out := make([]string, 0, len(exp.Stops))
	for _, st := range exp.Stops {
		label := fmt.Sprintf("$%d", st.Index)
		if st.At.End > st.At.Start {
			label += fmt.Sprintf(" %q", string([]rune(exp.Text)[st.At.Start:st.At.End]))
		}
		if len(st.Mirrors) > 0 {
			label += fmt.Sprintf(" (mirrored %dx)", len(st.Mirrors))
		}
		out = append(out, label)
	}
	return out
}
