// Package flashcards holds the flashcards feature: settings, scheduling and
// card state.
package flashcards

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Settings holds the values of the flashcards object in vault.json. A value
// that is missing or not valid has its default.
type Settings struct {
	Enabled       bool
	GroupBy       string
	NewPerDay     int
	ReviewsPerDay int
	DayStartHour  int
	FSRS          FSRSSettings
}

// FSRSSettings holds the scheduler values. Empty Weights means that the
// scheduler library uses its own default weights.
type FSRSSettings struct {
	DesiredRetention    float64
	MaximumIntervalDays int
	Weights             []float64
	LearningSteps       []time.Duration
	RelearningSteps     []time.Duration
}

// Problem describes one value that ParseSettings replaced with its default.
type Problem struct {
	// Key is the dotted path of the value, for example "flashcards.newPerDay".
	Key string
	// Value is the text of the value that zn refused.
	Value string
	// Reason says in short why zn refused the value.
	Reason string
}

func (p Problem) String() string {
	return fmt.Sprintf("%s: %s (%s)", p.Key, p.Reason, p.Value)
}

const (
	maxStep  = 24 * time.Hour
	maxSteps = 10
)

// DefaultSettings returns the settings of a vault with no flashcards object.
func DefaultSettings() Settings {
	return Settings{
		GroupBy:       "field",
		NewPerDay:     20,
		ReviewsPerDay: 200,
		DayStartHour:  4,
		FSRS: FSRSSettings{
			DesiredRetention:    0.9,
			MaximumIntervalDays: 36500,
			LearningSteps:       []time.Duration{time.Minute, 10 * time.Minute},
			RelearningSteps:     []time.Duration{10 * time.Minute},
		},
	}
}

// ParseSettings reads the flashcards settings from the whole decoded
// vault.json object. It checks each value alone: a value that is not valid
// gives the default and one Problem, and the other values stay as they are.
func ParseSettings(raw map[string]any) (Settings, []Problem) {
	s := DefaultSettings()
	var problems []Problem
	value, present := raw["flashcards"]
	if !present {
		return s, nil
	}
	obj, ok := value.(map[string]any)
	if !ok {
		return s, []Problem{newProblem("flashcards", value, "must be an object")}
	}

	if b, ok := obj["enabled"].(bool); ok {
		s.Enabled = b
	}
	if v, ok := obj["groupBy"]; ok {
		if key, ok := v.(string); ok && validGroupKey(key) {
			s.GroupBy = key
		} else {
			problems = append(problems, newProblem("flashcards.groupBy", v, "must be a frontmatter key that is not reserved"))
		}
	}
	s.NewPerDay = intIn(obj, "newPerDay", 0, 9999, s.NewPerDay, &problems)
	s.ReviewsPerDay = intIn(obj, "reviewsPerDay", 0, 99999, s.ReviewsPerDay, &problems)
	s.DayStartHour = intIn(obj, "dayStartHour", 0, 23, s.DayStartHour, &problems)

	fv, present := obj["fsrs"]
	if !present {
		return s, problems
	}
	fobj, ok := fv.(map[string]any)
	if !ok {
		return s, append(problems, newProblem("flashcards.fsrs", fv, "must be an object"))
	}
	if v, ok := fobj["desiredRetention"]; ok {
		if f, isNum := v.(float64); isNum && f >= 0.7 && f <= 0.99 {
			s.FSRS.DesiredRetention = f
		} else {
			problems = append(problems, newProblem("flashcards.fsrs.desiredRetention", v, "must be a number from 0.7 to 0.99"))
		}
	}
	s.FSRS.MaximumIntervalDays = intIn(fobj, "maximumIntervalDays", 1, 36500, s.FSRS.MaximumIntervalDays, &problems)
	if v, ok := fobj["weights"]; ok {
		if w, ok := parseWeights(v); ok {
			s.FSRS.Weights = w
		} else {
			problems = append(problems, newProblem("flashcards.fsrs.weights", v, "must be empty or hold 17, 19 or 21 numbers"))
		}
	}
	s.FSRS.LearningSteps = stepsIn(fobj, "learningSteps", s.FSRS.LearningSteps, &problems)
	s.FSRS.RelearningSteps = stepsIn(fobj, "relearningSteps", s.FSRS.RelearningSteps, &problems)
	return s, problems
}

func newProblem(key string, value any, reason string) Problem {
	text := fmt.Sprintf("%v", value)
	if len(text) > 80 {
		text = text[:80] + "..."
	}
	return Problem{Key: key, Value: text, Reason: reason}
}

// intIn reads a whole number from obj[key]. The number must be from lo to hi.
func intIn(obj map[string]any, key string, lo, hi, def int, problems *[]Problem) int {
	v, ok := obj[key]
	if !ok {
		return def
	}
	if f, isNum := v.(float64); isNum && f == math.Trunc(f) && f >= float64(lo) && f <= float64(hi) {
		return int(f)
	}
	*problems = append(*problems, newProblem("flashcards."+key, v, fmt.Sprintf("must be a whole number from %d to %d", lo, hi)))
	return def
}

func parseWeights(v any) ([]float64, bool) {
	list, ok := v.([]any)
	if !ok {
		return nil, false
	}
	switch len(list) {
	case 0:
		return nil, true
	case 17, 19, 21:
	default:
		return nil, false
	}
	out := make([]float64, len(list))
	for i, item := range list {
		f, ok := item.(float64)
		if !ok || math.IsNaN(f) || math.IsInf(f, 0) {
			return nil, false
		}
		out[i] = f
	}
	return out, true
}

func stepsIn(obj map[string]any, key string, def []time.Duration, problems *[]Problem) []time.Duration {
	v, ok := obj[key]
	if !ok {
		return def
	}
	if steps, ok := parseSteps(v); ok {
		return steps
	}
	*problems = append(*problems, newProblem("flashcards.fsrs."+key, v,
		"must be a list of at most 10 steps from 1m to 24h, in minutes (m) or hours (h)"))
	return def
}

func parseSteps(v any) ([]time.Duration, bool) {
	list, ok := v.([]any)
	if !ok || len(list) > maxSteps {
		return nil, false
	}
	out := make([]time.Duration, 0, len(list))
	for _, item := range list {
		text, ok := item.(string)
		if !ok {
			return nil, false
		}
		d, ok := parseStep(text)
		if !ok {
			return nil, false
		}
		out = append(out, d)
	}
	return out, true
}

// parseStep reads a whole number with the unit m or h. The step must be from
// 1m to 24h.
func parseStep(text string) (time.Duration, bool) {
	if len(text) < 2 {
		return 0, false
	}
	unit := time.Minute
	switch text[len(text)-1] {
	case 'm':
	case 'h':
		unit = time.Hour
	default:
		return 0, false
	}
	digits := text[:len(text)-1]
	for _, r := range digits {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(digits)
	if err != nil || n < 1 || n > 24*60 {
		return 0, false
	}
	d := time.Duration(n) * unit
	if d < time.Minute || d > maxStep {
		return 0, false
	}
	return d, true
}

// validGroupKey accepts a frontmatter key that the list can group by. The
// keys tags and flashcard-id, and each key that starts with flashcard, are
// not fields.
func validGroupKey(key string) bool {
	if key == "" || len(key) > 64 {
		return false
	}
	for _, r := range key {
		ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_'
		if !ok {
			return false
		}
	}
	lower := strings.ToLower(key)
	return lower != "tags" && !strings.HasPrefix(lower, "flashcard")
}
