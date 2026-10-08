package omp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeConfig writes a fake OMP config under a temp HOME and returns that HOME.
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".omp", "agent")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return home
}

func TestSideCallNoteMappedAndUnknown(t *testing.T) {
	t.Run("mapped roles", func(t *testing.T) {
		home := writeConfig(t, "modelRoles:\n  tiny: omlx/Ornith-1.5-9B-MLX-4bit # side\n  smol: \"omlx/Ornith-1.5-9B-MLX-4bit\"\n")
		note := sideCallNote(home)
		for _, want := range []string{"tiny=omlx/Ornith-1.5-9B-MLX-4bit", "smol=omlx/Ornith-1.5-9B-MLX-4bit"} {
			if !strings.Contains(note, want) {
				t.Errorf("note %q lacks %q", note, want)
			}
		}
		if strings.Contains(note, "unknown") {
			t.Errorf("mapped note reports unknown: %q", note)
		}
	})

	t.Run("role missing", func(t *testing.T) {
		home := writeConfig(t, "modelRoles:\n  tiny: omlx/x\n")
		note := sideCallNote(home)
		if !strings.Contains(note, "unknown for roles smol") {
			t.Errorf("note = %q, want smol reported unknown", note)
		}
	})

	t.Run("config missing", func(t *testing.T) {
		note := sideCallNote(t.TempDir())
		if !strings.Contains(note, "unknown") || !strings.Contains(note, "does not exist") {
			t.Errorf("note = %q, want unknown with missing file", note)
		}
	})

	t.Run("no home", func(t *testing.T) {
		if note := sideCallNote(""); !strings.Contains(note, "unknown") {
			t.Errorf("note = %q, want unknown", note)
		}
	})
}

func TestRunNoteStatesSideCallMapping(t *testing.T) {
	home := writeConfig(t, "modelRoles:\n  tiny: omlx/Ornith-1.5-9B-MLX-4bit\n  smol: omlx/Ornith-1.5-9B-MLX-4bit\n")
	installStub(t, testModel)
	req := testRequest(t)
	opts := Options{Dial: okDial, Home: home}
	res := Run(context.Background(), req, opts)
	if res.State != StateExit || !strings.Contains(res.Note, "smol=omlx/Ornith-1.5-9B-MLX-4bit") {
		t.Fatalf("exit note = %q (state %s), want side-call mapping", res.Note, res.State)
	}

	t.Setenv("PATH", t.TempDir())
	blocked := Run(context.Background(), req, opts)
	if blocked.State != StateBlocked || !strings.Contains(blocked.Note, "tiny=") {
		t.Errorf("blocked note = %q (state %s), want side-call mapping", blocked.Note, blocked.State)
	}
}
