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
	"taskfactory/internal/verify"
)

// version is the stable development version string printed by --version. It is a
// fixed, human-readable constant; do not derive it from build metadata here so
// that output stays predictable across checkouts and builds.
const version = "dev"

// usageText is the help text printed by --help and -h. It names the command and
// both global flags so that "taskfactory --help" documents the supported
// top-level flags.
const usageText = `taskfactory coordinates software work into explicit, verifiable tasks.

Usage:
  taskfactory [global flags]
  taskfactory <command> [flags]
  taskfactory status
  taskfactory validate [task-file]
  taskfactory claim <ID> --owner <name>
  taskfactory verify <ID>
  taskfactory integrate <ID>
  taskfactory check-main

Global flags:
  -help, --help      print this usage and exit successfully.
  -version, --version print the CLI version string and exit successfully.`

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

func main() {
	fs := flag.NewFlagSet("taskfactory", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	// Keep flag.Parse from printing the full usage text on invalid input.
	fs.Usage = func() {}

	help := fs.Bool("help", false, "print this usage and exit successfully")
	showVersion := fs.Bool("version", false, "print the CLI version string and exit successfully")

	if err := fs.Parse(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "taskfactory: %v\n", err)
		os.Exit(exitUsage)
	}

	if *help {
		fmt.Fprint(os.Stdout, usageText)
		return
	}

	if *showVersion {
		fmt.Fprintln(os.Stdout, "taskfactory version "+version)
		return
	}

	args := fs.Args()
	if len(args) == 1 && args[0] == "init" {
		if err := initializeProject(); err != nil {
			fmt.Fprintf(os.Stderr, "taskfactory init: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if len(args) >= 1 && args[0] == "validate" {
		if len(args) > 2 {
			fmt.Fprintln(os.Stderr, "taskfactory validate: usage: taskfactory validate [task-file]")
			os.Exit(exitUsage)
		}
		if err := validateProject(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "taskfactory validate: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if len(args) == 1 && args[0] == "status" {
		if err := statusProject(); err != nil {
			fmt.Fprintf(os.Stderr, "taskfactory status: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if len(args) > 0 && args[0] == "claim" {
		if err := claimTask(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "taskfactory claim: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if len(args) > 0 && args[0] == "verify" {
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "taskfactory verify: usage: taskfactory verify <ID>")
			os.Exit(exitUsage)
		}
		root, err := projectRoot()
		if err == nil {
			err = verify.Run(root, args[1])
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "taskfactory verify: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if len(args) > 0 && args[0] == "integrate" {
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "taskfactory integrate: usage: taskfactory integrate <ID>")
			os.Exit(exitUsage)
		}
		root, err := projectRoot()
		if err == nil {
			err = integrate.Run(root, args[1])
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "taskfactory integrate: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stdout, "integrated %s\n", args[1])
		return
	}
	if len(args) > 0 && args[0] == "check-main" {
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "taskfactory check-main: usage: taskfactory check-main")
			os.Exit(exitUsage)
		}
		root, err := projectRoot()
		if err == nil {
			err = integrate.CheckMain(root)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "taskfactory check-main: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stdout, "check-main: ok")
		return
	}

	fmt.Fprintln(os.Stderr, usageText)
	os.Exit(exitUsage)
}

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
	if selected == "" {
		fmt.Fprintln(os.Stdout, "task tree is valid")
	} else {
		fmt.Fprintf(os.Stdout, "%s is valid\n", filepath.ToSlash(selected))
	}
	return nil
}

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
