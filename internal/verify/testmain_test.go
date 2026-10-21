package verify

import (
	"os"
	"testing"

	"taskfactory/internal/testgit"
)

// TestMain removes the shared git fixture templates after the package's tests.
func TestMain(m *testing.M) {
	code := m.Run()
	testgit.Cleanup()
	os.Exit(code)
}
