package integrate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"taskfactory/internal/config"
)

// acquireIntegrationLock takes the repository-local integration lock and
// returns a function that releases it. Callers must invoke the function exactly
// once.
func acquireIntegrationLock(root string) (func(), error) {
	lockPath := filepath.Join(root, ".taskfactory/integration.lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0755); err != nil {
		return nil, err
	}
	lf, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		_ = lf.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(lf.Fd()), syscall.LOCK_UN)
		_ = lf.Close()
	}, nil
}

// CheckMain reruns the configured verification.main commands against the
// current local main checkout while holding the integration lock.
//
// On success it clears the canonical stop record and any valid lone temporary
// sibling. On failure it writes or replaces the canonical stop record with the
// commit that was checked, the failing command, its exit code or start error,
// and its output. A malformed stop record, or both canonical and temporary
// records, fails closed without any change and without running checks. Main
// must not move while checks run; if it does, no stop state is changed.
func CheckMain(root string) (retErr error) {
	root, retErr = filepath.EvalSymlinks(root)
	if retErr != nil {
		return retErr
	}
	unlock, err := acquireIntegrationLock(root)
	if err != nil {
		return err
	}
	defer unlock()

	cfg, err := config.Load(root)
	if err != nil {
		return err
	}
	stop, err := loadStopState(root)
	if err != nil {
		return err
	}
	checks := cfg.Verification.Main
	if stop.present && len(checks) == 0 {
		return fmt.Errorf("integration stopped by %s but verification.main has no commands; configure main checks in .taskfactory/config.toml and rerun taskfactory check-main, or repair main and remove the record manually", stop.path)
	}
	if len(checks) == 0 {
		return nil
	}
	if branchName, _ := git(root, "branch", "--show-current"); branchName != "main" {
		return fmt.Errorf("repository worktree must have local main checked out (currently %q)", branchName)
	}
	mainCommit, err := git(root, "rev-parse", "--verify", "refs/heads/main^{commit}")
	if err != nil {
		return fmt.Errorf("local refs/heads/main is unavailable: %w", err)
	}
	for _, command := range checks {
		if err = requireMainAt(root, mainCommit); err != nil {
			return err
		}
		out, code, startErr := runCommand(root, command)
		if err = requireMainAt(root, mainCommit); err != nil {
			return err
		}
		if startErr == nil && *code == 0 {
			continue
		}
		if cfg.Integration.StopOnMainFailure || stop.present {
			rec := stopRecord{Main: mainCommit, Command: command, Exit: code, Output: out}
			if startErr != nil {
				rec.Error = startErr.Error()
			}
			if err = persistStop(root, rec); err != nil {
				return errors.Join(fmt.Errorf("main check %q failed on %s", command, mainCommit), fmt.Errorf("persist integration stop: %w", err))
			}
		}
		if startErr != nil {
			return fmt.Errorf("start main check %q: %w; integration remains stopped", command, startErr)
		}
		return fmt.Errorf("main check %q failed with exit code %d on %s; integration remains stopped", command, *code, mainCommit)
	}
	if err = requireMainAt(root, mainCommit); err != nil {
		return err
	}
	return clearStop(root)
}

// stopState describes the stop record found by loadStopState.
type stopState struct {
	path    string
	present bool
}

// loadStopState validates the canonical stop record or a lone temporary sibling.
// Both files present, or any malformed or non-regular file, is an error.
func loadStopState(root string) (stopState, error) {
	canonical := filepath.Join(root, ".taskfactory/integration-stop.json")
	temp := canonical + ".tmp"
	cInfo, cErr := os.Lstat(canonical)
	tInfo, tErr := os.Lstat(temp)
	for _, e := range []error{cErr, tErr} {
		if e != nil && !os.IsNotExist(e) {
			return stopState{}, e
		}
	}
	cExists, tExists := cErr == nil, tErr == nil
	switch {
	case cExists && tExists:
		return stopState{}, fmt.Errorf("integration stop state is ambiguous: both %s and %s exist; inspect and repair manually", canonical, temp)
	case !cExists && !tExists:
		return stopState{path: canonical}, nil
	}
	path, info := canonical, cInfo
	if tExists {
		path, info = temp, tInfo
	}
	if !info.Mode().IsRegular() {
		return stopState{}, fmt.Errorf("integration stop record %s is not a regular file; inspect it manually", path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return stopState{}, fmt.Errorf("read integration stop record %s: %w", path, err)
	}
	if _, err = parseStop(b); err != nil {
		return stopState{}, fmt.Errorf("integration stop record %s is malformed; inspect it manually", path)
	}
	return stopState{path: path, present: true}, nil
}

// requireMainAt verifies that local main still points at want.
func requireMainAt(root, want string) error {
	got, err := git(root, "rev-parse", "--verify", "refs/heads/main^{commit}")
	if err != nil {
		return fmt.Errorf("local refs/heads/main is unavailable: %w", err)
	}
	if got != want {
		return fmt.Errorf("main moved from %s to %s during main checks; integration stop state was not changed; rerun taskfactory check-main", want, got)
	}
	return nil
}

// persistStop atomically writes the canonical stop record through the sibling
// temporary path, replacing any existing canonical record or valid temp file.
func persistStop(root string, rec stopRecord) error {
	canonical := filepath.Join(root, ".taskfactory/integration-stop.json")
	temp := canonical + ".tmp"
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(temp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(temp, canonical); err != nil {
		return err
	}
	return syncDir(filepath.Dir(canonical))
}

// clearStop removes the canonical record and any lone temp sibling. It is only
// called after loadStopState validated them and all checks passed on main.
func clearStop(root string) error {
	canonical := filepath.Join(root, ".taskfactory/integration-stop.json")
	removed := false
	for _, p := range []string{canonical, canonical + ".tmp"} {
		if _, err := os.Lstat(p); err == nil {
			if err = os.Remove(p); err != nil {
				return fmt.Errorf("clear integration stop record %s: %w", p, err)
			}
			removed = true
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if !removed {
		return nil
	}
	return syncDir(filepath.Dir(canonical))
}

func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	err = d.Sync()
	if cerr := d.Close(); err == nil {
		err = cerr
	}
	return err
}
