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

	"taskfactory/internal/config"
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

	fmt.Fprintln(os.Stderr, usageText)
	os.Exit(exitUsage)
}

func initializeProject() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("determine current directory: %w", err)
	}
	command := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel")
	output, err := command.Output()
	if err != nil {
		return fmt.Errorf("current directory is not inside a Git working tree")
	}
	root := filepath.Clean(strings.TrimSuffix(string(output), "\n"))
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
