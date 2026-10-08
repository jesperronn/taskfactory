// Package claim provides the local guard and eligibility checks used by claim.
package claim

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

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
