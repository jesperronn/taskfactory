// Package claim provides the local guard and eligibility checks used by claim.
package claim

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"taskfactory/internal/config"
	"taskfactory/internal/taskvalidate"
)

var taskIDPattern = regexp.MustCompile(`^TF-[0-9]{3}$`)

// WithClaimLock holds the project-local advisory lock while fn runs.
func WithClaimLock(projectRoot string, fn func() error) error {
	if fn == nil {
		return errors.New("claim lock callback must not be nil")
	}
	lockPath := filepath.Join(projectRoot, ".taskfactory", "claim.lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return fmt.Errorf("create claim lock directory %s: %w", filepath.Dir(lockPath), err)
	}
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open claim lock %s: %w", lockPath, err)
	}
	defer file.Close()
	if err := lockFile(file); err != nil {
		return fmt.Errorf("acquire claim lock %s: %w", lockPath, err)
	}
	callbackErr := fn()
	if err := unlockFile(file); err != nil {
		return errors.Join(callbackErr, fmt.Errorf("release claim lock %s: %w", lockPath, err))
	}
	return callbackErr
}

// CheckEligibility checks whether id names a valid, claimable ready task.
// It reads project state only and does not move or modify task files.
func CheckEligibility(projectRoot, id string) error {
	if !taskIDPattern.MatchString(id) {
		return fmt.Errorf("task %q is not a valid task ID (expected TF-NNN)", id)
	}
	configValue, err := config.Load(projectRoot)
	if err != nil {
		return fmt.Errorf("task %s eligibility: %w", id, err)
	}

	readyDir := filepath.Join(projectRoot, "tasks", "ready")
	entries, err := os.ReadDir(readyDir)
	if err != nil {
		return fmt.Errorf("task %s is not eligible: read ready tasks at %s: %w", id, readyDir, err)
	}
	prefix := id + "-"
	var readyPath string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) && strings.HasSuffix(entry.Name(), ".md") {
			if readyPath != "" {
				return fmt.Errorf("task %s is not eligible: multiple ready task files match this ID", id)
			}
			readyPath = filepath.Join(readyDir, entry.Name())
		}
	}
	if readyPath == "" {
		return fmt.Errorf("task %s is not eligible: no ready task file exists for this ID", id)
	}

	relPath, err := filepath.Rel(projectRoot, readyPath)
	if err != nil {
		return fmt.Errorf("task %s is not eligible: resolve ready task path: %w", id, err)
	}
	if diagnostics := taskvalidate.Validate(projectRoot, relPath); len(diagnostics) > 0 {
		return fmt.Errorf("task %s is not eligible: %s", id, diagnostics[0])
	}

	activeDir := filepath.Join(projectRoot, "tasks", "active")
	active, err := os.ReadDir(activeDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("task %s is not eligible: read active tasks at %s: %w", id, activeDir, err)
	}
	activeCount := 0
	for _, entry := range active {
		if entry.IsDir() || entry.Name() == ".gitkeep" || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("task %s is not eligible: inspect active task %s: %w", id, entry.Name(), err)
		}
		if info.Mode().IsRegular() {
			activeCount++
		}
	}
	if activeCount >= configValue.Workers.MaxParallel {
		return fmt.Errorf("task %s is not eligible: worker capacity is full (%d active tasks; workers.max_parallel is %d)", id, activeCount, configValue.Workers.MaxParallel)
	}
	return nil
}

// Claim creates a task branch and worktree, then atomically moves its contract
// from ready to active with the protocol's claim metadata.
func Claim(projectRoot, id, owner string) error {
	if strings.TrimSpace(owner) == "" {
		return errors.New("claim owner must be non-empty")
	}
	canonicalRoot, err := filepath.EvalSymlinks(projectRoot)
	if err != nil {
		return fmt.Errorf("resolve project root %s: %w", projectRoot, err)
	}
	projectRoot = canonicalRoot
	return WithClaimLock(projectRoot, func() error {
		if err := validateTaskStateDirectories(projectRoot); err != nil {
			return err
		}
		if err := CheckEligibility(projectRoot, id); err != nil {
			return err
		}
		cfg, err := config.Load(projectRoot)
		if err != nil {
			return err
		}
		readyDir := filepath.Join(projectRoot, "tasks", "ready")
		readyPath, err := findTaskPath(readyDir, id)
		if err != nil {
			return err
		}
		name := filepath.Base(readyPath)
		activeDir := filepath.Join(projectRoot, "tasks", "active")
		activePath := filepath.Join(activeDir, name)
		if _, err := os.Lstat(activePath); err == nil {
			return fmt.Errorf("task %s active path %s already exists", id, activePath)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect active task path %s: %w", activePath, err)
		}
		branch := "task/" + id
		if _, err := runGit(projectRoot, "show-ref", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
			return fmt.Errorf("task %s branch %s already exists", id, branch)
		} else if exit, ok := gitExitCode(err); !ok || exit != 1 {
			return fmt.Errorf("check branch %s: %w", branch, err)
		}
		worktree := filepath.Join(cfg.Git.WorktreeRoot, id)
		if _, err := os.Lstat(worktree); err == nil {
			return fmt.Errorf("task %s worktree path %s already exists", id, worktree)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect worktree path %s: %w", worktree, err)
		}
		base, err := runGit(projectRoot, "rev-parse", "--verify", "refs/heads/main^{commit}")
		if err != nil {
			return fmt.Errorf("resolve local main commit refs/heads/main: %w", err)
		}
		base = strings.TrimSpace(base)
		if !regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`).MatchString(base) {
			return fmt.Errorf("resolve local main commit refs/heads/main: Git returned invalid full object ID %q", base)
		}
		data, err := os.ReadFile(readyPath)
		if err != nil {
			return fmt.Errorf("read ready task %s: %w", readyPath, err)
		}
		metadata := fmt.Sprintf("\n## Claim\n\nOwner: %s\nBranch: %s\nWorktree: %s\nBase commit: %s\nStarted at: %s\n", owner, branch, worktree, base, time.Now().UTC().Format(time.RFC3339))
		if strings.Contains(owner, "\n") || strings.Contains(owner, "\r") {
			return errors.New("claim owner must be a single line")
		}
		complete := append(append([]byte(nil), data...), []byte(metadata)...)
		if err := os.MkdirAll(filepath.Dir(worktree), 0o755); err != nil {
			return fmt.Errorf("create worktree root %s: %w", filepath.Dir(worktree), err)
		}
		if info, statErr := os.Stat(cfg.Git.WorktreeRoot); statErr == nil && !info.IsDir() {
			return fmt.Errorf("worktree root %s is not a directory", cfg.Git.WorktreeRoot)
		} else if statErr != nil && !os.IsNotExist(statErr) {
			return fmt.Errorf("inspect worktree root %s: %w", cfg.Git.WorktreeRoot, statErr)
		}
		// Track only the branch and worktree successfully created below. Rollback
		// never removes a path or ref whose creation was not confirmed.
		createdBranch, createdWorktree := false, false
		rollback := func(cause error) error {
			var cleanup []error
			if createdWorktree {
				if _, e := runGit(projectRoot, "worktree", "remove", "--force", worktree); e != nil {
					cleanup = append(cleanup, fmt.Errorf("remove created worktree %s: %w", worktree, e))
				}
			}
			if createdBranch {
				if _, e := runGit(projectRoot, "branch", "-D", branch); e != nil {
					cleanup = append(cleanup, fmt.Errorf("remove created branch %s: %w", branch, e))
				}
			}
			if len(cleanup) > 0 {
				return fmt.Errorf("%w; manual recovery required: branch=%s path=%s task_state=ready; cleanup errors: %v", cause, branch, worktree, errors.Join(cleanup...))
			}
			return cause
		}
		if _, err := runGit(projectRoot, "branch", branch, base); err != nil {
			return fmt.Errorf("create task branch %s from %s: %w", branch, base, err)
		}
		createdBranch = true
		if _, err := runGit(projectRoot, "worktree", "add", worktree, branch); err != nil {
			return rollback(fmt.Errorf("create task worktree %s: %w", worktree, err))
		}
		createdWorktree = true
		tmp, err := os.CreateTemp(activeDir, ".claim-*.tmp")
		if err != nil {
			return rollback(fmt.Errorf("prepare active task file: %w", err))
		}
		tmpName := tmp.Name()
		if _, err = tmp.Write(complete); err == nil {
			err = tmp.Sync()
		}
		closeErr := tmp.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(tmpName)
			return rollback(fmt.Errorf("write active task file: %w", err))
		}
		backup, err := os.CreateTemp(filepath.Join(projectRoot, ".taskfactory"), ".claim-backup-*.tmp")
		if err != nil {
			_ = os.Remove(tmpName)
			return rollback(fmt.Errorf("prepare ready task rollback: %w", err))
		}
		backupPath := backup.Name()
		if err := backup.Close(); err != nil {
			_ = os.Remove(backupPath)
			_ = os.Remove(tmpName)
			return rollback(fmt.Errorf("prepare ready task rollback: %w", err))
		}
		if err := os.Remove(backupPath); err != nil {
			_ = os.Remove(tmpName)
			return rollback(fmt.Errorf("prepare ready task rollback: %w", err))
		}
		if err := os.Rename(readyPath, backupPath); err != nil {
			_ = os.Remove(tmpName)
			return rollback(fmt.Errorf("move task out of ready state: %w", err))
		}
		restoreReady := func(cause error) error {
			var restoreErr error
			if _, statErr := os.Lstat(activePath); statErr == nil {
				if err := os.Remove(activePath); err != nil {
					restoreErr = fmt.Errorf("remove incomplete active task %s: %w", activePath, err)
				}
			} else if !os.IsNotExist(statErr) {
				restoreErr = fmt.Errorf("inspect active task %s: %w", activePath, statErr)
			}
			if restoreErr == nil {
				if err := os.Rename(backupPath, readyPath); err != nil {
					restoreErr = fmt.Errorf("restore ready task %s from %s: %w", readyPath, backupPath, err)
				}
			}
			_ = os.Remove(tmpName)
			cleanupErr := rollback(cause)
			if restoreErr != nil {
				return fmt.Errorf("%w; manual recovery required: branch=%s path=%s task_state=see %s and %s; task restore error: %v; %v", cause, branch, worktree, readyPath, activePath, restoreErr, cleanupErr)
			}
			return cleanupErr
		}
		if err := os.Rename(tmpName, activePath); err != nil {
			return restoreReady(fmt.Errorf("write complete active task file: %w", err))
		}
		if diagnostics := taskvalidate.Validate(projectRoot, filepath.Join("tasks", "active", name)); len(diagnostics) > 0 {
			return restoreReady(fmt.Errorf("validate active task: %s", diagnostics[0]))
		}
		if err := os.Remove(backupPath); err != nil {
			return restoreReady(fmt.Errorf("remove ready task backup %s: %w", backupPath, err))
		}
		return nil
	})
}

func validateTaskStateDirectories(projectRoot string) error {
	root, err := filepath.EvalSymlinks(projectRoot)
	if err != nil {
		return fmt.Errorf("resolve project root %s: %w", projectRoot, err)
	}
	paths := []string{filepath.Join(root, "tasks")}
	for _, state := range []string{"inbox", "ready", "active", "failed", "archive"} {
		paths = append(paths, filepath.Join(root, "tasks", state))
	}
	for _, path := range paths {
		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("inspect task state directory %s: %w", path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("task state directory %s must not be a symlink", path)
		}
		if !info.IsDir() {
			return fmt.Errorf("task state path %s is not a directory", path)
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return fmt.Errorf("resolve task state directory %s: %w", path, err)
		}
		rel, err := filepath.Rel(root, resolved)
		if err != nil || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("task state directory %s resolves outside project root %s", path, root)
		}
	}
	return nil
}

func findTaskPath(directory, id string) (string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return "", fmt.Errorf("read task directory %s: %w", directory, err)
	}
	var found string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), id+"-") || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		if found != "" {
			return "", fmt.Errorf("multiple task files match %s in %s", id, directory)
		}
		found = filepath.Join(directory, entry.Name())
	}
	if found == "" {
		return "", fmt.Errorf("no task file found for %s in %s", id, directory)
	}
	return found, nil
}

func runGit(root string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func gitExitCode(err error) (int, bool) {
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return 0, false
	}
	return exit.ExitCode(), true
}
