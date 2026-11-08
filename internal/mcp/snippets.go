package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/ZenNotes/tui/internal/config"
	"github.com/ZenNotes/tui/internal/snippets"
)

// snippetTools provides MCP tools for managing snippets.
func snippetTools() []toolDef {
	return []toolDef{
		{
			name:        "snippets_list",
			description: "List all available snippets with their triggers, descriptions, and bodies. Useful for discovering what snippets are available.",
			schema:      `{"type":"object","properties":{}}`,
			handler: func(ctx context.Context, _ args, b Backend) (any, error) {
				lib, _ := loadSnippetLibrary()
				var results []map[string]interface{}
				for _, trigger := range lib.Triggers() {
					sn, _ := lib.Lookup(trigger)
					preview := sn.Body
					if len([]rune(preview)) > 100 {
						preview = string([]rune(preview)[:97]) + "…"
					}
					results = append(results, map[string]interface{}{
						"trigger":     sn.Trigger,
						"description": sn.Description,
						"preview":     preview,
					})
				}
				return map[string]interface{}{"snippets": results}, nil
			},
		},
		{
			name:        "snippet_create",
			description: "Create a new snippet. The trigger must be a word (letters, digits, underscore, hyphen only) and cannot be empty. If a snippet with this trigger already exists, it will be updated.",
			schema: `{
				"type": "object",
				"properties": {
					"trigger": {"type": "string", "description": "The trigger word (e.g., 'snip')"},
					"description": {"type": "string", "description": "A brief description of what the snippet does"},
					"body": {"type": "string", "description": "The snippet body with VS Code style placeholders like $1, $0, ${1:default}"}
				},
				"required": ["trigger", "body"]
			}`,
			handler: func(ctx context.Context, args args, b Backend) (any, error) {
				trigger, ok := args.String("trigger")
				if !ok || trigger == "" {
					return nil, fmt.Errorf("trigger is required and must not be empty")
				}
				if !snippets.IsWordRune('_') {
					return nil, fmt.Errorf("trigger must contain only letters, digits, underscore and hyphen")
				}
				for _, r := range trigger {
					if !snippets.IsWordRune(r) {
						return nil, fmt.Errorf("trigger must contain only letters, digits, underscore and hyphen, got %q", trigger)
					}
				}

				body, ok := args.String("body")
				if !ok || body == "" {
					return nil, fmt.Errorf("body is required and must not be empty")
				}

				desc, _ := args.String("description")

				lib, _ := loadSnippetLibrary()
				lib[trigger] = snippets.Snippet{
					Trigger:     trigger,
					Description: desc,
					Body:        body,
				}

				if err := saveSnippetLibrary(lib); err != nil {
					return nil, fmt.Errorf("failed to save snippet: %w", err)
				}

				return map[string]interface{}{
					"created": true,
					"trigger": trigger,
				}, nil
			},
		},
		{
			name:        "snippet_delete",
			description: "Delete a snippet by its trigger word.",
			schema: `{
				"type": "object",
				"properties": {
					"trigger": {"type": "string", "description": "The trigger word to delete"}
				},
				"required": ["trigger"]
			}`,
			handler: func(ctx context.Context, args args, b Backend) (any, error) {
				trigger, ok := args.String("trigger")
				if !ok || trigger == "" {
					return nil, fmt.Errorf("trigger is required")
				}

				lib, _ := loadSnippetLibrary()
				if _, ok := lib.Lookup(trigger); !ok {
					return nil, fmt.Errorf("snippet %q not found", trigger)
				}

				delete(lib, trigger)
				if err := saveSnippetLibrary(lib); err != nil {
					return nil, fmt.Errorf("failed to delete snippet: %w", err)
				}

				return map[string]interface{}{
					"deleted": true,
					"trigger": trigger,
				}, nil
			},
		},
	}
}

// loadSnippetLibrary loads the snippet library from the config directory.
func loadSnippetLibrary() (snippets.Library, error) {
	path := filepath.Join(config.PortableConfigDir(), "snippets.toml")
	return snippets.LoadLibrary(path)
}

// saveSnippetLibrary saves the library to snippets.toml.
func saveSnippetLibrary(lib snippets.Library) error {
	path := filepath.Join(config.PortableConfigDir(), "snippets.toml")

	// Convert library to TOML format
	type snippetEntry struct {
		Trigger     string `json:"trigger"`
		Description string `json:"description,omitempty"`
		Body        string `json:"body"`
	}

	var entries []snippetEntry
	for _, trigger := range lib.Triggers() {
		sn, _ := lib.Lookup(trigger)
		entries = append(entries, snippetEntry{
			Trigger:     sn.Trigger,
			Description: sn.Description,
			Body:        sn.Body,
		})
	}

	// Build TOML format
	tomlContent := ""
	for _, entry := range entries {
		tomlContent += fmt.Sprintf("\n[[snippet]]\n")
		tomlContent += fmt.Sprintf("trigger = %q\n", entry.Trigger)
		if entry.Description != "" {
			tomlContent += fmt.Sprintf("description = %q\n", entry.Description)
		}
		tomlContent += fmt.Sprintf("body = '''\n%s\n'''\n", entry.Body)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(tomlContent), 0644)
}
