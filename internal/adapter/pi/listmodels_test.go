package pi

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// listFixture is a `pi --list-models` listing in the real column format. It is
// kept as a file so the test reads the format as observed, not as rewritten.
const listFixture = "testdata/list-models.txt"

// fixtureRows returns the header and the data rows of the fixture, with each
// row keyed by its provider column.
func fixtureRows(t *testing.T) (string, map[string]string) {
	t.Helper()
	data, err := os.ReadFile(listFixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	rows := map[string]string{}
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) > 0 {
			rows[fields[0]] = line
		}
	}
	return lines[0], rows
}

// installListingStub makes a fake pi whose --list-models prints listing and
// whose other invocations never run. No real harness or model is started.
func installListingStub(t *testing.T, listing string) {
	t.Helper()
	binDir := t.TempDir()
	listPath := filepath.Join(binDir, "listing.txt")
	if err := os.WriteFile(listPath, []byte(listing), 0o644); err != nil {
		t.Fatalf("write listing: %v", err)
	}
	// PATH holds only the stub directory, so the stub prints the listing with
	// the shell builtin printf rather than an external command such as cat.
	script := "#!/bin/sh\nif [ \"$1\" = \"--list-models\" ]; then while IFS= read -r line || [ -n \"$line\" ]; do printf '%s\\n' \"$line\"; done < '" + listPath + "'; exit 0; fi\nexit 1\n"
	if err := os.WriteFile(filepath.Join(binDir, Binary), []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	t.Setenv("PATH", binDir)
}

func TestPreflightReadsListModelsFormat(t *testing.T) {
	header, rows := fixtureRows(t)
	if want := "provider  model  context  max-out  thinking  images"; header != want {
		t.Fatalf("fixture header = %q, want %q", header, want)
	}
	omlxRow, ok := rows["omlx"]
	if !ok {
		t.Fatal("fixture has no omlx row")
	}
	ollaRow, ok := rows["olla"]
	if !ok {
		t.Fatal("fixture has no olla row")
	}
	if !listsModel(omlxRow, testModel) {
		t.Errorf("omlx row does not match %s: %q", testModel, omlxRow)
	}
	if listsModel(ollaRow, testModel) {
		t.Errorf("olla-only row matches %s: %q", testModel, ollaRow)
	}

	t.Run("full listing passes the model check", func(t *testing.T) {
		data, err := os.ReadFile(listFixture)
		if err != nil {
			t.Fatalf("read fixture: %v", err)
		}
		installListingStub(t, string(data))
		pf := Preflight(context.Background(), testRequest(t), testOpts(t, okDial))
		if err := checkCheck(pf, "model omlx/"+testModel); err != nil {
			t.Fatalf("model check refused a listed model: %v", err)
		}
	})

	t.Run("olla-only listing refuses the model", func(t *testing.T) {
		installListingStub(t, header+"\n"+ollaRow+"\n")
		pf := Preflight(context.Background(), testRequest(t), testOpts(t, okDial))
		err := checkCheck(pf, "model omlx/"+testModel)
		if err == nil || !strings.Contains(err.Error(), "is not listed") {
			t.Fatalf("model check = %v, want a not-listed refusal for an olla-only listing", err)
		}
	})
}

// checkCheck returns the error of the named preflight check, or a failure if
// the check is absent.
func checkCheck(pf PreflightResult, name string) error {
	for _, c := range pf.Checks {
		if c.Name == name {
			return c.Err
		}
	}
	return errNoCheck{name}
}

type errNoCheck struct{ name string }

func (e errNoCheck) Error() string { return "no preflight check named " + e.name }
