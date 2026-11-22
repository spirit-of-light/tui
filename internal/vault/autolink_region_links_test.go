package vault

import "testing"

// autolinkRegions holds a link inside each region that autolink leaves alone.
var autolinkRegions = []struct{ name, region string }{
	{"heading", "## More on [[X]]"},
	{"first heading", "# Title [[X]]"},
	{"callout title", "> [!note] [[X]]"},
	{"html comment", "<!-- old [[X]] -->"},
	{"percent comment", "%% old [[X]] %%"},
	{"math block", "$$\n[[X]]\n$$"},
	{"environment", "\\begin{align}\n[[X]]\n\\end{align}"},
	{"inline math", "$a [[X]] b$"},
	{"html element", "<span>[[X]]</span>"},
	{"markdown link in heading", "## More on [label](X.md)"},
}

func TestAutolinkRegionLinkDoesNotMove(t *testing.T) {
	x := autolinkNote("inbox/X.md", "X")
	for _, c := range autolinkRegions {
		for _, remove := range []bool{false, true} {
			t.Run(c.name, func(t *testing.T) {
				body := c.region + "\nX first."
				want := c.region + "\n[[X]] first."
				if _, got := autolinkPlanWith(body, remove, x); got != want {
					t.Errorf("remove=%v: got %q, want %q", remove, got, want)
				}
			})
		}
	}
}

func TestAutolinkRegionLinkIsNotRemoved(t *testing.T) {
	x := autolinkNote("inbox/X.md", "X")
	for _, c := range autolinkRegions {
		t.Run(c.name, func(t *testing.T) {
			body := "See [[X]].\n\n" + c.region
			if changes, got := autolinkPlanWith(body, true, x); got != body {
				t.Errorf("got %q, want %q (changes %+v)", got, body, changes)
			}
		})
	}
}
