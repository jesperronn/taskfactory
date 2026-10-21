// Package skillinstall installs the embedded agent skill into a skills
// directory and reports whether an installed copy is current. It uses no
// network, never runs git and never reads or changes any configuration.
package skillinstall

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// SkillFile is the skill text file name.
	SkillFile = "SKILL.md"
	// StampFile is the sidecar that records the hash and binary version.
	StampFile = ".taskfactory-skill"
	skillName = "taskfactory"
)

// Target selects the skill directory.
type Target struct {
	// Name is "", "claude", "agents" or "dir:<path>".
	Name   string
	Global bool
	// Root is the project root, "" when outside a project.
	Root string
	// Home is the user home directory, used with Global.
	Home string
}

// ErrNoProject is returned when no skill dir can be chosen without a project.
var ErrNoProject = errors.New("not inside a project; run in a Git project, or use --global or --target dir:<path>")

// ValidateTarget checks the target name and flag combination.
func ValidateTarget(name string, global bool) error {
	switch {
	case name == "" || name == "claude" || name == "agents":
		return nil
	case strings.HasPrefix(name, "dir:"):
		if strings.TrimPrefix(name, "dir:") == "" {
			return fmt.Errorf("--target dir: needs a path")
		}
		if global {
			return fmt.Errorf("--global cannot be combined with --target dir:<path>")
		}
		return nil
	}
	return fmt.Errorf("unknown --target %q; use claude, agents or dir:<path>", name)
}

// ResolveDir returns the skill directory for t.
func ResolveDir(t Target) (string, error) {
	if err := ValidateTarget(t.Name, t.Global); err != nil {
		return "", err
	}
	if strings.HasPrefix(t.Name, "dir:") {
		return strings.TrimPrefix(t.Name, "dir:"), nil
	}
	base := t.Root
	if t.Global {
		base = t.Home
		if base == "" {
			return "", errors.New("--global needs HOME to be set")
		}
	} else if base == "" {
		return "", ErrNoProject
	}
	switch t.Name {
	case "claude":
		return filepath.Join(base, ".claude", "skills", skillName), nil
	case "agents":
		return filepath.Join(base, ".agents", "skills", skillName), nil
	}
	if info, err := os.Stat(filepath.Join(base, ".claude")); err == nil && info.IsDir() {
		return filepath.Join(base, ".claude", "skills", skillName), nil
	}
	return filepath.Join(base, ".agents", "skills", skillName), nil
}

// Hash returns the hex SHA-256 of text.
func Hash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// Stamp returns the content of the stamp file.
func Stamp(skill, version string) string {
	return "sha256 " + Hash(skill) + "\nversion " + version + "\n"
}

// Options configures Install.
type Options struct {
	Skill   string
	Version string
	Force   bool
	DryRun  bool
	// Verbose, when set, receives a line for each path written.
	Verbose func(format string, args ...any)
	// Would, when set, receives the planned actions of a dry run.
	Would func(format string, args ...any)
}

// ExistsError reports that the skill directory is already there.
type ExistsError struct{ Dir string }

func (e ExistsError) Error() string {
	return fmt.Sprintf("skill directory %s already exists", e.Dir)
}

// Install writes SKILL.md and the stamp file into dir.
func Install(dir string, o Options) error {
	_, err := os.Lstat(dir)
	exists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if exists && !o.Force {
		return ExistsError{dir}
	}
	if o.DryRun {
		if o.Would != nil {
			if exists {
				o.Would("replace directory %s", dir)
			} else {
				o.Would("create directory %s", dir)
			}
			o.Would("write file %s", filepath.Join(dir, SkillFile))
			o.Would("write file %s", filepath.Join(dir, StampFile))
		}
		return nil
	}
	parent := filepath.Dir(dir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(parent, ".taskfactory-skill-tmp-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	say := func(p string) {
		if o.Verbose != nil {
			o.Verbose("write %s", p)
		}
	}
	files := []struct{ name, text string }{
		{SkillFile, o.Skill},
		{StampFile, Stamp(o.Skill, o.Version)},
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(tmp, f.name), []byte(f.text), 0o644); err != nil {
			return err
		}
		say(filepath.Join(dir, f.name))
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		return err
	}
	if exists {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	return os.Rename(tmp, dir)
}

// State is the result of Status.
type State string

const (
	Present State = "present"
	Missing State = "missing"
	Stale   State = "stale"
)

// Report is the outcome of Status.
type Report struct {
	State     State
	Checked   []string
	Installed string // installed hash prefix, for Stale
	Embedded  string // embedded hash prefix
}

func prefix(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}

// Status inspects dir and never writes.
func Status(dir, skill string) Report {
	skillPath := filepath.Join(dir, SkillFile)
	stampPath := filepath.Join(dir, StampFile)
	r := Report{Checked: []string{skillPath}, Embedded: prefix(Hash(skill))}
	data, err := os.ReadFile(skillPath)
	if err != nil {
		r.State = Missing
		return r
	}
	r.Installed = prefix(Hash(string(data)))
	r.Checked = append(r.Checked, stampPath)
	if string(data) != skill {
		r.State = Stale
		return r
	}
	stamp, err := os.ReadFile(stampPath)
	if err != nil || !stampAgrees(string(stamp), Hash(skill)) {
		r.State = Stale
		return r
	}
	r.State = Present
	return r
}

func stampAgrees(stamp, hash string) bool {
	lines := strings.Split(strings.TrimSuffix(stamp, "\n"), "\n")
	return len(lines) >= 1 && lines[0] == "sha256 "+hash
}
