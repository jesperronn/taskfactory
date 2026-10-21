// Package statecommit commits task moves into an external state directory that
// is itself a git repository. It runs plain git with the owner's configuration:
// signing and identity are never overridden.
package statecommit

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"taskfactory/internal/paths"
)

// IsRepo reports whether the layout's state root is its own git repository:
// state is outside the project and its top level is the state root itself.
func IsRepo(l paths.Layout) bool {
	if !l.External() {
		return false
	}
	out, err := Git(l.State, "rev-parse", "--path-format=absolute", "--show-toplevel")
	if err != nil {
		return false
	}
	return samePath(strings.TrimSpace(out), l.State)
}

// GitRoot is the directory where task moves are committed: the project in the
// default layout, the state repository for an external one, and "" for a plain
// external folder, which gets renames only.
func GitRoot(l paths.Layout) string {
	if !l.External() {
		return l.Project
	}
	if IsRepo(l) {
		return l.State
	}
	return ""
}

// Tracked reports whether rel is tracked in the state repository.
func Tracked(stateRoot, rel string) bool {
	_, err := Git(stateRoot, "ls-files", "--error-unmatch", "--", rel)
	return err == nil
}

// Staged returns the paths staged in the state repository's index.
func Staged(stateRoot string) (string, error) {
	out, err := Git(stateRoot, "diff", "--cached", "--name-only")
	return strings.TrimSpace(out), err
}

// Commit stages the given paths and commits only their contents. Each message
// becomes one -m paragraph.
func Commit(stateRoot string, messages, stage []string) error {
	if _, err := Git(stateRoot, append([]string{"add", "--all", "--"}, stage...)...); err != nil {
		return err
	}
	args := []string{"commit", "--quiet", "--only"}
	for _, m := range messages {
		args = append(args, "-m", m)
	}
	if _, err := Git(stateRoot, append(append(args, "--"), stage...)...); err != nil {
		_, _ = Git(stateRoot, append([]string{"reset", "-q", "--"}, stage...)...)
		return err
	}
	return nil
}

// Git runs git -C dir and returns stdout; failures include stderr.
func Git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func samePath(a, b string) bool {
	ra, e1 := filepath.EvalSymlinks(a)
	rb, e2 := filepath.EvalSymlinks(b)
	if e1 != nil || e2 != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return ra == rb
}
