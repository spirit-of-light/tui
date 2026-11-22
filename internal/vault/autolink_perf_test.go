package vault

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// autolinkLargeInput builds a vault of many small notes and one body of about
// 1 MB. The body has a reference to 500 different notes.
func autolinkLargeInput() (string, string, []NoteMeta) {
	const targets = 2000
	self := "inbox/Big.md"
	notes := make([]NoteMeta, 0, targets+1)
	for i := 1; i <= targets; i++ {
		notes = append(notes, autolinkNote(fmt.Sprintf("inbox/Topic %04d.md", i), fmt.Sprintf("Topic %04d", i)))
	}
	notes = append(notes, autolinkNote(self, "Big"))
	var b strings.Builder
	for n := 0; b.Len() < 1<<20; n++ {
		fmt.Fprintf(&b, "Line %d talks about topic %04d and other prose words here.\n", n, n%500+1)
	}
	return b.String(), self, notes
}

func TestAutolinkPlanLargeNoteIsFast(t *testing.T) {
	if testing.Short() {
		t.Skip("large autolink plan skipped in short mode")
	}
	body, self, notes := autolinkLargeInput()
	start := time.Now()
	changes := PlanAutolinks(body, self, notes, AutolinkOptions{})
	elapsed := time.Since(start)
	if len(changes) != 500 {
		t.Fatalf("got %d changes, want 500", len(changes))
	}
	// The limit is generous so that the race detector stays within it.
	if elapsed > 10*time.Second {
		t.Fatalf("plan took %v, want less than 10s", elapsed)
	}
}

func BenchmarkPlanAutolinksLargeNote(b *testing.B) {
	body, self, notes := autolinkLargeInput()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		PlanAutolinks(body, self, notes, AutolinkOptions{})
	}
}
