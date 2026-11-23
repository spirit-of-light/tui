package flashcards

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// parse decodes the text as a whole vault.json and reads its flashcards
// settings, the way the code reads a file from disk (numbers are float64).
func parse(t *testing.T, text string) (Settings, []Problem) {
	t.Helper()
	var raw map[string]any
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		t.Fatal(err)
	}
	return ParseSettings(raw)
}

// object wraps the body in a flashcards object.
func object(body string) string {
	return `{"flashcards":{` + body + `}}`
}

func problemFor(problems []Problem, key string) *Problem {
	for i := range problems {
		if strings.Contains(problems[i].Key, key) {
			return &problems[i]
		}
	}
	return nil
}

func wantOneProblem(t *testing.T, problems []Problem, key string) {
	t.Helper()
	if len(problems) != 1 {
		t.Fatalf("got %d problems, want 1: %v", len(problems), problems)
	}
	if problemFor(problems, key) == nil {
		t.Fatalf("no problem names %q: %v", key, problems)
	}
}

func durations(ms ...int) []time.Duration {
	out := make([]time.Duration, len(ms))
	for i, m := range ms {
		out[i] = time.Duration(m) * time.Minute
	}
	return out
}

func repeat(item string, n int) string {
	return strings.TrimSuffix(strings.Repeat(item+",", n), ",")
}

func TestSettingsDefaultsWithNoObject(t *testing.T) {
	for _, text := range []string{`{}`, `{"flashcards":{}}`} {
		s, problems := parse(t, text)
		if len(problems) != 0 {
			t.Errorf("%s: problems = %v, want none", text, problems)
		}
		if s.Enabled {
			t.Errorf("%s: Enabled = true, want false", text)
		}
		if s.GroupBy != "field" {
			t.Errorf("%s: GroupBy = %q, want field", text, s.GroupBy)
		}
		if s.NewPerDay != 20 || s.ReviewsPerDay != 200 || s.DayStartHour != 4 {
			t.Errorf("%s: limits = %d, %d, %d; want 20, 200, 4", text, s.NewPerDay, s.ReviewsPerDay, s.DayStartHour)
		}
		if s.FSRS.DesiredRetention != 0.9 {
			t.Errorf("%s: DesiredRetention = %v, want 0.9", text, s.FSRS.DesiredRetention)
		}
		if s.FSRS.MaximumIntervalDays != 36500 {
			t.Errorf("%s: MaximumIntervalDays = %d, want 36500", text, s.FSRS.MaximumIntervalDays)
		}
		if len(s.FSRS.Weights) != 0 {
			t.Errorf("%s: Weights = %v, want empty", text, s.FSRS.Weights)
		}
		if !reflect.DeepEqual(s.FSRS.LearningSteps, durations(1, 10)) {
			t.Errorf("%s: LearningSteps = %v, want [1m 10m]", text, s.FSRS.LearningSteps)
		}
		if !reflect.DeepEqual(s.FSRS.RelearningSteps, durations(10)) {
			t.Errorf("%s: RelearningSteps = %v, want [10m]", text, s.FSRS.RelearningSteps)
		}
	}
}

func TestSettingsReadValidValues(t *testing.T) {
	s, problems := parse(t, object(`"enabled":true,"groupBy":"level","newPerDay":5,"reviewsPerDay":50,"dayStartHour":6,`+
		`"fsrs":{"desiredRetention":0.85,"maximumIntervalDays":365,"learningSteps":["2m","15m"],"relearningSteps":["20m"]}`))
	if len(problems) != 0 {
		t.Fatalf("problems = %v, want none", problems)
	}
	if !s.Enabled || s.GroupBy != "level" || s.NewPerDay != 5 || s.ReviewsPerDay != 50 || s.DayStartHour != 6 {
		t.Errorf("settings = %+v", s)
	}
	if s.FSRS.DesiredRetention != 0.85 || s.FSRS.MaximumIntervalDays != 365 {
		t.Errorf("FSRS = %+v", s.FSRS)
	}
	if !reflect.DeepEqual(s.FSRS.LearningSteps, durations(2, 15)) || !reflect.DeepEqual(s.FSRS.RelearningSteps, durations(20)) {
		t.Errorf("steps = %v, %v", s.FSRS.LearningSteps, s.FSRS.RelearningSteps)
	}
}

func TestBadRetentionUsesDefault(t *testing.T) {
	for _, v := range []string{"1.5", "1", "0.69", "0", "-0.9", `"high"`} {
		s, problems := parse(t, object(`"fsrs":{"desiredRetention":`+v+`}`))
		if s.FSRS.DesiredRetention != 0.9 {
			t.Errorf("desiredRetention %s: got %v, want 0.9", v, s.FSRS.DesiredRetention)
		}
		wantOneProblem(t, problems, "desiredRetention")
	}
	for _, v := range []string{"0.7", "0.99", "0.95"} {
		s, problems := parse(t, object(`"fsrs":{"desiredRetention":`+v+`}`))
		if len(problems) != 0 {
			t.Errorf("desiredRetention %s: problems = %v, want none", v, problems)
		}
		if fmt.Sprint(s.FSRS.DesiredRetention) != v {
			t.Errorf("desiredRetention %s: got %v", v, s.FSRS.DesiredRetention)
		}
	}
}

func TestBadWeightCountUsesLibraryDefault(t *testing.T) {
	s, problems := parse(t, object(`"fsrs":{"weights":[`+repeat("0.5", 20)+`]}`))
	if len(s.FSRS.Weights) != 0 {
		t.Errorf("Weights = %v, want empty (the library default)", s.FSRS.Weights)
	}
	wantOneProblem(t, problems, "weights")

	for _, n := range []int{17, 19, 21} {
		s, problems := parse(t, object(`"fsrs":{"weights":[`+repeat("0.5", n)+`]}`))
		if len(problems) != 0 {
			t.Errorf("%d weights: problems = %v, want none", n, problems)
		}
		if len(s.FSRS.Weights) != n {
			t.Errorf("%d weights: kept %d values", n, len(s.FSRS.Weights))
		}
	}
	for _, n := range []int{1, 16, 18, 22} {
		s, problems := parse(t, object(`"fsrs":{"weights":[`+repeat("0.5", n)+`]}`))
		if len(s.FSRS.Weights) != 0 {
			t.Errorf("%d weights: Weights = %v, want empty", n, s.FSRS.Weights)
		}
		wantOneProblem(t, problems, "weights")
	}
}

func TestBadMaximumIntervalUsesDefault(t *testing.T) {
	for _, v := range []string{"0", "40000", "36501", "-1", `"long"`} {
		s, problems := parse(t, object(`"fsrs":{"maximumIntervalDays":`+v+`}`))
		if s.FSRS.MaximumIntervalDays != 36500 {
			t.Errorf("maximumIntervalDays %s: got %d, want 36500", v, s.FSRS.MaximumIntervalDays)
		}
		wantOneProblem(t, problems, "maximumIntervalDays")
	}
	for v, want := range map[string]int{"1": 1, "36500": 36500, "90": 90} {
		s, problems := parse(t, object(`"fsrs":{"maximumIntervalDays":`+v+`}`))
		if len(problems) != 0 || s.FSRS.MaximumIntervalDays != want {
			t.Errorf("maximumIntervalDays %s: got %d, problems %v", v, s.FSRS.MaximumIntervalDays, problems)
		}
	}
}

func TestBadLearningStepsUseDefault(t *testing.T) {
	bad := map[string]string{
		"zero":         `["0m"]`,
		"negative":     `["-5m"]`,
		"too long":     `["25h"]`,
		"not duration": `["abc"]`,
		"eleven":       `[` + repeat(`"5m"`, 11) + `]`,
		"a number":     `[10]`,
		"not a list":   `"10m"`,
		"bad in list":  `["1m","abc"]`,
	}
	for name, v := range bad {
		t.Run("learning "+name, func(t *testing.T) {
			s, problems := parse(t, object(`"fsrs":{"learningSteps":`+v+`}`))
			if !reflect.DeepEqual(s.FSRS.LearningSteps, durations(1, 10)) {
				t.Errorf("LearningSteps = %v, want [1m 10m]", s.FSRS.LearningSteps)
			}
			wantOneProblem(t, problems, "learningSteps")
		})
		t.Run("relearning "+name, func(t *testing.T) {
			s, problems := parse(t, object(`"fsrs":{"relearningSteps":`+v+`}`))
			if !reflect.DeepEqual(s.FSRS.RelearningSteps, durations(10)) {
				t.Errorf("RelearningSteps = %v, want [10m]", s.FSRS.RelearningSteps)
			}
			wantOneProblem(t, problems, "relearningSteps")
		})
	}
}

func TestEmptyStepListsAreValid(t *testing.T) {
	s, problems := parse(t, object(`"fsrs":{"learningSteps":[],"relearningSteps":[]}`))
	if len(problems) != 0 {
		t.Errorf("problems = %v, want none", problems)
	}
	if len(s.FSRS.LearningSteps) != 0 {
		t.Errorf("LearningSteps = %v, want empty", s.FSRS.LearningSteps)
	}
	if len(s.FSRS.RelearningSteps) != 0 {
		t.Errorf("RelearningSteps = %v, want empty", s.FSRS.RelearningSteps)
	}
}

func TestStepValuesInMinutesAndHours(t *testing.T) {
	s, problems := parse(t, object(`"fsrs":{"learningSteps":["1m","90m","24h","2h"],"relearningSteps":["1m"]}`))
	if len(problems) != 0 {
		t.Fatalf("problems = %v, want none", problems)
	}
	want := []time.Duration{time.Minute, 90 * time.Minute, 24 * time.Hour, 2 * time.Hour}
	if !reflect.DeepEqual(s.FSRS.LearningSteps, want) {
		t.Errorf("LearningSteps = %v, want %v", s.FSRS.LearningSteps, want)
	}
	s, problems = parse(t, object(`"fsrs":{"learningSteps":[`+repeat(`"5m"`, 10)+`]}`))
	if len(problems) != 0 || len(s.FSRS.LearningSteps) != 10 {
		t.Errorf("ten steps: got %d steps, problems %v", len(s.FSRS.LearningSteps), problems)
	}
}

func TestLimitAndDayStartRanges(t *testing.T) {
	cases := []struct {
		key, value string
		bad        bool
	}{
		{"newPerDay", "0", false}, {"newPerDay", "9999", false}, {"newPerDay", "-1", true}, {"newPerDay", "10000", true}, {"newPerDay", `"many"`, true},
		{"reviewsPerDay", "0", false}, {"reviewsPerDay", "99999", false}, {"reviewsPerDay", "-1", true}, {"reviewsPerDay", "100000", true},
		{"dayStartHour", "0", false}, {"dayStartHour", "23", false}, {"dayStartHour", "-1", true}, {"dayStartHour", "24", true},
	}
	get := func(s Settings, key string) int {
		switch key {
		case "newPerDay":
			return s.NewPerDay
		case "reviewsPerDay":
			return s.ReviewsPerDay
		}
		return s.DayStartHour
	}
	defaults := map[string]int{"newPerDay": 20, "reviewsPerDay": 200, "dayStartHour": 4}
	for _, c := range cases {
		s, problems := parse(t, object(`"`+c.key+`":`+c.value))
		if c.bad {
			if got := get(s, c.key); got != defaults[c.key] {
				t.Errorf("%s %s: got %d, want default %d", c.key, c.value, got, defaults[c.key])
			}
			wantOneProblem(t, problems, c.key)
			continue
		}
		if len(problems) != 0 {
			t.Errorf("%s %s: problems = %v, want none", c.key, c.value, problems)
		}
		if fmt.Sprint(get(s, c.key)) != c.value {
			t.Errorf("%s %s: got %d", c.key, c.value, get(s, c.key))
		}
	}

	for _, key := range []string{"field", "folder", "level", "topic-2"} {
		s, problems := parse(t, object(`"groupBy":"`+key+`"`))
		if s.GroupBy != key || len(problems) != 0 {
			t.Errorf("groupBy %q: got %q, problems %v", key, s.GroupBy, problems)
		}
	}
	for _, key := range []string{"tags", "my key", "flashcard-id", "flashcard"} {
		s, problems := parse(t, object(`"groupBy":"`+key+`"`))
		if s.GroupBy != "field" {
			t.Errorf("groupBy %q: got %q, want field", key, s.GroupBy)
		}
		wantOneProblem(t, problems, "groupBy")
	}
}

func TestOneBadValueDoesNotChangeOthers(t *testing.T) {
	s, problems := parse(t, object(`"enabled":true,"newPerDay":7,"fsrs":{"desiredRetention":1.5,"maximumIntervalDays":100}`))
	wantOneProblem(t, problems, "desiredRetention")
	if !s.Enabled || s.NewPerDay != 7 || s.FSRS.MaximumIntervalDays != 100 {
		t.Errorf("good values changed: %+v", s)
	}
	if s.FSRS.DesiredRetention != 0.9 {
		t.Errorf("DesiredRetention = %v, want 0.9", s.FSRS.DesiredRetention)
	}
}

func TestEnabledFollowsTheJSONBoolean(t *testing.T) {
	for text, want := range map[string]bool{
		object(`"enabled":true`):  true,
		object(`"enabled":false`): false,
		object(`"enabled":"yes"`): false,
		object(`"enabled":1`):     false,
		`{}`:                      false,
	} {
		s, _ := parse(t, text)
		if s.Enabled != want {
			t.Errorf("%s: Enabled = %v, want %v", text, s.Enabled, want)
		}
	}
}
