package config

import (
	"fmt"
	"sort"

	"github.com/ZenNotes/tui/internal/keymaps"
)

// readTerminalKeymaps reads the [terminal.keymaps] table. A string is one
// key, a list of strings is a list of keys, and "" or [] unbinds the action.
// An entry that zn cannot use is left out and named in the problems, which
// come back sorted so the order does not change between loads. A value that
// is not a table gives no override and one problem.
func readTerminalKeymaps(raw any) (map[string][]string, []string) {
	overrides := map[string][]string{}
	if raw == nil {
		return overrides, nil
	}
	table, ok := raw.(map[string]any)
	if !ok {
		return overrides, []string{"keymaps: the value must be a table of actions"}
	}
	known := map[string]bool{}
	for _, e := range keymaps.TerminalCatalog {
		known[e.ID] = true
	}
	var problems []string
	for id, value := range table {
		if !known[id] {
			problems = append(problems, id+": zn does not know this action")
			continue
		}
		keys, ok := terminalKeyList(value)
		if !ok {
			problems = append(problems, id+": the value must be a string or a list of strings")
			continue
		}
		bad := ""
		for _, key := range keys {
			if _, parsed := keymaps.ParseKeys(key); !parsed {
				bad = key
				break
			}
		}
		if bad != "" {
			problems = append(problems, fmt.Sprintf("%s: zn cannot read the key %q", id, bad))
			continue
		}
		overrides[id] = keys
	}
	sort.Strings(problems)
	return overrides, problems
}

// terminalKeyList turns a TOML value into key strings. A blank string and
// blank list items give no key.
func terminalKeyList(value any) ([]string, bool) {
	keys := []string{}
	switch v := value.(type) {
	case string:
		if v != "" {
			keys = append(keys, v)
		}
	case []any:
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			if s != "" {
				keys = append(keys, s)
			}
		}
	case []string:
		for _, s := range v {
			if s != "" {
				keys = append(keys, s)
			}
		}
	default:
		return nil, false
	}
	return keys, true
}
