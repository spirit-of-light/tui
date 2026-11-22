package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// snippetVault makes a vault with the given group files and returns its root.
func snippetVault(t *testing.T, files map[string]string) string {
	t.Helper()
	root := isolatedCLI(t)
	dir := filepath.Join(root, ".zennotes", "snippets")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, raw := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(raw), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

type doctorReport struct {
	OK     bool              `json:"ok"`
	Checks []diagnosticCheck `json:"checks"`
}

// runDoctorJSON runs `zn doctor --json` and returns the report, the raw output and the exit code.
func runDoctorJSON(t *testing.T, args ...string) (doctorReport, string, int) {
	t.Helper()
	var code int
	out := captureOutput(t, func() { code = Main(append([]string{"doctor", "--json"}, args...)) })
	var report doctorReport
	if err := json.Unmarshal([]byte(strings.SplitN(out, "\n{\"error\"", 2)[0]), &report); err != nil {
		t.Fatalf("doctor json: %v\n%s", err, out)
	}
	return report, out, code
}

func snippetChecks(checks []diagnosticCheck) []diagnosticCheck {
	var out []diagnosticCheck
	for _, c := range checks {
		if c.Name == "snippets" {
			out = append(out, c)
		}
	}
	return out
}

func TestDoctorSnippetsBadHeadingNamesFileAndLine(t *testing.T) {
	root := snippetVault(t, map[string]string{
		"x.md": "## link\n```\nx\n```\n## meet now\n```\ny\n```\n",
	})
	report, out, _ := runDoctorJSON(t, "--vault", root)
	got := snippetChecks(report.Checks)
	if len(got) != 1 {
		t.Fatalf("want one snippets check:\n%s", out)
	}
	want := `.zennotes/snippets/x.md line 5: The heading "meet now" is not one word. zn ignores this snippet.`
	if got[0].Message != want {
		t.Errorf("message %q, want %q", got[0].Message, want)
	}
	if !got[0].OK || !got[0].Warning {
		t.Errorf("a snippet problem is a warning with ok true: %+v", got[0])
	}
}

func TestDoctorSnippetsNoFencedBlock(t *testing.T) {
	root := snippetVault(t, map[string]string{
		"x.md": "## link\n```\nx\n```\n## empty\n\nOnly a description.\n",
	})
	report, out, _ := runDoctorJSON(t, "--vault", root)
	got := snippetChecks(report.Checks)
	if len(got) != 1 {
		t.Fatalf("want one snippets check:\n%s", out)
	}
	want := `.zennotes/snippets/x.md line 5: The snippet "empty" has no fenced block. zn ignores this snippet.`
	if got[0].Message != want {
		t.Errorf("message %q, want %q", got[0].Message, want)
	}
}

func TestDoctorSnippetsUnreadableFileNamesFile(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("a file mode of 0 is not unreadable here")
	}
	root := snippetVault(t, map[string]string{"x.md": "## a\n```\nx\n```\n", "y.md": "## b\n```\ny\n```\n"})
	if err := os.Chmod(filepath.Join(root, ".zennotes", "snippets", "x.md"), 0); err != nil {
		t.Fatal(err)
	}
	report, out, _ := runDoctorJSON(t, "--vault", root)
	got := snippetChecks(report.Checks)
	if len(got) != 1 {
		t.Fatalf("want one snippets check:\n%s", out)
	}
	want := ".zennotes/snippets/x.md: zn cannot read this file. zn ignores this file."
	if got[0].Message != want {
		t.Errorf("message %q, want %q", got[0].Message, want)
	}
}

var duplicateFiles = map[string]string{
	"a.md": "## other\n```\no\n```\n\n\n## meet\n```\nA\n```\n",
	"b.md": "## x1\n```\n1\n```\n## x2\n```\n2\n```\n## x3\n```\n3\n```\n## meet\n```\nB\n```\n",
}

func TestDoctorSnippetsDuplicateText(t *testing.T) {
	root := snippetVault(t, duplicateFiles)
	report, out, _ := runDoctorJSON(t, "--vault", root)
	got := snippetChecks(report.Checks)
	if len(got) != 1 {
		t.Fatalf("want one snippets check:\n%s", out)
	}
	want := `The trigger "meet" is in .zennotes/snippets/a.md line 7 and in .zennotes/snippets/b.md line 13. zn uses a.md line 7 and ignores b.md line 13.`
	if got[0].Message != want {
		t.Errorf("message %q, want %q", got[0].Message, want)
	}
}

func TestDoctorSnippetsDuplicateJSON(t *testing.T) {
	root := snippetVault(t, duplicateFiles)
	_, out, _ := runDoctorJSON(t, "--vault", root)
	var raw struct {
		Checks []map[string]any `json:"checks"`
	}
	if err := json.Unmarshal([]byte(strings.SplitN(out, "\n{\"error\"", 2)[0]), &raw); err != nil {
		t.Fatal(err)
	}
	var found map[string]any
	for _, c := range raw.Checks {
		if c["name"] == "snippets" {
			found = c
		}
	}
	if found == nil {
		t.Fatalf("no snippets check:\n%s", out)
	}
	if found["ok"] != true || found["warning"] != true {
		t.Errorf("want ok and warning true: %v", found)
	}
	sn, _ := found["snippet"].(map[string]any)
	want := map[string]any{
		"trigger":     "meet",
		"usedPath":    ".zennotes/snippets/a.md",
		"usedLine":    float64(7),
		"ignoredPath": ".zennotes/snippets/b.md",
		"ignoredLine": float64(13),
	}
	for k, v := range want {
		if sn[k] != v {
			t.Errorf("snippet.%s = %v, want %v (%v)", k, sn[k], v, sn)
		}
	}
}

func TestDoctorSnippetsWarningDoesNotChangeStatus(t *testing.T) {
	clean := snippetVault(t, map[string]string{"a.md": "## meet\n```\nA\n```\n"})
	cleanReport, _, cleanCode := runDoctorJSON(t, "--vault", clean)
	dup := snippetVault(t, duplicateFiles)
	dupReport, _, dupCode := runDoctorJSON(t, "--vault", dup)
	if cleanCode != dupCode {
		t.Errorf("exit code changed: %d and %d", cleanCode, dupCode)
	}
	if cleanReport.OK != dupReport.OK {
		t.Errorf("top-level ok changed: %v and %v", cleanReport.OK, dupReport.OK)
	}
}

func TestDoctorSnippetsTextOutputWarn(t *testing.T) {
	root := snippetVault(t, duplicateFiles)
	out := captureOutput(t, func() { _ = Main([]string{"doctor", "--vault", root}) })
	if !strings.Contains(out, "WARN  snippets: The trigger \"meet\"") {
		t.Errorf("no WARN line:\n%s", out)
	}
	if strings.Contains(out, "FAIL  snippets") {
		t.Errorf("a snippet problem must not fail:\n%s", out)
	}
}

func TestDoctorSnippetsQuietWithNoProblem(t *testing.T) {
	good := snippetVault(t, map[string]string{"a.md": "## meet\n```\nA\n```\n", "b.md": "## meet1\n```\nB\n```\n"})
	none := isolatedCLI(t)
	for _, root := range []string{good, none} {
		report, out, _ := runDoctorJSON(t, "--vault", root)
		if got := snippetChecks(report.Checks); len(got) != 0 {
			t.Errorf("unexpected snippets check:\n%s", out)
		}
	}
}

func TestDoctorSnippetsRemoteSkipLine(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusNotFound)
	}))
	defer srv.Close()
	isolatedCLI(t)
	report, out, _ := runDoctorJSON(t, "--server", srv.URL, "--token", "tok")
	got := snippetChecks(report.Checks)
	if len(got) != 1 {
		t.Fatalf("want one snippets check:\n%s", out)
	}
	want := "zn skipped the snippet check. Snippets are not available for a remote vault."
	if got[0].Message != want || !got[0].OK || got[0].Warning {
		t.Errorf("%+v", got[0])
	}
}
