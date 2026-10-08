package integrate

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// setMainChecks replaces the verification.main lines in the fixture config and
// commits the change so Run's clean-config requirement still holds.
func setMainChecks(t *testing.T, root string, commands ...string) {
	t.Helper()
	p := filepath.Join(root, ".taskfactory/config.toml")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var kept []string
	for _, line := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
		if !strings.HasPrefix(line, "main=") {
			kept = append(kept, line)
		}
	}
	quoted := make([]string, 0, len(commands))
	for _, c := range commands {
		quoted = append(quoted, strconv.Quote(c))
	}
	updated := strings.Join(kept, "\n") + "\n"
	if len(commands) > 0 {
		updated += "main=[" + strings.Join(quoted, ", ") + "]\n"
	}
	if err = os.WriteFile(p, []byte(updated), 0644); err != nil {
		t.Fatal(err)
	}
	run(t, root, "git", "add", "--", ".taskfactory/config.toml")
	if _, err = gitOutputErr(root, "diff", "--cached", "--quiet"); err == nil {
		return // unchanged config: nothing to commit
	}
	run(t, root, "git", "commit", "-m", "set main checks")
}

func mainHead(t *testing.T, root string) string {
	t.Helper()
	v, err := git(root, "rev-parse", "refs/heads/main")
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func stopPath(root string) string { return filepath.Join(root, ".taskfactory/integration-stop.json") }

func readOptional(t *testing.T, p string) ([]byte, bool) {
	t.Helper()
	b, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return nil, false
	}
	if err != nil {
		t.Fatal(err)
	}
	return b, true
}

func exitPtr(v int) *int { return &v }

// expectedStop renders a record exactly as the protocol specifies it on disk.
func expectedStop(t *testing.T, rec stopRecord) []byte {
	t.Helper()
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(b, '\n')
}

func mustStop(t *testing.T, root string) []byte {
	t.Helper()
	b, ok := readOptional(t, stopPath(root))
	if !ok {
		t.Fatalf("stop record %s missing", stopPath(root))
	}
	return b
}

// failingMainRun creates a real stop record through Run's post-merge failure path.
func failingMainRun(t *testing.T, f fixture) {
	t.Helper()
	setMainChecks(t, f.root, "false")
	if err := Run(f.root, "TF-901"); err == nil {
		t.Fatal("Run unexpectedly succeeded")
	}
	mustStop(t, f.root)
}

// C1: a failed main check persists the exact stop record and blocks later integration.
func TestCheckMainFailurePersistsExactStopAndBlocksIntegration(t *testing.T) {
	f := newFixture(t)
	setMainChecks(t, f.root, "echo boom; exit 3")
	main := mainHead(t, f.root)
	if err := CheckMain(f.root); err == nil {
		t.Fatal("CheckMain unexpectedly succeeded")
	}
	want := expectedStop(t, stopRecord{Main: main, Command: "echo boom; exit 3", Exit: exitPtr(3), Output: "boom\n"})
	if got := mustStop(t, f.root); !bytes.Equal(got, want) {
		t.Fatalf("stop bytes\n got: %q\nwant: %q", got, want)
	}
	if _, ok := readOptional(t, stopPath(f.root)+".tmp"); ok {
		t.Fatal("temporary stop sibling left behind")
	}
	before := mainHead(t, f.root)
	if err := Run(f.root, "TF-901"); err == nil || !strings.Contains(err.Error(), "integration stopped") {
		t.Fatalf("Run after failed check-main err=%v", err)
	}
	if mainHead(t, f.root) != before {
		t.Fatal("blocked integration moved main")
	}
}

// C2: a failed recheck updates the existing stop record and keeps it present.
func TestCheckMainFailedRecheckUpdatesAndRetainsStop(t *testing.T) {
	f := newFixture(t)
	failingMainRun(t, f)
	setMainChecks(t, f.root, "echo second >&2; exit 4")
	main := mainHead(t, f.root)
	if err := CheckMain(f.root); err == nil {
		t.Fatal("CheckMain unexpectedly succeeded")
	}
	want := expectedStop(t, stopRecord{Main: main, Command: "echo second >&2; exit 4", Exit: exitPtr(4), Output: "second\n"})
	if got := mustStop(t, f.root); !bytes.Equal(got, want) {
		t.Fatalf("updated stop bytes\n got: %q\nwant: %q", got, want)
	}
}

// C3: passing checks on current main clear valid stop state and allow integration.
func TestCheckMainPassingRecheckClearsStopAndAllowsIntegration(t *testing.T) {
	f := newFixture(t)
	failingMainRun(t, f)
	setMainChecks(t, f.root, "true")
	if err := CheckMain(f.root); err != nil {
		t.Fatalf("CheckMain: %v", err)
	}
	if _, ok := readOptional(t, stopPath(f.root)); ok {
		t.Fatal("canonical stop record remains after passing checks")
	}
	if _, ok := readOptional(t, stopPath(f.root)+".tmp"); ok {
		t.Fatal("temp stop record remains after passing checks")
	}
	if err := Run(f.root, "TF-901"); err != nil && strings.Contains(err.Error(), "integration stopped") {
		t.Fatalf("integration still stopped after clearing: %v", err)
	}
}

// Checks run in configured order and stop at the first failure.
func TestCheckMainRunsChecksInOrderAndStopsAtFirstFailure(t *testing.T) {
	f := newFixture(t)
	log := filepath.Join(t.TempDir(), "log")
	setMainChecks(t, f.root, "echo a >> "+log, "echo b >> "+log+"; exit 1", "echo c >> "+log)
	if err := CheckMain(f.root); err == nil {
		t.Fatal("CheckMain unexpectedly succeeded")
	}
	got, err := os.ReadFile(log)
	if err != nil || string(got) != "a\nb\n" {
		t.Fatalf("check order log=%q err=%v", got, err)
	}
	var rec struct {
		Command string `json:"command"`
	}
	if err = json.Unmarshal(mustStop(t, f.root), &rec); err != nil || rec.Command != "echo b >> "+log+"; exit 1" {
		t.Fatalf("stop command=%q err=%v", rec.Command, err)
	}
}

func TestCheckMainPassingChecksRunInOrder(t *testing.T) {
	f := newFixture(t)
	log := filepath.Join(t.TempDir(), "log")
	setMainChecks(t, f.root, "echo a >> "+log, "echo b >> "+log)
	if err := CheckMain(f.root); err != nil {
		t.Fatalf("CheckMain: %v", err)
	}
	if got, _ := os.ReadFile(log); string(got) != "a\nb\n" {
		t.Fatalf("check order log=%q", got)
	}
}

// C2/C4: a valid lone temp sibling is treated as the stop and replaced on failure.
func TestCheckMainLoneValidTempIsReplacedOnFailure(t *testing.T) {
	f := newFixture(t)
	tempRec := expectedStop(t, stopRecord{Main: strings.Repeat("a", 40), Command: "old", Exit: exitPtr(9), Output: "old\n"})
	if err := os.WriteFile(stopPath(f.root)+".tmp", tempRec, 0600); err != nil {
		t.Fatal(err)
	}
	setMainChecks(t, f.root, "echo again; exit 5")
	main := mainHead(t, f.root)
	if err := CheckMain(f.root); err == nil {
		t.Fatal("CheckMain unexpectedly succeeded")
	}
	want := expectedStop(t, stopRecord{Main: main, Command: "echo again; exit 5", Exit: exitPtr(5), Output: "again\n"})
	if got := mustStop(t, f.root); !bytes.Equal(got, want) {
		t.Fatalf("stop bytes\n got: %q\nwant: %q", got, want)
	}
	if _, ok := readOptional(t, stopPath(f.root)+".tmp"); ok {
		t.Fatal("temp sibling remains after replacement")
	}
}

// C4: a lone valid temp sibling is cleared by passing checks.
func TestCheckMainLoneValidTempIsClearedOnPass(t *testing.T) {
	f := newFixture(t)
	tempRec := expectedStop(t, stopRecord{Main: strings.Repeat("b", 40), Command: "old", Exit: exitPtr(9)})
	if err := os.WriteFile(stopPath(f.root)+".tmp", tempRec, 0600); err != nil {
		t.Fatal(err)
	}
	setMainChecks(t, f.root, "true")
	if err := CheckMain(f.root); err != nil {
		t.Fatalf("CheckMain: %v", err)
	}
	if _, ok := readOptional(t, stopPath(f.root)+".tmp"); ok {
		t.Fatal("valid temp sibling not cleared")
	}
}

// C4: malformed canonical and temp records fail closed, stay byte-identical, and
// do not run any main check.
func TestCheckMainMalformedStopFailsClosedUnchanged(t *testing.T) {
	cases := map[string][]byte{
		"not json":       []byte("partial"),
		"missing key":    []byte(`{"main_commit":"` + strings.Repeat("c", 40) + `","command":"x","exit_code":1,"output":""}`),
		"extra key":      []byte(`{"main_commit":"` + strings.Repeat("c", 40) + `","command":"x","exit_code":1,"output":"","error":"","extra":true}`),
		"bad oid":        []byte(`{"main_commit":"ABC","command":"x","exit_code":1,"output":"","error":""}`),
		"empty command":  []byte(`{"main_commit":"` + strings.Repeat("c", 40) + `","command":"","exit_code":1,"output":"","error":""}`),
		"null exit only": []byte(`{"main_commit":"` + strings.Repeat("c", 40) + `","command":"x","exit_code":null,"output":"","error":""}`),
	}
	for name, content := range cases {
		for _, place := range []string{"canonical", "temp"} {
			t.Run(name+"/"+place, func(t *testing.T) {
				f := newFixture(t)
				marker := filepath.Join(t.TempDir(), "ran")
				path := stopPath(f.root)
				if place == "temp" {
					path += ".tmp"
				}
				if err := os.WriteFile(path, content, 0600); err != nil {
					t.Fatal(err)
				}
				setMainChecks(t, f.root, "touch "+marker)
				if err := CheckMain(f.root); err == nil {
					t.Fatal("CheckMain unexpectedly succeeded")
				}
				if got, _ := os.ReadFile(path); !bytes.Equal(got, content) {
					t.Fatalf("malformed record changed: %q", got)
				}
				if _, ok := readOptional(t, marker); ok {
					t.Fatal("main checks ran despite malformed stop record")
				}
			})
		}
	}
}

func TestCheckMainAmbiguousCanonicalAndTempFailsClosed(t *testing.T) {
	f := newFixture(t)
	failingMainRun(t, f)
	canonical := mustStop(t, f.root)
	tempRec := expectedStop(t, stopRecord{Main: strings.Repeat("d", 40), Command: "y", Exit: exitPtr(2)})
	if err := os.WriteFile(stopPath(f.root)+".tmp", tempRec, 0600); err != nil {
		t.Fatal(err)
	}
	setMainChecks(t, f.root, "true")
	if err := CheckMain(f.root); err == nil {
		t.Fatal("CheckMain unexpectedly succeeded with canonical and temp records")
	}
	if got := mustStop(t, f.root); !bytes.Equal(got, canonical) {
		t.Fatal("canonical record changed")
	}
	if got, _ := os.ReadFile(stopPath(f.root) + ".tmp"); !bytes.Equal(got, tempRec) {
		t.Fatal("temp record changed")
	}
}

// Stop present but no main checks configured fails closed with an actionable error.
func TestCheckMainStopWithoutMainChecksFailsClosed(t *testing.T) {
	f := newFixture(t)
	failingMainRun(t, f)
	before := mustStop(t, f.root)
	setMainChecks(t, f.root)
	err := CheckMain(f.root)
	if err == nil || !strings.Contains(err.Error(), "has no commands") {
		t.Fatalf("CheckMain err=%v; want actionable verification.main error", err)
	}
	if got := mustStop(t, f.root); !bytes.Equal(got, before) {
		t.Fatal("stop record changed")
	}
}

// Without a stop and without main checks there is nothing to verify or clear.
func TestCheckMainNoStopNoChecksSucceeds(t *testing.T) {
	f := newFixture(t)
	setMainChecks(t, f.root)
	if err := CheckMain(f.root); err != nil {
		t.Fatalf("CheckMain: %v", err)
	}
	if _, ok := readOptional(t, stopPath(f.root)); ok {
		t.Fatal("stop record created")
	}
}

// Main moving during checks changes no stop state and records nothing.
func TestCheckMainMainMovingDuringChecksChangesNoStopState(t *testing.T) {
	t.Run("no prior stop", func(t *testing.T) {
		f := newFixture(t)
		setMainChecks(t, f.root, moveAndFailCommand(t, f.root))
		if err := CheckMain(f.root); err == nil || !strings.Contains(err.Error(), "moved") {
			t.Fatalf("CheckMain err=%v; want main-moved error", err)
		}
		if _, ok := readOptional(t, stopPath(f.root)); ok {
			t.Fatal("stop record written while main moved")
		}
	})
	t.Run("prior stop bytes preserved", func(t *testing.T) {
		f := newFixture(t)
		failingMainRun(t, f)
		before := mustStop(t, f.root)
		setMainChecks(t, f.root, moveOnceCommand(t, f.root))
		if err := CheckMain(f.root); err == nil || !strings.Contains(err.Error(), "moved") {
			t.Fatalf("CheckMain err=%v; want main-moved error", err)
		}
		if got := mustStop(t, f.root); !bytes.Equal(got, before) {
			t.Fatalf("stop bytes changed\n got: %q\nwant: %q", got, before)
		}
	})
}

// Main moving alone never clears the stop; a failing recheck records the new main.
func TestCheckMainRecordsCurrentMainAfterMainMoves(t *testing.T) {
	f := newFixture(t)
	failingMainRun(t, f)
	before := mustStop(t, f.root)
	run(t, f.root, "git", "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-m", "unrelated move")
	setMainChecks(t, f.root, "echo new >&2; exit 1")
	if err := CheckMain(f.root); err == nil {
		t.Fatal("CheckMain unexpectedly succeeded")
	}
	if got := mustStop(t, f.root); bytes.Equal(got, before) {
		t.Fatal("stop record was not updated to the current main commit")
	}
}

// The check-main run waits for the integration lock like integrate does.
func TestCheckMainWaitsForIntegrationLock(t *testing.T) {
	f := newFixture(t)
	setMainChecks(t, f.root)
	unlock, err := acquireIntegrationLock(f.root)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- CheckMain(f.root) }()
	select {
	case err = <-done:
		t.Fatalf("CheckMain did not wait for lock: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	unlock()
	select {
	case err = <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("CheckMain stayed blocked after lock release")
	}
}

func moveAndFailCommand(t *testing.T, root string) string {
	t.Helper()
	once := filepath.Join(t.TempDir(), "once")
	return "if mkdir " + once + "; then git -C " + root + " -c user.name=Test -c user.email=test@example.invalid commit --allow-empty -m moved; fi; exit 1"
}

func moveOnceCommand(t *testing.T, root string) string {
	t.Helper()
	once := filepath.Join(t.TempDir(), "once")
	return "if mkdir " + once + "; then git -C " + root + " -c user.name=Test -c user.email=test@example.invalid commit --allow-empty -m moved; fi; exit 0"
}
