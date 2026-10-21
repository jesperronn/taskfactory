// Package testgit builds a git repository template once per test binary and
// hands each test its own private copy. It exists for tests only.
//
// A template is built under a directory created by os.MkdirTemp and removed by
// Cleanup, which a package calls from TestMain. The template is read-only once
// built: tests never touch it, they only copy it into their own t.TempDir. A
// template must not contain worktrees or any text holding the template's own
// absolute path, because the copy lives somewhere else.
package testgit

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

type entry struct {
	once sync.Once
	dir  string
	err  error
}

var (
	mu      sync.Mutex
	base    string
	baseErr error
	entries = map[string]*entry{}
	counter int
)

// Run runs a command in dir with global and system git configuration disabled
// and returns an error carrying its combined output on failure.
func Run(dir, name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull, "NO_COLOR=1")
	if out, err := c.CombinedOutput(); err != nil {
		return fmt.Errorf("%s %v: %v\n%s", name, args, err, out)
	}
	return nil
}

// Git runs git in dir, as Run does.
func Git(dir string, args ...string) error {
	return Run(dir, "git", args...)
}

// Init runs git init on main in dir, then sets each key and value pair of
// config. The caller supplies every setting, including the identity and the
// signing choice, so this package holds no signing configuration of its own.
func Init(dir string, config ...string) error {
	if err := Git(dir, "init", "-q", "-b", "main"); err != nil {
		return err
	}
	for i := 0; i+1 < len(config); i += 2 {
		if err := Git(dir, "config", config[i], config[i+1]); err != nil {
			return err
		}
	}
	return nil
}

// Clone returns a new t.TempDir holding a copy of the template named key. The
// template is built by build on first use only; concurrent callers wait for it.
func Clone(t testing.TB, key string, build func(dir string) error) string {
	t.Helper()
	return CloneInto(t, key, build, t.TempDir())
}

// CloneInto is Clone with the copy placed in dst, which is created if absent.
// It returns dst.
func CloneInto(t testing.TB, key string, build func(dir string) error, dst string) string {
	t.Helper()
	mu.Lock()
	e := entries[key]
	if e == nil {
		e = &entry{}
		entries[key] = e
	}
	mu.Unlock()
	e.once.Do(func() {
		mu.Lock()
		if base == "" && baseErr == nil {
			base, baseErr = os.MkdirTemp("", "testgit-template-")
		}
		counter++
		dir := filepath.Join(base, fmt.Sprint(counter))
		err := baseErr
		mu.Unlock()
		if err == nil {
			err = os.MkdirAll(dir, 0o755)
		}
		if err == nil {
			err = build(dir)
		}
		e.dir, e.err = dir, err
	})
	if e.err != nil {
		t.Fatalf("build git template %q: %v", key, e.err)
	}
	if err := copyTree(e.dir, dst); err != nil {
		t.Fatalf("copy git template %q: %v", key, err)
	}
	return dst
}

// Cleanup removes every template. Call it from TestMain after m.Run.
func Cleanup() {
	mu.Lock()
	defer mu.Unlock()
	if base != "" {
		os.RemoveAll(base)
	}
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case info.Mode()&fs.ModeSymlink != 0:
			link, err := os.Readlink(p)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
		return os.Chmod(target, info.Mode().Perm()|0o200)
	})
}
