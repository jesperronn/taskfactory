package promote

import (
	"os"
	"testing"
)

// TestMain disables global and system Git configuration for every test in the
// package once, before any test starts. Fixtures set their own identity and
// commit.gpgsign. Doing it here instead of with t.Setenv in each fixture keeps
// the tests free to run in parallel.
func TestMain(m *testing.M) {
	os.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	os.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	os.Exit(m.Run())
}
