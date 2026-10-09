// Command taskfactory is the TaskFactory CLI. It coordinates software work into
// explicit, verifiable tasks that coding agents can execute safely in parallel.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"taskfactory/internal/claim"
	"taskfactory/internal/config"
	"taskfactory/internal/integrate"
	"taskfactory/internal/promote"
	"taskfactory/internal/taskvalidate"
	"taskfactory/internal/ui"
	"taskfactory/internal/verify"
)

// version is printed by --version. It defaults to "dev" for plain go build or
// go run; bin/build overrides it at link time with -ldflags "-X main.version=...".
var version = "dev"

// commandSummary is one top-level command: its name, its usage synopsis and a
// short description shown in the top-level usage.
type commandSummary struct {
	name     string
	synopsis string
	summary  string
}

// commandSummaries lists every command in top-level usage order.
var commandSummaries = []commandSummary{
	{"init", "init", "initialize project config"},
	{"status", "status", "show tasks by state"},
	{"validate", "validate [path...]", "check task files"},
	{"claim", "claim <ID> --owner <name>", "claim a ready task"},
	{"verify", "verify <ID>", "run a task's checks"},
	{"integrate", "integrate <ID>", "fast-forward a verified task"},
	{"check-main", "check-main", "recheck a stopped main"},
	{"promote", "promote <ID>", "move an inbox task to ready"},
}

// commandHelps maps each command in commandSummaries to its per-command help.
var commandHelps = map[string]string{
	"init":       initHelp,
	"status":     statusHelp,
	"validate":   validateHelp,
	"claim":      claimHelp,
	"verify":     verifyHelp,
	"integrate":  integrateHelp,
	"check-main": checkMainHelp,
	"promote":    promoteHelp,
}

// styleArguments colors the words after a subcommand name: flags starting with
// "--" are cyan and everything else, such as <ID>, is dim. Words are joined with
// the original spaces so the plain text is unchanged.
func styleArguments(p ui.Painter, rest string) string {
	words := strings.Split(rest, " ")
	for i, word := range words {
		if strings.HasPrefix(word, "--") {
			words[i] = p.Cyan(word)
		} else {
			words[i] = p.Dim(word)
		}
	}
	return strings.Join(words, " ")
}

// styleHelp applies color to a per-command help constant: the subcommand name is
// yellow, flags are cyan, placeholders are dim and headings are bold. With color
// disabled the result is byte-identical to text.
func styleHelp(p ui.Painter, text string) string {
	var b strings.Builder
	for _, line := range strings.SplitAfter(text, "\n") {
		body := strings.TrimSuffix(line, "\n")
		newline := line[len(body):]
		switch {
		case strings.HasPrefix(body, "Usage: taskfactory "):
			rest := strings.TrimPrefix(body, "Usage: taskfactory ")
			name := strings.Fields(rest)[0]
			b.WriteString(p.Bold("Usage:") + " " + p.Cyan("taskfactory") + " " + p.Yellow(name) + p.Dim(rest[len(name):]) + newline)
		case body == "Flags:" || body == "Exit codes:":
			b.WriteString(p.Bold(body) + newline)
		case strings.HasPrefix(body, "  --"):
			rest := body[2:]
			flag, tail := rest, ""
			if k := strings.Index(rest, "  "); k >= 0 {
				flag, tail = rest[:k], rest[k:]
			}
			styled := p.Cyan(flag)
			if i := strings.Index(flag, " <"); i >= 0 {
				styled = p.Cyan(flag[:i]) + p.Dim(flag[i:])
			}
			b.WriteString("  " + styled + tail + newline)
		default:
			b.WriteString(line)
		}
	}
	return b.String()
}

// wantsHelp reports whether args contain a help flag in any position.
func wantsHelp(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "-h", "-help", "--h", "--help":
			return true
		}
	}
	return false
}

// usage returns the help text printed by --help and -h. It lists every command
// with a description aligned in one column, and documents only the global flags
// --help and --version. Styling is applied only when p is enabled, so plain
// output is unchanged when color is off.
func usage(p ui.Painter) string {
	synopsisWidth := 0
	for _, command := range commandSummaries {
		if len(command.synopsis) > synopsisWidth {
			synopsisWidth = len(command.synopsis)
		}
	}

	var b strings.Builder
	b.WriteString("taskfactory coordinates software work into explicit, verifiable tasks.\n\n")
	b.WriteString(p.Bold("Usage:") + "\n")
	b.WriteString("  " + p.Cyan("taskfactory") + " " + p.Dim("[global flags]") + "\n")
	b.WriteString("  " + p.Cyan("taskfactory") + " " + p.Dim("<command> [flags]") + "\n\n")
	b.WriteString(p.Bold("Commands:") + "\n")
	for _, command := range commandSummaries {
		rest := strings.TrimPrefix(command.synopsis, command.name)
		padding := strings.Repeat(" ", synopsisWidth-len(command.synopsis)+2)
		b.WriteString("  " + p.Yellow(command.name) + styleArguments(p, rest) + padding + command.summary + "\n")
	}
	b.WriteString("\n" + p.Bold("Global flags:") + "\n")
	b.WriteString("  " + p.Cyan("--help") + "     print this usage and exit successfully\n")
	b.WriteString("  " + p.Cyan("--version") + "  print the CLI version string and exit successfully\n\n")
	b.WriteString("Run \"taskfactory <command> --help\" for details on one command.\n")
	return b.String()
}

// errorLine formats a stderr error as "<prefix>: <msg>" with a red prefix when
// color is enabled for that stream.
func errorLine(p ui.Painter, prefix, msg string) string {
	return p.Red(prefix) + ": " + msg
}

// exitUsage is the exit code returned when global flags are used incorrectly or
// an unknown flag is supplied.
const exitUsage = 2

const defaultConfig = `protocol_version = 1

[workers]
max_parallel = 4

[git]
use_worktrees = true
worktree_root = ".taskfactory/worktrees"
integration_strategy = "ff-only"

[integration]
stop_on_main_failure = true

[verification]
worker = ["go test ./..."]
integration = ["go test ./...", "go vet ./..."]
`

var taskStates = []string{"inbox", "ready", "active", "failed", "archive"}

// verifyHelp is printed by "verify --help" and "verify -h".
const verifyHelp = `Usage: taskfactory verify <ID>

Run the success-criteria checks of the claimed task <ID> and record each outcome
in .taskfactory/evidence/<ID>.jsonl. After the attempt is recorded it prints one
summary line on stdout, such as "verify TF-001: PASS (3 checks)" or
"verify TF-001: FAIL (1 of 3 checks failed)". The command exits 1 when any
check fails.

Flags:
  --help, -h  print this help and exit successfully

Exit codes:
  0  every check passed
  1  a check failed or verification could not run
  2  invalid usage
`

// integrateHelp is printed by "integrate --help" and "integrate -h".
const integrateHelp = `Usage: taskfactory integrate <ID>

Fast-forward main to the verified task <ID> using the configured integration
strategy, after running the integration verification commands. On success it
prints "integrated <ID>". A failed check or fast-forward stops integration and
is reported on stderr.

Flags:
  --help, -h  print this help and exit successfully

Exit codes:
  0  the task was integrated
  1  integration failed
  2  invalid usage
`

// checkMainHelp is printed by "check-main --help" and "check-main -h".
const checkMainHelp = `Usage: taskfactory check-main

Recheck main after integration has stopped it by running the integration
verification commands again. It prints "check-main: ok" when they pass.

Flags:
  --help, -h  print this help and exit successfully

Exit codes:
  0  main passes the integration checks
  1  a check failed or main could not be checked
  2  invalid usage
`

// promoteHelp is printed by "promote --help" and "promote -h".
const promoteHelp = `Usage: taskfactory promote <ID>

Promote the inbox task <ID> to tasks/ready when its complete executable contract
validates. The task file moves unchanged from tasks/inbox to tasks/ready, the
whole task tree is validated, and only the two task paths are committed. Any
refusal or failure restores the file to tasks/inbox.

The commit runs as a plain git commit and follows your Git signing
configuration (commit.gpgsign, gpg.program, user.signingkey). TaskFactory never
disables or overrides signing.

Flags:
  --help, -h  print this help and exit successfully

Exit codes:
  0  the task was promoted and committed
  1  promotion was refused or failed; the file was restored
  2  invalid usage
`

func promoteTask(args []string) error {
	root, err := projectRoot()
	if err != nil {
		return err
	}
	ready, err := promote.Promote(root, args[0])
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "promoted %s to %s\n", args[0], ready)
	return nil
}

func main() {
	fs := flag.NewFlagSet("taskfactory", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	// Keep flag.Parse from printing the full usage text on invalid input.
	fs.Usage = func() {}
	// Color is decided per stream: stdout for help, stderr for errors.
	stdoutStyle := ui.For(os.Stdout)
	stderrStyle := ui.For(os.Stderr)

	help := fs.Bool("help", false, "print this usage and exit successfully")
	showVersion := fs.Bool("version", false, "print the CLI version string and exit successfully")

	if err := fs.Parse(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory", err.Error()))
		os.Exit(exitUsage)
	}

	if *help {
		fmt.Fprint(os.Stdout, usage(stdoutStyle))
		return
	}

	if *showVersion {
		fmt.Fprintln(os.Stdout, "taskfactory version "+version)
		return
	}

	args := fs.Args()
	if len(args) > 0 && wantsHelp(args[1:]) {
		if text, ok := commandHelps[args[0]]; ok {
			fmt.Fprint(os.Stdout, styleHelp(stdoutStyle, text))
			return
		}
	}
	if len(args) == 1 && args[0] == "init" {
		if err := initializeProject(); err != nil {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory init", err.Error()))
			os.Exit(1)
		}
		return
	}
	if len(args) >= 1 && args[0] == "validate" {
		if err := validateProject(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory validate", err.Error()))
			var usage usageError
			if errors.As(err, &usage) {
				os.Exit(exitUsage)
			}
			os.Exit(1)
		}
		return
	}
	if len(args) == 1 && args[0] == "status" {
		if err := statusProject(); err != nil {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory status", err.Error()))
			os.Exit(1)
		}
		return
	}
	if len(args) > 0 && args[0] == "claim" {
		if err := claimTask(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory claim", err.Error()))
			os.Exit(1)
		}
		return
	}
	if len(args) > 0 && args[0] == "verify" {
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory verify", "usage: taskfactory verify <ID>"))
			os.Exit(exitUsage)
		}
		if looksLikePath(args[1]) {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory verify", "usage: taskfactory verify <ID>; did you mean `taskfactory validate <path>`?"))
			os.Exit(exitUsage)
		}
		root, err := projectRoot()
		var result verify.Result
		if err == nil {
			result, err = verify.Verify(root, args[1])
		}
		if result.Outcome != "" {
			fmt.Fprintln(os.Stdout, result.Summary(stdoutStyle))
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory verify", err.Error()))
			os.Exit(1)
		}
		return
	}
	if len(args) > 0 && args[0] == "integrate" {
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory integrate", "usage: taskfactory integrate <ID>"))
			os.Exit(exitUsage)
		}
		root, err := projectRoot()
		if err == nil {
			err = integrate.Run(root, args[1])
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory integrate", err.Error()))
			os.Exit(1)
		}
		fmt.Fprintf(os.Stdout, "integrated %s\n", args[1])
		return
	}
	if len(args) > 0 && args[0] == "promote" {
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory promote", "usage: taskfactory promote <ID>"))
			os.Exit(exitUsage)
		}
		if err := promoteTask(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory promote", err.Error()))
			var usage promote.UsageError
			if errors.As(err, &usage) {
				os.Exit(exitUsage)
			}
			os.Exit(1)
		}
		return
	}
	if len(args) > 0 && args[0] == "check-main" {
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory check-main", "usage: taskfactory check-main"))
			os.Exit(exitUsage)
		}
		root, err := projectRoot()
		if err == nil {
			err = integrate.CheckMain(root)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory check-main", err.Error()))
			os.Exit(1)
		}
		fmt.Fprintln(os.Stdout, "check-main: ok")
		return
	}

	fmt.Fprint(os.Stderr, usage(stderrStyle))
	os.Exit(exitUsage)
}

// claimHelp is printed by "claim --help" and "claim -h".
const claimHelp = `Usage: taskfactory claim <ID> --owner <name>

Claim the ready task <ID> for the named worker. The task file moves from
tasks/ready to tasks/active and records the claim details, including the owner.
The owner must be non-empty.

Flags:
  --owner <name>  worker identifier (required)
  --help, -h      print this help and exit successfully

Exit codes:
  0  the task was claimed
  1  the claim was refused or the arguments were invalid
`

func claimTask(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: taskfactory claim <ID> --owner <name>")
	}
	id := args[0]
	fs := flag.NewFlagSet("claim", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	owner := fs.String("owner", "", "worker identifier")
	if err := fs.Parse(args[1:]); err != nil {
		return fmt.Errorf("usage: taskfactory claim <ID> --owner <name>: %w", err)
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("usage: taskfactory claim <ID> --owner <name>")
	}
	if strings.TrimSpace(*owner) == "" {
		return fmt.Errorf("--owner must be non-empty")
	}
	root, err := projectRoot()
	if err != nil {
		return err
	}
	if err := claim.Claim(root, id, *owner); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "claimed %s for %s\n", id, *owner)
	return nil
}

// validateHelp is printed by "validate --help" and "validate -h".
const validateHelp = `Usage: taskfactory validate [path...]

Validate task files. With no arguments, validate every task file in tasks/inbox
and tasks/ready. Each path is a task file or a folder inside the project's tasks
directory; a folder selects the task files directly inside it, and "tasks"
selects all five state directories. Relative paths are resolved from the
project root. Task ID uniqueness and dependency references are always checked
across the whole tree, but only diagnostics for the selected files are reported.

Flags:
  --help, -h  print this help and exit successfully

Exit codes:
  0  every selected task file is valid
  1  a selected task file is invalid or the project configuration is invalid
  2  invalid usage, such as a path outside the tasks directory
`

// usageError marks invalid validate arguments, which exit with exitUsage.
type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func validateProject(args []string) error {
	root, err := projectRoot()
	if err != nil {
		return err
	}
	if _, err := config.Load(root); err != nil {
		return err
	}
	files, err := selectValidateFiles(root, args)
	if err != nil {
		return err
	}
	diagnostics := taskvalidate.ValidateFiles(root, files)
	for _, diagnostic := range diagnostics {
		fmt.Fprintln(os.Stderr, diagnostic.Error())
	}
	if len(diagnostics) > 0 {
		return fmt.Errorf("%d validation error(s)", len(diagnostics))
	}
	style := ui.For(os.Stdout)
	if len(args) == 0 {
		fmt.Fprintln(os.Stdout, style.Green("inbox and ready tasks are valid"))
	} else {
		fmt.Fprintln(os.Stdout, style.Green(fmt.Sprintf("%d task file(s) valid", len(files))))
	}
	return nil
}

// selectValidateFiles resolves validate arguments to slash-separated task file
// paths relative to root. No arguments selects inbox and ready.
func selectValidateFiles(root string, args []string) ([]string, error) {
	if len(args) == 0 {
		var files []string
		for _, state := range []string{"inbox", "ready"} {
			listed, err := listDirectoryFiles(root, filepath.Join("tasks", state))
			if err != nil {
				return nil, err
			}
			files = append(files, listed...)
		}
		return files, nil
	}
	seen := map[string]bool{}
	var files []string
	for _, arg := range args {
		listed, err := resolveValidateArg(root, arg)
		if err != nil {
			return nil, err
		}
		for _, file := range listed {
			if !seen[file] {
				seen[file] = true
				files = append(files, file)
			}
		}
	}
	return files, nil
}

// resolveValidateArg maps one validate argument to the task files it selects.
func resolveValidateArg(root, arg string) ([]string, error) {
	outside := usageError{"task path must resolve inside the project tasks directory"}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	path := arg
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, usageError{fmt.Sprintf("resolve task path: %v", err)}
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		if !insideTasks(root, path) && !insideTasks(resolvedRoot, path) {
			return nil, outside
		}
		return nil, usageError{fmt.Sprintf("task path %s cannot be read", arg)}
	}
	path = resolved
	rel, err := filepath.Rel(resolvedRoot, path)
	if err != nil || !insideTasks(resolvedRoot, path) {
		return nil, outside
	}
	rel = filepath.ToSlash(rel)
	info, err := os.Stat(path)
	if err != nil {
		return nil, usageError{fmt.Sprintf("task path %s cannot be read", arg)}
	}
	if info.IsDir() {
		if rel == "tasks" {
			return listTreeFiles(resolvedRoot, rel)
		}
		return listDirectoryFiles(resolvedRoot, rel)
	}
	if !info.Mode().IsRegular() || !strings.HasSuffix(rel, ".md") {
		return nil, usageError{fmt.Sprintf("%s is not a task file", rel)}
	}
	return []string{rel}, nil
}

// insideTasks reports whether path is the tasks directory under root or below it.
func insideTasks(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	return rel == "tasks" || strings.HasPrefix(rel, "tasks/")
}

// listDirectoryFiles returns the regular files directly inside the directory
// rel, a slash-separated path relative to root. A missing directory has none.
func listDirectoryFiles(root, rel string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(rel)))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		files = append(files, rel+"/"+entry.Name())
	}
	return files, nil
}

// listTreeFiles returns every regular file below the directory rel.
func listTreeFiles(root, rel string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(rel)), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			child, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(child))
		}
		return nil
	})
	return files, err
}

// looksLikePath reports whether a verify argument is a file path rather than a
// task ID, so the user can be pointed at validate.
func looksLikePath(arg string) bool {
	return strings.Contains(arg, "/") || strings.HasSuffix(arg, ".md")
}

// statusHelp is printed by "status --help" and "status -h".
const statusHelp = `Usage: taskfactory status

Print the number of task files in each state: inbox, ready, active, failed and
archive. The whole task tree is validated first; any diagnostics are printed to
stderr and no task file is changed.

Flags:
  --help, -h  print this help and exit successfully

Exit codes:
  0  the counts were printed
  1  the configuration, a state directory or a task file is invalid
  2  invalid usage
`

func statusProject() error {
	root, err := projectRoot()
	if err != nil {
		return err
	}
	if _, err := config.Load(root); err != nil {
		return err
	}
	for _, state := range taskStates {
		dir := filepath.Join(root, "tasks", state)
		info, err := os.Stat(dir)
		if err != nil {
			return fmt.Errorf("task state directory %s: %w", dir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("task state path %s is not a directory", dir)
		}
	}
	diagnostics := taskvalidate.Validate(root, "")
	for _, diagnostic := range diagnostics {
		fmt.Fprintln(os.Stderr, diagnostic.Error())
	}
	if len(diagnostics) > 0 {
		return fmt.Errorf("%d validation error(s)", len(diagnostics))
	}
	counts := make(map[string]int, len(taskStates))
	for _, state := range taskStates {
		entries, err := os.ReadDir(filepath.Join(root, "tasks", state))
		if err != nil {
			return fmt.Errorf("read task state directory %s: %w", filepath.Join(root, "tasks", state), err)
		}
		for _, entry := range entries {
			if entry.IsDir() || entry.Name() == ".gitkeep" {
				continue
			}
			counts[state]++
		}
	}
	for _, state := range taskStates {
		fmt.Fprintf(os.Stdout, "%s: %d\n", state, counts[state])
	}
	return nil
}

func projectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("determine current directory: %w", err)
	}
	command := exec.Command("git", "-C", cwd, "rev-parse", "--path-format=absolute", "--show-toplevel")
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("current directory is not inside a Git working tree")
	}
	rootValue := filepath.Clean(strings.TrimSuffix(string(output), "\n"))
	root, err := filepath.Abs(rootValue)
	if err == nil {
		root, err = filepath.EvalSymlinks(root)
	}
	if err != nil {
		return "", fmt.Errorf("resolve Git project root: %w", err)
	}
	return root, nil
}

// initHelp is printed by "init --help" and "init -h".
const initHelp = `Usage: taskfactory init

Create the project configuration and the task state directories in the Git
repository that contains the current directory. An existing valid configuration
is kept unchanged, and missing task state directories are restored.

Flags:
  --help, -h  print this help and exit successfully

Exit codes:
  0  the project is initialized
  1  initialization failed
  2  invalid usage
`

func initializeProject() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("determine current directory: %w", err)
	}
	command := exec.Command("git", "-C", cwd, "rev-parse", "--path-format=absolute", "--show-toplevel")
	output, err := command.Output()
	if err != nil {
		return fmt.Errorf("current directory is not inside a Git working tree")
	}
	root := filepath.Clean(strings.TrimSuffix(string(output), "\n"))
	root, _ = filepath.Abs(root)
	if root == "" {
		return fmt.Errorf("Git returned an empty project root")
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve Git project root: %w", err)
	}
	return initializeAt(root)
}

func initializeAt(root string) error {
	configPath := filepath.Join(root, ".taskfactory", "config.toml")
	if _, err := os.Stat(configPath); err == nil {
		if _, err := config.Load(root); err != nil {
			return fmt.Errorf("existing configuration conflicts with protocol v1: %w", err)
		}
		return createTaskStateDirectories(root)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect configuration file %s: %w", configPath, err)
	}

	if err := ensureProjectDirectory(root, filepath.Dir(configPath)); err != nil {
		return fmt.Errorf("create configuration directory: %w", err)
	}
	if err := createTaskStateDirectories(root); err != nil {
		return err
	}
	file, err := os.OpenFile(configPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			if _, loadErr := config.Load(root); loadErr == nil {
				return createTaskStateDirectories(root)
			} else {
				return fmt.Errorf("existing configuration conflicts with protocol v1: %w", loadErr)
			}
		}
		return fmt.Errorf("create configuration file %s: %w", configPath, err)
	}
	if _, err := file.WriteString(defaultConfig); err != nil {
		_ = file.Close()
		_ = os.Remove(configPath)
		return fmt.Errorf("write configuration file %s: %w", configPath, err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(configPath)
		return fmt.Errorf("close configuration file %s: %w", configPath, err)
	}
	return nil
}

func createTaskStateDirectories(root string) error {
	tasksDir := filepath.Join(root, "tasks")
	if err := ensureProjectDirectory(root, tasksDir); err != nil {
		return fmt.Errorf("create task directory: %w", err)
	}
	for _, state := range taskStates {
		path := filepath.Join(tasksDir, state)
		if err := ensureProjectDirectory(root, path); err != nil {
			return fmt.Errorf("create task state directory %s: %w", path, err)
		}
	}
	return nil
}

func ensureProjectDirectory(root, path string) error {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	if relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("path %s is outside Git project %s", path, root)
	}
	current := root
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("%s is a symlink; refusing to write outside the Git project", current)
			}
			if !info.IsDir() {
				return fmt.Errorf("%s is not a directory", current)
			}
			continue
		}
		if !os.IsNotExist(err) {
			return fmt.Errorf("inspect %s: %w", current, err)
		}
		if err := os.Mkdir(current, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", current, err)
		}
	}
	return nil
}
