package vault

import (
	"reflect"
	"testing"
)

// conflictPlan plans body for inbox/Y.md with the answers in resolved.
func conflictPlan(body string, resolved map[int]string, notes ...NoteMeta) ([]AutolinkChange, string) {
	all := append([]NoteMeta{autolinkNote("inbox/Y.md", "Y")}, notes...)
	changes := PlanAutolinks(body, "inbox/Y.md", all, AutolinkOptions{Resolved: resolved})
	return changes, ApplyAutolinks(body, changes, nil)
}

func TestAutolinkConflictOverlap(t *testing.T) {
	la := autolinkNote("inbox/LA.md", "Linear Algebra")
	al := autolinkNote("inbox/A.md", "Algebra")
	body := "Linear Algebra"

	changes, got := conflictPlan(body, nil, la, al)
	if len(changes) != 1 || changes[0].Kind != AutolinkConflict {
		t.Fatalf("changes = %+v, want one conflict", changes)
	}
	c := changes[0]
	if want := []string{"inbox/LA.md", "inbox/A.md"}; !reflect.DeepEqual(c.Choices, want) {
		t.Errorf("choices = %v, want %v", c.Choices, want)
	}
	if c.Offset != 0 || c.Line != 1 || c.Context != body || len(c.Edits) != 0 {
		t.Errorf("conflict = %+v", c)
	}
	if got != body {
		t.Errorf("an unresolved conflict changed the body: %q", got)
	}

	if _, got := conflictPlan(body, map[int]string{0: "inbox/A.md"}, la, al); got != "Linear [[Algebra]]" {
		t.Errorf("choice Algebra: got %q", got)
	}
	if _, got := conflictPlan(body, map[int]string{0: "inbox/LA.md"}, la, al); got != "[[Linear Algebra]]" {
		t.Errorf("choice Linear Algebra: got %q", got)
	}
	if changes, got := conflictPlan(body, map[int]string{0: ""}, la, al); len(changes) != 0 || got != body {
		t.Errorf("skip: changes = %+v, body %q", changes, got)
	}
}

func TestAutolinkConflictLaterChange(t *testing.T) {
	la := autolinkNote("inbox/LA.md", "Linear Algebra")
	al := autolinkNote("inbox/A.md", "Algebra")
	body := "Linear Algebra and\nLinear Algebra again"

	changes, _ := conflictPlan(body, nil, la, al)
	if len(changes) != 1 || changes[0].Kind != AutolinkConflict {
		t.Fatalf("changes = %+v, want one conflict that holds back the later reference", changes)
	}

	changes, got := conflictPlan(body, map[int]string{0: "inbox/A.md"}, la, al)
	if want := "Linear [[Algebra]] and\n[[Linear Algebra]] again"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if len(changes) != 2 || changes[1].Target != "inbox/LA.md" || changes[1].Kind != AutolinkAdd {
		t.Errorf("changes = %+v, want an add for Linear Algebra after the choice", changes)
	}
}

func TestAutolinkConflictWithAlias(t *testing.T) {
	la := autolinkNote("inbox/LA.md", "Linear Algebra")
	z := autolinkNote("inbox/Z.md", "Z", "Algebra")
	changes, _ := conflictPlan("Linear Algebra", nil, la, z)
	if len(changes) != 1 || changes[0].Kind != AutolinkConflict || len(changes[0].Choices) != 2 {
		t.Fatalf("changes = %+v, want one conflict with two choices", changes)
	}
}

func TestAutolinkConflictSameTitle(t *testing.T) {
	a := autolinkNote("a/X.md", "X")
	b := autolinkNote("b/X.md", "X")
	changes, _ := conflictPlan("X", nil, a, b)
	if len(changes) != 1 || changes[0].Kind != AutolinkConflict {
		t.Fatalf("changes = %+v, want one conflict", changes)
	}
	if want := []string{"a/X.md", "b/X.md"}; !reflect.DeepEqual(changes[0].Choices, want) {
		t.Errorf("choices = %v, want %v", changes[0].Choices, want)
	}

	if _, got := conflictPlan("X", map[int]string{0: ""}, a, b); got != "X" {
		t.Errorf("skip: got %q", got)
	}
	_, got := conflictPlan("X", map[int]string{0: "b/X.md"}, a, b)
	if got != "[[b/X|X]]" {
		t.Errorf("choice b/X.md: got %q, want [[b/X|X]]", got)
	}
	all := []NoteMeta{autolinkNote("inbox/Y.md", "Y"), a, b}
	if n, ok := ResolveWikilink(all, "b/X"); !ok || n.Path != "b/X.md" {
		t.Errorf("the new link resolves to %q (%v), want b/X.md", n.Path, ok)
	}
}

func TestAutolinkConflictSameAlias(t *testing.T) {
	p := autolinkNote("inbox/P.md", "P", "LA")
	q := autolinkNote("inbox/Q.md", "Q", "LA")
	changes, _ := conflictPlan("LA here", nil, p, q)
	if len(changes) != 1 || changes[0].Kind != AutolinkConflict {
		t.Fatalf("changes = %+v, want one conflict", changes)
	}
	if want := []string{"inbox/P.md", "inbox/Q.md"}; !reflect.DeepEqual(changes[0].Choices, want) {
		t.Errorf("choices = %v, want %v", changes[0].Choices, want)
	}
	if _, got := conflictPlan("LA here", map[int]string{0: "inbox/Q.md"}, p, q); got != "[[Q|LA]] here" {
		t.Errorf("choice Q: got %q", got)
	}
}

func TestAutolinkConflictIgnoresBadAnswer(t *testing.T) {
	a := autolinkNote("a/X.md", "X")
	b := autolinkNote("b/X.md", "X")
	changes, _ := conflictPlan("X", map[int]string{0: "inbox/other.md"}, a, b)
	if len(changes) != 1 || changes[0].Kind != AutolinkConflict {
		t.Errorf("changes = %+v, want the conflict to stay", changes)
	}
}

func TestAutolinkConflictLeavesOtherChanges(t *testing.T) {
	a := autolinkNote("a/X.md", "X")
	b := autolinkNote("b/X.md", "X")
	z := autolinkNote("inbox/Z.md", "Z")
	changes, got := conflictPlan("Z then X", nil, a, b, z)
	if len(changes) != 2 || changes[0].Target != "inbox/Z.md" || changes[1].Kind != AutolinkConflict {
		t.Fatalf("changes = %+v", changes)
	}
	if got != "[[Z]] then X" {
		t.Errorf("got %q", got)
	}
}
