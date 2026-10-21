package skills

import (
	"os"
	"testing"
)

func TestEmbeddedSkillMatchesCanonicalFile(t *testing.T) {
	want, err := os.ReadFile("taskfactory/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if SkillMD != string(want) {
		t.Fatalf("embedded skill differs from skills/taskfactory/SKILL.md")
	}
	if SkillMD == "" {
		t.Fatal("embedded skill is empty")
	}
}
