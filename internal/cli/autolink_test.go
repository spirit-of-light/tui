package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// runAutolink runs `zn autolink` with scripted answers and a terminal check
// that says yes or no. It returns the exit code and the output.
func runAutolink(t *testing.T, tty bool, input string, argv ...string) (int, string) {
	t.Helper()
	prevIn, prevTTY := autolinkInput, autolinkIsTTY
	autolinkInput, autolinkIsTTY = strings.NewReader(input), func() bool { return tty }
	t.Cleanup(func() { autolinkInput, autolinkIsTTY = prevIn, prevTTY })
	code := 0
	out := captureOutput(t, func() { code = Main(argv) })
	return code, out
}

func readVaultFile(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func autolinkTestVault(t *testing.T, files map[string]string) string {
	t.Helper()
	isolatedCLI(t)
	root := t.TempDir()
	for rel, body := range files {
		writeVaultFile(t, root, rel, body)
	}
	return root
}

func TestAutolinkCommandNeedsATerminal(t *testing.T) {
	root := autolinkTestVault(t, map[string]string{"inbox/X.md": "# X\n", "inbox/Y.md": "Y has X.\n"})
	for _, argv := range [][]string{
		{"autolink", "inbox/Y.md", "--vault", root},
		{"autolink", "--all", "--vault", root},
	} {
		code, out := runAutolink(t, false, "\ny\n", argv...)
		if code == 0 || !strings.Contains(out, "needs a terminal") {
			t.Fatalf("%v: code %d\n%s", argv, code, out)
		}
		if got := readVaultFile(t, root, "inbox/Y.md"); got != "Y has X.\n" {
			t.Fatalf("%v changed the note: %q", argv, got)
		}
	}
}

func TestAutolinkCommandNeedsAPathOrAll(t *testing.T) {
	root := autolinkTestVault(t, map[string]string{"inbox/X.md": "# X\n"})
	if code, _ := runAutolink(t, true, "", "autolink", "--vault", root); code == 0 {
		t.Fatal("no path and no --all must fail")
	}
	if code, _ := runAutolink(t, true, "", "autolink", "--all", "inbox/X.md", "--vault", root); code == 0 {
		t.Fatal("a path with --all must fail")
	}
}

func TestAutolinkCommandCancelAndShortInputChangeNothing(t *testing.T) {
	root := autolinkTestVault(t, map[string]string{"inbox/X.md": "# X\n", "inbox/Y.md": "Y has X.\n"})
	for name, input := range map[string]string{
		"answer n":          "\nn\n",
		"empty answer":      "\n\n",
		"stops after list":  "",
		"stops before sure": "\n",
	} {
		code, out := runAutolink(t, true, input, "autolink", "inbox/Y.md", "--vault", root)
		if name == "answer n" || name == "empty answer" {
			if code != 0 || !strings.Contains(out, "Cancelled") {
				t.Fatalf("%s: code %d\n%s", name, code, out)
			}
		} else if code == 0 {
			t.Fatalf("%s: input that ends early must fail\n%s", name, out)
		}
		if got := readVaultFile(t, root, "inbox/Y.md"); got != "Y has X.\n" {
			t.Fatalf("%s changed the note: %q", name, got)
		}
	}
}

func TestAutolinkCommandAllListsBeforeTheWritePrompt(t *testing.T) {
	root := autolinkTestVault(t, map[string]string{
		".zennotes/vault.json": `{"templates":{"folder":"inbox/Templates"}}`,
		"inbox/X.md":           "# X\n",
		"inbox/Y.md":           "Y has X.\n",
		"inbox/Z.md":           "Z has X too.\n",
		"archive/B.md":         "B has X.\n",
	})
	code, out := runAutolink(t, true, "\ny\n", "autolink", "--all", "--vault", root)
	if code != 0 {
		t.Fatalf("code %d\n%s", code, out)
	}
	list, prompt := strings.Index(out, "inbox/Z.md:1"), strings.Index(out, "Write ")
	if list < 0 || prompt < 0 || list > prompt || !strings.Contains(out, "inbox/Y.md:1") {
		t.Fatalf("the list must come before the write prompt:\n%s", out)
	}
	if got := readVaultFile(t, root, "inbox/Y.md"); got != "Y has [[X]].\n" {
		t.Fatalf("Y: %q", got)
	}
	if got := readVaultFile(t, root, "inbox/Z.md"); got != "Z has [[X]] too.\n" {
		t.Fatalf("Z: %q", got)
	}
	if got := readVaultFile(t, root, "archive/B.md"); got != "B has X.\n" {
		t.Fatalf("archive note changed: %q", got)
	}
	if strings.Contains(out, "archive/B.md") {
		t.Fatalf("the review lists an archive note:\n%s", out)
	}
}

func TestAutolinkCommandReviewShowsEachChangeAndOmitsByNumber(t *testing.T) {
	root := autolinkTestVault(t, map[string]string{
		"inbox/X.md": "# X\n",
		"inbox/W.md": "# W\n",
		"inbox/Y.md": "First X then W.\n",
	})
	code, out := runAutolink(t, true, "2\ny\n", "autolink", "inbox/Y.md", "--vault", root)
	if code != 0 {
		t.Fatalf("code %d\n%s", code, out)
	}
	for _, want := range []string{"  1  add    inbox/Y.md:1  X → [[X]]", "  2  add    inbox/Y.md:1  W → [[W]]", "Context: First X then W.", "omit", "Write 1 change to 1 note?"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q:\n%s", want, out)
		}
	}
	// The change with the number 2 is omitted, so only the first link stays.
	got := readVaultFile(t, root, "inbox/Y.md")
	if strings.Count(got, "[[") != 1 {
		t.Fatalf("exactly one link expected: %q", got)
	}
}

func TestAutolinkCommandRowsShowTheTextAndTheOldLink(t *testing.T) {
	root := autolinkTestVault(t, map[string]string{
		"inbox/X.md": "# X\n",
		"inbox/W.md": "# W\n",
		"inbox/Y.md": "Plain x and w.\n\nLater [[X|the x]] and [[X]].\n",
	})
	code, out := runAutolink(t, true, "\nn\n", "autolink", "inbox/Y.md", "--vault", root)
	if code != 0 {
		t.Fatalf("code %d\n%s", code, out)
	}
	for _, want := range []string{
		"move   inbox/Y.md:1  x → [[X|x]]",
		"Old link: inbox/Y.md:3  [[X|the x]] → the x",
		"remove inbox/Y.md:3  [[X]] → X\n       Context: Later [[X|the x]] and [[X]].",
		"add    inbox/Y.md:1  w → [[W|w]]",
		"Write 3 changes to 1 note? [y/N]",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q:\n%s", want, out)
		}
	}
}

func TestAutolinkCommandConflictAnswerPicksTheNote(t *testing.T) {
	root := autolinkTestVault(t, map[string]string{
		"inbox/Linear Algebra.md": "# Linear Algebra\n",
		"inbox/Algebra.md":        "# Algebra\n",
		"inbox/W.md":              "# W\n",
		"inbox/Y.md":              "W and Linear Algebra here.\n",
	})
	// The first run skips the conflict and shows the order of the choices.
	_, out := runAutolink(t, true, "1\n\nn\n", "autolink", "inbox/Y.md", "--vault", root)
	choice := ""
	for _, line := range strings.Split(out, "\n") {
		if strings.HasSuffix(line, "link to inbox/Algebra.md") && strings.HasPrefix(line, "  ") {
			choice = strings.TrimSpace(strings.SplitN(strings.TrimSpace(line), " ", 2)[0])
		}
	}
	if choice == "" {
		t.Fatalf("no choice for Algebra:\n%s", out)
	}
	code, out := runAutolink(t, true, choice+"\n\ny\n", "autolink", "inbox/Y.md", "--vault", root)
	if code != 0 {
		t.Fatalf("code %d\n%s", code, out)
	}
	if got := readVaultFile(t, root, "inbox/Y.md"); got != "[[W]] and Linear [[Algebra]] here.\n" {
		t.Fatalf("Y: %q\n%s", got, out)
	}
}

func TestAutolinkCommandRemoveDuplicatesFollowsTheSetting(t *testing.T) {
	for setting, want := range map[string]string{
		"true":  "[[X]] and X.\n",
		"false": "[[X]] and [[X]].\n",
	} {
		root := autolinkTestVault(t, map[string]string{"inbox/X.md": "# X\n", "inbox/Y.md": "[[X]] and [[X]].\n"})
		dir := os.Getenv("ZENNOTES_CONFIG_DIR")
		writeVaultFile(t, dir, "config.toml", "[terminal]\nautolink_remove_duplicates = "+setting+"\n")
		code, out := runAutolink(t, true, "\ny\n", "autolink", "inbox/Y.md", "--vault", root)
		if code != 0 {
			t.Fatalf("%s: code %d\n%s", setting, code, out)
		}
		if got := readVaultFile(t, root, "inbox/Y.md"); got != want {
			t.Fatalf("%s: %q", setting, got)
		}
	}
}

func TestAutolinkCommandServerGetsNoRequest(t *testing.T) {
	isolatedCLI(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	code, out := runAutolink(t, true, "\ny\n", "autolink", "--all", "--server", server.URL, "--token", "t")
	if code == 0 || !strings.Contains(out, "local vault") {
		t.Fatalf("code %d\n%s", code, out)
	}
	if n := requests.Load(); n != 0 {
		t.Fatalf("the server got %d requests", n)
	}
}

func TestAutolinkCommandHelpRows(t *testing.T) {
	isolatedCLI(t)
	out := captureOutput(t, func() { Main([]string{"help"}) })
	for _, want := range []string{"autolink <path>", "autolink --all"} {
		if !strings.Contains(out, want) {
			t.Fatalf("zn help lacks %q", want)
		}
	}
	out = captureOutput(t, func() { Main([]string{"autolink", "--help"}) })
	if strings.Contains(strings.ToLower(out), "duplicate") {
		t.Fatalf("autolink help shows a flag for duplicates:\n%s", out)
	}
	if _, ok := commandSpecs()["autolink --all"]; ok {
		t.Fatal("autolink --all is a command of its own")
	}
	count := 0
	for _, root := range commandRoots() {
		if root == "autolink" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("commandRoots lists autolink %d times", count)
	}
}
