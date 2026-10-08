// Command taskfactory is the TaskFactory CLI. It coordinates software work into
// explicit, verifiable tasks that coding agents can execute safely in parallel.
package main

import (
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
	{"validate", "validate [task-file]", "check task files"},
	{"claim", "claim <ID> --owner <name>", "claim a ready task"},
	{"verify", "verify <ID>", "run a task's checks"},
	{"integrate", "integrate <ID>", "fast-forward a verified task"},
	{"check-main", "check-main", "recheck a stopped main"},
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
		b.WriteString("  " + p.Cyan(command.name) + p.Dim(rest) + padding + command.summary + "\n")
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
in .taskfactory/evidence/<ID>.jsonl. The command exits 1 when any check fails.

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
			fmt.Fprint(os.Stdout, text)
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
		if len(args) > 2 {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory validate", "usage: taskfactory validate [task-file]"))
			os.Exit(exitUsage)
		}
		if err := validateProject(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory validate", err.Error()))
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
		root, err := projectRoot()
		if err == nil {
			err = verify.Run(root, args[1])
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
const validateHelp = `Usage: taskfactory validate [task-file]

Validate the whole task tree, or only the task file given. A task file must be
inside the project's tasks directory; relative paths are resolved from the
project root. Checking one file still checks task ID uniqueness and dependency
references across the tree, but reports only diagnostics for the selected file.

Flags:
  --help, -h  print this help and exit successfully

Exit codes:
  0  the task tree or task file is valid
  1  validation failed or the project configuration is invalid
  2  invalid usage
`

func validateProject(args []string) error {
	root, err := projectRoot()
	if err != nil {
		return err
	}
	if _, err := config.Load(root); err != nil {
		return err
	}
	selected := ""
	if len(args) == 1 {
		selected = args[0]
		if !filepath.IsAbs(selected) {
			selected = filepath.Join(root, selected)
		}
		selected, err = filepath.Abs(selected)
		if err != nil {
			return fmt.Errorf("resolve task path: %w", err)
		}
		selected, err = filepath.EvalSymlinks(selected)
		if err != nil {
			return fmt.Errorf("resolve task path: %w", err)
		}
		rel, relErr := filepath.Rel(root, selected)
		if relErr != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("task path must resolve inside the project tasks directory")
		}
		if !strings.HasPrefix(filepath.ToSlash(rel), "tasks/") {
			return fmt.Errorf("task path must resolve inside the project tasks directory")
		}
		if info, statErr := os.Stat(selected); statErr != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("task file %s cannot be read", selected)
		}
		selected = rel
	}
	diagnostics := taskvalidate.Validate(root, selected)
	for _, diagnostic := range diagnostics {
		fmt.Fprintln(os.Stderr, diagnostic.Error())
	}
	if len(diagnostics) > 0 {
		return fmt.Errorf("%d validation error(s)", len(diagnostics))
	}
	style := ui.For(os.Stdout)
	if selected == "" {
		fmt.Fprintln(os.Stdout, style.Green("task tree is valid"))
	} else {
		fmt.Fprintln(os.Stdout, style.Green(filepath.ToSlash(selected)+" is valid"))
	}
	return nil
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
