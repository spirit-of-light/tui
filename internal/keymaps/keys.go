package keymaps

import (
	"strings"
	"unicode"

	"github.com/ZenNotes/tui/internal/vim"
)

// ParseKeys parses a binding like `Alt+H`, `Ctrl+P`, `g t`, `Space` or
// `Alt+ArrowLeft` into a key sequence. An empty binding gives no keys and
// true. A binding with a part that zn cannot read gives nil and false.
func ParseKeys(binding string) ([]vim.Key, bool) {
	binding = strings.TrimSpace(binding)
	if binding == "" {
		return nil, true
	}
	out := []vim.Key{}
	for _, chord := range strings.Fields(binding) {
		parts := strings.Split(chord, "+")
		k := vim.Key{}
		base := parts[len(parts)-1]
		for _, mod := range parts[:len(parts)-1] {
			switch strings.ToLower(mod) {
			case "ctrl", "mod":
				k.Ctrl = true
			case "alt", "option", "meta":
				k.Alt = true
			case "shift":
				k.Shift = true
			}
		}
		switch strings.ToLower(base) {
		case "space":
			k.Rune = ' '
		case "enter", "return":
			k.Name = "enter"
		case "esc", "escape":
			k.Name = "esc"
		case "tab":
			k.Name = "tab"
		case "arrowleft", "left":
			k.Name = "left"
		case "arrowright", "right":
			k.Name = "right"
		case "arrowup", "up":
			k.Name = "up"
		case "arrowdown", "down":
			k.Name = "down"
		case "backspace":
			k.Name = "backspace"
		case "delete":
			k.Name = "delete"
		default:
			r := []rune(base)
			if len(r) != 1 {
				return nil, false
			}
			if k.Ctrl || k.Alt {
				k.Rune = unicode.ToLower(r[0])
				if k.Shift && !k.Ctrl {
					k.Rune = unicode.ToUpper(r[0])
				}
				if k.Ctrl {
					k.Shift = false
				}
			} else {
				k.Rune = r[0]
				if k.Shift {
					k.Rune = unicode.ToUpper(r[0])
					k.Shift = false
				}
			}
		}
		out = append(out, k)
	}
	return out, true
}
