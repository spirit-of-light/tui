package vault

import (
	"strings"
	"testing"
)

// maskedAt reports whether every byte of the nth occurrence of needle is masked.
func maskedAt(t *testing.T, body string, mask []bool, needle string, occurrence int) bool {
	t.Helper()
	pos, from := -1, 0
	for k := 0; k <= occurrence; k++ {
		i := strings.Index(body[from:], needle)
		if i < 0 {
			t.Fatalf("occurrence %d of %q not in %q", occurrence, needle, body)
		}
		pos = from + i
		from = pos + len(needle)
	}
	for i := pos; i < pos+len(needle); i++ {
		if !mask[i] {
			return false
		}
	}
	return true
}

func TestAutolinkMaskRegions(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"frontmatter", "---\ntitle: X\n---\nprose MARK\n"},
		{"fenced code", "```\nX\n```\nprose MARK\n"},
		{"inline code", "`X` and prose MARK\n"},
		{"inline math", "$X + 1$ and prose MARK\n"},
		{"display math", "$$\nX\n$$\nprose MARK\n"},
		{"environment", "\\begin{align}\nX\n\\end{align}\nprose MARK\n"},
		{"heading", "## About X\nprose MARK\n"},
		{"leading title", "# X\nprose MARK\n"},
		{"url", "see https://example.com/X now MARK\n"},
		{"bare autolink", "see <https://example.com/X> now MARK\n"},
		{"html element", "see <span>X</span> now MARK\n"},
		{"table", "| a | X |\n|---|---|\nprose MARK\n"},
		{"html comment", "<!-- X\nmore -->\nprose MARK\n"},
		{"percent comment", "%% X\nmore %%\nprose MARK\n"},
		{"task line", "- [ ] do X\n- [x] did X\nprose MARK\n"},
		{"callout title", "> [!note] About X\nprose MARK\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mask := autolinkMask(c.body)
			if len(mask) != len(c.body) {
				t.Fatalf("mask length %d, body length %d", len(mask), len(c.body))
			}
			if !maskedAt(t, c.body, mask, "X", 0) {
				t.Fatalf("X is not masked in %q", c.body)
			}
			if maskedAt(t, c.body, mask, "MARK", 0) {
				t.Fatalf("prose after the region is masked in %q", c.body)
			}
		})
	}
}

func TestAutolinkMaskCurrencyIsText(t *testing.T) {
	body := "costs $5 and X costs $10\n"
	if autolinkMask(body)[strings.Index(body, "X")] {
		t.Fatalf("X is masked in %q", body)
	}
}

func TestAutolinkMaskInlineMath(t *testing.T) {
	for _, body := range []string{"a $x$ b\n", "a $a+b$ b\n"} {
		mask := autolinkMask(body)
		i := strings.Index(body, "$")
		j := strings.LastIndex(body, "$")
		for k := i; k <= j; k++ {
			if !mask[k] {
				t.Fatalf("byte %d is not masked in %q", k, body)
			}
		}
		if mask[0] || mask[len(body)-2] {
			t.Fatalf("text around the math is masked in %q", body)
		}
	}
}

func TestAutolinkMaskLinkSpans(t *testing.T) {
	for _, body := range []string{"[[Z|about X]]", "[[X]]", "![[X]]", "[about X](Z.md)"} {
		mask := autolinkMask(body + "\nprose MARK\n")
		for k := 0; k < len(body); k++ {
			if !mask[k] {
				t.Fatalf("byte %d of %q is not masked", k, body)
			}
		}
		if mask[len(body)+2] {
			t.Fatalf("prose after %q is masked", body)
		}
	}
}

func TestAutolinkMaskUnclosedRegion(t *testing.T) {
	for _, body := range []string{"a\n$$\nX\n", "a\n<!-- X\n", "a\n%% X\n", "a\n\\begin{align}\nX\n", "a\n```\nX\n"} {
		mask := autolinkMask(body)
		if !maskedAt(t, body, mask, "X", 0) {
			t.Fatalf("X is not masked in %q", body)
		}
		if mask[0] {
			t.Fatalf("text before the region is masked in %q", body)
		}
	}
}

func TestAutolinkMaskLinks(t *testing.T) {
	body := "---\n[[F]]\n---\n[[A#h|shown]] ![[E]] [lbl](B.md) `[[C]]`\n"
	links := autolinkLinks(body)
	if len(links) != 3 {
		t.Fatalf("got %d links, want 3: %+v", len(links), links)
	}
	if l := links[0]; !l.Wiki || l.Embed || l.Target != "A" || l.Anchor != "#h" || l.Text != "shown" {
		t.Fatalf("first link %+v", l)
	}
	if !links[1].Embed {
		t.Fatalf("second link is not an embed: %+v", links[1])
	}
	if l := links[2]; l.Wiki || l.Target != "B.md" || l.Text != "lbl" {
		t.Fatalf("third link %+v", l)
	}
	if body[links[2].Start:links[2].End] != "[lbl](B.md)" {
		t.Fatalf("span %q", body[links[2].Start:links[2].End])
	}
}
