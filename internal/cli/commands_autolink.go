package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/ZenNotes/tui/internal/backend"
	"github.com/ZenNotes/tui/internal/config"
	"github.com/ZenNotes/tui/internal/vault"
)

// The review reads its answers from autolinkInput and asks autolinkIsTTY
// before it starts. Tests replace both. The review writes to stdout.
var (
	autolinkInput io.Reader = os.Stdin
	autolinkIsTTY           = stdinIsTTY
)

var errAutolinkInput = errors.New("the input ended before the review finished. No note changed")

// autolinkReview reads the answers of one review.
type autolinkReview struct {
	in *bufio.Reader
}

// ask prints a prompt and reads answers until valid accepts one. It fails when
// the input ends before a full answer.
func (r *autolinkReview) ask(prompt string, valid func(string) (bool, string)) (string, error) {
	for {
		fmt.Fprint(stdout, prompt)
		text, err := r.in.ReadString('\n')
		if err != nil && (!errors.Is(err, io.EOF) || text == "") {
			fmt.Fprintln(stdout)
			return "", errAutolinkInput
		}
		answer := strings.TrimSpace(text)
		ok, problem := valid(answer)
		if ok {
			return answer, nil
		}
		fmt.Fprintln(stdout, problem)
	}
}

func cmdAutolink(ctx context.Context, b backend.Backend, args Args) error {
	if b.Kind() == backend.KindRemote {
		return backend.ErrAutolinkRemote
	}
	if !autolinkIsTTY() || args.Bool("no-input") {
		return errors.New("autolink needs a terminal for the review. No note changed")
	}
	all := args.Bool("all")
	var paths []string
	if p := args.Str("path"); p != "" {
		paths = append(paths, p)
	}
	paths = append(paths, args.Positionals...)
	if all == (len(paths) > 0) {
		return errors.New("usage: zn autolink <path> or zn autolink --all")
	}

	prefs, _, _ := config.LoadPrefs()
	run, err := backend.PlanAutolinkRun(ctx, b, paths, prefs.AutolinkRemoveDuplicates)
	if err != nil {
		return err
	}
	review := &autolinkReview{in: bufio.NewReader(autolinkInput)}

	plans := make([]backend.AutolinkNote, 0, len(run.Plans))
	for _, plan := range run.Plans {
		plan, err := review.resolveConflicts(plan, run)
		if err != nil {
			return err
		}
		if len(plan.Changes) > 0 {
			plans = append(plans, plan)
		}
	}
	type ref struct{ plan, change int }
	var refs []ref
	index := map[ref]int{}
	for i, plan := range plans {
		for j := range plan.Changes {
			index[ref{i, j}] = len(refs)
			refs = append(refs, ref{i, j})
		}
	}
	if len(refs) == 0 {
		emitLine("No change. No reference needs a link.")
		return nil
	}

	emitLine("Planned changes:")
	for n, r := range refs {
		c := plans[r.plan].Changes[r.change]
		describeAutolinkChange(n+1, plans[r.plan], c)
	}

	omitted := map[int]bool{}
	_, err = review.ask("Numbers of the changes to omit, or press Enter to keep all: ", func(s string) (bool, string) {
		found := map[int]bool{}
		for _, field := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' }) {
			n, err := strconv.Atoi(field)
			if err != nil || n < 1 || n > len(refs) {
				return false, fmt.Sprintf("Enter numbers from 1 to %d.", len(refs))
			}
			found[n-1] = true
		}
		omitted = found
		return true, ""
	})
	if err != nil {
		return err
	}

	kept := len(refs) - len(omitted)
	if kept == 0 {
		emitLine("All changes omitted. No note changed.")
		return nil
	}
	touched := map[int]bool{}
	for n, r := range refs {
		if !omitted[n] {
			touched[r.plan] = true
		}
	}
	confirm, err := review.ask(fmt.Sprintf("Write %s to %s? [y/N] ", plural(kept, "change"), plural(len(touched), "note")), func(string) (bool, string) { return true, "" })
	if err != nil {
		return err
	}
	if !strings.EqualFold(confirm, "y") {
		emitLine("Cancelled. No note changed.")
		return nil
	}
	written, err := backend.WriteAutolinks(ctx, b, plans, func(p, c int) bool { return !omitted[index[ref{p, c}]] })
	for _, path := range written {
		emitLine("Wrote " + path)
	}
	return err
}

// resolveConflicts asks for the answer to each conflict of a plan and plans
// the body again after each answer, because an answer can change later changes.
func (r *autolinkReview) resolveConflicts(plan backend.AutolinkNote, run backend.AutolinkRun) (backend.AutolinkNote, error) {
	opts := run.Options
	opts.Resolved = map[int]string{}
	for {
		var conflict *vault.AutolinkChange
		for i := range plan.Changes {
			if plan.Changes[i].Kind == vault.AutolinkConflict {
				conflict = &plan.Changes[i]
				break
			}
		}
		if conflict == nil {
			return plan, nil
		}
		emitLine(fmt.Sprintf("Conflict in %s:%d  %s", plan.Path, conflict.Line, truncate(conflict.Context, 100)))
		emitLine("  1  skip: do not link this text")
		for i, choice := range conflict.Choices {
			emitLine(fmt.Sprintf("  %d  link to %s", i+2, choice))
		}
		limit := len(conflict.Choices) + 1
		answer, err := r.ask("Choose a number: ", func(s string) (bool, string) {
			n, err := strconv.Atoi(s)
			if err != nil || n < 1 || n > limit {
				return false, fmt.Sprintf("Enter a number from 1 to %d.", limit)
			}
			return true, ""
		})
		if err != nil {
			return plan, err
		}
		n, _ := strconv.Atoi(answer)
		chosen := ""
		if n > 1 {
			chosen = conflict.Choices[n-2]
		}
		opts.Resolved[conflict.Offset] = chosen
		plan.Changes = vault.PlanAutolinks(plan.Body, plan.Path, run.Notes, opts)
	}
}

// plural gives "1 note" or "2 notes".
func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// describeAutolinkChange shows one change as a numbered row with the text that
// it changes, then the line of context. A move also shows its old link.
func describeAutolinkChange(number int, plan backend.AutolinkNote, c vault.AutolinkChange) {
	if len(c.Edits) == 0 {
		return
	}
	edit := func(e vault.AutolinkEdit) string {
		return truncate(plan.Body[e.Start:e.End], 60) + " → " + truncate(e.Text, 60)
	}
	lineOf := func(offset int) int { return 1 + strings.Count(plan.Body[:offset], "\n") }
	emitLine(fmt.Sprintf("%3d  %-6s %s:%d  %s", number, c.Kind, plan.Path, lineOf(c.Edits[0].Start), edit(c.Edits[0])))
	emitLine("       Context: " + truncate(c.Context, 100))
	if c.Kind == vault.AutolinkMove && len(c.Edits) > 1 {
		emitLine(fmt.Sprintf("       Old link: %s:%d  %s", plan.Path, lineOf(c.Edits[1].Start), edit(c.Edits[1])))
	}
}
