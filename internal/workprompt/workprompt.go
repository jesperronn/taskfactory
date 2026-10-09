// Package workprompt builds the single prompt string a worker adapter receives
// for a claimed task, and names and opens the per-run log file under
// .taskfactory/logs. It reads files only and never runs a command or launches
// a process. It never reads the environment.
package workprompt

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"taskfactory/internal/taskvalidate"
	"taskfactory/internal/verify"
)

// Prompt is the built worker prompt and the Claim fields it was built from.
type Prompt struct {
	Text       string
	Worktree   string
	Branch     string
	BaseCommit string
}

// Header is the first block of a run log. It holds only the run parameters,
// never environment values.
type Header struct {
	TaskID  string
	Adapter string
	Model   string
	Timeout time.Duration
	Start   time.Time
}

// Adapters lists the adapter names accepted by OpenLog.
var Adapters = []string{"omp", "pi", "claude"}

const (
	instructionsPath = "docs/worker-instructions.md"
	logTimeLayout    = "20060102T150405Z"
)

var (
	taskIDPattern  = regexp.MustCompile(`^TF-[0-9]{3}$`)
	logPathPattern = regexp.MustCompile(`(?:^|/)\.taskfactory/logs/(TF-[0-9]{3})/(omp|pi|claude)-[0-9]{8}T[0-9]{6}Z\.log$`)
)

// Build returns the prompt for the single active task file for id. The text is,
// in order: a header naming the task, branch and worktree; the task file bytes
// verbatim; the worker instructions read from the claimed worktree; and a
// closing rules block. It refuses a task that is not in tasks/active, a task
// without a complete Claim block, and a worktree without the worker
// instructions only when the read fails for a reason other than the file not
// existing; a missing file falls back to built-in text. It reads files only and is deterministic.
func Build(projectRoot, id string) (Prompt, error) {
	if !taskIDPattern.MatchString(id) {
		return Prompt{}, fmt.Errorf("task %q is not a valid task ID (expected TF-NNN)", id)
	}
	root, err := filepath.EvalSymlinks(projectRoot)
	if err != nil {
		return Prompt{}, fmt.Errorf("resolve project root %s: %w", projectRoot, err)
	}
	activeAbs, err := findActive(root, id)
	if err != nil {
		return Prompt{}, err
	}
	rel := filepath.ToSlash(strings.TrimPrefix(activeAbs, root+string(filepath.Separator)))
	if diagnostics := taskvalidate.Validate(root, rel); len(diagnostics) > 0 {
		return Prompt{}, fmt.Errorf("refusing to build prompt for %s: %s", id, diagnostics[0])
	}
	data, err := os.ReadFile(activeAbs)
	if err != nil {
		return Prompt{}, fmt.Errorf("read active task %s: %w", rel, err)
	}
	fields := verify.ClaimFields(data)
	branch, base, worktree := fields["Branch"], fields["Base commit"], fields["Worktree"]
	if branch == "" || base == "" || worktree == "" {
		return Prompt{}, fmt.Errorf("refusing to build prompt for %s: incomplete Claim block", id)
	}
	instructions, err := os.ReadFile(filepath.Join(worktree, filepath.FromSlash(instructionsPath)))
	builtIn := false
	switch {
	case err == nil:
	case errors.Is(err, os.ErrNotExist):
		instructions, builtIn = []byte(defaultInstructions), true
	default:
		return Prompt{}, fmt.Errorf("refusing to build prompt for %s: read %s in worktree %s: %w", id, instructionsPath, worktree, err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# TaskFactory worker prompt\n\nTask: %s\nBranch: %s\nWorktree: %s\n\n", id, branch, worktree)
	fmt.Fprintf(&b, "----- BEGIN task file %s -----\n", rel)
	b.Write(data)
	b.WriteString(terminator(data))
	fmt.Fprintf(&b, "----- END task file %s -----\n\n", rel)
	if builtIn {
		fmt.Fprintf(&b, "%s\n", BuiltInMarker)
	} else {
		fmt.Fprintf(&b, "----- BEGIN %s from the worktree -----\n", instructionsPath)
	}
	b.Write(instructions)
	b.WriteString(terminator(instructions))
	if builtIn {
		b.WriteString("----- END built-in worker instructions -----\n\n")
	} else {
		fmt.Fprintf(&b, "----- END %s -----\n\n", instructionsPath)
	}
	b.WriteString(rules(worktree))
	return Prompt{Text: b.String(), Worktree: worktree, Branch: branch, BaseCommit: base}, nil
}

// BuiltInMarker is the line that introduces the built-in instructions in a
// prompt built for a worktree without docs/worker-instructions.md.
const BuiltInMarker = "Built-in worker instructions (the project has no docs/worker-instructions.md)"

// defaultInstructions is used only when the worktree has no
// docs/worker-instructions.md. It is constant, so the prompt stays
// deterministic.
const defaultInstructions = `Read the task contract above before editing anything.
Work only inside the worktree named in the header. Keep unrelated files
untouched.
Run the task's own verification commands. Also run bin/test and bin/lint from
the worktree root when they exist.
Commit the result with plain git commit. Never change Git signing settings.
Report each command you ran with its exit code, and the result commit.
If a check cannot pass or you are blocked, stop and report the blocker with the
failing command output. Do not weaken or skip a check to obtain a pass.
`

// terminator returns the newline needed to end data on its own line.
func terminator(data []byte) string {
	if len(data) == 0 || data[len(data)-1] == '\n' {
		return ""
	}
	return "\n"
}

// rules is the closing block of every prompt.
func rules(worktree string) string {
	return "----- RULES -----\n" +
		"- Work only inside the worktree " + worktree + ".\n" +
		"- Commit the result with plain git commit. Never change Git signing configuration.\n" +
		"- Do not run integrate, fail or promote, and do not push.\n" +
		"- Run the task's own checks, then bin/test and bin/lint from the worktree root,\n" +
		"  and report each exit code.\n" +
		"- If a check cannot pass, stop and report a blocker. Do not weaken a check.\n" +
		"----- END RULES -----\n"
}

// findActive returns the path of the only active task file for id.
func findActive(root, id string) (string, error) {
	dir := filepath.Join(root, "tasks", "active")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read tasks/active: %w", err)
	}
	var found []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), id+"-") && strings.HasSuffix(entry.Name(), ".md") {
			found = append(found, entry.Name())
		}
	}
	switch len(found) {
	case 0:
		return "", fmt.Errorf("refusing to build prompt for %s: task is not in tasks/active", id)
	case 1:
		return filepath.Join(dir, found[0]), nil
	default:
		return "", fmt.Errorf("refusing to build prompt for %s: multiple active task files match this ID", id)
	}
}

// LogPath returns <projectRoot>/.taskfactory/logs/<id>/<adapter>-<UTC>.log,
// where <UTC> is at in UTC as 20060102T150405Z.
func LogPath(projectRoot, id, adapter string, at time.Time) string {
	name := adapter + "-" + at.UTC().Format(logTimeLayout) + ".log"
	return filepath.Join(projectRoot, ".taskfactory", "logs", id, name)
}

// OpenLog creates the log for path with mode 0644, writes the header block and
// returns the file. path must be a LogPath result for h.TaskID and h.Adapter;
// an unknown adapter, an invalid header and a path in another layout are
// refused. It creates the parent directories with mode 0755. If the base name
// exists, it tries <base>_2.log, <base>_3.log and so on, up to maxLogAttempts
// names in all, each created with O_EXCL, so concurrent opens never overwrite
// each other. The returned file's Name() is the path actually used. The caller
// closes the returned file.
func OpenLog(path string, h Header) (*os.File, error) {
	if !taskIDPattern.MatchString(h.TaskID) {
		return nil, fmt.Errorf("log task %q is not a valid task ID (expected TF-NNN)", h.TaskID)
	}
	if !knownAdapter(h.Adapter) {
		return nil, fmt.Errorf("log adapter %q is not one of %s", h.Adapter, strings.Join(Adapters, ", "))
	}
	if strings.ContainsAny(h.Model, "\r\n") {
		return nil, errors.New("log model must be a single line")
	}
	match := logPathPattern.FindStringSubmatch(filepath.ToSlash(path))
	if match == nil || match[1] != h.TaskID || match[2] != h.Adapter {
		return nil, fmt.Errorf("log path %s does not match the layout for %s %s", path, h.TaskID, h.Adapter)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create log directory %s: %w", filepath.Dir(path), err)
	}
	var file *os.File
	for n := 1; n <= maxLogAttempts && file == nil; n++ {
		candidate := path
		if n > 1 {
			candidate = suffixedLogPath(path, n)
		}
		f, err := os.OpenFile(candidate, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		switch {
		case err == nil:
			file = f
		case os.IsExist(err):
			continue
		default:
			return nil, fmt.Errorf("create log %s: %w", candidate, err)
		}
	}
	if file == nil {
		return nil, fmt.Errorf("refusing to open a log for %s: all %d names from %s are taken", h.TaskID, maxLogAttempts, path)
	}
	text := fmt.Sprintf("TaskFactory worker log\nTask: %s\nAdapter: %s\nModel: %s\nTimeout: %s\nStarted at: %s\n\n",
		h.TaskID, h.Adapter, h.Model, h.Timeout, h.Start.UTC().Format(time.RFC3339))
	if _, err := file.WriteString(text); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("write log header %s: %w", file.Name(), err)
	}
	return file, nil
}

// suffixedLogPath returns the nth candidate name for a base log path, n >= 2:
// <base>_<n>.log. The "_" separator sorts after "." in byte order, so suffixed
// names sort after the base name of the same second.
func suffixedLogPath(base string, n int) string {
	return strings.TrimSuffix(base, ".log") + "_" + strconv.Itoa(n) + ".log"
}

// maxLogAttempts bounds the names OpenLog tries for one base path: the base
// name, then the suffixes _2 to _1000.
const maxLogAttempts = 1000

func knownAdapter(name string) bool {
	for _, a := range Adapters {
		if a == name {
			return true
		}
	}
	return false
}
