package vault

import "testing"

func autolinkPlanWith(body string, remove bool, notes ...NoteMeta) ([]AutolinkChange, string) {
	all := append([]NoteMeta{autolinkNote("inbox/Y.md", "Y")}, notes...)
	changes := PlanAutolinks(body, "inbox/Y.md", all, AutolinkOptions{RemoveDuplicates: remove})
	return changes, ApplyAutolinks(body, changes, nil)
}

func TestAutolinkMove(t *testing.T) {
	x := autolinkNote("inbox/X.md", "X")
	cases := []struct {
		name   string
		body   string
		remove bool
		want   string
	}{
		{"alias text", "X here\n[[X|later text]]", true, "[[X]] here\nlater text"},
		{"lower case target", "X here\n[[x]]", true, "[[X]] here\nx"},
		{"markdown label", "X here\n[label](X.md)", true, "[[X]] here\nlabel"},
		{"link first", "[label](X.md)\nX here", true, "[label](X.md)\nX here"},
		{"move without remove", "X here\n[[X|later text]]", false, "[[X]] here\nlater text"},
		{"duplicate wikilinks", "[[X]] and [[X]]", true, "[[X]] and X"},
		{"duplicate markdown", "[[X]] and [label](X.md)", true, "[[X]] and label"},
		{"duplicates stay", "[[X]] and [[X]]", false, "[[X]] and [[X]]"},
		{"plain references after a link", "[[X]]\nX and X", false, "[[X]]\nX and X"},
		{"move keeps later link", "X here\n[[X|later text]]\n[[X]]", false, "[[X]] here\nlater text\n[[X]]"},
		{"move keeps later markdown link", "X here\n[label](X.md)", false, "[[X]] here\nlabel"},
		{"table link", "| [[X\\|text]] |\n| - |\nX here", true, "| [[X\\|text]] |\n| - |\n[[X]] here"},
		{"task link", "- [ ] read [[X]]\nX here", true, "- [ ] read [[X]]\n[[X]] here"},
		{"table link after prose", "X here\n| [[X]] |\n| - |", true, "[[X]] here\n| [[X]] |\n| - |"},
		{"task link before prose link", "- [ ] [[X]]\n[[X]] and [[X]]", true, "- [ ] [[X]]\n[[X]] and X"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			changes, got := autolinkPlanWith(c.body, c.remove, x)
			if got != c.want {
				t.Errorf("got %q, want %q (changes %+v)", got, c.want, changes)
			}
			if !c.remove {
				for _, ch := range changes {
					if ch.Kind == AutolinkRemove {
						t.Errorf("remove change without RemoveDuplicates: %+v", ch)
					}
				}
			}
		})
	}
}

func TestAutolinkMoveKinds(t *testing.T) {
	x := autolinkNote("inbox/X.md", "X")
	body := "X here\n[[X|later]]\n[[X]]"
	changes, _ := autolinkPlanWith(body, true, x)
	if len(changes) != 2 || changes[0].Kind != AutolinkMove || changes[1].Kind != AutolinkRemove {
		t.Fatalf("changes = %+v, want a move and a remove", changes)
	}
	if len(changes[0].Edits) != 2 || changes[0].Line != 1 || changes[1].Line != 3 {
		t.Errorf("changes = %+v", changes)
	}
	if got := ApplyAutolinks(body, changes, func(i int) bool { return i == 1 }); got != "X here\n[[X|later]]\nX" {
		t.Errorf("only the remove kept: got %q", got)
	}
	changes, _ = autolinkPlanWith("[[X]] and [[X]]", true, x)
	if len(changes) != 1 || changes[0].Kind != AutolinkRemove || changes[0].Target != "inbox/X.md" {
		t.Errorf("changes = %+v, want one remove", changes)
	}
}
