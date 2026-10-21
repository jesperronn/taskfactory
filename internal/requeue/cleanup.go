package requeue

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"taskfactory/internal/config"
	"taskfactory/internal/paths"
)

// CleanupError reports that the task WAS requeued and committed but removing
// its old worktree or branch failed. The Result returned with it is complete.
type CleanupError struct{ msg string }

func (e CleanupError) Error() string { return e.msg }

// cleanupPlan names the old worktree and branch of a task and what exists.
// It is built, and every safety condition checked, before requeue changes
// anything. The removal itself runs after the requeue commit.
type cleanupPlan struct {
	worktree   string // absolute, symlink-resolved path, or "" when none
	branch     string // branch name without refs/heads/, or "" when none
	hasDir     bool   // the worktree directory exists and is registered
	hasBranch  bool   // the branch exists
	staleEntry bool   // Git still lists the worktree but its directory is gone
}

type worktreeEntry struct {
	path   string
	branch string // without refs/heads/, empty when detached
}

// planCleanup decides which worktree and branch belong to the task and checks
// that removing them loses nothing. claimBlock is the Claim block text, or
// empty when the task has none. A refusal is returned as an error.
func planCleanup(root, id, claimBlock string) (cleanupPlan, error) {
	base, err := config.BaseBranchOf(root)
	if err != nil {
		return cleanupPlan{}, err
	}
	wtRoot := resolvePath(paths.For(root).DefaultWorktreeRoot())
	worktree := filepath.Join(wtRoot, id)
	branch := "task/" + id
	if claimBlock != "" {
		if worktree, branch, err = claimedLocation(root, id, claimBlock); err != nil {
			return cleanupPlan{}, err
		}
		worktree = resolvePath(worktree)
	}
	if rel, err := filepath.Rel(wtRoot, worktree); err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return cleanupPlan{}, unsafeCleanup(id, worktree, branch, fmt.Sprintf("the worktree path %s is outside %s, so requeue never touches it", worktree, wtRoot))
	}
	if worktree == root {
		return cleanupPlan{}, unsafeCleanup(id, worktree, branch, "the worktree is the project's own main checkout")
	}
	if branch == "" || branch == "main" || branch == base || strings.HasPrefix(branch, "-") {
		return cleanupPlan{}, unsafeCleanup(id, worktree, branch, fmt.Sprintf("the branch %q is not a task branch", branch))
	}
	if _, err := git(root, "check-ref-format", "refs/heads/"+branch); err != nil {
		return cleanupPlan{}, unsafeCleanup(id, worktree, branch, fmt.Sprintf("the branch %q is not a valid branch name", branch))
	}

	plan := cleanupPlan{worktree: worktree, branch: branch}
	entries, err := listWorktrees(root)
	if err != nil {
		return cleanupPlan{}, err
	}
	var own *worktreeEntry
	for i, entry := range entries {
		if entry.path == worktree {
			if i == 0 {
				return cleanupPlan{}, unsafeCleanup(id, worktree, branch, "the worktree is the project's own main checkout")
			}
			own = &entries[i]
		} else if entry.branch == branch {
			return cleanupPlan{}, unsafeCleanup(id, worktree, branch, fmt.Sprintf("the branch %s is checked out in another worktree, %s", branch, entry.path))
		}
	}
	_, statErr := os.Lstat(worktree)
	switch {
	case statErr == nil:
		if own == nil {
			return cleanupPlan{}, unsafeCleanup(id, worktree, branch, fmt.Sprintf("%s exists but is not a worktree registered with this project", worktree))
		}
		plan.hasDir = true
	case os.IsNotExist(statErr):
		plan.staleEntry = own != nil
	default:
		return cleanupPlan{}, fmt.Errorf("refusing to requeue %s: inspect worktree %s: %w", id, worktree, statErr)
	}

	if _, err := git(root, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
		plan.hasBranch = true
	}
	if !plan.hasDir && !plan.hasBranch {
		return plan, nil
	}
	if _, err := git(root, "rev-parse", "--verify", "--quiet", "refs/heads/"+base); err != nil {
		return cleanupPlan{}, fmt.Errorf("refusing to requeue %s: cannot check %s against local %s: branch %s does not exist", id, branch, base, base)
	}

	var problems []string
	if plan.hasDir {
		status, err := git(worktree, "status", "--porcelain")
		if err != nil {
			return cleanupPlan{}, fmt.Errorf("refusing to requeue %s: check worktree %s: %w", id, worktree, err)
		}
		if strings.TrimSpace(status) != "" {
			problems = append(problems, fmt.Sprintf("the worktree %s has uncommitted or untracked files:\n%s", worktree, strings.TrimRight(status, "\n")))
		}
		if n, err := countRange(worktree, "refs/heads/"+base+"..HEAD"); err != nil {
			return cleanupPlan{}, fmt.Errorf("refusing to requeue %s: check worktree %s: %w", id, worktree, err)
		} else if n > 0 && own.branch != branch {
			problems = append(problems, fmt.Sprintf("the worktree %s has %d commit(s) on its HEAD that are not on %s; inspect them with git -C %s log --oneline %s..HEAD", worktree, n, base, worktree, base))
		}
	}
	if plan.hasBranch {
		n, err := countRange(root, "refs/heads/"+base+"..refs/heads/"+branch)
		if err != nil {
			return cleanupPlan{}, fmt.Errorf("refusing to requeue %s: check branch %s: %w", id, branch, err)
		}
		if n > 0 {
			problems = append(problems, fmt.Sprintf("the branch %s has %d unmerged commit(s) that are not on %s; inspect them with git log --oneline %s..%s", branch, n, base, base, branch))
		}
	}
	if len(problems) > 0 {
		return cleanupPlan{}, unsafeCleanup(id, worktree, branch, strings.Join(problems, "; and "))
	}
	return plan, nil
}

// unsafeCleanup builds the refusal for a failed safety condition. Nothing was
// changed when it is returned, and it prints the manual commands.
func unsafeCleanup(id, worktree, branch, reason string) error {
	return fmt.Errorf("refusing to requeue %s: the old worktree and branch cannot be removed safely: %s.\nNothing was changed. Unmerged worker commits and dirty files always need a human decision. To clean up by hand, then rerun requeue:\n  git worktree remove %s\n  git branch -d %s\nUse git branch -D %s instead if you decide to discard unmerged commits, and add --force to git worktree remove only if you decide to discard the uncommitted files", id, reason, worktree, branch, branch)
}

// run removes the worktree and then the branch, never with force. It returns
// what it removed. A missing worktree or branch is not an error.
func (p cleanupPlan) run(root string) (worktree, branch string, err error) {
	if p.hasDir {
		if _, err := git(root, "worktree", "remove", p.worktree); err != nil {
			return "", "", err
		}
		worktree = p.worktree
	} else if p.staleEntry {
		if _, err := git(root, "worktree", "prune"); err != nil {
			return "", "", err
		}
	}
	if p.hasBranch {
		if _, err := git(root, "branch", "-d", p.branch); err != nil {
			return worktree, "", err
		}
		branch = p.branch
	}
	return worktree, branch, nil
}

// cleanupFailure states plainly that the task was requeued and what is left.
func cleanupFailure(id, targetRel string, p cleanupPlan, removedWorktree string, cause error) error {
	var left []string
	if p.hasDir && removedWorktree == "" {
		left = append(left, "worktree "+p.worktree)
	}
	if p.hasBranch {
		left = append(left, "branch "+p.branch)
	}
	return CleanupError{fmt.Sprintf("task %s WAS requeued and committed (%s), but removing its old worktree and branch failed: %v\nLeft behind: %s. Remove them by hand:\n  git worktree remove %s\n  git branch -d %s", id, targetRel, cause, strings.Join(left, ", "), p.worktree, p.branch)}
}

// claimedLocation reads Worktree and Branch from the Claim block. A missing or
// conflicting field is a refusal.
func claimedLocation(root, id, block string) (worktree, branch string, err error) {
	worktrees, branches := map[string]bool{}, map[string]bool{}
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if v, ok := strings.CutPrefix(line, "Worktree:"); ok {
			worktrees[strings.TrimSpace(v)] = true
			worktree = strings.TrimSpace(v)
		}
		if v, ok := strings.CutPrefix(line, "Branch:"); ok {
			branches[strings.TrimSpace(v)] = true
			branch = strings.TrimSpace(v)
		}
	}
	if worktree == "" || branch == "" || len(worktrees) > 1 || len(branches) > 1 {
		return "", "", fmt.Errorf("refusing to requeue %s: the Claim block must record exactly one Worktree and one Branch to clean up", id)
	}
	if !filepath.IsAbs(worktree) {
		worktree = filepath.Join(root, worktree)
	}
	return filepath.Clean(worktree), branch, nil
}

// listWorktrees returns the registered worktrees. The first is the main
// checkout. Paths are symlink-resolved.
func listWorktrees(root string) ([]worktreeEntry, error) {
	out, err := git(root, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
	}
	var entries []worktreeEntry
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			entries = append(entries, worktreeEntry{path: resolvePath(strings.TrimPrefix(line, "worktree "))})
		case strings.HasPrefix(line, "branch refs/heads/") && len(entries) > 0:
			entries[len(entries)-1].branch = strings.TrimPrefix(line, "branch refs/heads/")
		}
	}
	return entries, nil
}

func countRange(dir, rng string) (int, error) {
	out, err := git(dir, "rev-list", "--count", rng)
	if err != nil {
		return 0, err
	}
	var n int
	if _, err := fmt.Sscanf(strings.TrimSpace(out), "%d", &n); err != nil {
		return 0, errors.New("unreadable commit count " + out)
	}
	return n, nil
}

// resolvePath resolves symlinks in the longest existing prefix of path, so a
// path that does not exist yet compares equal to the same path once created.
func resolvePath(path string) string {
	path = filepath.Clean(path)
	rest := ""
	for p := path; ; {
		if resolved, err := filepath.EvalSymlinks(p); err == nil {
			return filepath.Join(resolved, rest)
		}
		parent := filepath.Dir(p)
		if parent == p {
			return path
		}
		rest = filepath.Join(filepath.Base(p), rest)
		p = parent
	}
}
