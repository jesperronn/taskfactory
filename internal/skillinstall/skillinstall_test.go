package skillinstall

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func mkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultTarget(t *testing.T) {
	root := t.TempDir()
	got, err := ResolveDir(Target{Root: root})
	if err != nil || got != filepath.Join(root, ".agents", "skills", "taskfactory") {
		t.Fatalf("without .claude: %q %v", got, err)
	}
	mkdir(t, filepath.Join(root, ".claude"))
	got, err = ResolveDir(Target{Root: root})
	if err != nil || got != filepath.Join(root, ".claude", "skills", "taskfactory") {
		t.Fatalf("with .claude: %q %v", got, err)
	}
	if _, err := ResolveDir(Target{}); err == nil {
		t.Fatal("outside a project must fail")
	}
}

func TestTargetsAndGlobal(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	mkdir(t, filepath.Join(root, ".claude"))
	cases := []struct {
		t    Target
		want string
	}{
		{Target{Name: "agents", Root: root}, filepath.Join(root, ".agents", "skills", "taskfactory")},
		{Target{Name: "claude", Root: t.TempDir()}, ""},
		{Target{Global: true, Home: home, Root: root}, filepath.Join(home, ".agents", "skills", "taskfactory")},
		{Target{Name: "claude", Global: true, Home: home}, filepath.Join(home, ".claude", "skills", "taskfactory")},
		{Target{Name: "dir:/x/y", Root: root}, "/x/y"},
	}
	for _, c := range cases {
		got, err := ResolveDir(c.t)
		if err != nil {
			t.Fatalf("%+v: %v", c.t, err)
		}
		if c.want != "" && got != c.want {
			t.Errorf("%+v: got %q want %q", c.t, got, c.want)
		}
	}
	mkdir(t, filepath.Join(home, ".claude"))
	if got, _ := ResolveDir(Target{Global: true, Home: home}); got != filepath.Join(home, ".claude", "skills", "taskfactory") {
		t.Errorf("global default with .claude: %q", got)
	}
	if _, err := ResolveDir(Target{Name: "dir:/x", Global: true, Home: home}); err == nil {
		t.Error("global with dir: must fail")
	}
	if _, err := ResolveDir(Target{Name: "bogus", Root: root}); err == nil {
		t.Error("unknown target must fail")
	}
}

func TestInstallWritesOnlySkillDir(t *testing.T) {
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "other.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(base, ".agents", "skills", "taskfactory")
	skill := "---\nname: taskfactory\n---\nbody\n"
	if err := Install(dir, Options{Skill: skill, Version: "v1"}); err != nil {
		t.Fatal(err)
	}
	var found []string
	filepath.Walk(base, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			rel, _ := filepath.Rel(base, p)
			found = append(found, rel)
		}
		return nil
	})
	sort.Strings(found)
	want := []string{
		filepath.Join(".agents", "skills", "taskfactory", ".taskfactory-skill"),
		filepath.Join(".agents", "skills", "taskfactory", "SKILL.md"),
		"other.txt",
	}
	if !reflect.DeepEqual(found, want) {
		t.Fatalf("files %v want %v", found, want)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if string(got) != skill {
		t.Fatal("SKILL.md not byte-identical")
	}
	stamp, _ := os.ReadFile(filepath.Join(dir, ".taskfactory-skill"))
	if string(stamp) != "sha256 "+Hash(skill)+"\nversion v1\n" {
		t.Fatalf("stamp %q", stamp)
	}
	if entries, _ := os.ReadDir(filepath.Dir(dir)); len(entries) != 1 {
		t.Fatalf("temporary sibling left behind: %v", entries)
	}
	if r := Status(dir, skill); r.State != Present {
		t.Fatalf("status %v", r)
	}
}

func TestForce(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "s")
	mkdir(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "extra"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Install(dir, Options{Skill: "new", Version: "v"})
	if err == nil || !strings.Contains(err.Error(), dir) {
		t.Fatalf("want refusal naming dir, got %v", err)
	}
	if _, e := os.Stat(filepath.Join(dir, "extra")); e != nil {
		t.Fatal("refused install changed the directory")
	}
	if err := Install(dir, Options{Skill: "new", Version: "v", Force: true}); err != nil {
		t.Fatal(err)
	}
	if _, e := os.Stat(filepath.Join(dir, "extra")); e == nil {
		t.Fatal("force did not replace the directory")
	}
	if r := Status(dir, "new"); r.State != Present {
		t.Fatalf("status %v", r)
	}
}

func TestStatusStaleAndMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "s")
	if r := Status(dir, "a"); r.State != Missing {
		t.Fatalf("%v", r)
	}
	if err := Install(dir, Options{Skill: "a", Version: "v"}); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(dir, StampFile))
	if r := Status(dir, "a"); r.State != Stale {
		t.Fatalf("no stamp: %v", r)
	}
	if r := Status(dir, "b"); r.State != Stale || r.Installed == r.Embedded {
		t.Fatalf("hash differs: %v", r)
	}
}
