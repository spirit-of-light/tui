package vault

import "testing"

func TestAutolinkFormsMatch(t *testing.T) {
	cases := []struct {
		name   string
		text   string
		phrase string
		want   []string
	}{
		{"plural of the title", "thermodynamics", "Thermodynamics", []string{"thermodynamics"}},
		{"singular of the title", "thermodynamic", "Thermodynamics", []string{"thermodynamic"}},
		{"plural s", "notes", "Note", []string{"notes"}},
		{"irregular plural", "matrices", "Matrix", []string{"matrices"}},
		{"irregular singular", "Matrix", "matrices", []string{"Matrix"}},
		{"y to ies", "theories", "Theory", []string{"theories"}},
		{"ies to y", "theory", "Theories", []string{"theory"}},
		{"es after x", "boxes", "Box", []string{"boxes"}},
		{"inside a word", "ago", "Go", nil},
		{"prefix of a word", "Gopher", "Go", nil},
		{"exact word", "go", "Go", []string{"go"}},
		{"longer word", "Kinder", "Kind", nil},
		{"lower case", "linux", "Linux", []string{"linux"}},
		{"last word plural", "linear algebras", "Linear Algebra", []string{"linear algebras"}},
		{"earlier word must match", "Linearly Algebra", "Linear Algebra", nil},
		{"unicode word", "übung", "Übung", []string{"übung"}},
		{"letter before", "xÜbung", "Übung", nil},
		{"underscore after", "foo_bar", "foo", nil},
		{"hyphen after", "foo-bar", "foo", []string{"foo"}},
		{"several matches", "go and Go", "go", []string{"go", "Go"}},
		{"one match for each position", "notes note", "note", []string{"notes", "note"}},
		{"empty phrase", "text", "", nil},
		{"spaces only", "text", "   ", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			spans := autolinkMatches(c.text, c.phrase)
			var got []string
			for _, s := range spans {
				got = append(got, c.text[s[0]:s[1]])
			}
			if len(got) != len(c.want) {
				t.Fatalf("matches of %q in %q = %q, want %q", c.phrase, c.text, got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("matches of %q in %q = %q, want %q", c.phrase, c.text, got, c.want)
				}
			}
		})
	}
}

func TestAutolinkFormsOffsets(t *testing.T) {
	text := "see the Boxes and a box"
	spans := autolinkMatches(text, "box")
	if len(spans) != 2 {
		t.Fatalf("spans = %v, want 2", spans)
	}
	if spans[0] != [2]int{8, 13} || spans[1] != [2]int{20, 23} {
		t.Fatalf("spans = %v", spans)
	}
}

func TestAutolinkFormsWordForms(t *testing.T) {
	cases := map[string][]string{
		"analysis": {"analyses"},
		"analyses": {"analysis"},
		"physic":   {"physics"},
		"physics":  {"physic"},
		"church":   {"churches"},
		"churches": {"church"},
		"boy":      {"boys"},
	}
	for word, want := range cases {
		forms := autolinkWordForms(word)
		for _, w := range want {
			found := false
			for _, f := range forms {
				if f == w {
					found = true
				}
			}
			if !found {
				t.Errorf("forms of %q = %q, want %q in it", word, forms, w)
			}
		}
	}
	for _, f := range autolinkWordForms("boy") {
		if f == "boies" {
			t.Errorf("forms of boy hold %q", f)
		}
	}
}
