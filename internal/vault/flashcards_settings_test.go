package vault

import (
	"encoding/json"
	"testing"
)

// The flashcards object in vault.json is read as two flags. The feature is on
// only when `enabled` is the JSON value true. Any other value means off, and
// the object still exists.
func TestFlashcardsObjectFlags(t *testing.T) {
	cases := []struct {
		name       string
		raw        string
		wantObject bool
		wantOn     bool
	}{
		{"no object", `{}`, false, false},
		{"enabled true", `{"flashcards":{"enabled":true}}`, true, true},
		{"enabled false", `{"flashcards":{"enabled":false}}`, true, false},
		{"enabled is a string", `{"flashcards":{"enabled":"yes"}}`, true, false},
		{"enabled is a number", `{"flashcards":{"enabled":1}}`, true, false},
		{"object with no enabled key", `{"flashcards":{"groupBy":"level"}}`, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var raw map[string]any
			if err := json.Unmarshal([]byte(tc.raw), &raw); err != nil {
				t.Fatal(err)
			}
			s := ParseSettings(raw)
			if s.FlashcardsObject != tc.wantObject {
				t.Errorf("FlashcardsObject = %v, want %v", s.FlashcardsObject, tc.wantObject)
			}
			if s.FlashcardsEnabled != tc.wantOn {
				t.Errorf("FlashcardsEnabled = %v, want %v", s.FlashcardsEnabled, tc.wantOn)
			}
		})
	}
}

// The new flags do not change the other settings, and the raw object stays
// in Raw so that a targeted write keeps it.
func TestFlashcardsFlagsDoNotChangeOtherSettings(t *testing.T) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(`{"favorites":["a.md"],"flashcards":{"enabled":true}}`), &raw); err != nil {
		t.Fatal(err)
	}
	s := ParseSettings(raw)
	if len(s.Favorites) != 1 || s.Favorites[0] != "a.md" {
		t.Errorf("Favorites = %v, want [a.md]", s.Favorites)
	}
	if _, ok := s.Raw["flashcards"]; !ok {
		t.Errorf("Raw lost the flashcards object")
	}
}
